package sofhir

import (
	"os"
	"strings"
)

// fhir related
var (
	PROVIDERS_TENANT_ID = os.Getenv("PROVIDERS_TENANT_ID")
	PATIENTS_TENANT_ID  = os.Getenv("PATIENTS_TENANT_ID")
	API_GATEWAY_HOST    = "https://" + os.Getenv("API_GATEWAY_HOST")

	PATIENT_ROLE_SCOPES   = strings.Split(os.Getenv("PATIENT_ROLE_SCOPES"), ";")
	PROVIDER_ROLE_SCOPES  = strings.Split(os.Getenv("PROVIDER_ROLE_SCOPES"), ";")
	ORG_IDENTIFIER_SYSTEM = "https://fhirgen.ai/fhir/identifiers/organization"
)

const (
	ALL_RESOURCES          = "all"
	PATIENT_RESOURCE_TYPE  = "Patient"
	PATIENT_ROLE           = "patient"
	PROVIDER_ROLE          = "user"
	ROLE_CLAIM             = "role"
	PATIENT_ID_CLAIM       = "patientId"
	PROVIDER_ID_CLAIM      = "providerId"
	ORGANIZATION_ID_CLAIM  = "organizationId"
	ID_SEARCH_PARM         = "_id"
	PATIENT_ID_SEARCH_PARM = "patient"
	SUBJECT_SEARCH_PARM    = "subject"
	GET_RERQUEST           = "GET"
	PUT_REQUEST            = "PUT"
	POST_REQUEST           = "POST"
	RAG_REQUEST            = "RAG"
	UNAUTHORIZED_STATUS    = 401
)

// alloy-db stuff
var (
	REGION       = os.Getenv("REGION")
	PROJECT_ID   = os.Getenv("PROJECT_ID")
	ADB_IAM_USER = os.Getenv("ADB_IAM_USER")
	ADB_CLUSTER  = os.Getenv("ADB_CLUSTER")
	ADB_INSTANCE = os.Getenv("ADB_INSTANCE")
	ADB_IP       = os.Getenv("ADB_IP")
	ADB_PORT     = os.Getenv("ADB_PORT")
	ADB_DATABASE = os.Getenv("ADB_DATABASE")
)
