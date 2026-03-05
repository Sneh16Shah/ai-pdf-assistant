package services

import (
	"fmt"
	"os"
	"strings"

	"ai-pdf-assistant-backend/proto"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

const (
	maxChunkSize  = 1500 // Characters per chunk
	overlapSize   = 200  // Overlap between page boundaries
	outlineMaxLen = 100  // Max chars per page in outline
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

// ProcessPDF extracts text from a PDF file and creates page-aware chunks
func (s *PDFService) ProcessPDF(filePath string, filename string) (*proto.Document, error) {
	// Open the PDF file
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer file.Close()

	totalPages := reader.NumPage()

	// Phase 1: Extract text per page
	pageTexts := make(map[int32]string, totalPages)
	var fullTextBuilder strings.Builder
	var outlineBuilder strings.Builder

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue // Skip pages with extraction errors
		}

		trimmedText := strings.TrimSpace(text)
		if trimmedText == "" {
			continue
		}

		pageTexts[int32(pageNum)] = trimmedText
		fullTextBuilder.WriteString(trimmedText)
		fullTextBuilder.WriteString("\n\n")

		// Build outline: first ~100 chars of each page
		outlineLine := trimmedText
		if len(outlineLine) > outlineMaxLen {
			outlineLine = outlineLine[:outlineMaxLen] + "..."
		}
		outlineBuilder.WriteString(fmt.Sprintf("Page %d: %s\n", pageNum, outlineLine))
	}

	extractedText := fullTextBuilder.String()
	if strings.TrimSpace(extractedText) == "" {
		return nil, fmt.Errorf("no text could be extracted from PDF")
	}

	// Phase 2: Create page-aware chunks with overlap
	var allChunks []*proto.Chunk
	chunkIndex := 0
	var prevPageLastText string // For overlap between pages

	for pageNum := int32(1); pageNum <= int32(totalPages); pageNum++ {
		pageText, exists := pageTexts[pageNum]
		if !exists {
			continue
		}

		// Prepend overlap from previous page to first chunk of this page
		textToChunk := pageText
		if prevPageLastText != "" && len(prevPageLastText) > 0 {
			overlapText := prevPageLastText
			if len(overlapText) > overlapSize {
				overlapText = overlapText[len(overlapText)-overlapSize:]
			}
			textToChunk = overlapText + " " + textToChunk
		}

		// Chunk this page's text
		pageChunks := s.chunkText(textToChunk, maxChunkSize)

		for _, chunkText := range pageChunks {
			allChunks = append(allChunks, &proto.Chunk{
				Id:         uuid.New().String(),
				Text:       chunkText,
				ChunkIndex: int32(chunkIndex),
				PageNumber: pageNum,
			})
			chunkIndex++
		}

		// Store last portion of this page for next page's overlap
		prevPageLastText = pageText
	}

	// Create document with outline
	doc := &proto.Document{
		Id:       uuid.New().String(),
		Filename: filename,
		Text:     extractedText,
		Pages:    int32(totalPages),
		Chunks:   allChunks,
		Outline:  outlineBuilder.String(),
		FilePath: filePath, // Store for on-demand image extraction when user asks visual questions
	}

	return doc, nil
}

// chunkText splits text into chunks of approximately maxChunkSize characters
func (s *PDFService) chunkText(text string, chunkSize int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	words := strings.Fields(text)

	var currentChunk strings.Builder

	for _, word := range words {
		// Check if adding this word would exceed the limit
		if currentChunk.Len()+len(word)+1 > chunkSize && currentChunk.Len() > 0 {
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
