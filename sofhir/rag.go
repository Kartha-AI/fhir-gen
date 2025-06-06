package sofhir

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// this will handle the rag request
func ProcessRagRequest(ctx context.Context, r *http.Request, userClaims map[string]interface{}, patientId string, prompt string) ([]byte, error) {

	granted, err := AuthorizeRAGRequest(userClaims, patientId)
	if !granted {
		return nil, fmt.Errorf("Invalid Access Reqeust: %v", err)
	}
	log.Printf("Access granted for RAG Request")

	ragFunctionResponse, err := ExecuteRagFunction(ctx, patientId, prompt)
	if err != nil {
		return nil, fmt.Errorf("Error executing RAG function: %v", err)
	}

	// Send response
	responseMap := map[string]interface{}{
		"response": ragFunctionResponse.Predictions[0].Content,
	}
	// Marshal the map to JSON
	responseJSON, err := json.Marshal(responseMap)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal JSON response: %v", err)
	}

	return responseJSON, nil
}

func AuthorizeRAGRequest(userClaims map[string]interface{}, requestedPatientId string) (bool, error) {

	role := userClaims[ROLE_CLAIM].(string)
	if role == PATIENT_ROLE {
		userPatientId := userClaims[PATIENT_ID_CLAIM].(string)
		if userPatientId != requestedPatientId {
			return false, fmt.Errorf("Access Denied: Patient can only access Patient data assocaited to them")
		}
	} else if role == PROVIDER_ROLE {
		//Provide role is allwoed to access all patients data with in their organization
	} else {
		return false, fmt.Errorf("Access Denied: Unknown Role: %s", role)
	}

	return true, nil
}
