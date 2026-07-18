package services

// Embedder is the interface that both Gemini EmbeddingService and
// NvidiaEmbeddingService implement. This allows HybridSearch and
// PDFUseCase to work with either embedding provider.
type Embedder interface {
	EmbedText(text string) ([]float32, error)
	EmbedBatch(texts []string) ([][]float32, error)
}

// Reranker is the interface that both Gemini RerankerService and
// NvidiaRerankerService implement. This allows EnterpriseContextBuilder
// to work with either reranking provider.
type Reranker interface {
	Rerank(query string, candidates []*HybridResult, topK int) ([]*RerankResult, error)
}
