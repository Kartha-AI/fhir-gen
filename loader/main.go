package loader

import (
	"context"
	"fmt"
	"log"
	"strings"

	"fhirgen.ai/loader/common"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
)

// "FHIRPubSub" is the entrypoint for the Cloud Function
func init() {
	functions.CloudEvent("FHIRPubSub", fhirPubSub)
}

// MessagePublishedData contains the full Pub/Sub message
// https://cloud.google.com/eventarc/docs/cloudevents#pubsub
type MessagePublishedData struct {
	Message PubSubMessage
}

// PubSubMessage is the payload of a Pub/Sub event.
// https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage
type PubSubMessage struct {
	Attributes struct {
		ResourceType string `json:"resourceType"`
		Action       string `json:"action"`
		VersionID    string `json:"versionId"`
	} `json:"attributes"`
	Data []byte `json:"data"`
}

// fhirPubSub consumes a CloudEvent message and extracts the Pub/Sub message.
func fhirPubSub(ctx context.Context, e event.Event) error {
	var msg MessagePublishedData
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("event.DataAs: %w", err)
	}
	log.Printf("Received:, %s", msg.Message)

	var resourceType = string(msg.Message.Attributes.ResourceType)
	var action = string(msg.Message.Attributes.Action) //CreateResource, UpdateResource, DeleteResource
	var resourceURI = string(msg.Message.Data)         //eg. projects/X/locations/X/datasets/X/fhirStores/X/fhir/resoruceTyep/id

	process(ctx, resourceType, action, resourceURI)

	return nil
}

// process the resoruce data received in the pubsub
func process(ctx context.Context, resourceType string, action string, resourceURI string) error {

	// List of included FHIR resource types
	includedResourceTypes := []string{
		"Condition", "Observation", "MedicationRequest", "Encounter",
		"AllergyIntolerance", "Procedure", "Immunization", "CarePlan", "ServiceRequest"}

	// Check if the given resource type is included
	var included bool
	for _, rType := range includedResourceTypes {
		if resourceType == rType {
			included = true
			break
		}
	}

	// If the resource type is included, retrieve and further process it
	if included {
		fhirJSONString, err := common.GetFHIRResource(ctx, resourceType, resourceURI)
		if err != nil {
			fmt.Println("Erorr Getting the FHIR Resource:", resourceType)
		}
		fmt.Println("Got resource..")

		fmt.Println("Generating Sumamry..")
		//get resoruceId from the resourceURI
		parts := strings.Split(resourceURI, "/")
		resourceId := parts[len(parts)-1]
		resourceSummary, err := common.GetResourceSummary(ctx, resourceType, resourceId, fhirJSONString)
		if err != nil {
			fmt.Println(err)
			fmt.Println("Error Generating Sumamry for the FHIR Resource:", resourceType)
			return nil
		}

		// Print the Resource Sumamry
		fmt.Println("ResourceId:", resourceSummary.ResourceId)
		fmt.Println("PatientId:", resourceSummary.PatientId)
		fmt.Println("ResourceType:", resourceSummary.ResourceType)
		fmt.Println("Timestamp:", resourceSummary.Timestamp)
		fmt.Println("GeneratedContent:", resourceSummary.GeneratedContent)
		fmt.Println("OriginalFHIRJSON:", resourceSummary.OriginalFHIRJSON)

		err = common.SaveSumamry(ctx, resourceSummary)
		if err != nil {
			fmt.Printf("Error saving summary for the FHIR resource: %v\n", err)
			return nil
		}

	} else {
		fmt.Println("Skipping processing for resource type:", resourceType)
	}

	return nil
}
