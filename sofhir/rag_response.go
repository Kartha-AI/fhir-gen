package sofhir

type Prediction struct {
	SafetyAttributes SafetyAttributes `json:"safetyAttributes"`
	CitationMetadata CitationMetadata `json:"citationMetadata"`
	Content          string           `json:"content"`
}

type SafetyAttributes struct {
	Blocked       bool           `json:"blocked"`
	SafetyRatings []SafetyRating `json:"safetyRatings"`
	Categories    []string       `json:"categories"`
	Scores        []float64      `json:"scores"`
}

type SafetyRating struct {
	ProbabilityScore float64 `json:"probabilityScore"`
	SeverityScore    float64 `json:"severityScore"`
	Severity         string  `json:"severity"`
	Category         string  `json:"category"`
}

type CitationMetadata struct {
	Citations []interface{} `json:"citations"` // You might want to define this more accurately if you know its structure
}

type RagFunctionResponse struct {
	Predictions []Prediction `json:"predictions"`
	Metadata    Metadata     `json:"metadata"`
}

type Metadata struct {
	TokenMetadata TokenMetadata `json:"tokenMetadata"`
}

type TokenMetadata struct {
	OutputTokenCount OutputTokenCount `json:"outputTokenCount"`
	InputTokenCount  InputTokenCount  `json:"inputTokenCount"`
}

type OutputTokenCount struct {
	TotalBillableCharacters int `json:"totalBillableCharacters"`
	TotalTokens             int `json:"totalTokens"`
}

type InputTokenCount struct {
	TotalBillableCharacters int `json:"totalBillableCharacters"`
	TotalTokens             int `json:"totalTokens"`
}
