package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NvidiaVisionService handles image understanding using NVIDIA NIM's
// meta/llama-3.2-90b-vision-instruct model.
type NvidiaVisionService struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewNvidiaVisionService creates a new vision service backed by NVIDIA NIM.
func NewNvidiaVisionService(apiKey string) *NvidiaVisionService {
	return &NvidiaVisionService{
		apiKey:  apiKey,
		baseURL: "https://integrate.api.nvidia.com/v1/chat/completions",
		model:   "meta/llama-3.2-90b-vision-instruct",
		client: &http.Client{
			Timeout: 120 * time.Second, // large model, give it time
		},
	}
}

// DescribePageImage sends a page image to the NVIDIA vision model for understanding.
// Uses the same OpenAI-compatible multimodal format.
func (v *NvidiaVisionService) DescribePageImage(imageData []byte, question string, textContext string) (string, error) {
	if v.apiKey == "" {
		return "", fmt.Errorf("NVIDIA_VLM_API_KEY not set, NVIDIA vision service unavailable")
	}

	// Encode image to base64
	b64Image := base64.StdEncoding.EncodeToString(imageData)
	imageURL := fmt.Sprintf("data:image/jpeg;base64,%s", b64Image)

	// Build the prompt
	systemPrompt := "You are an AI assistant that analyzes images from PDF documents. " +
		"Describe diagrams, charts, tables, and other visual elements in detail. " +
		"Explain what the visual element shows, its components, and how they relate to each other."

	userPrompt := question
	if textContext != "" {
		userPrompt = fmt.Sprintf("Context from the document:\n%s\n\nQuestion: %s", textContext, question)
	}

	// Build multimodal message content (OpenAI-compatible)
	content := []interface{}{
		visionTextContent{Type: "text", Text: userPrompt},
		visionImageContent{
			Type:     "image_url",
			ImageURL: visionImageURL{URL: imageURL},
		},
	}

	reqBody := visionRequest{
		Messages: []visionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		},
		Model:       v.model,
		MaxTokens:   2048,
		Temperature: 0.3,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nvidia vision request: %w", err)
	}

	req, err := http.NewRequest("POST", v.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create nvidia vision request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("nvidia vision API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read nvidia vision response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nvidia vision API error %d: %s", resp.StatusCode, string(body))
	}

	var visionResp visionResponse
	if err := json.Unmarshal(body, &visionResp); err != nil {
		return "", fmt.Errorf("failed to parse nvidia vision response: %w", err)
	}

	if visionResp.Error != nil {
		return "", fmt.Errorf("nvidia vision API error: %s", visionResp.Error.Message)
	}

	if len(visionResp.Choices) == 0 {
		return "", fmt.Errorf("no response from nvidia vision model")
	}

	return visionResp.Choices[0].Message.Content, nil
}
