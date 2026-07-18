package main

import (
	"log"
	"os"
	"strings"
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

	// --- FIX CVE-2: Enforce JWT_SECRET at startup ---
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable is not set. " +
			"Generate a strong secret and add it to your .env file before starting the server.")
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

	// Initialize AI service (Gemini > Groq > Puter AI > Mock)
	var aiService services.AIService
	groqAPIKey := os.Getenv("GROQ_API_KEY")
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	puterAIURL := os.Getenv("PUTER_AI_URL")
	puterAIKey := os.Getenv("PUTER_AI_KEY")

	// Priority: Gemini > Groq > Puter AI > Mock
	// Gemini 2.5 Flash-Lite: 1M context window, 1000 RPD, 15 RPM free
	if geminiAPIKey != "" {
		aiService = services.NewGeminiAIService(geminiAPIKey)
		log.Println("Using Gemini AI service (2.5 Flash-Lite, 1M context)")
	} else if groqAPIKey != "" {
		aiService = services.NewGroqAIServiceAdapter(appservices.NewGroqService(groqAPIKey))
		log.Println("Using Groq AI service (fallback)")
	} else if puterAIURL != "" || puterAIKey != "" {
		aiService = services.NewPuterAIService()
		log.Println("Using Puter AI service")
	} else {
		aiService = services.NewMockAIService()
		log.Println("Using Mock AI service (set GEMINI_API_KEY for best experience)")
	}

	// Initialize vision service (NVIDIA NIM > Groq)
	var visionService *services.VisionService
	nvidiaVLMKey := os.Getenv("NVIDIA_VLM_API_KEY")
	if nvidiaVLMKey != "" {
		nvidiaVision := services.NewNvidiaVisionService(nvidiaVLMKey)
		// Wrap NVIDIA vision in a compatible VisionService adapter
		visionService = services.NewVisionServiceFromNvidia(nvidiaVision)
		log.Println("Vision service enabled (NVIDIA NIM — llama-3.2-90b-vision-instruct)")
	} else if groqAPIKey != "" {
		visionService = services.NewVisionService(groqAPIKey)
		log.Println("Vision service enabled (Groq — Llama 4 Scout)")
	}

	// Initialize image generation service (NVIDIA NIM > Pollinations.ai)
	var imageGenService *services.ImageGenService
	nvidiaImageGenKey := os.Getenv("NVIDIA_IMAGEGEN_API_KEY")
	if nvidiaImageGenKey != "" {
		nvidiaImgGen := services.NewNvidiaImageGenService(nvidiaImageGenKey)
		// Wrap NVIDIA image gen in a compatible ImageGenService adapter
		imageGenService = services.NewImageGenServiceFromNvidia(nvidiaImgGen)
		log.Println("Image generation enabled (NVIDIA NIM — FLUX.1-dev)")
	} else {
		imageGenService = services.NewImageGenService("")
		log.Println("Image generation enabled (Pollinations.ai)")
	}

	// Initialize use cases
	pdfUseCase := usecases.NewPDFUseCase(docRepo, sessionRepo, pdfService)
	chatUseCase := usecases.NewChatUseCase(sessionRepo, aiService, vectorSearch, visionService, imageGenService, pdfUseCase)
	summaryUseCase := usecases.NewSummaryUseCase(sessionRepo, aiService)

	// Initialize auth and persistence
	userRepo := repositories.NewUserRepository()
	authHandler := handlers.NewAuthHandler(userRepo)
	persistenceRepo := repositories.NewPersistenceRepository()
	userHandler := handlers.NewUserHandler(persistenceRepo)
	metricsRepo := repositories.NewMetricsRepository()
	metricsHandler := handlers.NewMetricsHandler(metricsRepo)

	// Initialize smart scaling services (BM25 + TextRank + Preprocessing)
	bm25Search := services.NewBM25Search()
	textRank := services.NewTextRank()
	preprocessor := services.NewPreprocessor()
	smartContextBuilder := usecases.NewSmartContextBuilder(bm25Search, textRank, preprocessor)
	smartChatUseCase := usecases.NewSmartChatUseCase(sessionRepo, aiService, smartContextBuilder, bm25Search, vectorSearch, visionService, imageGenService)
	log.Println("Smart scaling services enabled (BM25 + TextRank + Preprocessing)")

	// Initialize enterprise RAG pipeline services (NVIDIA NIM > Gemini)
	var enterpriseHandler *handlers.EnterpriseHandler
	vectorStore := services.NewVectorStore()
	citationService := services.NewCitationService()

	// Determine embedding service: NVIDIA > Gemini
	nvidiaEmbedKey := os.Getenv("NVIDIA_EMBED_API_KEY")
	var embedder services.Embedder
	if nvidiaEmbedKey != "" {
		embedder = services.NewNvidiaEmbeddingService(nvidiaEmbedKey)
		log.Println("Embedding service enabled (NVIDIA NIM — llama-nemotron-embed-vl-1b-v2)")
	} else if geminiAPIKey != "" {
		embedder = services.NewEmbeddingService(geminiAPIKey)
		log.Println("Embedding service enabled (Gemini — text-embedding-004)")
	}

	// Determine reranker service: NVIDIA > Gemini
	nvidiaRerankKey := os.Getenv("NVIDIA_RERANK_API_KEY")
	var reranker services.Reranker
	if nvidiaRerankKey != "" {
		reranker = services.NewNvidiaRerankerService(nvidiaRerankKey)
		log.Println("Reranker service enabled (NVIDIA NIM — llama-nemotron-rerank-1b-v2)")
	} else if geminiAPIKey != "" {
		reranker = services.NewRerankerService(geminiAPIKey)
		log.Println("Reranker service enabled (Gemini — prompt-based)")
	}

	if embedder != nil {
		hybridSearch := services.NewHybridSearch(bm25Search, vectorStore, embedder)

		if reranker == nil {
			log.Println("Reranker unavailable, enterprise pipeline will skip reranking")
		}

		enterpriseContextBuilder := usecases.NewEnterpriseContextBuilder(hybridSearch, reranker, citationService, preprocessor)
		enterpriseChatUseCase := usecases.NewEnterpriseChatUseCase(sessionRepo, aiService, enterpriseContextBuilder, citationService, visionService, imageGenService)
		enterpriseHandler = handlers.NewEnterpriseHandler(enterpriseChatUseCase, smartChatUseCase, chatUseCase, persistenceRepo)

		// Hook embedding generation into PDF upload flow
		pdfUseCase.SetEnterpriseServices(aiService, embedder, vectorStore)

		log.Println("Enterprise RAG pipeline enabled (Embeddings + Hybrid Search + Reranking + Citations)")
	} else {
		log.Println("Enterprise RAG pipeline disabled (requires NVIDIA_EMBED_API_KEY or GEMINI_API_KEY)")
	}

	// Initialize handlers
	pdfHandler := handlers.NewPDFHandler(pdfUseCase, persistenceRepo)
	chatHandler := handlers.NewChatHandler(chatUseCase, persistenceRepo)
	summaryHandler := handlers.NewSummaryHandler(summaryUseCase)
	smartChatHandler := handlers.NewSmartChatHandler(smartChatUseCase, persistenceRepo)

	// Start session cleanup goroutine
	go startSessionCleanup(sessionRepo)

	// Initialize Gin router
	r := gin.Default()

	// --- FIX CVE-3: Tightened CORS — removed wildcard chrome-extension allowlist ---
	config := cors.DefaultConfig()
	config.AllowOriginFunc = func(origin string) bool {
		// 1. Allow origins specified in ALLOWED_ORIGINS env var
		//    (e.g., "https://my-frontend.onrender.com,chrome-extension://EXACT_ID_HERE")
		envOrigins := os.Getenv("ALLOWED_ORIGINS")
		if envOrigins != "" {
			for _, allowed := range strings.Split(envOrigins, ",") {
				if strings.TrimSpace(allowed) == origin {
					return true
				}
			}
		}

		// 2. Allow localhost origins for local development
		allowedOrigins := map[string]bool{
			"http://localhost:3000": true,
			"http://localhost:3001": true,
			"http://localhost:5173": true,
			"http://localhost:80":   true,
		}
		if allowedOrigins[origin] {
			return true
		}

		// NOTE: Chrome extension origins are NO LONGER allowed by wildcard.
		// To whitelist your extension, add its exact origin to ALLOWED_ORIGINS:
		//   ALLOWED_ORIGINS=https://your-frontend.com,chrome-extension://YOUR_EXTENSION_ID
		return false
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}
	config.AllowCredentials = true
	r.Use(cors.New(config))

	// --- FIX CVE-5: IP-based Rate Limiting ---
	r.Use(rateLimitMiddleware())

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

		// Metrics route (public — no auth required)
		api.GET("/metrics", metricsHandler.GetMetrics)

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
			pdf.POST("/rehydrate", pdfHandler.Rehydrate)
			pdf.GET("/status/:id", pdfHandler.Status)
			pdf.GET("/session/:sessionId/documents", pdfHandler.ListSessionDocuments)
			pdf.POST("/session/:sessionId/add", pdfHandler.AddToSession)
			pdf.DELETE("/document/:documentId", pdfHandler.DeleteDocument)
			pdf.GET("/document/:documentId/download", pdfHandler.DownloadDocument)
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

		// Smart chat routes (optimized context with BM25 + TextRank + preprocessing)
		smart := api.Group("/smart")
		smart.Use(handlers.OptionalAuthMiddleware())
		{
			smart.POST("/message", smartChatHandler.Message)
			smart.GET("/stats/:sessionId", smartChatHandler.Stats)
		}

		// Enterprise RAG routes (hybrid retrieval + reranking + citations)
		if enterpriseHandler != nil {
			enterprise := api.Group("/enterprise")
			enterprise.Use(handlers.OptionalAuthMiddleware())
			{
				enterprise.POST("/message", enterpriseHandler.Message)
				enterprise.GET("/stats/:sessionId", enterpriseHandler.Stats)
				enterprise.POST("/compare", enterpriseHandler.Compare)
			}
		}
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
