package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// AIService interface for AI providers
type AIService interface {
	AnswerQuestion(context string, question string, history []string) (string, bool, error)
	GenerateSummary(text string) (string, []string, []string, error)
}

// PuterAIService implements AIService using Puter AI
type PuterAIService struct {
	baseURL string
	client  *http.Client
}

// NewPuterAIService creates a new Puter AI service
func NewPuterAIService() *PuterAIService {
	baseURL := os.Getenv("PUTER_AI_URL")
	if baseURL == "" {
		baseURL = "https://api.puter.ai/v1/chat/completions" // Default endpoint
	}

	return &PuterAIService{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PuterAIRequest represents a request to Puter AI
type PuterAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PuterAIResponse represents a response from Puter AI
type PuterAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// AnswerQuestion answers a question based on context
func (s *PuterAIService) AnswerQuestion(context string, question string, history []string) (string, bool, error) {
	// Build prompt
	prompt := buildQuestionPrompt(context, question, history)

	// Prepare request
	reqBody := PuterAIRequest{
		Model: "gpt-3.5-turbo", // Default model
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that answers questions based ONLY on the provided document context. If the answer is not in the context, say 'I cannot find this information in the document.'",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", false, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	req, err := http.NewRequest("POST", s.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Note: Puter AI may require API key in header if available
	apiKey := os.Getenv("PUTER_AI_KEY")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var aiResp PuterAIResponse
	if err := json.Unmarshal(body, &aiResp); err != nil {
		return "", false, fmt.Errorf("failed to parse response: %w", err)
	}

	if aiResp.Error != nil {
		return "", false, fmt.Errorf("AI error: %s", aiResp.Error.Message)
	}

	if len(aiResp.Choices) == 0 {
		return "", false, fmt.Errorf("no response from AI")
	}

	answer := aiResp.Choices[0].Message.Content
	answerFound := !strings.Contains(strings.ToLower(answer), "cannot find") &&
		!strings.Contains(strings.ToLower(answer), "not in the document")

	return answer, answerFound, nil
}

// GenerateSummary generates a summary of the text
func (s *PuterAIService) GenerateSummary(text string) (string, []string, []string, error) {
	prompt := buildSummaryPrompt(text)

	reqBody := PuterAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that creates concise summaries in bullet point format.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", s.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("PUTER_AI_KEY")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil, nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var aiResp PuterAIResponse
	if err := json.Unmarshal(body, &aiResp); err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if aiResp.Error != nil {
		return "", nil, nil, fmt.Errorf("AI error: %s", aiResp.Error.Message)
	}

	if len(aiResp.Choices) == 0 {
		return "", nil, nil, fmt.Errorf("no response from AI")
	}

	summary := aiResp.Choices[0].Message.Content
	// Parse summary into takeaways and topics (simplified for MVP)
	takeaways := extractTakeaways(summary)
	topics := extractTopics(summary)

	return summary, takeaways, topics, nil
}

// buildQuestionPrompt builds the prompt for question answering
func buildQuestionPrompt(context string, question string, history []string) string {
	var builder strings.Builder

	builder.WriteString(context)
	builder.WriteString("\n\n")

	if len(history) > 0 {
		builder.WriteString("Previous conversation:\n")
		for i, msg := range history {
			if i >= len(history)-4 { // Last 4 messages
				builder.WriteString("- ")
				builder.WriteString(msg)
				builder.WriteString("\n")
			}
		}
		builder.WriteString("\n")
	}

	builder.WriteString("Question: ")
	builder.WriteString(question)
	builder.WriteString("\n\n")
	builder.WriteString("Answer the question based ONLY on the document context above. If the answer is not in the context, respond with: 'I cannot find this information in the document.'")

	return builder.String()
}

// buildSummaryPrompt builds the prompt for summary generation
func buildSummaryPrompt(text string) string {
	// Limit text length for prompt
	maxLength := 8000
	if len(text) > maxLength {
		text = text[:maxLength] + "..."
	}

	return fmt.Sprintf(`Please provide a comprehensive summary of the following document in bullet point format.

Include:
1. Main topics and themes
2. Key takeaways
3. Important details

Document:
%s

Format your response as:
- Summary: [brief overview]
- Key Takeaways:
  • [takeaway 1]
  • [takeaway 2]
- Main Topics:
  • [topic 1]
  • [topic 2]`, text)
}

// extractTakeaways extracts key takeaways from summary (simplified)
func extractTakeaways(summary string) []string {
	lines := strings.Split(summary, "\n")
	takeaways := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "•") || strings.HasPrefix(line, "-") {
			takeaway := strings.TrimPrefix(strings.TrimPrefix(line, "•"), "-")
			takeaway = strings.TrimSpace(takeaway)
			if len(takeaway) > 0 {
				takeaways = append(takeaways, takeaway)
			}
		}
	}

	if len(takeaways) == 0 {
		// Fallback: return first few sentences
		sentences := strings.Split(summary, ".")
		for i := 0; i < len(sentences) && i < 3; i++ {
			s := strings.TrimSpace(sentences[i])
			if len(s) > 20 {
				takeaways = append(takeaways, s+".")
			}
		}
	}

	return takeaways
}

// extractTopics extracts main topics from summary (simplified)
func extractTopics(summary string) []string {
	// Simple extraction - look for topic markers
	topics := []string{}
	lines := strings.Split(summary, "\n")

	for _, line := range lines {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(line, "topic") || strings.Contains(line, "theme") {
			// Extract potential topics
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				topic := strings.TrimSpace(parts[1])
				if len(topic) > 0 {
					topics = append(topics, topic)
				}
			}
		}
	}

	if len(topics) == 0 {
		// Fallback: extract first few capitalized words/phrases
		words := strings.Fields(summary)
		for i := 0; i < len(words) && len(topics) < 3; i++ {
			word := strings.Trim(words[i], ".,!?;:")
			if len(word) > 3 && strings.ToUpper(word[:1]) == word[:1] {
				topics = append(topics, word)
			}
		}
	}

	return topics
}

