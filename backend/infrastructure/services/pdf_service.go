package services

import (
	"fmt"
	"os"
	"strings"

	"ai-pdf-assistant-backend/proto"
	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

// PDFService handles PDF parsing and text extraction
type PDFService struct {
	uploadDir string
}

// NewPDFService creates a new PDF service
func NewPDFService(uploadDir string) *PDFService {
	os.MkdirAll(uploadDir, 0755)
	return &PDFService{uploadDir: uploadDir}
}

// ProcessPDF extracts text from a PDF file and creates chunks
func (s *PDFService) ProcessPDF(filePath string, filename string) (*proto.Document, error) {
	// Open the PDF file
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer file.Close()

	var textBuilder strings.Builder
	totalPages := reader.NumPage()

	// Extract text from all pages
	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue // Skip pages with extraction errors
		}

		textBuilder.WriteString(text)
		textBuilder.WriteString("\n\n")
	}

	extractedText := textBuilder.String()
	if strings.TrimSpace(extractedText) == "" {
		return nil, fmt.Errorf("no text could be extracted from PDF")
	}

	// Create chunks
	chunks := s.chunkText(extractedText, 2000) // 2000 character chunks
	protoChunks := make([]*proto.Chunk, len(chunks))

	for i, chunkText := range chunks {
		protoChunks[i] = &proto.Chunk{
			Id:         uuid.New().String(),
			Text:       chunkText,
			ChunkIndex: int32(i),
			PageNumber: 1, // Simplified - could track actual page
		}
	}

	// Create document
	doc := &proto.Document{
		Id:       uuid.New().String(),
		Filename: filename,
		Text:     extractedText,
		Pages:    int32(totalPages),
		Chunks:   protoChunks,
	}

	return doc, nil
}

// chunkText splits text into chunks of approximately maxChunkSize characters
func (s *PDFService) chunkText(text string, maxChunkSize int) []string {
	if len(text) <= maxChunkSize {
		return []string{text}
	}

	var chunks []string
	words := strings.Fields(text)

	var currentChunk strings.Builder

	for _, word := range words {
		// Check if adding this word would exceed the limit
		if currentChunk.Len()+len(word)+1 > maxChunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
			currentChunk.Reset()
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(word)
	}

	// Add the last chunk if it has content
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

