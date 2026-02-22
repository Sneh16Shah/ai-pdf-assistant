package services

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ImageGenService handles AI image generation using Pollinations.ai
type ImageGenService struct {
	client *http.Client
}

// NewImageGenService creates a new image generation service
func NewImageGenService(_ string) *ImageGenService {
	return &ImageGenService{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// IsAvailable checks if the image generation service has an API key configured (always true for Pollinations)
func (s *ImageGenService) IsAvailable() bool {
	return true
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

// ImageGenResult contains the result of an image generation request
type ImageGenResult struct {
	ImageBase64 string // Base64-encoded image data
	MimeType    string // e.g., "image/jpeg"
	TextReply   string // Any text the model returned alongside the image
}

// GenerateImage generates an image based on a text prompt
func (s *ImageGenService) GenerateImage(prompt string) (*ImageGenResult, error) {
	encodedPrompt := url.PathEscape(prompt)
	apiURL := fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=800&height=600&nologo=true", encodedPrompt)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create image gen request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image gen API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("image gen API error %d: %s", resp.StatusCode, string(body))
	}

	imgBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated image: %w", err)
	}

	base64Encoded := base64.StdEncoding.EncodeToString(imgBytes)

	return &ImageGenResult{
		ImageBase64: base64Encoded,
		MimeType:    "image/jpeg",
		TextReply:   "Here is the image I generated for you using Pollinations.ai.",
	}, nil
}

// GenerateImageFromContext generates a diagram based on document context
func (s *ImageGenService) GenerateImageFromContext(userRequest string, documentContext string) (*ImageGenResult, error) {
	// Extract a short summary for the image prompt (Pollinations uses GET URLs, so prompt must be short)
	contextSnippet := truncateForPrompt(documentContext, 200)
	prompt := fmt.Sprintf("Professional diagram: %s. Context: %s. Clean, labeled, modern style.",
		truncateForPrompt(userRequest, 100), contextSnippet)

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
