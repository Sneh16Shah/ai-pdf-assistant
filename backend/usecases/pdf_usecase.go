package usecases

import (
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// PDFUseCase handles PDF-related business logic
type PDFUseCase struct {
	docRepo          *repositories.DocumentRepository
	sessionRepo      *repositories.SessionRepository
	pdfService       *services.PDFService
	aiService        services.AIService  // nil if enterprise pipeline disabled
	embeddingService services.Embedder   // nil if enterprise pipeline disabled (accepts Gemini or NVIDIA)
	vectorStore      *services.VectorStore // nil if enterprise pipeline disabled
	markitdown       *services.MarkItDownService // nil = basic parser only (all pipelines)
	markdownChunker  *services.MarkdownChunker
}

// NewPDFUseCase creates a new PDF use case
func NewPDFUseCase(
	docRepo *repositories.DocumentRepository,
	sessionRepo *repositories.SessionRepository,
	pdfService *services.PDFService,
) *PDFUseCase {
	return &PDFUseCase{
		docRepo:     docRepo,
		sessionRepo: sessionRepo,
		pdfService:  pdfService,
	}
}

// SetEnterpriseServices enables embedding generation on upload.
// Called from main.go when embedding service is available (Gemini or NVIDIA).
func (uc *PDFUseCase) SetEnterpriseServices(aiService services.AIService, embeddingService services.Embedder, vectorStore *services.VectorStore) {
	uc.aiService = aiService
	uc.embeddingService = embeddingService
	uc.vectorStore = vectorStore
}

// SetMarkItDownService enables Markdown conversion via the MarkItDown sidecar.
// Called from main.go when MARKITDOWN_URL is set. Pass nil to disable — uploads
// then fall back to the basic parser for all pipelines (Standard/Smart/Enterprise).
func (uc *PDFUseCase) SetMarkItDownService(md *services.MarkItDownService) {
	uc.markitdown = md
	if md != nil {
		uc.markdownChunker = services.NewMarkdownChunker()
	}
}

// enrichWithMarkdown converts the PDF to Markdown via the sidecar (if configured)
// and attaches the result to the document. Non-fatal: on any error, the document
// keeps its basic Chunks/Text and Smart/Enterprise pipelines fall back to those.
func (uc *PDFUseCase) enrichWithMarkdown(doc *proto.Document) {
	if uc.markitdown == nil || uc.markdownChunker == nil || doc == nil {
		return
	}

	result, err := uc.markitdown.ConvertFile(doc.FilePath)
	if err != nil {
		log.Printf("MarkItDown failed for '%s': %v (falling back to basic chunks)", doc.Filename, err)
		return
	}
	if strings.TrimSpace(result.Markdown) == "" {
		return
	}

	doc.MarkdownText = result.Markdown
	doc.MarkdownChunks = uc.markdownChunker.Chunk(result.Markdown)
	doc.MarkdownOutline = uc.markdownChunker.BuildOutline(result.Markdown)
	doc.MarkdownSource = "markitdown"

	log.Printf("MarkItDown enriched '%s': %d markdown chunks, %d outline lines",
		doc.Filename, len(doc.MarkdownChunks), strings.Count(doc.MarkdownOutline, "\n"))
}

// generateEmbeddings generates embeddings for all chunks in a document
// and stores them both on the chunks and in the vector store.
//
// When MarkItDown has populated MarkdownChunks, those are embedded instead of
// the basic Chunks — Smart/Enterprise retrieve over the richer representation.
// Basic Chunks are left untouched so Standard still works.
func (uc *PDFUseCase) generateEmbeddings(doc *proto.Document) {
	if uc.embeddingService == nil || uc.vectorStore == nil {
		return
	}

	// Prefer markdown chunks when available; fall back to basic chunks.
	chunks := doc.MarkdownChunks
	isMarkdown := true
	if len(chunks) == 0 {
		chunks = doc.Chunks
		isMarkdown = false
	}
	if len(chunks) == 0 {
		return
	}

	// Extract texts from chunks
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Text
	}

	log.Printf("Generating embeddings for %d %s chunks of '%s'...", len(texts), map[bool]string{true: "markdown", false: "basic"}[isMarkdown], doc.Filename)

	// Generate embeddings via Gemini
	embeddings, err := uc.embeddingService.EmbedBatch(texts)
	if err != nil {
		log.Printf("WARNING: Failed to generate embeddings for '%s': %v (enterprise search will fall back to BM25-only)", doc.Filename, err)
		return
	}

	// Store embeddings on chunks
	for i, emb := range embeddings {
		if i < len(chunks) && emb != nil {
			chunks[i].Embedding = emb
		}
	}

	// Index in vector store for fast retrieval
	uc.vectorStore.IndexChunks(chunks, embeddings)

	log.Printf("Embeddings generated and indexed for '%s' (%d chunks, %d dimensions)",
		doc.Filename, len(embeddings), services.EmbeddingDimension)
}

