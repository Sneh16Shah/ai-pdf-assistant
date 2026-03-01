# 🚀 Upcoming Features — AI PDF Assistant

> A comprehensive roadmap of features to handle larger PDFs with fewer tokens.  
> Sorted by **free first**, then paid. Each feature has implementation notes for future reference.

---

## ✅ Already Implemented

| Feature | Status | Technique |
|:--------|:-------|:----------|
| BM25 Search | ✅ Done | Mathematical ranking (Okapi BM25) replaces basic keyword matching |
| TextRank Summarization | ✅ Done | Graph-based extractive summarization (PageRank for sentences) |
| Boilerplate Stripping | ✅ Done | Auto-detect repeated headers/footers across pages |
| Chunk Deduplication | ✅ Done | Jaccard similarity to remove near-duplicate chunks |
| Smart Mode Toggle | ✅ Done | Side-by-side comparison with token stats |

---

## 🆓 FREE Features (Zero Cost — Pure Algorithms)

### 1. Semantic Chunking
**Impact: 🔥🔥🔥 High** | **Difficulty: Easy**

Currently chunks are split at fixed 1500 chars. Instead, split at paragraph/section boundaries so each chunk is a complete thought.

**How to implement:**
- New file: `infrastructure/services/semantic_chunker.go`
- Detect paragraph breaks (`\n\n`), heading patterns (ALL CAPS, numbered sections), and bullet lists
- Each chunk becomes a semantically complete unit → BM25 ranking becomes much more accurate
- No API calls needed — pure string analysis

---

### 2. Chat History Compression
**Impact: 🔥🔥🔥 High** | **Difficulty: Medium**

Long conversations eat tokens. After 5+ turns, summarize older turns into a compact digest instead of sending full history.

**How to implement:**
- Use TextRank (already built!) to extract key sentences from old conversation turns
- Keep last 3 turns verbatim, compress earlier turns into a ~200 word summary
- New file: `usecases/history_compressor.go`
- Integrate into `smart_chat_usecase.go`

---

### 3. Query Expansion / Rewriting
**Impact: 🔥🔥 Medium** | **Difficulty: Easy**

Vague questions like "explain this" miss relevant chunks. Auto-expand using the current page context.

**How to implement:**
- Before BM25 search, prepend key terms from the current page to the query
- Example: "explain this" on page about databases → "explain this database replication master slave page 5"
- Pure string manipulation — extract top 5 TF-IDF terms from current page and append to query
- New file: `infrastructure/services/query_expander.go`

---

### 4. Hierarchical Document Index
**Impact: 🔥🔥🔥 High** | **Difficulty: Medium**

On upload, build a 3-level hierarchy: Document → Chapters → Sections. Search the hierarchy top-down instead of scanning all chunks.

**How to implement:**
- Detect chapter/section headings using regex patterns (numbered headings, ALL CAPS, font-size changes)
- Build a tree structure with summaries at each level (using TextRank)
- Search starts at chapter level, then drills into relevant sections only
- New files: `infrastructure/services/doc_hierarchy.go`, `proto/hierarchy.go`

---

### 5. Answer Confidence Scoring
**Impact: 🔥🔥 Medium** | **Difficulty: Easy**

Show users how confident the system is that the answer is correct, based on BM25 match quality.

**How to implement:**
- After BM25 retrieval, compute average BM25 score of top-K chunks
- Map to a 0-100% confidence scale
- Show in UI next to the response: "Confidence: 87%"
- If confidence < 30%, warn user: "The document may not contain information about this topic"
- Minimal changes: add field to `SmartChatResponse`, small UI badge in `ExplanationPanel.tsx`

---

### 6. Response Caching (Semantic)
**Impact: 🔥🔥 Medium** | **Difficulty: Medium**

Cache AI responses for similar questions. If someone asks "what is database replication?" twice, return the cached answer instantly — zero tokens.

**How to implement:**
- Store `(query_hash, context_hash) → response` in memory map
- Use BM25 similarity between new query and cached queries (threshold > 0.85)
- New file: `infrastructure/services/response_cache.go`
- Add TTL-based expiry (e.g., cache for 1 hour)

---

### 7. Streaming Responses (SSE)
**Impact: 🔥🔥🔥 High** | **Difficulty: Medium**

Show AI text word-by-word as it generates (like ChatGPT). The `/chat/stream` endpoint stub already exists.

**How to implement:**
- Backend: Implement SSE (Server-Sent Events) in `smart_chat_handler.go` using Gemini's streaming API
- Frontend: Use `EventSource` or `fetch` with `ReadableStream` to consume the stream
- No extra token cost — same tokens, just delivered incrementally
- Dramatically improves perceived speed

---

### 8. Table / Structured Data Extraction
**Impact: 🔥🔥 Medium** | **Difficulty: Medium**

PDFs often contain tables that get mangled during text extraction. Detect and preserve table structure.

**How to implement:**
- Detect table patterns in extracted text (aligned columns, repeated delimiters, grid-like whitespace)
- Convert to markdown table format before chunking
- New file: `infrastructure/services/table_extractor.go`
- Tables become searchable and AI can reason about them properly

---

### 9. "Lost in the Middle" Fix
**Impact: 🔥🔥 Medium** | **Difficulty: Easy**

Research shows LLMs perform worse on information in the middle of long contexts. Place the most relevant chunks at the **beginning and end** of the context.

**How to implement:**
- In `smart_context_builder.go`, after BM25 ranking, reorder chunks:
  - Top 3 most relevant → beginning
  - Next 3 most relevant → end
  - Rest → middle
- Pure reordering — zero cost, significant quality improvement

---

