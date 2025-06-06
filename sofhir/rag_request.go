package sofhir

// Request Body
type RagRequest struct {
	PatientId string `json:"patientId"`
	Prompt    string `json:"prompt"`
}
