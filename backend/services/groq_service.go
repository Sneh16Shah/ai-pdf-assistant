package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GroqService struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqRequest struct {
	Messages    []GroqMessage `json:"messages"`
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type GroqChoice struct {
	Index        int         `json:"index"`
	Message      GroqMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type GroqUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type GroqResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []GroqChoice `json:"choices"`
	Usage   GroqUsage    `json:"usage"`
}

func NewGroqService(apiKey string) *GroqService {
	return &GroqService{
		apiKey:  apiKey,
		baseURL: "https://api.groq.com/openai/v1",
		model:   "llama-3.3-70b-versatile", // 128K context window for full PDF support
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (g *GroqService) ChatWithPDF(pdfText, userQuestion, sessionID string) (*ChatResponse, error) {
	// Create context-aware prompt
	systemPrompt := fmt.Sprintf(`You are an AI assistant helping users understand and analyze PDF documents. 

Here is the content of the PDF document:

%s

Please answer questions about this document accurately and helpfully. If the answer is not found in the document, clearly state that the information is not available in the provided PDF.`, pdfText)

	messages := []GroqMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userQuestion,
		},
	}

	resp, err := g.makeRequest(messages, 1000, 0.7)
	if err != nil {
		return nil, fmt.Errorf("Groq API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from Groq")
	}

	return &ChatResponse{
		Message:   resp.Choices[0].Message.Content,
		SessionID: sessionID,
	}, nil
}

func (g *GroqService) ChatWithContext(pdfText, userQuestion string, conversationHistory []ChatMessage, sessionID string) (*ChatResponse, error) {
	// Truncate context if it's extremely long (safety net for very large PDFs)
	maxContextLen := 100000 // ~100K chars, well within 128K token limit
	if len(pdfText) > maxContextLen {
		pdfText = pdfText[:maxContextLen] + "\n... [content truncated due to length]"
	}

	// Build conversation with PDF context
	messages := []GroqMessage{
		{
			Role: "system",
			Content: fmt.Sprintf(`You are an AI assistant helping users understand and analyze PDF documents. 

Here is the content of the PDF document:

%s

Please answer questions about this document accurately and helpfully. Maintain context from previous messages in this conversation. If the answer is not found in the document, clearly state that the information is not available in the provided PDF.`, pdfText),
		},
	}

	// Add conversation history
	for _, msg := range conversationHistory {
		role := "user"
		if msg.Role == "assistant" {
			role = "assistant"
		}

		messages = append(messages, GroqMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// Add current question
	messages = append(messages, GroqMessage{
		Role:    "user",
		Content: userQuestion,
	})

	resp, err := g.makeRequest(messages, 1000, 0.7)
	if err != nil {
		return nil, fmt.Errorf("Groq API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from Groq")
	}

	return &ChatResponse{
		Message:   resp.Choices[0].Message.Content,
		SessionID: sessionID,
	}, nil
}

func (g *GroqService) SummarizePDF(pdfText string) (string, error) {
	// Truncate text if it's too long for the API
	maxLength := 12000 // Leave room for prompt and response
	if len(pdfText) > maxLength {
		pdfText = pdfText[:maxLength] + "... [content truncated]"
	}

	messages := []GroqMessage{
		{
			Role:    "system",
			Content: "You are an AI assistant that creates concise, informative summaries of PDF documents. Focus on the main points, key findings, and important details.",
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Please provide a comprehensive summary of the following PDF content:\n\n%s", pdfText),
		},
	}

	resp, err := g.makeRequest(messages, 500, 0.5)
	if err != nil {
		return "", fmt.Errorf("Groq API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from Groq")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (g *GroqService) makeRequest(messages []GroqMessage, maxTokens int, temperature float64) (*GroqResponse, error) {
	reqBody := GroqRequest{
		Messages:    messages,
		Model:       g.model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", g.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var groqResp GroqResponse
	err = json.Unmarshal(body, &groqResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &groqResp, nil
}
