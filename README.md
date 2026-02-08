# AskMyPDF

AI-powered PDF chat application. Upload PDFs and ask questions about their content.

## Quick Start

```bash
docker-compose up --build
```

- **Frontend**: http://localhost:3001
- **Backend API**: http://localhost:8081/api/v1
- **Health Check**: http://localhost:8081/api/v1/health

## Using an AI Provider (Optional)

By default, the app uses mock AI responses. To enable real AI:

```bash
# Create .env in project root
echo "GROQ_API_KEY=your_groq_api_key_here" > .env

# Get a free key at https://console.groq.com/keys
docker-compose up --build
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/pdf/upload` | Upload PDF |
| GET | `/api/v1/pdf/status/:id` | Get document status |
| POST | `/api/v1/chat/message` | Send chat message |
| GET | `/api/v1/chat/history/:sessionId` | Get chat history |
| DELETE | `/api/v1/chat/session/:sessionId` | Clear session |
| POST | `/api/v1/pdf/summary` | Generate summary |
| GET | `/api/v1/health` | Health check |

## Local Development

### Backend
```bash
cd backend
go mod download
go run main.go
```

### Frontend
```bash
cd frontend
npm install
npm run dev
```

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

```
backend/
├── handlers/       # HTTP handlers
├── usecases/       # Business logic
├── infrastructure/ # Services & repositories
└── proto/          # Domain models (Protobuf)

frontend/
├── src/components/ # React components
└── src/services/   # API client
```
