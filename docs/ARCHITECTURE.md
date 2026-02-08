# System Architecture
## AskMyPDF - AI-Powered PDF & FAQ Chatbot Platform

**Version:** 1.0  
**Date:** 2024

---

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         React Frontend (Vite + Tailwind)             │   │
│  │  - PDF Upload UI                                      │   │
│  │  - Chat Interface                                     │   │
│  │  - Summary Display                                    │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            │ HTTP/REST
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      API GATEWAY LAYER                       │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         REST API Handlers (Gin Framework)            │   │
│  │  - JSON Request/Response                              │   │
│  │  - JSON ↔ Protobuf Conversion                        │   │
│  │  - CORS, Validation, Error Handling                  │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            │ Protobuf Models
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    APPLICATION LAYER                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ PDF UseCase  │  │ Chat UseCase │  │Summary UseCase│      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│         │                 │                   │              │
│         └─────────────────┴───────────────────┘             │
└─────────────────────────────────────────────────────────────┘
                            │
                            │ Interfaces
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      DOMAIN LAYER                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Protobuf Domain Models                   │   │
│  │  - Document, Chunk, Session, Message, etc.          │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    INFRASTRUCTURE LAYER                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ PDF Service  │  │ AI Provider  │  │  Storage     │      │
│  │ (Parser)     │  │ (Puter/Mock) │  │ (In-Memory)  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         Vector Search (In-Memory Similarity)          │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## 2. Clean Architecture Layers

### 2.1 Presentation Layer (Handlers)
**Location**: `backend/handlers/`

**Responsibilities**:
- HTTP request/response handling
- JSON serialization/deserialization
- JSON ↔ Protobuf conversion
- Input validation
- Error response formatting

**Components**:
- `pdf_handler.go` - PDF upload endpoints
- `chat_handler.go` - Chat endpoints
- `summary_handler.go` - Summary endpoints

### 2.2 Application Layer (Use Cases)
**Location**: `backend/usecases/`

**Responsibilities**:
- Business logic orchestration
- Coordinate between repositories and services
- Work with Protobuf models only
- Transaction management (if needed)

**Components**:
- `pdf_usecase.go` - PDF processing logic
- `chat_usecase.go` - Chat conversation logic
- `summary_usecase.go` - Summary generation logic

### 2.3 Domain Layer (Models)
**Location**: `backend/proto/`

**Responsibilities**:
- Core business entities (Protobuf)
- Domain rules and validations
- No dependencies on external frameworks

**Protobuf Models**:
- `document.proto` - PDF document structure
- `chat.proto` - Chat messages and sessions
- `common.proto` - Common types

### 2.4 Infrastructure Layer (Repositories & Services)
**Location**: `backend/infrastructure/`

**Responsibilities**:
- External service integrations
- Data persistence (in-memory)
- PDF parsing
- AI provider abstraction
- Vector search implementation

**Components**:
- `repositories/` - Data access
- `services/pdf_service.go` - PDF parsing
- `services/ai_service.go` - AI provider interface
- `services/puter_ai.go` - Puter AI implementation
- `services/mock_ai.go` - Mock AI fallback
- `services/vector_search.go` - Similarity search

## 3. Request Flow

### 3.1 PDF Upload Flow

```
1. User uploads PDF via React UI
   ↓
2. Frontend sends multipart/form-data to POST /api/v1/pdf/upload
   ↓
3. Handler receives request, validates file
   ↓
4. Handler calls PDF UseCase.UploadPDF()
   ↓
5. UseCase:
   - Calls PDF Service to parse PDF
   - Calls Chunking Service to create chunks
   - Calls Repository to store document
   - Creates session
   ↓
6. UseCase returns Protobuf Document
   ↓
7. Handler converts Protobuf → JSON
   ↓
8. Handler returns JSON response to frontend
```

### 3.2 Chat Question Flow

```
1. User types question in chat UI
   ↓
2. Frontend sends POST /api/v1/chat/message
   {
     "session_id": "...",
     "message": "What is the main topic?"
   }
   ↓
3. Handler receives request, validates
   ↓
4. Handler calls Chat UseCase.AskQuestion()
   ↓
5. UseCase:
   - Retrieves session from Repository
   - Retrieves document chunks
   - Calls Vector Search to find relevant chunks
   - Builds context from top N chunks
   - Calls AI Service with context + question
   - Validates AI response (document-only check)
   - Stores message in session
   ↓
6. UseCase returns Protobuf ChatResponse
   ↓
7. Handler converts Protobuf → JSON
   ↓
8. Handler returns JSON response
   {
     "response": "Based on the document...",
     "session_id": "..."
   }
```

### 3.3 Summary Flow

