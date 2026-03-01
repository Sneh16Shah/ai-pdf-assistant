package usecases

import (
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"strings"
)

// SmartContextBuilder builds optimized context using BM25 + TextRank + preprocessing.
// This is a drop-in alternative to ContextBuilder that trades CPU time for token savings.
type SmartContextBuilder struct {
	bm25         *services.BM25Search
	textRank     *services.TextRank
	preprocessor *services.Preprocessor
}

// NewSmartContextBuilder creates a new smart context builder
func NewSmartContextBuilder(
	bm25 *services.BM25Search,
	textRank *services.TextRank,
	preprocessor *services.Preprocessor,
) *SmartContextBuilder {
	return &SmartContextBuilder{
		bm25:         bm25,
		textRank:     textRank,
		preprocessor: preprocessor,
	}
}

// BuildContext creates optimized context using the full pipeline:
// 1. Preprocess (strip boilerplate, deduplicate)
// 2. BM25 retrieval (find truly relevant chunks)
// 3. TextRank summarization (summarize non-selected pages for overview)
// Returns context string and detailed token stats.
func (scb *SmartContextBuilder) BuildContext(
	allChunks []*proto.Chunk,
	fullText string,
	outline string,
	query string,
	pageNumber int32,
) (string, *TokenStats) {
	stats := &TokenStats{
		RawTokens:      EstimateTokens(fullText),
		ChunksOriginal: len(allChunks),
	}

	// Small document shortcut — same as original, no optimization needed
	if len(fullText) < SmallDocThreshold {
		stats.FinalTokens = stats.RawTokens
		stats.SavingsPercent = 0
		stats.ChunksUsed = len(allChunks)
		stats.ChunksAfterDedup = len(allChunks)
		return fullText, stats
	}

	// === STEP 1: Build page text map for preprocessing ===
	pageTexts := buildPageTextMap(allChunks)

	// === STEP 2: Preprocess — boilerplate stripping + dedup ===
	processedChunks, prepStats := scb.preprocessor.ProcessChunks(allChunks, pageTexts)
	stats.ChunksAfterDedup = len(processedChunks)
	stats.BoilerplateRemoved = prepStats.CharsRemoved

	// Compute tokens after preprocessing
	preprocessedText := chunksToText(processedChunks)
	stats.AfterPreprocessing = EstimateTokens(preprocessedText)

	// === STEP 3: BM25 retrieval — find the most relevant chunks ===
	scb.bm25.Index(processedChunks)

	maxChars := EstimateCharsFromTokens(MaxContextTokens)
	var contextBuilder strings.Builder
	usedChars := 0

	// Always include outline (it's compact)
	if outline != "" {
		header := "=== DOCUMENT OUTLINE ===\n"
		contextBuilder.WriteString(header)
		contextBuilder.WriteString(outline)
		contextBuilder.WriteString("\n=== END OUTLINE ===\n\n")
		usedChars += len(header) + len(outline) + 25
	}

	// Current page chunks (always included if page is known)
	addedChunks := make(map[string]bool)
	if pageNumber > 0 {
		contextBuilder.WriteString(fmt.Sprintf("=== CURRENT PAGE %d ===\n", pageNumber))
		for _, chunk := range processedChunks {
			if chunk.PageNumber == pageNumber {
				if usedChars+len(chunk.Text) > maxChars {
					break
				}
				contextBuilder.WriteString(chunk.Text)
				contextBuilder.WriteString("\n\n")
				usedChars += len(chunk.Text) + 2
				addedChunks[chunk.Id] = true
			}
		}
	}

	// BM25 top-K relevant chunks (much better than keyword matching)
	bm25TopK := 15
	relevantChunks := scb.bm25.Search(query, bm25TopK)
	stats.AfterRetrieval = 0

	contextBuilder.WriteString("=== BM25-RANKED RELEVANT SECTIONS ===\n")
	for _, chunk := range relevantChunks {
		if addedChunks[chunk.Id] {
			continue
		}
		if usedChars+len(chunk.Text) > maxChars*70/100 { // Leave 30% for summaries
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("[Page %d] ", chunk.PageNumber))
		contextBuilder.WriteString(chunk.Text)
		contextBuilder.WriteString("\n\n")
		usedChars += len(chunk.Text) + 12
		addedChunks[chunk.Id] = true
		stats.AfterRetrieval += EstimateTokens(chunk.Text)
	}

	// === STEP 4: TextRank summaries for remaining pages ===
	// Instead of sending raw text from non-relevant pages,
	// send extractive summaries (2-3 key sentences each).
	if usedChars < maxChars*90/100 {
		contextBuilder.WriteString("=== KEY EXCERPTS FROM OTHER SECTIONS ===\n")

		// Find pages not yet covered
		coveredPages := make(map[int32]bool)
		for id := range addedChunks {
			for _, chunk := range processedChunks {
				if chunk.Id == id {
					coveredPages[chunk.PageNumber] = true
				}
			}
		}

		summaryChars := 0
		for pageNum, text := range pageTexts {
			if coveredPages[pageNum] {
				continue
			}
			if usedChars+summaryChars > maxChars*95/100 {
				break
			}

			// TextRank: extract 2 most important sentences from this page
			summary := scb.textRank.Summarize(text, 2)
			if len(summary) > 0 {
				line := fmt.Sprintf("[Page %d summary] %s\n", pageNum, summary)
				contextBuilder.WriteString(line)
				summaryChars += len(line)
			}
		}
		stats.TextRankSummaryLen = summaryChars
	}

	context := contextBuilder.String()
	stats.FinalTokens = EstimateTokens(context)
	stats.ChunksUsed = len(addedChunks)

	if stats.RawTokens > 0 {
		stats.SavingsPercent = float64(stats.RawTokens-stats.FinalTokens) / float64(stats.RawTokens) * 100
	}

	return context, stats
}

// buildPageTextMap creates a page number → full text map from chunks
func buildPageTextMap(chunks []*proto.Chunk) map[int32]string {
	pageTexts := make(map[int32]string)
	for _, chunk := range chunks {
		if existing, ok := pageTexts[chunk.PageNumber]; ok {
			pageTexts[chunk.PageNumber] = existing + " " + chunk.Text
		} else {
			pageTexts[chunk.PageNumber] = chunk.Text
		}
	}
	return pageTexts
}

// chunksToText concatenates all chunk texts
func chunksToText(chunks []*proto.Chunk) string {
	var builder strings.Builder
	for _, chunk := range chunks {
		builder.WriteString(chunk.Text)
		builder.WriteString(" ")
	}
	return builder.String()
}
