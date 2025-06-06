package sofhir

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// this will be used to send the request to the FHIR server after authorization
func ProcessFHIRRequest(
	ctx context.Context,
	r *http.Request,
	resourceType string,
	requestType string,
	resourceURI string,
	requestBody []byte,
	userClaims map[string]interface{},
) (bool, int, []byte, error) { //returns success, status code, response body and error

	//Check if the user has the permissions to access requested resoruce data.
	//It also returns the resoruceURI with appeneded patientId for patient role to restrict access to the resources
	//that are related to the patient that user is associated with
	granted, modifiedResourceURI, err := AuthorizeResourceAccess(ctx, userClaims, requestType, resourceType, resourceURI, requestBody)
	if !granted {
		return false, UNAUTHORIZED_STATUS, nil, fmt.Errorf("Invalid Access Reqeust: %v", err)
	}
	log.Printf("Access granted for : %s on %s", requestType, resourceURI)

	//Get GCP AccessToekn to access CloudHealthcare API (FHIR)
	accessToken, err := GetAccessToken(ctx)
	if err != nil {
		return false, UNAUTHORIZED_STATUS, nil, fmt.Errorf("Error Getting AccessToken: %v", err)
	}

	responseBody, statusCode, err := SendFHIRRequest(ctx, accessToken, requestType, modifiedResourceURI, requestBody)
	if err != nil {
		return false, statusCode, nil, fmt.Errorf("Error on FHIR Request: %v", err)
	}

	//replace the gogole healthcare API host in the response body with the current host
	replacedBody := replaceHostInResponse(r, responseBody)
	log.Printf("Replaced GCP FHIR URLSs with sofhir URLS")
	return true, statusCode, replacedBody, nil
}

// Struct to represent the JSON response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

// Get FHIR Patient using email
func GetPatientByEmail(ctx context.Context, email string) (*Patient, error) {

	// Authenticate with the FHIR server to obtain an access token
	accessToken, err := GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	resourceURI := "/Patient?email=" + email
	responseBody, _, err := SendFHIRRequest(ctx, accessToken, "GET", resourceURI, nil)
	if err != nil {
		return nil, err
	}

	var searchResults PatientSearchResults
	if err := json.Unmarshal([]byte(responseBody), &searchResults); err != nil {
		return nil, fmt.Errorf("Error parsing responseBody: %v", err)
	}

	if searchResults.Total == 0 {
		return nil, fmt.Errorf("Patient Not Found")
	}
	if searchResults.Total > 1 {
		return nil, fmt.Errorf("More then one Patient found")
	}

	return &searchResults.Entry[0].Patient, nil
}

// Get FHIR Practitioner using email
func GetPractitionerByEmail(ctx context.Context, email string) (*Practitioner, error) {

	// Authenticate with the FHIR server to obtain an access token
	accessToken, err := GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	resourceURI := "/Practitioner?email=" + email //search for Practitioner with email
	responseBody, _, err := SendFHIRRequest(ctx, accessToken, "GET", resourceURI, nil)
	if err != nil {
		return nil, err
	}

	var searchResults PractitionerSearchResults
	if err := json.Unmarshal([]byte(responseBody), &searchResults); err != nil {
		return nil, fmt.Errorf("Error parsing responseBody: %v", err)
	}

	if searchResults.Total == 0 {
		return nil, fmt.Errorf("Practitioner Not Found")
	}
	if searchResults.Total > 1 {
		return nil, fmt.Errorf("More then one Practitioner found")
	}

	return &searchResults.Entry[0].Practitioner, nil
}

// get a identifier vlaue for a specific system in an array of identifiers
func GetIdentifier(syetem string, identifiers []Identifier) string {

	var value string
	for _, identifier := range identifiers {
		if identifier.System == syetem {
			value = identifier.Value
			break
		}
	}
	return value
}

func IsResourceAssociatedWithPatient(ctx context.Context, patientId string, resourceType string, resourceId string) (bool, error) {
	// Authenticate with the FHIR server to obtain an access token
	log.Printf("In IsResourceAssociatedWithPatient(): Checking if the resource %s/%s is associated with the patient %s", resourceType, resourceId, patientId)
	accessToken, err := GetAccessToken(ctx)
	if err != nil {
		return false, err
	}
	resourceURI := fmt.Sprintf("/%s/%s", resourceType, resourceId)
	log.Printf("Getting Resource: %s", resourceURI)
	responseBody, statusCode, err := SendFHIRRequest(ctx, accessToken, "GET", resourceURI, nil)
	if err != nil {
		return false, err
	}
	if statusCode != http.StatusOK {
		return false, fmt.Errorf("Error getting resource. Status Code: %d", statusCode)
	}

	log.Print("Got Resource")
	var resource Resource
	if err := json.Unmarshal([]byte(responseBody), &resource); err != nil {
		log.Printf("Error parsing resource: %v", err)
		return false, fmt.Errorf("Error parsing resource: %v", err)
	}

	patientRef := resource.Subject.Reference //Patient/{id}
	log.Printf("Resource Subject/Patient Ref: %s", patientRef)
	patienRefId := strings.Split(patientRef, "/")[1]
	log.Printf("Patient Ref Id: %s", patienRefId)
	if patienRefId != patientId {
		log.Printf("Resoruce Subject: %s is not associated to the user's patient : %s", patientRef, patientId)
		return false, fmt.Errorf("Resoruce Subject is not associated to the user's patient : %s", patientId)
	}

	return true, nil
}

// replace the gogole healthcare API host in the response body with the current host of sofhir api
func replaceHostInResponse(r *http.Request, responseBody []byte) []byte {

	gcpFHIRURL := os.Getenv("GCP_FHIR_API_URL")
	sofhirURL := API_GATEWAY_HOST + "/fhir" //API_GATEWAY_HOST is the current host of sofhir api. Comes from env vars
	log.Print("Replacing ", gcpFHIRURL, " with ", sofhirURL)

	responseBodyString := string(responseBody)
	replacedBodyString := strings.Replace(responseBodyString, gcpFHIRURL, sofhirURL, -1) //replace all occurances

	return []byte(replacedBodyString)
}
