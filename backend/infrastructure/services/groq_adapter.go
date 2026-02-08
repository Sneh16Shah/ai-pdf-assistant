package services

import (
	"strings"

	appservices "ai-pdf-assistant-backend/services"
)

// GroqAIServiceAdapter adapts GroqService to the AIService interface
type GroqAIServiceAdapter struct {
	groq *appservices.GroqService
}

// NewGroqAIServiceAdapter creates a new adapter for GroqService
func NewGroqAIServiceAdapter(groq *appservices.GroqService) *GroqAIServiceAdapter {
	return &GroqAIServiceAdapter{groq: groq}
}

// AnswerQuestion implements AIService interface using Groq
func (a *GroqAIServiceAdapter) AnswerQuestion(context string, question string, history []string) (string, bool, error) {
	// Convert history strings to ChatMessage format
	var chatHistory []appservices.ChatMessage
	for i, msg := range history {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		chatHistory = append(chatHistory, appservices.ChatMessage{
			Role:    role,
			Content: msg,
		})
	}

	resp, err := a.groq.ChatWithContext(context, question, chatHistory, "")
	if err != nil {
		return "", false, err
	}

	// Check if answer was found in context
	answerFound := !strings.Contains(strings.ToLower(resp.Message), "not found") &&
		!strings.Contains(strings.ToLower(resp.Message), "not available") &&
		!strings.Contains(strings.ToLower(resp.Message), "cannot find")

	return resp.Message, answerFound, nil
}

// GenerateSummary implements AIService interface using Groq
func (a *GroqAIServiceAdapter) GenerateSummary(text string) (string, []string, []string, error) {
	summary, err := a.groq.SummarizePDF(text)
	if err != nil {
		return "", nil, nil, err
	}

	// Extract takeaways and topics from summary
	takeaways := extractTakeaways(summary)
	topics := extractTopics(summary)

	return summary, takeaways, topics, nil
}
