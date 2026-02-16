# Debug 500 Errors — Quick Reference

## 1. View Backend Logs (Most Common)

```bash
# Last 50 lines of backend logs
docker logs askmypdf-backend --tail 50

# Follow live logs
docker logs askmypdf-backend -f

# Filter for errors only
docker logs askmypdf-backend 2>&1 | findstr /I "ERROR panic FATAL 500 413"
```

## 2. Common 500 Causes & Fixes

| Error Pattern | Cause | Fix |
|---|---|---|
| `status 413` / `Request too large` | Context too big for Groq API | Reduce `MaxContextTokens` in `context_builder.go` |
| `rate_limit_exceeded` | Too many Groq requests | Wait 60s, or check billing at console.groq.com |
| `SESSION_NOT_FOUND` | Session expired or invalid | Re-upload PDF |
| `AI_SERVICE_ERROR` | Groq/Puter API down | Check API status, try again |
| `MESSAGE_STORAGE_ERROR` | DB write failed | Check postgres: `docker logs askmypdf-postgres` |

## 3. Check Container Health

```bash
# Status of all containers
docker compose ps

# Check if backend is responding
curl http://localhost:8081/api/v1/health

# Restart just the backend
docker compose restart backend
```

## 4. Database Logs

```bash
docker logs askmypdf-postgres --tail 30
```

## 5. Frontend Errors

```bash
docker logs askmypdf-frontend --tail 30
```

## 6. Full Debug Cycle

```bash
# Stop, rebuild, start with fresh logs
docker compose down
docker compose up --build 2>&1 | tee debug.log

# In another terminal, trigger the error, then search logs:
findstr /I "ERROR" debug.log
```

## 7. Test Chat Endpoint Directly

```bash
# Replace SESSION_ID with your actual session ID
curl -X POST http://localhost:8081/api/v1/chat/message ^
  -H "Content-Type: application/json" ^
  -d "{\"session_id\": \"SESSION_ID\", \"message\": \"hello\", \"page_number\": 1}"
```
