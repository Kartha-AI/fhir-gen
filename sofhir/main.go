package sofhir

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"firebase.google.com/go/v4/auth"
)

// cloud fucntion for : "GET /fhir/{type}/{id}" HTTP request
func Read(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract parameters (the API Gateway path parms come in as query parms to cloud functions)
	requestType := r.Method
	resourceType := r.URL.Query().Get("type")
	resourceId := r.URL.Query().Get("id")
	resourceURI := fmt.Sprintf("/%s/%s", resourceType, resourceId)

	//authorizes and processes the request and writes the response to the http.ResponseWriter
	processResourceRequest(ctx, r, w, resourceType, requestType, resourceURI, nil)
}

// cloud fucntion for: "GET /fhir/{type}" HTTP request with search query parms
func Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract parameters (the API Gateway path parms come in as query parms to cloud functions).
	requestType := r.Method
	resourceType := r.URL.Query().Get("type")

	// Constructing new resoruceURI with other search parms. eg. /Patient?parm1=value1&parm2=value2
	otherParams := r.URL.Query()
	delete(otherParams, "type") // Remove type from otherParams

	resourceURI := "/" + resourceType // eg. /Pattient or /Observation
	if len(otherParams) > 0 {
		resourceURI += "?"
		for key, values := range otherParams {
			for _, value := range values {
				resourceURI += key + "=" + value + "&" // Append query parameters
			}
		}
		resourceURI = resourceURI[:len(resourceURI)-1] // Remove the trailing "&"
	}

	//authorizes and processes the request and writes the response to the http.ResponseWriter
	processResourceRequest(ctx, r, w, resourceType, requestType, resourceURI, nil)
}

// cloud fucntion for: "PUT /fhir/{type}{id}" HTTP request to update a resource
func Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract parameters (the API Gateway path parms come in as query parms to cloud functions)
	requestType := r.Method
	resourceType := r.URL.Query().Get("type")
	resourceId := r.URL.Query().Get("id")
	resourceURI := fmt.Sprintf("/%s/%s", resourceType, resourceId)

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	//authorizes and processes the request and writes the response to the http.ResponseWriter
	processResourceRequest(ctx, r, w, resourceType, requestType, resourceURI, body)
}

// cloud fucntion for: "POST /fhir/{type}" HTTP request to create a resource
func Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestType := r.Method
	resourceType := r.URL.Query().Get("type")
	resourceURI := "/" + resourceType // eg. /Patient

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	//authorizes and processes the request and writes the response to the http.ResponseWriter
	processResourceRequest(ctx, r, w, resourceType, requestType, resourceURI, body)
}

// cloud fucntion for: "POST /rag" HTTP request to generate a resource for user's prompt using Gen AI
func Rag(w http.ResponseWriter, r *http.Request) {

	// Get the context associated with the request
	ctx := r.Context()

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// You can unmarshal the JSON request body into a struct if needed
	var requestData RagRequest
	err = json.Unmarshal(body, &requestData)
	if err != nil {
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return
	}

	//Authorize the request by retrieving user claims from http request header and checks scopes
	authorized, claims, err := AuthorizeRequest(r, ALL_RESOURCES, RAG_REQUEST)
	if !authorized {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	log.Printf("RAG Request Authorized")

	responseJSON, err := ProcessRagRequest(ctx, r, claims, requestData.PatientId, requestData.Prompt)
	if err != nil {
		http.Error(w, "Error executing RAG function:"+err.Error(), http.StatusBadRequest)
		return
	}

	// Set Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}

// cloud fucntion that gets triggered by firebase:onUserCreate
// Checks to see they are either a Patient (for Patient App) or a Practitioner/Provider (for Provider App).
// this is to make sure only authorized patients or providers can create an account
func UserCreationHandler(ctx context.Context, user auth.UserRecord) error {

	// check if user is eligible to sign up and if so, returns custom claims to be set for the user
	eligigle, claims := IsUserEligible(ctx, &user)
	if !eligigle {
		// If user is not eligible, delete the user
		log.Print("User is not eligible to sign up..Deleting the user")
		if err := DeleteUser(&user); err != nil {
			return fmt.Errorf("failed to delete user: %v", err)
		}
		return fmt.Errorf("user is not eligible to sign up")
	}

	//set custom claims for the user
	if err := SetCustomUserClaims(ctx, &user, claims); err != nil {
		log.Printf("Error setting custom claims for user : %s - %v", user.Email, err)
		return fmt.Errorf("failed to set custom claims: %v", err)
	}

	log.Printf("Set the  custom claims for user : %s", user.Email)

	return nil
}

// common function to process FHIR requests
func processResourceRequest(
	ctx context.Context,
	r *http.Request,
	w http.ResponseWriter,
	resourceType string,
	requestType string,
	resourceURI string,
	requestBody []byte,
) {

	//Authorize the request by retrieving user claims from http request header and checks scopes
	authorized, claims, err := AuthorizeRequest(r, resourceType, requestType)
	if !authorized {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	log.Printf("Authorized for : %s on %s", requestType, resourceType)

	// Process the FHIR request
	accessGranted, statusCode, responseBody, err := ProcessFHIRRequest(ctx, r, resourceType, requestType, resourceURI, requestBody, claims)
	if !accessGranted {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	log.Printf("FHIR Request Processed successfully for : %s on %s", requestType, resourceURI)

	// Return the response from the FHIR server
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.WriteHeader(statusCode)
	w.Write(responseBody)
}
