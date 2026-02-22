# AskMyPDF

A full-stack AI-powered PDF assistant that lets users upload documents, ask natural language questions, and receive context-aware answers with page citations. Built with a Go backend, React frontend, PostgreSQL persistence, and a Chrome Extension for in-browser PDF chat — containerized with Docker and deployed on Render.

**Key Features:** Multi-document sessions, streaming AI responses with source citations, session history & resume, JWT authentication, Chrome Side Panel extension, dark mode, and one-command Docker deployment.

---

## 🚀 Quick Start (Docker)

```bash
# 1. Clone the repository
git clone https://github.com/Sneh16Shah/ai-pdf-assistant.git
cd ai-pdf-assistant

# 2. Create .env file in the project root
cp .env.example .env
# Edit .env and add your keys:
#   POSTGRES_PASSWORD=your_secure_password
#   GROQ_API_KEY=your_groq_api_key        (free at https://console.groq.com/keys)
#   JWT_SECRET=any_long_random_string

# 3. Start the entire stack
docker-compose up -d --build
```

| Service       | URL                                     |
|---------------|-----------------------------------------|
| Frontend      | http://localhost:3001                    |
| Backend API   | http://localhost:8081/api/v1             |
| Health Check  | http://localhost:8081/api/v1/health      |
| PostgreSQL    | localhost:5432 (user: `askmypdf`)        |

---

## 🛠 Docker Commands

```bash
# Start all services
docker-compose up -d --build

# Stop all services
docker-compose down

# View backend logs
docker logs askmypdf-backend -f

# View frontend logs
docker logs askmypdf-frontend -f

# Rebuild only the backend
docker-compose up -d --build backend

# Force full rebuild (no cache)
docker-compose up -d --build --force-recreate
```

---

## 🗄 Database (PostgreSQL via Docker)

```bash
# Connect to the local PostgreSQL database
docker exec -it askmypdf-postgres psql -U askmypdf -d askmypdf

# Once connected, useful queries:

# List all tables
\dt

# View all registered users
SELECT id, email, name, created_at FROM users;

# View all chat sessions
SELECT id, title, created_at, last_activity FROM sessions ORDER BY last_activity DESC;

# View documents uploaded per session
SELECT d.filename, d.pages, s.title FROM documents d JOIN sessions s ON d.session_id = s.id;

# View chat history for a session
SELECT role, LEFT(content, 80) AS preview, created_at FROM chat_messages WHERE session_id = '<SESSION_ID>' ORDER BY created_at;

# Exit psql
\q
```

---

## 📡 API Endpoints

| Method | Endpoint                              | Description                 |
|--------|---------------------------------------|-----------------------------|
| POST   | `/api/v1/auth/register`               | Register a new user         |
| POST   | `/api/v1/auth/login`                  | Login and get JWT token     |
| POST   | `/api/v1/chat/upload`                 | Upload a PDF to a session   |
| POST   | `/api/v1/chat/query`                  | Ask a question (JSON)       |
| POST   | `/api/v1/chat/stream`                 | Ask a question (SSE stream) |
| GET    | `/api/v1/chat/sessions`               | List all user sessions      |
| GET    | `/api/v1/chat/sessions/:id/documents` | Get documents in a session  |
| GET    | `/api/v1/chat/sessions/:id/messages`  | Get chat history            |
| DELETE | `/api/v1/chat/sessions/:id`           | Delete a session            |
| GET    | `/api/v1/health`                      | Health check                |

---

## 🧩 Chrome Extension

The project includes a Chrome Side Panel extension for chatting with any PDF open in your browser.

**Load the extension locally:**
1. Open `chrome://extensions/`
2. Enable **Developer mode** (top right)
3. Click **Load unpacked** → select the `src/` folder
4. Open any PDF in Chrome → click the extension icon to open the side panel

The extension auto-detects whether you are running locally (unpacked/dev mode → `localhost`) or in production (installed from store → Render URLs).

---

## 🏗 Architecture

```
backend/                          # Go (Gin) REST API
├── handlers/                     # HTTP request handlers
├── usecases/                     # Business logic layer
├── infrastructure/
│   ├── repositories/             # PostgreSQL data access
│   └── services/                 # AI (Groq, Pollinations), PDF processing
├── database/                     # DB connection + schema.sql
└── Dockerfile

frontend/                         # React + TypeScript + Vite
├── src/
│   ├── components/               # UI components (PDFViewer, Dashboard, etc.)
│   ├── contexts/                 # Auth & Theme providers
│   └── services/                 # Axios API client
└── Dockerfile

src/                              # Chrome Extension (Manifest V3)
├── background.js                 # Service worker + auth
├── chat-interface.js/html/css    # Side panel UI
└── manifest.json
```

---

## ☁️ Production Deployment (Render)

The app is deployed on [Render](https://render.com) as three separate services:

| Service    | Type         | URL                                                        |
|------------|--------------|------------------------------------------------------------|
| Frontend   | Static Site  | https://ai-pdf-assistant-z6lh.onrender.com                 |
| Backend    | Web Service  | https://ai-pdf-assistant-backend-x8jo.onrender.com         |
| Database   | PostgreSQL   | Render Managed PostgreSQL                                  |

**Required environment variables on Render (Backend):**
- `DATABASE_URL` — Internal Database URL from Render PostgreSQL
- `GROQ_API_KEY` — Free API key from [Groq](https://console.groq.com/keys)
- `JWT_SECRET` — Any long random string
- `ALLOWED_ORIGINS` — Frontend URL (e.g. `https://ai-pdf-assistant-z6lh.onrender.com`)

**Required environment variables on Render (Frontend):**
- `VITE_API_URL` — Backend URL (e.g. `https://ai-pdf-assistant-backend-x8jo.onrender.com/api/v1`)
