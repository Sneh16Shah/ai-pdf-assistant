package services

import (
	"math"
	"sort"
	"strings"
)

// TextRank implements extractive summarization using graph-based sentence ranking.
// Inspired by Google's PageRank: sentences that are similar to many other important
// sentences get ranked higher. Zero API calls — pure computation.
type TextRank struct {
	damping    float64 // PageRank damping factor (standard: 0.85)
	iterations int     // Convergence iterations
	threshold  float64 // Convergence threshold
}

// NewTextRank creates a new TextRank summarizer
func NewTextRank() *TextRank {
	return &TextRank{
		damping:    0.85,
		iterations: 30,
		threshold:  0.0001,
	}
}

// Summarize extracts the N most important sentences from text.
// Returns sentences in their original order (preserves reading flow).
func (tr *TextRank) Summarize(text string, numSentences int) string {
	sentences := splitSentences(text)
	if len(sentences) <= numSentences {
		return text
	}

	if numSentences <= 0 {
		numSentences = 3
	}

	// Build similarity matrix between all sentence pairs
	n := len(sentences)
	similarity := make([][]float64, n)
	for i := range similarity {
		similarity[i] = make([]float64, n)
	}

	// Tokenize all sentences
	sentTokens := make([]map[string]int, n)
	for i, s := range sentences {
		sentTokens[i] = tokenizeToMap(s)
	}

	// Compute cosine similarity between all pairs
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			sim := cosineSimilarity(sentTokens[i], sentTokens[j])
			similarity[i][j] = sim
			similarity[j][i] = sim
		}
	}

	// Run PageRank-style iterative scoring
	scores := make([]float64, n)
	for i := range scores {
		scores[i] = 1.0 / float64(n) // Initialize uniformly
	}

	for iter := 0; iter < tr.iterations; iter++ {
		newScores := make([]float64, n)
		maxDiff := 0.0

		for i := 0; i < n; i++ {
			sum := 0.0
			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				// Sum of outgoing weights from j
				outSum := 0.0
				for k := 0; k < n; k++ {
					if k != j {
						outSum += similarity[j][k]
					}
				}
				if outSum > 0 {
					sum += similarity[j][i] * scores[j] / outSum
				}
			}
			newScores[i] = (1 - tr.damping) + tr.damping*sum
			diff := math.Abs(newScores[i] - scores[i])
			if diff > maxDiff {
				maxDiff = diff
			}
		}

		scores = newScores
		if maxDiff < tr.threshold {
			break // Converged
		}
	}

	// Rank sentences by score
	type rankedSent struct {
		index    int
		score    float64
		sentence string
	}

	ranked := make([]rankedSent, n)
	for i := range ranked {
		ranked[i] = rankedSent{index: i, score: scores[i], sentence: sentences[i]}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	// Take top N, then sort by original position (preserves reading order)
	top := ranked[:numSentences]
	sort.Slice(top, func(i, j int) bool {
		return top[i].index < top[j].index
	})

	// Build summary
	var builder strings.Builder
	for i, s := range top {
		if i > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(s.sentence)
	}

	return builder.String()
}

// SummarizePages generates extractive summaries for each page.
// Returns a map of page number → summary text.
func (tr *TextRank) SummarizePages(pageTexts map[int32]string, sentencesPerPage int) map[int32]string {
	summaries := make(map[int32]string, len(pageTexts))
	for pageNum, text := range pageTexts {
		summaries[pageNum] = tr.Summarize(text, sentencesPerPage)
	}
	return summaries
}

// splitSentences splits text into sentences using simple heuristics
func splitSentences(text string) []string {
	// Split on sentence-ending punctuation followed by space or end
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)

		// Check for sentence boundary
		if (r == '.' || r == '!' || r == '?') &&
			(i == len(runes)-1 || (i+1 < len(runes) && (runes[i+1] == ' ' || runes[i+1] == '\n'))) {
			s := strings.TrimSpace(current.String())
			if len(s) > 10 { // Skip very short fragments
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}

	// Add remaining text if meaningful
	remaining := strings.TrimSpace(current.String())
	if len(remaining) > 10 {
		sentences = append(sentences, remaining)
	}

	return sentences
}

// tokenizeToMap converts text into a term frequency map
func tokenizeToMap(text string) map[string]int {
	tf := make(map[string]int)
	for _, word := range strings.Fields(strings.ToLower(text)) {
		word = strings.Trim(word, ".,;:!?\"'()[]{}—–-/\\")
		if len(word) > 1 {
			tf[word]++
		}
	}
	return tf
}

// cosineSimilarity computes cosine similarity between two term frequency maps
func cosineSimilarity(a, b map[string]int) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	dotProduct := 0.0
	normA := 0.0
	normB := 0.0

	for term, freqA := range a {
		fa := float64(freqA)
		normA += fa * fa
		if freqB, exists := b[term]; exists {
			dotProduct += fa * float64(freqB)
		}
	}

	for _, freqB := range b {
		fb := float64(freqB)
		normB += fb * fb
	}

	denominator := math.Sqrt(normA) * math.Sqrt(normB)
	if denominator == 0 {
		return 0
	}

	return dotProduct / denominator
}
