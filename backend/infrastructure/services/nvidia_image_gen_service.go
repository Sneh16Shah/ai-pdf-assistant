package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NvidiaImageGenService handles AI image generation using NVIDIA NIM's
// black-forest-labs/flux.1-dev model.
type NvidiaImageGenService struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewNvidiaImageGenService creates a new image generation service backed by NVIDIA NIM.
func NewNvidiaImageGenService(apiKey string) *NvidiaImageGenService {
	return &NvidiaImageGenService{
		apiKey:  apiKey,
		baseURL: "https://integrate.api.nvidia.com/v1/images/generations",
		model:   "black-forest-labs/flux.1-dev",
		client: &http.Client{
			Timeout: 120 * time.Second, // image gen can be slow
		},
	}
}

// IsAvailable checks if the NVIDIA image generation service has an API key configured
func (s *NvidiaImageGenService) IsAvailable() bool {
	return s.apiKey != ""
}

// nvidiaImageGenRequest represents a request to the NVIDIA image generation API
type nvidiaImageGenRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// nvidiaImageGenResponse represents a response from the NVIDIA image generation API
type nvidiaImageGenResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateImage generates an image based on a text prompt using FLUX.1-dev.
func (s *NvidiaImageGenService) GenerateImage(prompt string) (*ImageGenResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("NVIDIA_IMAGEGEN_API_KEY not set, NVIDIA image gen unavailable")
	}

	reqBody := nvidiaImageGenRequest{
		Model:  s.model,
		Prompt: prompt,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nvidia image gen request: %w", err)
	}

	req, err := http.NewRequest("POST", s.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create nvidia image gen request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nvidia image gen API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read nvidia image gen response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nvidia image gen API error %d: %s", resp.StatusCode, string(body))
	}

	var genResp nvidiaImageGenResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return nil, fmt.Errorf("failed to parse nvidia image gen response: %w", err)
	}

	if genResp.Error != nil {
		return nil, fmt.Errorf("nvidia image gen API error: %s", genResp.Error.Message)
	}

	if len(genResp.Data) == 0 {
		return nil, fmt.Errorf("no image generated from NVIDIA API")
	}

	return &ImageGenResult{
		ImageBase64: genResp.Data[0].B64JSON,
		MimeType:    "image/png",
		TextReply:   "Here is the image I generated for you using FLUX.1-dev.",
	}, nil
}

// GenerateImageFromContext generates a diagram based on document context.
func (s *NvidiaImageGenService) GenerateImageFromContext(userRequest string, documentContext string) (*ImageGenResult, error) {
	// With FLUX.1-dev, we can use much longer, richer prompts
	contextSnippet := truncateForPrompt(documentContext, 500)
	prompt := fmt.Sprintf("Professional high-quality illustration: %s. Based on the following context: %s. Clean, detailed, modern style, photorealistic quality.",
		truncateForPrompt(userRequest, 300), contextSnippet)

	return s.GenerateImage(prompt)
}
