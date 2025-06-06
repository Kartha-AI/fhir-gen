package sofhir

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"cloud.google.com/go/compute/metadata"
)

// Function to obtain an access token using the default service account credentials
func GetAccessToken(ctx context.Context) (string, error) {
	// Check if running on Google Cloud and obtain access token
	if metadata.OnGCE() {
		tokenJSON, err := metadata.GetWithContext(ctx, "instance/service-accounts/default/token")
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

// gets FHIR Resources using Google Cloud Healthcare API: returns response, satatus code and error
func SendFHIRRequest(
	ctx context.Context,
	accessToken string,
	requestType string,
	resourceURI string,
	requestBody []byte,
) ([]byte, int, error) {
	// Get the FHIR API URL from the environment variables
	fhirApiRUL := os.Getenv("GCP_FHIR_API_URL")
	resourceURL := fhirApiRUL + resourceURI //resourceURI = "/Patient" or "/Patient/{id}"
	fmt.Println("FHIR API URL: ", resourceURL)

	// Create an HTTP client
	client := &http.Client{}

	// Create a HTTP request to google FHRI API
	body := io.Reader(nil)
	if requestBody != nil {
		body = bytes.NewReader(requestBody) // Convert []byte to io.Reader
	}
	req, _ := http.NewRequestWithContext(ctx, requestType, resourceURL, body)

	// Set the authorization header with the access token
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Prefer", "handling=strict")

	// Make the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to make FHIR API HTTP request: %v", err)
	}

	defer resp.Body.Close()

	// Read the response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to read FHIR API response body: %v", err)
	}

	return responseBody, resp.StatusCode, nil
}
