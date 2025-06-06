package sofhir

type PatientSearchResults struct {
	Entry []PatientEntry `json:"entry"`
	Total int            `json:"total"`
}

type PatientEntry struct {
	FullURL string  `json:"fullUrl"`
	Patient Patient `json:"resource"`
}

type Patient struct {
	Id  string       `json:"id"`
	Org Organization `json:"managingOrganization"`
}

type Organization struct {
	Reference string `json:"reference"`
}

type PractitionerSearchResults struct {
	Entry []PractitionerEntry `json:"entry"`
	Total int                 `json:"total"`
}

type PractitionerEntry struct {
	FullURL      string       `json:"fullUrl"`
	Practitioner Practitioner `json:"resource"`
}

type Practitioner struct {
	Id         string       `json:"id"`
	Identifier []Identifier `json:"identifier"`
}

type Identifier struct {
	System string `json:"system"`
	Value  string `json:"value"`
}

type Resource struct {
	ResourceType string  `json:"resourceType"`
	Id           string  `json:"id"`
	Subject      Subject `json:"subject"`
}

type Subject struct {
	Reference string `json:"reference"`
}
