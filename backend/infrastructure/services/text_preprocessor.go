package services

import (
	"ai-pdf-assistant-backend/proto"
	"strings"
)

// Preprocessor handles boilerplate removal and chunk deduplication.
// Detects repeated headers/footers across pages and removes them,
// then removes near-duplicate chunks to reduce redundancy.
type Preprocessor struct {
	// Minimum fraction of pages a line must appear on to be considered boilerplate
	boilerplateThreshold float64
	// Minimum Jaccard similarity to consider two chunks as near-duplicates
	dedupeThreshold float64
}

// NewPreprocessor creates a new text preprocessor
func NewPreprocessor() *Preprocessor {
	return &Preprocessor{
		boilerplateThreshold: 0.5, // Line must appear on 50%+ of pages
		dedupeThreshold:      0.8, // 80%+ word overlap = duplicate
	}
}

// PreprocessStats tracks how much text was removed by preprocessing
type PreprocessStats struct {
	OriginalChunks   int `json:"original_chunks"`
	AfterDedup       int `json:"after_dedup"`
	BoilerplateLines int `json:"boilerplate_lines"`
	CharsRemoved     int `json:"chars_removed"`
	OriginalChars    int `json:"original_chars"`
	ProcessedChars   int `json:"processed_chars"`
}

// StripBoilerplate detects lines that appear on many pages (headers, footers,
// watermarks, page numbers) and removes them.
// Returns cleaned page texts and stats.
func (p *Preprocessor) StripBoilerplate(pageTexts map[int32]string) (map[int32]string, *PreprocessStats) {
	stats := &PreprocessStats{}
	if len(pageTexts) < 3 {
		// Too few pages to detect patterns
		return pageTexts, stats
	}

	// Count occurrences of each line across all pages
	lineCounts := make(map[string]int)
	totalPages := len(pageTexts)

	for _, text := range pageTexts {
		lines := strings.Split(text, "\n")
		seen := make(map[string]bool) // Avoid counting duplicates within same page
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) < 3 {
				continue // Skip blank/tiny lines
			}
			// Normalize: lowercase, remove digits (to catch "Page 1", "Page 2", etc.)
			normalized := normalizeForBoilerplate(trimmed)
			if len(normalized) < 3 {
				continue
			}
			if !seen[normalized] {
				lineCounts[normalized]++
				seen[normalized] = true
			}
		}
	}

	// Identify boilerplate patterns
	boilerplate := make(map[string]bool)
	threshold := int(float64(totalPages) * p.boilerplateThreshold)
	for line, count := range lineCounts {
		if count >= threshold {
			boilerplate[line] = true
			stats.BoilerplateLines++
		}
	}

	if len(boilerplate) == 0 {
		return pageTexts, stats
	}

	// Remove boilerplate lines from all pages
	cleaned := make(map[int32]string, len(pageTexts))
	for pageNum, text := range pageTexts {
		stats.OriginalChars += len(text)
		lines := strings.Split(text, "\n")
		var kept []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			normalized := normalizeForBoilerplate(trimmed)
			if !boilerplate[normalized] {
				kept = append(kept, line)
			}
		}
		result := strings.Join(kept, "\n")
		cleaned[pageNum] = result
		stats.ProcessedChars += len(result)
	}

	stats.CharsRemoved = stats.OriginalChars - stats.ProcessedChars
	return cleaned, stats
}

// DeduplicateChunks removes chunks that are near-duplicates of earlier chunks.
// Uses Jaccard similarity on word sets.
func (p *Preprocessor) DeduplicateChunks(chunks []*proto.Chunk) ([]*proto.Chunk, int) {
	if len(chunks) <= 1 {
		return chunks, 0
	}

	// Pre-compute word sets for all chunks
	wordSets := make([]map[string]bool, len(chunks))
	for i, chunk := range chunks {
		wordSets[i] = toWordSet(chunk.Text)
	}

	var result []*proto.Chunk
	removed := 0

	for i, chunk := range chunks {
		isDupe := false
		for j := 0; j < len(result); j++ {
			resultIdx := -1
			// Find the original index of result[j] to get its word set
			for k, orig := range chunks {
				if orig.Id == result[j].Id {
					resultIdx = k
					break
				}
			}
			if resultIdx >= 0 {
				sim := jaccardSimilarity(wordSets[i], wordSets[resultIdx])
				if sim >= p.dedupeThreshold {
					isDupe = true
					break
				}
			}
		}
		if !isDupe {
			result = append(result, chunk)
		} else {
			removed++
		}
	}

	return result, removed
}

// ProcessChunks applies both boilerplate stripping and deduplication to chunks.
// This is the main entry point for the preprocessing pipeline.
func (p *Preprocessor) ProcessChunks(chunks []*proto.Chunk, pageTexts map[int32]string) ([]*proto.Chunk, *PreprocessStats) {
	stats := &PreprocessStats{
		OriginalChunks: len(chunks),
	}

	// Step 1: Strip boilerplate from page texts
	_, boilerplateStats := p.StripBoilerplate(pageTexts)
	stats.BoilerplateLines = boilerplateStats.BoilerplateLines
	stats.OriginalChars = boilerplateStats.OriginalChars
	stats.CharsRemoved = boilerplateStats.CharsRemoved

	// Step 2: Deduplicate chunks
	deduped, removedCount := p.DeduplicateChunks(chunks)
	stats.AfterDedup = len(deduped)

	_ = removedCount

	return deduped, stats
}

// normalizeForBoilerplate normalizes a line for boilerplate comparison.
// Lowercases and removes digits to match patterns like "Page 1", "Page 2".
func normalizeForBoilerplate(s string) string {
	s = strings.ToLower(s)
	var builder strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			continue // Strip digits
		}
		builder.WriteRune(r)
	}
	return strings.TrimSpace(builder.String())
}

// toWordSet converts text to a set of unique lowercase words
func toWordSet(text string) map[string]bool {
	set := make(map[string]bool)
	for _, word := range strings.Fields(strings.ToLower(text)) {
		word = strings.Trim(word, ".,;:!?\"'()[]{}—–-/\\")
		if len(word) > 1 {
			set[word] = true
		}
	}
	return set
}

// jaccardSimilarity computes Jaccard similarity: |A ∩ B| / |A ∪ B|
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	intersection := 0
	for word := range a {
		if b[word] {
			intersection++
		}
	}

	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}