// IsVisualQuery returns true if the user message is asking about visual content
// like charts, figures, diagrams, images, or tables.
func IsVisualQuery(message string) bool {
	lower := strings.ToLower(message)
	visualKeywords := []string{
		"figure", "fig.", "chart", "graph", "diagram", "image",
		"picture", "illustration", "visual", "table", "plot",
		"shows in", "shown in", "depicted", "screenshot",
		"what does it look like", "what is shown", "what does the image",
	}
	for _, kw := range visualKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// GetVisualContext extracts images from the PDF at filePath and returns
// AI-generated captions as a formatted string to be injected into the
// chat context. Only called when IsVisualQuery() returns true.
// Returns an empty string if vision is unavailable or no images found.
func (uc *PDFUseCase) GetVisualContext(filePath string) string {
	if uc.aiService == nil || filePath == "" {
		return ""
	}

	// Create a temporary directory for extracted images
	tmpDir, err := os.MkdirTemp("", "pdfimages-*")
	if err != nil {
		log.Printf("[Vision] Failed to create temp dir: %v", err)
		return ""
	}
	defer os.RemoveAll(tmpDir)

	conf := model.NewDefaultConfiguration()
	err = api.ExtractImagesFile(filePath, tmpDir, nil, conf)
	if err != nil {
		log.Printf("[Vision] No images extracted (or extraction failed): %v", err)
		return ""
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil || len(files) == 0 {
		return ""
	}

	log.Printf("[Vision] Found %d images. Describing up to 2 for visual query...", len(files))

	var captions []string
	processed := 0
	maxImages := 2

	for _, file := range files {
		if processed >= maxImages {
			break
		}
		if file.IsDir() {
			continue
		}

		// Skip tiny images (icons, watermarks) under 5KB
		info, infoErr := file.Info()
		if infoErr != nil || info.Size() < 5120 {
			continue
		}

		imgData, err := os.ReadFile(filepath.Join(tmpDir, file.Name()))
		if err != nil {
			continue
		}

		// Throttle: wait before each call so we don't burst the free tier
		if processed > 0 {
			time.Sleep(5 * time.Second)
		}

		caption, err := uc.aiService.DescribeImage(imgData)
		if err != nil {
			errStr := err.Error()
			log.Printf("[Vision] Failed to describe image %s: %v", file.Name(), err)
			if strings.Contains(errStr, "429") || strings.Contains(errStr, "RESOURCE_EXHAUSTED") {
				log.Printf("[Vision] Rate limit hit — stopping image description.")
				break
			}
			continue
		}

		if caption != "" {
			captions = append(captions, fmt.Sprintf("[Visual Element %d]: %s", processed+1, caption))
			processed++
		}
	}

	if len(captions) == 0 {
		return ""
	}

	return "\n\n=== Visual Content from Document ===\n" + strings.Join(captions, "\n\n") + "\n==="
}

// UploadPDF processes and stores a PDF file
func (uc *PDFUseCase) UploadPDF(filePath string, filename string) (*proto.UploadResponse, error) {
	// Process PDF
	doc, err := uc.pdfService.ProcessPDF(filePath, filename)
	if err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "PDF_PROCESSING_ERROR",
				Message: fmt.Sprintf("Failed to process PDF: %v", err),
			},
		}, nil
	}

	// Enrich with MarkItDown markdown (non-fatal; falls back to basic chunks).
	// Must run before generateEmbeddings so embeddings use markdown chunks.
	uc.enrichWithMarkdown(doc)

	// Generate embeddings for enterprise pipeline (non-blocking if it fails)
	uc.generateEmbeddings(doc)

	// Store document
	if err := uc.docRepo.Store(doc); err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "STORAGE_ERROR",
				Message: fmt.Sprintf("Failed to store document: %v", err),
			},
		}, nil
	}

	// Create session
	session, err := uc.sessionRepo.Create(doc.Id, doc)
	if err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "SESSION_ERROR",
				Message: fmt.Sprintf("Failed to create session: %v", err),
			},
		}, nil
	}

	return &proto.UploadResponse{
		Status:    proto.Status_STATUS_SUCCESS,
		Document:  doc,
		SessionId: session.Id,
	}, nil
}

