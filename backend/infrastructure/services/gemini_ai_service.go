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

// GeminiAIService implements AIService using Google Gemini API
// Uses Gemini 2.5 Flash-Lite: 1M context, 1000 RPD, 15 RPM, 250K TPM free tier
type GeminiAIService struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewGeminiAIService creates a new Gemini AI service
func NewGeminiAIService(apiKey string) *GeminiAIService {
	return &GeminiAIService{
		apiKey:  apiKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta/models",
		model:   "gemini-2.5-flash-lite",
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// geminiChatRequest for text-only chat (distinct from image generation)
type geminiChatRequest struct {
	Contents          []geminiChatContent  `json:"contents"`
	SystemInstruction *geminiChatContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiChatGenConfig `json:"generationConfig,omitempty"`
}

type geminiChatContent struct {
	Role  string           `json:"role,omitempty"`
	Parts []geminiChatPart `json:"parts"`
}

type geminiChatPart struct {
	Text string `json:"text"`
}

type geminiChatGenConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type geminiChatResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// AnswerQuestion implements AIService using Gemini
func (s *GeminiAIService) AnswerQuestion(context string, question string, history []string) (string, bool, error) {
	systemPrompt := fmt.Sprintf(`You are an AI assistant helping users understand and analyze PDF documents.

Here is the content of the PDF document:

%s

Answer the user's questions about this document accurately and helpfully. Use markdown formatting for better readability. If the answer is not found in the document, clearly state that the information is not available in the provided PDF.`, context)

	// Build conversation contents
	var contents []geminiChatContent

	// Add history
	for i, msg := range history {
		role := "user"
		if i%2 == 1 {
			role = "model"
		}
		contents = append(contents, geminiChatContent{
			Role:  role,
			Parts: []geminiChatPart{{Text: msg}},
		})
	}

	// Add current question
	contents = append(contents, geminiChatContent{
		Role:  "user",
		Parts: []geminiChatPart{{Text: question}},
	})

	reqBody := geminiChatRequest{
		Contents: contents,
		SystemInstruction: &geminiChatContent{
			Parts: []geminiChatPart{{Text: systemPrompt}},
		},
		GenerationConfig: &geminiChatGenConfig{
			MaxOutputTokens: 4096,
			Temperature:     0.7,
		},
	}

	response, err := s.makeRequest(reqBody)
	if err != nil {
		return "", false, err
	}

	answerFound := !strings.Contains(strings.ToLower(response), "not found") &&
		!strings.Contains(strings.ToLower(response), "not available") &&
		!strings.Contains(strings.ToLower(response), "cannot find")

	return response, answerFound, nil
}

// GenerateSummary implements AIService using Gemini
func (s *GeminiAIService) GenerateSummary(text string) (string, []string, []string, error) {
	prompt := `Analyze the following PDF document and provide:
1. A comprehensive summary (2-3 paragraphs)
2. Key takeaways (5-7 bullet points, each starting with "- ")
3. Main topics covered (3-5 topics, each on its own line starting with "• ")

Format your response exactly as:
SUMMARY:
[your summary here]

KEY TAKEAWAYS:
[bullet points here]

TOPICS:
[topics here]

Document text:
` + text

	contents := []geminiChatContent{
		{
			Role:  "user",
			Parts: []geminiChatPart{{Text: prompt}},
		},
	}

	reqBody := geminiChatRequest{
		Contents: contents,
		GenerationConfig: &geminiChatGenConfig{
			MaxOutputTokens: 4096,
			Temperature:     0.4,
		},
	}

	response, err := s.makeRequest(reqBody)
	if err != nil {
		return "", nil, nil, err
	}

	takeaways := extractTakeaways(response)
	topics := extractTopics(response)

	return response, takeaways, topics, nil
}

// DescribeImage generates a text description of an image using Gemini's vision capabilities
func (s *GeminiAIService) DescribeImage(imageData []byte) (string, error) {
	prompt := "This is an image extracted from a document. Provide a highly detailed description of what it shows, including any text, data points, chart trends, or structural relationships, so that the information can be indexed for semantic text search."

	b64Image := base64.StdEncoding.EncodeToString(imageData)

	// Build multimodal content payload for Gemini
	type inlineData struct {
		MimeType string `json:"mime_type"`
		Data     string `json:"data"`
	}
	type multimodalPart struct {
		Text       string      `json:"text,omitempty"`
		InlineData *inlineData `json:"inline_data,omitempty"`
	}
	type multimodalContent struct {
		Role  string           `json:"role"`
		Parts []multimodalPart `json:"parts"`
	}
	type multimodalReq struct {
		Contents []multimodalContent `json:"contents"`
	}

	reqBody := multimodalReq{
		Contents: []multimodalContent{
			{
				Role: "user",
				Parts: []multimodalPart{
					{Text: prompt},
					{
						InlineData: &inlineData{
							MimeType: "image/jpeg", // Assuming JPEG for now, could be PNG
							Data:     b64Image,
						},
					},
				},
			},
		},
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Gemini vision request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini vision request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Gemini vision API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Gemini vision response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini vision API error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiChatResponse // Re-use the existing response struct
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse Gemini vision response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("Gemini vision API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini vision")
	}

	var result strings.Builder
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		result.WriteString(part.Text)
	}

	return result.String(), nil
}

func (s *GeminiAIService) makeRequest(reqBody geminiChatRequest) (string, error) {
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", s.baseURL, s.model, s.apiKey)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Gemini request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Gemini API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Gemini response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("Gemini API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	var result strings.Builder
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		result.WriteString(part.Text)
	}

	return result.String(), nil
}
