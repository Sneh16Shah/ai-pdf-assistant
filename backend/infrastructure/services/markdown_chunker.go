package services

import (
	"regexp"
	"strings"

	"ai-pdf-assistant-backend/proto"

	"github.com/google/uuid"
)

const (
	// maxSectionSize caps a single chunk's length in runes. Sections longer
	// than this are split further (the section heading is preserved on each
	// continuation chunk so retrieval still has context).
	maxSectionSize = 2000
	// minHeadingLevel: headings at or below this level (#, ##, ###) start a new chunk.
	// Deeper headings (####+) do not split — they're kept with their parent section.
	minHeadingLevel = 3
)

// headingLineRe matches ATX markdown headings (# .. ######) at the start of a line.
var headingLineRe = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+?)\s*$`)

// MarkdownChunker splits Markdown text into semantically complete chunks by
// section boundary (# / ## / ### headings), rather than the fixed character
// split used by the basic parser. This preserves structural units (a whole
// section, table, or subsection) per chunk, which improves both BM25 and
// embedding retrieval quality.
type MarkdownChunker struct{}

// NewMarkdownChunker returns a stateless chunker.
func NewMarkdownChunker() *MarkdownChunker { return &MarkdownChunker{} }

// Chunk splits markdown into proto.Chunks. Page numbers are best-effort:
// MarkItDown's output does not reliably mark page boundaries, so chunks are
// tagged with page 1 by default and callers that care about citations should
// re-attribute via the basic parser's page map if available.
func (c *MarkdownChunker) Chunk(markdown string) []*proto.Chunk {
	if strings.TrimSpace(markdown) == "" {
		return nil
	}

	sections := splitByHeadings(markdown)

	chunks := make([]*proto.Chunk, 0, len(sections))
	for idx, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		// Further split very long sections; preserve the heading on each piece.
		for _, piece := range splitLongSection(section, maxSectionSize) {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}
			chunks = append(chunks, &proto.Chunk{
				Id:         uuid.New().String(),
				Text:       piece,
				ChunkIndex: int32(len(chunks)),
				PageNumber: 1, // best-effort; see doc comment
			})
		}
		_ = idx
	}

	return chunks
}

// BuildOutline produces a lightweight table-of-contents from headings (#/##/###).
// Each line is "  " (per level) + heading text, truncated.
func (c *MarkdownChunker) BuildOutline(markdown string) string {
	var b strings.Builder
	matches := headingLineRe.FindAllStringSubmatch(markdown, -1)
	for _, m := range matches {
		level := len(m[1])
		if level > minHeadingLevel {
			continue
		}
		title := strings.TrimSpace(m[2])
		if title == "" {
			continue
		}
		if len(title) > 100 {
			title = title[:100] + "..."
		}
		b.WriteString(strings.Repeat("  ", level-1))
		b.WriteString(title)
		b.WriteString("\n")
	}
	return b.String()
}

// splitByHeadings splits markdown into sections, where each section begins at a
// heading of level <= minHeadingLevel and includes everything until the next
// such heading (or end of document). The heading line itself is preserved at
// the start of its section.
func splitByHeadings(markdown string) []string {
	lines := strings.Split(markdown, "\n")

	var sections []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			sections = append(sections, current.String())
			current.Reset()
		}
	}

	for _, line := range lines {
		if m := headingLineRe.FindStringSubmatch(line); m != nil {
			if len(m[1]) <= minHeadingLevel {
				// New top-level section — flush the previous one.
				flush()
			}
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	flush()

	// If the document had no leading heading, the first section is preamble
	// (title page, abstract, etc.). That's fine — keep it as its own chunk.
	if len(sections) == 0 {
		return []string{markdown}
	}
	return sections
}

// splitLongSection breaks a section that exceeds maxLen into multiple pieces.
// It splits on blank lines (paragraph boundaries) when possible, falling back
// to a hard rune cut. The section heading (first line if it's a heading) is
// prepended to each continuation piece so each chunk stands alone.
func splitLongSection(section string, maxLen int) []string {
	if len(section) <= maxLen {
		return []string{section}
	}

	heading := ""
	body := section
	if strings.HasPrefix(strings.TrimSpace(section), "#") {
		if nl := strings.IndexByte(section, '\n'); nl >= 0 {
			heading = section[:nl+1]
			body = section[nl+1:]
		}
	}

	paragraphs := strings.Split(body, "\n\n")
	var pieces []string
	var current strings.Builder
	if heading != "" {
		current.WriteString(heading)
	}

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// If adding this paragraph would overflow, flush current and start fresh.
		if current.Len()+len(p)+2 > maxLen && current.Len() > len(heading) {
			pieces = append(pieces, current.String())
			current.Reset()
			if heading != "" {
				current.WriteString(heading)
			}
		}
		if current.Len() > len(heading) {
			current.WriteString("\n\n")
		}
		current.WriteString(p)
	}
	if current.Len() > len(heading) {
		pieces = append(pieces, current.String())
	}

	// Hard-cut any piece still over the limit (single giant paragraph, table, etc.)
	var finalized []string
	for _, piece := range pieces {
		if len(piece) <= maxLen {
			finalized = append(finalized, piece)
			continue
		}
		for start := 0; start < len(piece); start += maxLen {
			end := start + maxLen
			if end > len(piece) {
				end = len(piece)
			}
			finalized = append(finalized, piece[start:end])
		}
	}
	return finalized
}