```
1. User clicks "Generate Summary" button
   ↓
2. Frontend sends POST /api/v1/pdf/summary
   {
     "session_id": "..."
   }
   ↓
3. Handler receives request
   ↓
4. Handler calls Summary UseCase.GenerateSummary()
   ↓
5. UseCase:
   - Retrieves document from Repository
   - Calls AI Service with summary prompt
   - Formats response as bullet points
   ↓
6. UseCase returns Protobuf Summary
   ↓
7. Handler converts Protobuf → JSON
   ↓
8. Handler returns JSON response
```

## 4. Data Flow

### 4.1 Document Storage

```
PDF File
  ↓
PDF Service (Extract Text)
  ↓
Chunking Service (Split into chunks)
  ↓
Vector Search (Generate embeddings - optional/fake)
  ↓
In-Memory Repository
  {
    document_id: "...",
    chunks: [...],
    metadata: {...}
  }
```

### 4.2 Chat Context Building

```
User Question
  ↓
Vector Search (Find relevant chunks)
  ↓
Top 3-5 Chunks Selected
  ↓
Context Assembly:
  "Document Context:
   [Chunk 1]
   [Chunk 2]
   [Chunk 3]
   
   Question: {user_question}"
  ↓
AI Service (Puter AI or Mock)
  ↓
Response Validation
  ↓
Return to User
```

## 5. Component Interactions

### 5.1 PDF Processing Components

```
PDFHandler
    │
    ├─→ PDFUseCase
    │      │
    │      ├─→ PDFService (Parse PDF)
    │      ├─→ ChunkingService (Split text)
    │      └─→ DocumentRepository (Store)
    │
    └─→ Response (JSON)
```

### 5.2 Chat Components

```
ChatHandler
    │
    ├─→ ChatUseCase
    │      │
    │      ├─→ SessionRepository (Get session)
    │      ├─→ VectorSearch (Find relevant chunks)
    │      ├─→ AIService (Get answer)
    │      └─→ SessionRepository (Store message)
    │
    └─→ Response (JSON)
```

### 5.3 AI Service Abstraction

```
AIService Interface
    │
    ├─→ PuterAIService (Primary)
    │      └─→ HTTP Client → puter.ai API
    │
    └─→ MockAIService (Fallback)
           └─→ Returns mock responses
```

## 6. Technology Stack

### Backend
- **Language**: Go 1.21+
- **Framework**: Gin (HTTP router)
- **Protocol**: Protobuf 3
- **PDF Parsing**: github.com/ledongthuc/pdf
- **AI Provider**: Puter AI (HTTP client)
- **Storage**: In-memory (map-based)

### Frontend
- **Framework**: React 18+
- **Build Tool**: Vite
- **Styling**: Tailwind CSS
- **HTTP Client**: Fetch API / Axios
- **State Management**: React Hooks (useState, useEffect)

### Infrastructure
- **Containerization**: Docker
- **Orchestration**: docker-compose
- **Ports**: 
  - Backend: 8080
  - Frontend: 3000 (dev) / 80 (prod)

## 7. API Endpoints

### PDF Endpoints
- `POST /api/v1/pdf/upload` - Upload and process PDF
- `GET /api/v1/pdf/status/:id` - Get document status
- `POST /api/v1/pdf/summary` - Generate summary

### Chat Endpoints
- `POST /api/v1/chat/message` - Send chat message
- `GET /api/v1/chat/history/:sessionId` - Get chat history
- `DELETE /api/v1/chat/session/:sessionId` - Clear session

### Health
- `GET /api/v1/health` - Health check

## 8. Error Handling Strategy

### Error Types
1. **Validation Errors** (400) - Invalid input
2. **Not Found Errors** (404) - Resource not found
3. **Processing Errors** (500) - Internal server errors
4. **AI Service Errors** (503) - AI provider unavailable

### Error Response Format
```json
{
  "error": "Error message",
  "code": "ERROR_CODE",
  "details": {}
}
```

## 9. Security Considerations (MVP)

### Current (MVP)
- CORS enabled for frontend origin
- File size limits (50MB)
- Session-based isolation
- No authentication (MVP)

### Future Enhancements
- Rate limiting
- Input sanitization
- File type validation
- Size limits per user

## 10. Scalability Considerations

### Current (MVP)
- In-memory storage (single instance)
- Session-based (no persistence)
- Single AI provider

### Future Enhancements
- Redis for session storage
- Distributed vector database
- Multiple AI provider support
- Load balancing
- Caching layer

## 11. Deployment Architecture

```
┌─────────────────────────────────────────┐
│         Docker Compose                   │
│                                         │
│  ┌──────────────┐  ┌──────────────┐    │
│  │  Backend     │  │  Frontend    │    │
│  │  (Go)        │  │  (React)     │    │
│  │  :8080       │  │  :80         │    │
│  └──────────────┘  └──────────────┘    │
│                                         │
└─────────────────────────────────────────┘
```

### Single Command Startup
```bash
docker-compose up
```

---

**Document Owner**: Development Team  
**Last Updated**: 2024

