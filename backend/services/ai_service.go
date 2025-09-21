package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type AIService struct {
	client *openai.Client
	model  string
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

func NewAIService(apiKey string) *AIService {
	client := openai.NewClient(apiKey)
	return &AIService{
		client: client,
		model:  openai.GPT3Dot5Turbo, // You can change this to GPT4 if you have access
	}
}

func (ai *AIService) ChatWithPDF(pdfText, userQuestion, sessionID string) (*ChatResponse, error) {
	// Create context-aware prompt
	systemPrompt := fmt.Sprintf(`You are an AI assistant helping users understand and analyze PDF documents. 

Here is the content of the PDF document:

%s

Please answer questions about this document accurately and helpfully. If the answer is not found in the document, clearly state that the information is not available in the provided PDF.`, pdfText)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userQuestion,
		},
	}

	resp, err := ai.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       ai.model,
			Messages:    messages,
			MaxTokens:   1000,
			Temperature: 0.7,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	return &ChatResponse{
		Message:   resp.Choices[0].Message.Content,
		SessionID: sessionID,
	}, nil
}

func (ai *AIService) ChatWithContext(pdfText, userQuestion string, conversationHistory []ChatMessage, sessionID string) (*ChatResponse, error) {
	// Build conversation with PDF context
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: fmt.Sprintf(`You are an AI assistant helping users understand and analyze PDF documents. 

Here is the content of the PDF document:

%s

Please answer questions about this document accurately and helpfully. Maintain context from previous messages in this conversation. If the answer is not found in the document, clearly state that the information is not available in the provided PDF.`, pdfText),
		},
	}

	// Add conversation history
	for _, msg := range conversationHistory {
		role := openai.ChatMessageRoleUser
		if msg.Role == "assistant" {
			role = openai.ChatMessageRoleAssistant
		}
		
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// Add current question
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userQuestion,
	})

	resp, err := ai.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       ai.model,
			Messages:    messages,
			MaxTokens:   1000,
			Temperature: 0.7,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	return &ChatResponse{
		Message:   resp.Choices[0].Message.Content,
		SessionID: sessionID,
	}, nil
}

func (ai *AIService) SummarizePDF(pdfText string) (string, error) {
	// Truncate text if it's too long for the API
	maxLength := 12000 // Leave room for prompt and response
	if len(pdfText) > maxLength {
		pdfText = pdfText[:maxLength] + "... [content truncated]"
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: "You are an AI assistant that creates concise, informative summaries of PDF documents. Focus on the main points, key findings, and important details.",
		},
		{
			Role: openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("Please provide a comprehensive summary of the following PDF content:\n\n%s", pdfText),
		},
	}

	resp, err := ai.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       ai.model,
			Messages:    messages,
			MaxTokens:   500,
			Temperature: 0.5,
		},
	)

	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}