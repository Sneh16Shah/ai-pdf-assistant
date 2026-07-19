package usecases

import "ai-pdf-assistant-backend/proto"

// preferMarkdown returns the markdown representation of a document when the
// MarkItDown sidecar populated it, otherwise the basic text/chunks/outline.
//
// Smart and Enterprise pipelines use this so they retrieve over the richer
// structured Markdown when available (better BM25 + embedding + reranking
// quality). The Standard pipeline does NOT use this — it always reads the
// basic chunks via doc.Chunks directly.
func preferMarkdown(doc *proto.Document) (chunks []*proto.Chunk, text, outline string) {
	if doc == nil {
		return nil, "", ""
	}
	if len(doc.MarkdownChunks) > 0 {
		return doc.MarkdownChunks, doc.MarkdownText, doc.MarkdownOutline
	}
	return doc.Chunks, doc.Text, doc.Outline
}
