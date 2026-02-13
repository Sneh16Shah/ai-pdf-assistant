package main

import (
	"log"
	"os"
	"time"

	"ai-pdf-assistant-backend/database"
	"ai-pdf-assistant-backend/handlers"
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	appservices "ai-pdf-assistant-backend/services"
	"ai-pdf-assistant-backend/usecases"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using defaults")
	}

	// Initialize database connection
	if err := database.Connect(); err != nil {
		log.Printf("Warning: Could not connect to database: %v", err)
		log.Println("Continuing with in-memory storage...")
	} else {
		defer database.Close()
	}

	// Initialize repositories
	docRepo := repositories.NewDocumentRepository()
	sessionRepo := repositories.NewSessionRepository()

	// Initialize services
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	pdfService := services.NewPDFService(uploadDir)
	vectorSearch := services.NewVectorSearch()

	// Initialize AI service (Groq, Puter AI, or Mock)
	var aiService services.AIService
	groqAPIKey := os.Getenv("GROQ_API_KEY")
	puterAIURL := os.Getenv("PUTER_AI_URL")
	puterAIKey := os.Getenv("PUTER_AI_KEY")

	// Priority: Groq > Puter AI > Mock
	if groqAPIKey != "" {
		aiService = services.NewGroqAIServiceAdapter(appservices.NewGroqService(groqAPIKey))
		log.Println("Using Groq AI service")
	} else if puterAIURL != "" || puterAIKey != "" {
		aiService = services.NewPuterAIService()
		log.Println("Using Puter AI service")
	} else {
		aiService = services.NewMockAIService()
		log.Println("Using Mock AI service (set GROQ_API_KEY for real AI)")
	}

	// Initialize use cases
	pdfUseCase := usecases.NewPDFUseCase(docRepo, sessionRepo, pdfService)
	chatUseCase := usecases.NewChatUseCase(sessionRepo, aiService, vectorSearch)
	summaryUseCase := usecases.NewSummaryUseCase(sessionRepo, aiService)

	// Initialize auth and persistence
	userRepo := repositories.NewUserRepository()
	authHandler := handlers.NewAuthHandler(userRepo)
	persistenceRepo := repositories.NewPersistenceRepository()
	userHandler := handlers.NewUserHandler(persistenceRepo)

	// Initialize handlers
	pdfHandler := handlers.NewPDFHandler(pdfUseCase, persistenceRepo)
	chatHandler := handlers.NewChatHandler(chatUseCase, persistenceRepo)
	summaryHandler := handlers.NewSummaryHandler(summaryUseCase)

	// Start session cleanup goroutine
	go startSessionCleanup(sessionRepo)

	// Initialize Gin router
	r := gin.Default()

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:3000", "http://localhost:3001", "http://localhost:5173", "http://localhost:80"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}
	config.AllowCredentials = true
	r.Use(cors.New(config))

	// API Routes
	api := r.Group("/api/v1")
	{
		// Health check
		healthHandler := func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":  "healthy",
				"service": "AskMyPDF API",
				"version": "1.0.0",
			})
		}
		api.GET("/health", healthHandler)
		api.HEAD("/health", healthHandler)

		// Auth routes (public)
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.GET("/me", handlers.AuthMiddleware(), authHandler.Me)
		}

		// User routes (protected)
		user := api.Group("/user")
		user.Use(handlers.AuthMiddleware())
		{
			user.GET("/sessions", userHandler.GetSessions)
			user.GET("/sessions/:sessionId/messages", userHandler.GetSessionMessages)
			user.DELETE("/sessions/:sessionId", userHandler.DeleteSession)
		}

		// PDF routes (with optional auth to link sessions to users)
		pdf := api.Group("/pdf")
		pdf.Use(handlers.OptionalAuthMiddleware())
		{
			pdf.POST("/upload", pdfHandler.Upload)
			pdf.GET("/status/:id", pdfHandler.Status)
			pdf.GET("/session/:sessionId/documents", pdfHandler.ListSessionDocuments)
			pdf.POST("/session/:sessionId/add", pdfHandler.AddToSession)
			pdf.DELETE("/document/:documentId", pdfHandler.DeleteDocument)
		}

		// Chat routes (with optional auth to persist messages)
		chat := api.Group("/chat")
		chat.Use(handlers.OptionalAuthMiddleware())
		{
			chat.POST("/message", chatHandler.Message)
			chat.POST("/stream", chatHandler.Stream)
			chat.GET("/history/:sessionId", chatHandler.History)
			chat.DELETE("/session/:sessionId", chatHandler.ClearSession)
		}

		// Summary routes
		api.POST("/pdf/summary", summaryHandler.Generate)
	}

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("AskMyPDF API server starting on port %s", port)
	log.Printf("AI Service: %T", aiService)
	log.Fatal(r.Run(":" + port))
}

// startSessionCleanup periodically cleans up inactive sessions
func startSessionCleanup(sessionRepo *repositories.SessionRepository) {
	ticker := time.NewTicker(1 * time.Hour) // Run every hour
	defer ticker.Stop()

	for range ticker.C {
		cleaned := sessionRepo.CleanupInactive(1 * time.Hour) // Remove sessions inactive for 1 hour
		if cleaned > 0 {
			log.Printf("Cleaned up %d inactive sessions", cleaned)
		}
	}
}
