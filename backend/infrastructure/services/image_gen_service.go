package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ImageGenService handles AI image generation using Google Gemini 2.5 Flash Image
type ImageGenService struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewImageGenService creates a new image generation service
func NewImageGenService(geminiAPIKey string) *ImageGenService {
	return &ImageGenService{
		apiKey:  geminiAPIKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta/models",
		model:   "gemini-2.5-flash-image",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// IsAvailable checks if the image generation service has an API key configured
func (s *ImageGenService) IsAvailable() bool {
	return s.apiKey != ""
}

// ImageGenerationKeywords are terms that suggest the user wants an image generated
var ImageGenerationKeywords = []string{
	"draw", "generate image", "create diagram", "create image",
	"make a diagram", "illustrate", "visualize", "sketch",
	"generate a", "draw a", "make a", "create a chart",
	"simpler version", "simplified diagram", "redraw",
}

// IsImageGenRequest checks if the user's question is asking for image generation
func IsImageGenRequest(question string) bool {
	lower := strings.ToLower(question)
	for _, keyword := range ImageGenerationKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// geminiRequest represents a request to the Gemini API
type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *geminiInline `json:"inline_data,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiGenConfig struct {
	ResponseModalities []string `json:"responseModalities"`
}

// geminiResponse represents a response from the Gemini API
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text       string `json:"text,omitempty"`
				InlineData *struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData,omitempty"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// ImageGenResult contains the result of an image generation request
type ImageGenResult struct {
	ImageBase64 string // Base64-encoded image data
	MimeType    string // e.g., "image/png"
	TextReply   string // Any text the model returned alongside the image
}

// GenerateImage generates an image based on a text prompt
func (s *ImageGenService) GenerateImage(prompt string) (*ImageGenResult, error) {
	if !s.IsAvailable() {
		return nil, fmt.Errorf("GEMINI_API_KEY not set, image generation unavailable")
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: geminiGenConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal image gen request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create image gen request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image gen API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image gen response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image gen API error %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse image gen response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("Gemini API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no response from image generation model")
	}

	result := &ImageGenResult{}
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		if part.InlineData != nil {
			result.ImageBase64 = part.InlineData.Data
			result.MimeType = part.InlineData.MimeType
		}
		if part.Text != "" {
			result.TextReply += part.Text
		}
	}

	if result.ImageBase64 == "" {
		// Model returned text only (maybe it couldn't generate the image)
		if result.TextReply != "" {
			return result, nil
		}
		return nil, fmt.Errorf("image generation did not produce an image")
	}

	return result, nil
}

// GenerateImageFromContext generates a diagram based on document context
func (s *ImageGenService) GenerateImageFromContext(userRequest string, documentContext string) (*ImageGenResult, error) {
	prompt := fmt.Sprintf(`Based on the following document context, %s

Document Context:
%s

Create a clear, professional diagram or illustration. Use clean lines, labels, and a logical layout.`,
		userRequest, truncateForPrompt(documentContext, 4000))

	return s.GenerateImage(prompt)
}

// ImageToBase64DataURL converts raw image bytes to a data URL for frontend display
func ImageToBase64DataURL(imageData []byte, mimeType string) string {
	b64 := base64.StdEncoding.EncodeToString(imageData)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
}

// truncateForPrompt limits text length for image generation prompts
func truncateForPrompt(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
