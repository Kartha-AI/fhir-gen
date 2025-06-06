package sofhir

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"net/http"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

// UserData represents the response from the external API
type UserData struct {
	IsAdmin bool `json:"isAdmin"`
	// Add other fields as needed
}

// isUserEligible checks to see if the USER exists as a Patient or a Practitioner and returns corresponding claims
func IsUserEligible(ctx context.Context, user *auth.UserRecord) (bool, map[string]interface{}) {

	tenantId := user.TenantID
	log.Print("Tenant ID =: ", tenantId)

	if tenantId == PATIENTS_TENANT_ID { //PATIENT USER
		//patient user
		patient, err := GetPatientByEmail(ctx, user.Email)
		if err != nil {
			log.Print("GetPatientByEmail Error occurred: ", err)
			return false, nil
		}
		log.Print("Got Patient By Email")
		orgId := strings.Split(patient.Org.Reference, "/")[1]
		claims := map[string]interface{}{PATIENT_ID_CLAIM: patient.Id, ORGANIZATION_ID_CLAIM: orgId, ROLE_CLAIM: PATIENT_ROLE}

		return true, claims

	} else if tenantId == PROVIDERS_TENANT_ID { //PROVIDER USER
		//provider user
		practitioner, err := GetPractitionerByEmail(ctx, user.Email)
		if err != nil {
			log.Print("GetPractitionerByEmail Error occurred: ", err)
			return false, nil
		}
		log.Print("Got Practitioner By Email")
		orgId := GetIdentifier(ORG_IDENTIFIER_SYSTEM, practitioner.Identifier) //get organization id from the identifiers
		claims := map[string]interface{}{PROVIDER_ID_CLAIM: practitioner.Id, ORGANIZATION_ID_CLAIM: orgId, ROLE_CLAIM: PROVIDER_ROLE}

		return true, claims

	} else {
		return false, nil
	}
}

// deleteUser deletes the user from Firebase Authentication
func DeleteUser(user *auth.UserRecord) error {

	tenantId := user.TenantID
	log.Print("Tenant ID =: ", tenantId)

	// Initialize Firebase Admin SDK
	app, err := firebase.NewApp(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("Error initializing Firebase app: %v\n", err)
	}

	// Initialize Firebase Authentication client
	authClient, err := app.Auth(context.Background())
	if err != nil {
		return fmt.Errorf("Error getting Auth client: %v\n", err)
	}

	tm := authClient.TenantManager
	tenantClient, err := tm.AuthForTenant(tenantId)
	userRecord, err := tenantClient.GetUserByEmail(context.Background(), user.Email)
	if err != nil {
		return fmt.Errorf("Error Getting User Record By Email: %v\n", err)
	}

	log.Print("Deleting User: ", userRecord.UID)
	if err := tenantClient.DeleteUser(context.Background(), userRecord.UID); err != nil {
		return err
	}

	log.Printf("User %s has been deleted", userRecord.UID)

	return nil
}

func SetCustomUserClaims(ctx context.Context, user *auth.UserRecord, claims map[string]interface{}) error {

	tenantId := user.TenantID
	log.Print("Tenant ID =: ", tenantId)

	// Initialize Firebase Admin SDK
	app, err := firebase.NewApp(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("Error initializing Firebase app: %v\n", err)
	}

	// Initialize Firebase Authentication client
	authClient, err := app.Auth(context.Background())
	if err != nil {
		return fmt.Errorf("Error getting Auth client: %v\n", err)
	}

	tm := authClient.TenantManager
	tenantClient, err := tm.AuthForTenant(tenantId)
	if err != nil {
		log.Fatalf("error initializing tenant-aware auth client: %v\n", err)
	}

	userRecord, err := tenantClient.GetUserByEmail(context.Background(), user.Email)
	if err != nil {
		return fmt.Errorf("Error Getting User Record By Email: %v\n", err)
	}

	log.Print("Setting Custom Claims for User: ", userRecord.UID)
	log.Print("Claims: ", claims)

	err = tenantClient.SetCustomUserClaims(ctx, userRecord.UID, claims)
	if err != nil {
		return fmt.Errorf("error setting custom claims %v\n", err)
	}

	return nil
}

// Authorize checks the user's claims to see if they have the required scopes to access the requested resource
func AuthorizeRequest(r *http.Request, resourceType string, requestType string) (bool, map[string]interface{}, error) {

	//API Gateway will send the authentication result in the X-Apigateway-Api-Userinfo to the backend API.
	//This header is base64url encoded and contains the JWT payload.
	userinfoHeader := r.Header.Get("X-Apigateway-Api-Userinfo")
	// Validate the userinfoHeader header token
	if userinfoHeader == "" {
		return false, nil, fmt.Errorf("userinfoHeader  is missing")
	}
	// Add padding if needed
	if l := len(userinfoHeader) % 4; l > 0 {
		userinfoHeader += strings.Repeat("=", 4-l)
	}
	log.Print("userinfoHeader: ", userinfoHeader)
	// Decode the base64 string
	decodedBytes, err := base64.StdEncoding.DecodeString(userinfoHeader)
	if err != nil {
		return false, nil, fmt.Errorf("Error Decoding userinfoHeader: %v", err)
	}

	// Parse the decoded JSON byte slice into a map[string]interface{}
	var claims map[string]interface{}
	if err := json.Unmarshal(decodedBytes, &claims); err != nil {
		return false, nil, fmt.Errorf("Error Unmarshalling Decoded userinfoHeader: %v", err)
	}
	log.Print("Claims:", claims)

	//for RAG reuqests scope check is not required. May be in future we can add more checks on what resources patient can access
	if requestType == RAG_REQUEST {
		return true, claims, nil
	}

	// checks the reqeusted access with the scopes in the user's claims
	authorized, error := CheckScopes(claims, resourceType, requestType)
	if error != nil {
		return false, nil, fmt.Errorf("Error Trying to Check Scopes: %v", err)
	}
	if !authorized {
		return false, nil, fmt.Errorf("Unauthorized: %v", error)
	}

	return true, claims, nil
}
