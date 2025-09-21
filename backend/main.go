package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"ai-pdf-assistant-backend/services"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// Global services
var (
	pdfService     *services.PDFService
	aiService      services.AIProvider
	storageService *services.StorageService
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	// Initialize services
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	
	// Initialize AI service based on available API keys
	groqKey := os.Getenv("GROQ_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")
	
	if groqKey != "" {
		aiService = services.NewGroqService(groqKey)
		log.Println("Using Groq AI service")
	} else if openaiKey != "" {
		aiService = services.NewAIService(openaiKey)
		log.Println("Using OpenAI service")
	} else {
		log.Fatal("Either GROQ_API_KEY or OPENAI_API_KEY environment variable is required")
	}

	pdfService = services.NewPDFService(uploadDir)
	storageService = services.NewStorageService()

	log.Println("Services initialized successfully")

	// Initialize Gin router
	r := gin.Default()

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:3000", "chrome-extension://*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}
	config.AllowCredentials = true
	r.Use(cors.New(config))

	// API Routes
	api := r.Group("/api/v1")
	{
		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "healthy",
				"service": "AI PDF Assistant Backend",
			})
		})

		// PDF processing routes
		pdf := api.Group("/pdf")
		{
			pdf.POST("/upload", handlePDFUpload)
			pdf.POST("/extract-text", handleTextExtraction)
			pdf.GET("/status/:id", handlePDFStatus)
		}

		// Chat routes
		chat := api.Group("/chat")
		{
			chat.POST("/message", handleChatMessage)
			chat.GET("/history/:sessionId", handleChatHistory)
			chat.DELETE("/session/:sessionId", handleClearSession)
		}

		// WebSocket for real-time chat
		api.GET("/ws", handleWebSocket)
	}

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(r.Run(":" + port))
}

// PDF Upload handler
func handlePDFUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("pdf")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded: " + err.Error()})
		return
	}
	defer file.Close()

	// Create upload directory if it doesn't exist
	uploadDir := "./uploads"
	os.MkdirAll(uploadDir, 0755)

	// Save uploaded file
	filename := header.Filename
	filePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy file: " + err.Error()})
		return
	}

	// Process PDF
	doc, err := pdfService.ProcessPDF(filePath, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process PDF: " + err.Error()})
		return
	}

	// Store document
	err = storageService.StorePDF(doc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store document: " + err.Error()})
		return
	}

	// Create chat session
	session := storageService.CreateSession(doc)

	c.JSON(http.StatusOK, gin.H{
		"document_id": doc.ID,
		"session_id": session.ID,
		"filename": doc.Filename,
		"pages": doc.Pages,
		"chunks": len(doc.Chunks),
		"message": "PDF uploaded and processed successfully",
	})
}

// Text extraction handler (for testing with local files)
func handleTextExtraction(c *gin.Context) {
	var request struct {
		FilePath string `json:"file_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if file exists
	if _, err := os.Stat(request.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found: " + request.FilePath})
		return
	}

	// Extract filename from path
	filename := filepath.Base(request.FilePath)

	// Process PDF
	doc, err := pdfService.ProcessPDF(request.FilePath, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process PDF: " + err.Error()})
		return
	}

	// Store document
	err = storageService.StorePDF(doc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store document: " + err.Error()})
		return
	}

	// Create chat session
	session := storageService.CreateSession(doc)

	// Get summary
	summary, err := aiService.SummarizePDF(doc.Text)
	if err != nil {
		log.Printf("Failed to generate summary: %v", err)
		summary = "Summary generation failed"
	}

	c.JSON(http.StatusOK, gin.H{
		"document_id": doc.ID,
		"session_id": session.ID,
		"filename": doc.Filename,
		"pages": doc.Pages,
		"chunks": len(doc.Chunks),
		"summary": summary,
		"text_preview": doc.Text[:min(500, len(doc.Text))] + "...",
		"message": "PDF processed successfully",
	})
}

// PDF status handler
func handlePDFStatus(c *gin.Context) {
	id := c.Param("id")
	doc, err := storageService.GetPDF(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id": doc.ID,
		"filename": doc.Filename,
		"pages": doc.Pages,
		"chunks": len(doc.Chunks),
		"status": "processed",
	})
}

// Chat message handler
func handleChatMessage(c *gin.Context) {
	var request struct {
		SessionID string `json:"session_id" binding:"required"`
		Message   string `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get session
	session, err := storageService.GetSession(request.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Add user message to session
	userMessage := services.ChatMessage{
		Role:    "user",
		Content: request.Message,
	}
	err = storageService.AddMessageToSession(request.SessionID, userMessage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store message"})
		return
	}

	// Get AI response
	response, err := aiService.ChatWithContext(
		session.PDFDocument.Text,
		request.Message,
		session.Messages,
		request.SessionID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI response: " + err.Error()})
		return
	}

	// Add AI response to session
	aiMessage := services.ChatMessage{
		Role:    "assistant",
		Content: response.Message,
	}
	err = storageService.AddMessageToSession(request.SessionID, aiMessage)
	if err != nil {
		log.Printf("Failed to store AI message: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response.Message,
		"session_id": request.SessionID,
	})
}

// Chat history handler
func handleChatHistory(c *gin.Context) {
	sessionId := c.Param("sessionId")
	session, err := storageService.GetSession(sessionId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionId,
		"messages": session.Messages,
		"pdf_info": gin.H{
			"filename": session.PDFDocument.Filename,
			"pages": session.PDFDocument.Pages,
		},
	})
}

// Clear session handler
func handleClearSession(c *gin.Context) {
	sessionId := c.Param("sessionId")
	err := storageService.ClearSession(sessionId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionId,
		"message": "Session cleared successfully",
	})
}

// WebSocket handler (placeholder for now)
func handleWebSocket(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "WebSocket endpoint - to be implemented",
	})
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
