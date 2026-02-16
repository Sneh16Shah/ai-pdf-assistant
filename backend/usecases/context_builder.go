package usecases

import (
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"strings"
)

const (
	// MaxContextTokens is the token budget for document context
	// Gemini 2.5 Flash-Lite has 1M context window.
	// We allocate ~100K tokens for document context — generous but well within 1M.
	// If using Groq fallback (8K), groq_service.go has its own truncation.
	MaxContextTokens = 100000
	// SmallDocThreshold — if total doc text is under this, send everything
	// 400K chars ≈ 100K tokens, safe for Gemini
	SmallDocThreshold = 400000
	// AdjacentPageRange — how many pages ± to include
	AdjacentPageRange = 3
)

// ContextBuilder builds tiered context from document chunks within a token budget
type ContextBuilder struct {
	vectorSearch *services.VectorSearch
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(vectorSearch *services.VectorSearch) *ContextBuilder {
	return &ContextBuilder{vectorSearch: vectorSearch}
}

// BuildContext creates the optimal context string from chunks given the user's current page.
// For small documents (< 400K chars), sends everything.
// For large documents, uses tiered approach: selected page → adjacent → keyword relevant → outline.
func (cb *ContextBuilder) BuildContext(
	allChunks []*proto.Chunk,
	fullText string,
	outline string,
	query string,
	pageNumber int32,
) string {
	// Small document shortcut — send everything
	if len(fullText) < SmallDocThreshold {
		return fullText
	}

	// Large document — use tiered context building
	maxChars := EstimateCharsFromTokens(MaxContextTokens)
	var contextBuilder strings.Builder
	usedChars := 0

	// Track which chunk IDs we've already added to avoid duplicates
	addedChunks := make(map[string]bool)

	// === TIER 4 (always included first — it's small): Document Outline ===
	if outline != "" {
		header := "=== DOCUMENT OUTLINE ===\n"
		contextBuilder.WriteString(header)
		contextBuilder.WriteString(outline)
		contextBuilder.WriteString("\n=== END OUTLINE ===\n\n")
		usedChars += len(header) + len(outline) + 25
	}

	// === TIER 1: Selected page chunks (always included if page is known) ===
	if pageNumber > 0 {
		pageChunks := cb.getChunksByPage(allChunks, pageNumber)
		if len(pageChunks) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("=== CURRENT PAGE %d ===\n", pageNumber))
			for _, chunk := range pageChunks {
				if usedChars+len(chunk.Text) > maxChars {
					break
				}
				contextBuilder.WriteString(chunk.Text)
				contextBuilder.WriteString("\n\n")
				usedChars += len(chunk.Text) + 2
				addedChunks[chunk.Id] = true
			}
		}

		// === TIER 2: Adjacent pages (±3) ===
		contextBuilder.WriteString("=== NEARBY PAGES ===\n")
		for offset := int32(1); offset <= AdjacentPageRange; offset++ {
			for _, adjPage := range []int32{pageNumber - offset, pageNumber + offset} {
				if adjPage < 1 || adjPage == pageNumber {
					continue
				}
				adjChunks := cb.getChunksByPage(allChunks, adjPage)
				for _, chunk := range adjChunks {
					if addedChunks[chunk.Id] {
						continue
					}
					if usedChars+len(chunk.Text) > maxChars*80/100 { // Leave 20% for Tier 3
						break
					}
					contextBuilder.WriteString(fmt.Sprintf("[Page %d] ", adjPage))
					contextBuilder.WriteString(chunk.Text)
					contextBuilder.WriteString("\n\n")
					usedChars += len(chunk.Text) + 12
					addedChunks[chunk.Id] = true
				}
			}
		}
	}

	// === TIER 3: Keyword-relevant chunks from the rest of the document ===
	relevantChunks := cb.vectorSearch.FindRelevantChunks(allChunks, query, 30)
	contextBuilder.WriteString("=== RELEVANT SECTIONS ===\n")
	for _, chunk := range relevantChunks {
		if addedChunks[chunk.Id] {
			continue
		}
		if usedChars+len(chunk.Text) > maxChars {
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("[Page %d] ", chunk.PageNumber))
		contextBuilder.WriteString(chunk.Text)
		contextBuilder.WriteString("\n\n")
		usedChars += len(chunk.Text) + 12
		addedChunks[chunk.Id] = true
	}

	return contextBuilder.String()
}

// getChunksByPage returns all chunks belonging to a specific page
func (cb *ContextBuilder) getChunksByPage(chunks []*proto.Chunk, pageNum int32) []*proto.Chunk {
	var result []*proto.Chunk
	for _, chunk := range chunks {
		if chunk.PageNumber == pageNum {
			result = append(result, chunk)
		}
	}
	return result
}
