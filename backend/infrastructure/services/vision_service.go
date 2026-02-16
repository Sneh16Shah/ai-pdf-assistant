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

// VisionService handles diagram understanding using Groq's Llama 4 Scout (vision model)
type VisionService struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewVisionService creates a new vision service using Groq
func NewVisionService(groqAPIKey string) *VisionService {
	return &VisionService{
		apiKey:  groqAPIKey,
		baseURL: "https://api.groq.com/openai/v1/chat/completions",
		model:   "meta-llama/llama-4-scout-17b-16e-instruct",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// DiagramKeywords are terms that suggest the user is asking about a visual element
var DiagramKeywords = []string{
	"diagram", "figure", "chart", "image", "table", "graph",
	"illustration", "picture", "flowchart", "drawing", "visual",
	"plot", "map", "schematic", "architecture diagram",
}

// IsDiagramQuestion checks if the user's question is about a visual element
func IsDiagramQuestion(question string) bool {
	lower := strings.ToLower(question)
	for _, keyword := range DiagramKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// visionMessage represents a message with multimodal content for the vision API
type visionMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type visionTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type visionImageContent struct {
	Type     string         `json:"type"`
	ImageURL visionImageURL `json:"image_url"`
}

type visionImageURL struct {
	URL string `json:"url"`
}

type visionRequest struct {
	Messages    []visionMessage `json:"messages"`
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type visionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// DescribePageImage sends a page image to the vision model for understanding
func (v *VisionService) DescribePageImage(imageData []byte, question string, textContext string) (string, error) {
	if v.apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY not set, vision service unavailable")
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

	// Build multimodal message content
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
		return "", fmt.Errorf("failed to marshal vision request: %w", err)
	}

	req, err := http.NewRequest("POST", v.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create vision request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("vision API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read vision response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vision API error %d: %s", resp.StatusCode, string(body))
	}

	var visionResp visionResponse
	if err := json.Unmarshal(body, &visionResp); err != nil {
		return "", fmt.Errorf("failed to parse vision response: %w", err)
	}

	if visionResp.Error != nil {
		return "", fmt.Errorf("vision API error: %s", visionResp.Error.Message)
	}

	if len(visionResp.Choices) == 0 {
		return "", fmt.Errorf("no response from vision model")
	}

	return visionResp.Choices[0].Message.Content, nil
}
