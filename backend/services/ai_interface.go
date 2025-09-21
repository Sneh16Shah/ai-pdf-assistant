package services

// AIProvider interface that both OpenAI and Groq services implement
type AIProvider interface {
	ChatWithPDF(pdfText, userQuestion, sessionID string) (*ChatResponse, error)
	ChatWithContext(pdfText, userQuestion string, conversationHistory []ChatMessage, sessionID string) (*ChatResponse, error)
	SummarizePDF(pdfText string) (string, error)
}