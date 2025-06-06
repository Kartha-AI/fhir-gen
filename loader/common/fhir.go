package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/fhir/go/fhirversion"
	"github.com/google/fhir/go/jsonformat"
	r4pb "github.com/google/fhir/go/proto/google/fhir/proto/r4/core/resources/bundle_and_contained_resource_go_proto"

	"cloud.google.com/go/compute/metadata"
)

// gets FHIR Resources usign API

func GetFHIRResource(ctx context.Context, resourceType string, resourceURI string) (string, error) {
	// Authenticate with the FHIR server to obtain an access token
	accessToken, err := GetAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to obtain access token: %v", err)
	}

	// Construct the URL to fetch the FHIR resource by ID
	fhirURL := fmt.Sprintf("https://healthcare.googleapis.com/v1/%s", resourceURI)

	// Create an HTTP client
	client := &http.Client{}

	// Create a GET request
	req, err := http.NewRequestWithContext(ctx, "GET", fhirURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set the authorization header with the access token
	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Make the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %v", err)
	}

	defer resp.Body.Close()

	// Check the status code of the response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	var fhirData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&fhirData); err != nil {
		return "", fmt.Errorf("failed to decode JSON response: %v", err)
	}

	// Convert the JSON data to a string
	fhirJSON, err := json.Marshal(fhirData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON data: %v", err)
	}

	// Convert JSON byte slice to string
	fhirString := string(fhirJSON)

	return fhirString, nil
}

// Struct to represent the JSON response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

// Function to obtain an access token using the default service account credentials
func GetAccessToken() (string, error) {
	// Check if running on Google Cloud and obtain access token
	if metadata.OnGCE() {
		tokenJSON, err := metadata.Get("instance/service-accounts/default/token")
		if err != nil {
			return "", fmt.Errorf("failed to obtain access token: %v", err)
		}

		// Parse the JSON response
		var tokenResp TokenResponse
		if err := json.Unmarshal([]byte(tokenJSON), &tokenResp); err != nil {
			return "", fmt.Errorf("failed to parse access token: %v", err)
		}

		// Return the access token
		return tokenResp.AccessToken, nil
	}

	// If not running on Google Cloud, return an error
	return "", fmt.Errorf("not running on Google Cloud Platform")
}

type FHIRResourceSumamry struct {
	ResourceId       string
	PatientId        string
	ResourceType     string
	Timestamp        int64
	GeneratedContent string
	OriginalFHIRJSON string
}

// Generate content for the FHIR resource and build,return the FHIRResourceSumamry struct
func GetResourceSummary(ctx context.Context, resourceType, resourceId string, fhirJSONString string) (*FHIRResourceSumamry, error) {
	const (
		timeZone = "America/Chicago"
	)
	um, err := jsonformat.NewUnmarshaller(timeZone, fhirversion.R4)
	if err != nil {
		return nil, fmt.Errorf("Failed to create FHIR unmarshaller: %v", err)
	}
	fmt.Println("Created New FHIR R4 Unmarshaller")
	unmarshalled, err := um.Unmarshal([]byte(fhirJSONString))
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall FHIR JSON: %v", err)
	}
	fmt.Println("nmarshalled FHIR JSON")
	contained := unmarshalled.(*r4pb.ContainedResource)

	var patientId string
	var timestamp int64
	switch resourceType {
	case "Condition":
		condition := contained.GetCondition()
		patientId = condition.GetSubject().GetPatientId().GetValue()
		timestamp = condition.GetMeta().GetLastUpdated().GetValueUs()
	case "Observation":
		observation := contained.GetObservation()
		patientId = observation.GetSubject().GetPatientId().GetValue()
		timestamp = observation.GetMeta().GetLastUpdated().GetValueUs()
	case "MedicationRequest":
		medicationRequest := contained.GetMedicationRequest()
		patientId = medicationRequest.GetSubject().GetPatientId().GetValue()
		timestamp = medicationRequest.GetMeta().GetLastUpdated().GetValueUs()
	case "Encounter":
		encounter := contained.GetEncounter()
		patientId = encounter.GetSubject().GetPatientId().GetValue()
		timestamp = encounter.GetMeta().GetLastUpdated().GetValueUs()
	case "CarePlan":
		carePlan := contained.GetCarePlan()
		patientId = carePlan.GetSubject().GetPatientId().GetValue()
		timestamp = carePlan.GetMeta().GetLastUpdated().GetValueUs()
	case "AllergyIntolerance":
		allergyIntolerance := contained.GetAllergyIntolerance()
		patientId = allergyIntolerance.GetPatient().GetPatientId().GetValue()
		timestamp = allergyIntolerance.GetMeta().GetLastUpdated().GetValueUs()
	case "Procedure":
		procedure := contained.GetProcedure()
		patientId = procedure.GetSubject().GetPatientId().GetValue()
		timestamp = procedure.GetMeta().GetLastUpdated().GetValueUs()
	case "Immunization":
		immunization := contained.GetImmunization()
		patientId = immunization.GetPatient().GetPatientId().GetValue()
		timestamp = immunization.GetMeta().GetLastUpdated().GetValueUs()
	case "ServiceRequest":
		serviceRequest := contained.GetServiceRequest()
		patientId = serviceRequest.GetSubject().GetPatientId().GetValue()
		timestamp = serviceRequest.GetMeta().GetLastUpdated().GetValueUs()

	// Add cases for other resource types as needed
	default:
		fmt.Println("Unhandled resource type:", resourceType)
		return nil, fmt.Errorf("Unhandled resource type: %v", resourceType)
	}

	//!!## This is causing an issue with quotas on Gemini API cals.
	//     and it now done in AlloyDB SQL Insert using built in AI functions
	// fmt.Println("Generating the Contnet")
	// generatedConent, err := GenerateContent(ctx, GetContentGenPrompt(fhirJSONString))
	// if err != nil {
	// 	return nil, fmt.Errorf("Failed to Generate Content: %v", err)
	// }
	// fmt.Println("Generated the Contnet")

	generatedConent := "This is a generated content for the FHIR resource" //dummy content

	fhirResourceSummary := FHIRResourceSumamry{
		ResourceId:       resourceId,
		ResourceType:     resourceType,
		PatientId:        patientId,
		Timestamp:        timestamp,
		GeneratedContent: generatedConent,
		OriginalFHIRJSON: fhirJSONString,
	}
	return &fhirResourceSummary, nil
}

func GetContentGenPrompt(fhirJSONString string) string {
	return "You are a clinician and also an expert on Healthcare data especially FHIR JSON. \n" +
		"And you are also very sensitive about patient privacy and protecting PII like their names \n" +
		"emails, phone numbers, addresses and date of birth. \n" +
		"I would like you to summarize the below FHIR JSON into a short paragraph that includes the following info:\n" +
		"- convert the timestamp to a string that shows how many years/months/days/hours ago is the data \n" +
		"- Any other relationships to other resources based on the references if there are and a \n" +
		"- A clinical narrative of what the resource data is all about\n" +
		"Include data values and units. \n" +
		"Do not include any Personally Identifiable Information like names, emails,phone , addresses and date of birth \n" +
		"Make the final conent to be a paragraph of text so I can use that for \n" +
		"generating Emebeddings out of it\n" +
		"here is the JOSN:\n" + fhirJSONString + "\n" +
		"important:  Do not include any names of patient's and practitioners in the final content"
}