// GetDocumentStatus retrieves document status
func (uc *PDFUseCase) GetDocumentStatus(documentID string) (*proto.StatusResponse, error) {
	doc, err := uc.docRepo.Get(documentID)
	if err != nil {
		return &proto.StatusResponse{
			Status: proto.Status_STATUS_NOT_FOUND,
			Error: &proto.Error{
				Code:    "NOT_FOUND",
				Message: fmt.Sprintf("Document not found: %v", err),
			},
		}, nil
	}

	return &proto.StatusResponse{
		Status:   proto.Status_STATUS_SUCCESS,
		Document: doc,
	}, nil
}

// AddDocumentToSession adds a document to an existing session
func (uc *PDFUseCase) AddDocumentToSession(sessionID string, filePath string, filename string) (*proto.UploadResponse, error) {
	// Process PDF
	doc, err := uc.pdfService.ProcessPDF(filePath, filename)
	if err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "PDF_PROCESSING_ERROR",
				Message: fmt.Sprintf("Failed to process PDF: %v", err),
			},
		}, nil
	}

	// Enrich with MarkItDown markdown (non-fatal; falls back to basic chunks).
	// Must run before generateEmbeddings so embeddings use markdown chunks.
	uc.enrichWithMarkdown(doc)

	// Generate embeddings for enterprise pipeline
	uc.generateEmbeddings(doc)

	// Store document
	if err := uc.docRepo.Store(doc); err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "STORAGE_ERROR",
				Message: fmt.Sprintf("Failed to store document: %v", err),
			},
		}, nil
	}

	// Add to existing session
	if err := uc.sessionRepo.AddDocument(sessionID, doc); err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "SESSION_ERROR",
				Message: fmt.Sprintf("Failed to add document to session: %v", err),
			},
		}, nil
	}

	return &proto.UploadResponse{
		Status:    proto.Status_STATUS_SUCCESS,
		Document:  doc,
		SessionId: sessionID,
	}, nil
}

// GetSessionDocuments returns all documents in a session
func (uc *PDFUseCase) GetSessionDocuments(sessionID string) ([]*proto.Document, error) {
	return uc.sessionRepo.GetDocuments(sessionID)
}

// RemoveDocumentFromSession removes a document from a session
func (uc *PDFUseCase) RemoveDocumentFromSession(sessionID string, documentID string) error {
	return uc.sessionRepo.RemoveDocument(sessionID, documentID)
}

// RehydrateSession processes a PDF and loads it into an existing session ID.
// This restores the in-memory session state after a backend restart without
// creating a new session (which would cause duplicates in the DB).
func (uc *PDFUseCase) RehydrateSession(sessionID string, filePath string, filename string) (*proto.UploadResponse, error) {
	// Process PDF
	doc, err := uc.pdfService.ProcessPDF(filePath, filename)
	if err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "PDF_PROCESSING_ERROR",
				Message: fmt.Sprintf("Failed to process PDF: %v", err),
			},
		}, nil
	}

	// Enrich with MarkItDown markdown (non-fatal; falls back to basic chunks).
	// Must run before generateEmbeddings so embeddings use markdown chunks.
	uc.enrichWithMarkdown(doc)

	// Generate embeddings for enterprise pipeline
	uc.generateEmbeddings(doc)

	// Store document in memory
	if err := uc.docRepo.Store(doc); err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "STORAGE_ERROR",
				Message: fmt.Sprintf("Failed to store document: %v", err),
			},
		}, nil
	}

	// Check if session already exists in memory — if so, just add the document
	_, getErr := uc.sessionRepo.Get(sessionID)
	if getErr == nil {
		// Session exists, add document to it
		_ = uc.sessionRepo.AddDocument(sessionID, doc)
	} else {
		// Session doesn't exist in memory — create it with the given ID
		_, err := uc.sessionRepo.CreateWithID(sessionID, doc.Id, doc)
		if err != nil {
			return &proto.UploadResponse{
				Status: proto.Status_STATUS_ERROR,
				Error: &proto.Error{
					Code:    "SESSION_ERROR",
					Message: fmt.Sprintf("Failed to re-hydrate session: %v", err),
				},
			}, nil
		}
	}

	// Vision processing now happens on-demand when the user asks a visual question.

	return &proto.UploadResponse{
		Status:    proto.Status_STATUS_SUCCESS,
		Document:  doc,
		SessionId: sessionID,
	}, nil
}