### 10. Multi-Document Cross-Reference
**Impact: 🔥🔥 Medium** | **Difficulty: Medium**

When multiple PDFs are uploaded, enable questions that span documents: "Compare chapter 3 of PDF A with section 2 of PDF B."

**How to implement:**
- BM25 already indexes across all session documents
- Add document source tracking to citations
- Enhance prompt to mention which document each chunk comes from
- UI: Show document name in citation badges

---

## 💰 PAID Features (Require API Keys, Models, or Infrastructure)

### 11. Local Embedding Search (all-MiniLM-L6-v2)
**Cost: Server GPU/CPU** | **Impact: 🔥🔥🔥 High** | **Difficulty: Hard**

True semantic search — understands "automobile" matches "car". Runs locally but needs compute.

**How to implement:**
- Run `all-MiniLM-L6-v2` via ONNX runtime in Go or as a Python sidecar
- Generate 384-dim embeddings for each chunk on upload
- Store in vector DB (pgvector in existing Postgres)
- Combine with BM25 for hybrid search (best of both worlds)
- **Cost:** ~$0 for CPU inference (slow), ~$5-20/mo for a GPU instance

---

### 12. Re-Ranking with Cross-Encoder
**Cost: Server GPU** | **Impact: 🔥🔥🔥 High** | **Difficulty: Medium**

After BM25 retrieves top-50 chunks, a cross-encoder re-ranks them by truly understanding query-chunk relevance.

**How to implement:**
- Use `cross-encoder/ms-marco-MiniLM-L-6-v2` (open source)
- Run as Python sidecar service
- BM25 → top 50 → cross-encoder re-rank → top 10 → send to LLM
- **Cost:** CPU ~$0 (slow), GPU ~$5-20/mo for fast inference

---

### 13. Gemini Context Caching
**Cost: ~50% of normal token cost** | **Impact: 🔥🔥🔥 High** | **Difficulty: Easy**

Gemini API supports context caching — upload the document once, then all questions reuse the cached context at half price.

**How to implement:**
- Use Gemini's `cachedContent` API on first question per session
- Store cache ID, reuse for subsequent questions
- **Cost:** Charged at ~50% of input token price, but saves on repeated uploads
- Supported on: Gemini 1.5 Pro, Gemini 1.5 Flash

---

### 14. OCR for Scanned PDFs
**Cost: Free (Tesseract) to Paid (Cloud Vision)** | **Impact: 🔥🔥 Medium** | **Difficulty: Medium**

Currently only works with text-based PDFs. Scanned PDFs (images of text) produce empty text.

**How to implement:**
- **Free:** Integrate Tesseract OCR (open source) — add `gosseract` Go binding
- **Paid:** Use Google Cloud Vision API or AWS Textract for higher accuracy
- Run OCR on pages with no/minimal extracted text
- New file: `infrastructure/services/ocr_service.go`

---

### 15. Abstractive Summarization (LLM-Powered)
**Cost: API tokens** | **Impact: 🔥🔥 Medium** | **Difficulty: Easy**

TextRank is extractive (picks exact sentences). Abstractive summarization generates new, cleaner summaries.

**How to implement:**
- After TextRank extracts key sentences, pass them to a smaller LLM (Gemini Flash) for rewriting
- Produces more readable summaries but costs tokens
- Use tiered approach: TextRank for free, LLM summary as upgrade
- **Cost:** ~500-1000 tokens per page summarized

---

### 16. Knowledge Graph (GraphRAG)
**Cost: Neo4j or Postgres + LLM extraction** | **Impact: 🔥🔥🔥 High** | **Difficulty: Hard**

Build a knowledge graph of entities and relationships from the document. Enables complex reasoning queries.

**How to implement:**
- Extract entities (people, orgs, concepts) and relationships using LLM
- Store as graph in Neo4j or Postgres with ltree
- Query the graph for multi-hop reasoning: "Who reported to the CEO and worked on Project X?"
- **Cost:** LLM extraction tokens + graph DB hosting

---

### 17. Fine-Tuned Small Model
**Cost: Training compute + hosting** | **Impact: 🔥🔥🔥 High** | **Difficulty: Hard**

Train a small model (like Gemma 2B or Phi-3 Mini) specifically on your document types for faster, cheaper inference.

**How to implement:**
- Collect Q&A pairs from user interactions (with consent)
- Fine-tune using LoRA on a small model
- Deploy as a sidecar for simple questions, route complex ones to Gemini
- **Cost:** ~$10-50 for training, ~$20-50/mo for hosting

---

### 18. Multi-Modal PDF Analysis
**Cost: Vision API tokens** | **Impact: 🔥🔥 Medium** | **Difficulty: Medium**

Analyze charts, diagrams, and images embedded in PDFs — not just text.

**How to implement:**
- Render each PDF page as an image
- Send to Gemini Vision or Llama 4 Scout for visual Q&A
- Already partially implemented via `vision_service.go`
- **Cost:** Per-image token cost (~258 tokens per image on Gemini)

---

## 📊 Priority Matrix

```
                    HIGH IMPACT
                        │
    Semantic Chunking   │   Local Embeddings
    History Compression │   Cross-Encoder Re-rank
    Streaming (SSE)     │   GraphRAG
    Hierarchical Index  │   Fine-Tuned Model
                        │   Context Caching
────────────────────────┼────────────────────────
    Query Expansion     │   OCR
    Confidence Scoring  │   Abstractive Summary
    Lost-in-Middle Fix  │   Multi-Modal
    Response Caching    │
    Cross-Reference     │
    Table Extraction    │
                        │
                    LOW IMPACT
    
    FREE ←──────────────┼──────────────→ PAID
```

---

*Last updated: 2026-03-01*
