package services

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CitationService enforces grounded citations in AI responses.
// It tags context chunks with citation IDs, and post-processes
// the LLM response to validate and format citations.
//
// This prevents hallucination by ensuring every claim is traceable
// to a specific source passage. Common in enterprise RAG systems
// like Google Vertex AI Search, Azure AI Search, Perplexity, etc.
type CitationService struct {
	citationPattern *regexp.Regexp
}

// CitationTag is a tagged chunk used in the prompt
type CitationTag struct {
	ID         string
	ChunkID    string
	PageNumber int32
	Text       string
}

// ParsedCitation represents a citation found in the AI response
type ParsedCitation struct {
	ID         string `json:"id"`
	PageNumber int32  `json:"page_number"`
	Text       string `json:"text"` // The passage being cited
	Valid      bool   `json:"valid"`
}

// CitationStats tracks citation quality
type CitationStats struct {
	TotalCitations   int     `json:"total_citations"`
	ValidCitations   int     `json:"valid_citations"`
	InvalidCitations int     `json:"invalid_citations"`
	UniquePages      int     `json:"unique_pages"`
	GroundingScore   float64 `json:"grounding_score"` // 0-1, ratio of valid citations
}

// NewCitationService creates a new citation service
func NewCitationService() *CitationService {
	return &CitationService{
		citationPattern: regexp.MustCompile(`\[cite:(\d+)\]`),
	}
}

// TagChunks creates citation-tagged chunks for the prompt.
// Each chunk gets a short [cite:N] ID that the LLM must reference.
func (cs *CitationService) TagChunks(chunks []*RerankResult) []CitationTag {
	tags := make([]CitationTag, len(chunks))
	for i, chunk := range chunks {
		tags[i] = CitationTag{
			ID:         fmt.Sprintf("%d", i+1),
			ChunkID:    chunk.Chunk.Id,
			PageNumber: chunk.Chunk.PageNumber,
			Text:       chunk.Chunk.Text,
		}
	}
	return tags
}

// BuildCitationPrompt creates a system prompt that enforces citation usage.
// This is the key to grounding — the LLM MUST cite sources.
func (cs *CitationService) BuildCitationPrompt(tags []CitationTag) string {
	var builder strings.Builder

	builder.WriteString("You are a precise document assistant. Answer ONLY using the provided source passages. ")
	builder.WriteString("EVERY factual claim MUST include a citation in the format [cite:N] where N is the passage number. ")
	builder.WriteString("If you cannot answer from the sources, say so explicitly.\n\n")
	builder.WriteString("RULES:\n")
	builder.WriteString("1. ALWAYS cite your sources using [cite:N] format\n")
	builder.WriteString("2. Place citations INLINE after the relevant statement\n")
	builder.WriteString("3. You may cite multiple sources: [cite:1][cite:3]\n")
	builder.WriteString("4. DO NOT make claims without citations\n")
	builder.WriteString("5. If sources conflict, mention both with their citations\n\n")
	builder.WriteString("SOURCE PASSAGES:\n\n")

	for _, tag := range tags {
		text := tag.Text
		if len(text) > 2000 {
			text = text[:2000] + "..."
		}
		builder.WriteString(fmt.Sprintf("[cite:%s] (Page %d):\n%s\n\n", tag.ID, tag.PageNumber, text))
	}

	return builder.String()
}

// ExtractCitations parses [cite:N] references from the AI response
// and validates them against the provided tags.
func (cs *CitationService) ExtractCitations(response string, tags []CitationTag) ([]ParsedCitation, *CitationStats) {
	stats := &CitationStats{}

	// Build tag lookup
	tagMap := make(map[string]*CitationTag, len(tags))
	for i := range tags {
		tagMap[tags[i].ID] = &tags[i]
	}

	// Find all citation references
	matches := cs.citationPattern.FindAllStringSubmatch(response, -1)
	seenIDs := make(map[string]bool)
	seenPages := make(map[int32]bool)

	var citations []ParsedCitation
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id := match[1]
		if seenIDs[id] {
			continue // Skip duplicate citations
		}
		seenIDs[id] = true

		tag, exists := tagMap[id]
		citation := ParsedCitation{
			ID:    id,
			Valid: exists,
		}

		if exists {
			citation.PageNumber = tag.PageNumber
			if len(tag.Text) > 200 {
				citation.Text = tag.Text[:200] + "..."
			} else {
				citation.Text = tag.Text
			}
			seenPages[tag.PageNumber] = true
			stats.ValidCitations++
		} else {
			stats.InvalidCitations++
		}

		citations = append(citations, citation)
	}

	stats.TotalCitations = len(citations)
	stats.UniquePages = len(seenPages)

	if stats.TotalCitations > 0 {
		stats.GroundingScore = float64(stats.ValidCitations) / float64(stats.TotalCitations)
	}

	return citations, stats
}

// FormatResponseWithCitations transforms [cite:N] tags into readable
// page references like [Page 5] in the final response.
func (cs *CitationService) FormatResponseWithCitations(response string, tags []CitationTag) string {
	tagMap := make(map[string]*CitationTag, len(tags))
	for i := range tags {
		tagMap[tags[i].ID] = &tags[i]
	}

	formatted := cs.citationPattern.ReplaceAllStringFunc(response, func(match string) string {
		submatch := cs.citationPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		id := submatch[1]
		tag, exists := tagMap[id]
		if !exists {
			return match
		}
		return fmt.Sprintf("[Page %d]", tag.PageNumber)
	})

	return formatted
}

// BuildPageCitations generates the page citation list for the response
func (cs *CitationService) BuildPageCitations(citations []ParsedCitation) []int32 {
	pageSet := make(map[int32]bool)
	for _, c := range citations {
		if c.Valid && c.PageNumber > 0 {
			pageSet[c.PageNumber] = true
		}
	}

	pages := make([]int32, 0, len(pageSet))
	for p := range pageSet {
		pages = append(pages, p)
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i] < pages[j]
	})
	return pages
}

// FormatCitationFootnotes creates a footnotes section for the response
func (cs *CitationService) FormatCitationFootnotes(citations []ParsedCitation) string {
	if len(citations) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n\n---\n**Sources:**\n")

	for _, c := range citations {
		if !c.Valid {
			continue
		}
		// Parse the ID as int for display
		num, _ := strconv.Atoi(c.ID)
		preview := c.Text
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		builder.WriteString(fmt.Sprintf("- [%d] Page %d: %s\n", num, c.PageNumber, preview))
	}

	return builder.String()
}
