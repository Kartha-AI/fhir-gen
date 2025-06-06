// ##### This is not being used now. Content genration is being done in AlloyDB using ML Functions.
package common

import (
	"context"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const (
	GEMINI_API_KEY_ENV_VAR string = "GEMINI_API_KEY"
)

func GenerateContent(ctx context.Context, prompt string) (string, error) {
	geminAPIKey := os.Getenv(GEMINI_API_KEY_ENV_VAR)
	if geminAPIKey == "" {
		fmt.Println("Gemni API key is not set or empty")
		return "", fmt.Errorf("Gemini API key is no tset or empty")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(geminAPIKey))
	if err != nil {
		return "", fmt.Errorf("Failed to genAI client: %v", err)
	}
	fmt.Println("Got Gemini client")
	defer client.Close()

	// For text-only input, use the gemini-pro model
	model := client.GenerativeModel("gemini-1.0-pro")
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
	}
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("Failed to Generate Content: %v", err)
	}
	var content string
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				content += fmt.Sprintln(part)
			}
		}
	}
	return content, nil
}
