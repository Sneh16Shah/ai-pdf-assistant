package services

import (
	"fmt"
	"strings"
	"time"
)

// MockAIService provides mock AI responses for development/testing
type MockAIService struct {
}

// NewMockAIService creates a new mock AI service
func NewMockAIService() *MockAIService {
	return &MockAIService{}
}

// AnswerQuestion provides a mock answer based on simple keyword matching
func (s *MockAIService) AnswerQuestion(context string, question string, history []string) (string, bool, error) {
	// Simulate API delay
	time.Sleep(500 * time.Millisecond)

	questionLower := strings.ToLower(question)
	contextLower := strings.ToLower(context)

	// Simple keyword matching to determine if answer might be in context
	answerFound := false
	answer := ""

	// Check if question keywords appear in context
	questionWords := strings.Fields(questionLower)
	matches := 0
	for _, word := range questionWords {
		if len(word) > 3 && strings.Contains(contextLower, word) {
			matches++
		}
	}

	if matches > 0 {
		answerFound = true
		// Generate a mock answer
		answer = fmt.Sprintf("Based on the document, %s. The document mentions relevant information about this topic. [This is a mock response - connect to a real AI service for actual answers.]", question)
	} else {
		answerFound = false
		answer = "I cannot find this information in the document. [Mock response - connect to a real AI service for actual answers.]"
	}

	return answer, answerFound, nil
}

// GenerateSummary generates a mock summary
func (s *MockAIService) GenerateSummary(text string) (string, []string, []string, error) {
	// Simulate API delay
	time.Sleep(1 * time.Second)

	// Simple mock summary
	wordCount := len(strings.Fields(text))
	summary := fmt.Sprintf(`Summary:
This document contains approximately %d words covering various topics.

Key Takeaways:
• This is a mock summary generated for development/testing purposes
• Connect to a real AI service (Puter AI) for actual summaries
• The document appears to contain structured information

Main Topics:
• Document Analysis
• Information Extraction
• Mock Data Processing`, wordCount)

	takeaways := []string{
		"This is a mock summary - connect to real AI for actual content",
		"Document contains structured information",
		"Approximately " + fmt.Sprintf("%d", wordCount) + " words processed",
	}

	topics := []string{
		"Document Analysis",
		"Information Extraction",
		"Mock Processing",
	}

	return summary, takeaways, topics, nil
}

