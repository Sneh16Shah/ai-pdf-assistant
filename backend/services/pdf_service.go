package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

type PDFService struct {
	uploadDir string
}

type PDFDocument struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Text     string `json:"text"`
	Pages    int    `json:"pages"`
	Chunks   []string `json:"chunks"`
}

func NewPDFService(uploadDir string) *PDFService {
	// Create upload directory if it doesn't exist
	os.MkdirAll(uploadDir, 0755)
	return &PDFService{uploadDir: uploadDir}
}

func (s *PDFService) ProcessPDF(filePath string, filename string) (*PDFDocument, error) {
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

	// Create document with chunks
	doc := &PDFDocument{
		ID:       uuid.New().String(),
		Filename: filename,
		Text:     extractedText,
		Pages:    totalPages,
		Chunks:   s.chunkText(extractedText, 2000), // 2000 character chunks
	}

	return doc, nil
}

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

func (s *PDFService) ExtractTextFromFile(filePath string) (string, error) {
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer file.Close()

	var textBuilder strings.Builder
	totalPages := reader.NumPage()

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		
		textBuilder.WriteString(fmt.Sprintf("--- Page %d ---\n", pageNum))
		textBuilder.WriteString(text)
		textBuilder.WriteString("\n\n")
	}

	return textBuilder.String(), nil
}