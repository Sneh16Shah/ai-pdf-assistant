# 🗄️ Database Setup Guide — ai-pdf-assistant

**Render gives you an empty database — you need to create all tables manually.**

---

## Step 1: Get Your Connection String from Render

1. Go to your [Render Dashboard](https://dashboard.render.com/)
2. Click on your database **ai-pdf-assistant-db**
3. Scroll down to the **Connections** section
4. Copy the **External Database URL** (for connecting from your local machine)
   - It looks like: `postgresql://user:password@host:5432/dbname`

> **Note:** The **Internal** URL is for services running on Render. The **External** URL is for your local terminal.

---

## Step 2: Install `psql` (PostgreSQL Client)

### Windows
1. Download from [postgresql.org/download/windows](https://www.postgresql.org/download/windows/)
2. During install, select **Command Line Tools** only
3. Add to PATH: `C:\Program Files\PostgreSQL\<version>\bin`

### Alternative
Use Render's built-in **Shell** tab on your database page to run SQL directly (no install needed).

---

## Step 3: Connect to the Database

```powershell
psql "YOUR_EXTERNAL_DATABASE_URL_HERE"
```

If `psql` isn't recognized, use the full path:
```powershell
& "C:\Program Files\PostgreSQL\17\bin\psql.exe" "YOUR_EXTERNAL_DATABASE_URL_HERE"
```

You'll see a prompt like: `ai_pdf_assistant_db=>`

---

## Step 4: Run the Schema

### Option A: Run schema file directly (recommended)

```powershell
psql "YOUR_EXTERNAL_DATABASE_URL" -f "backend/database/schema.sql"
```

This runs the entire `schema.sql` at once — creates all 4 tables, indexes, and triggers.

### Option B: Run queries manually inside psql

If you prefer to run them one by one, paste each block from [`backend/database/schema.sql`](../backend/database/schema.sql) into the psql prompt.

The schema creates:
| Table | Purpose |
|-------|---------|
| `users` | User accounts (email, password, name) |
| `sessions` | Chat sessions per user |
| `documents` | PDFs attached to sessions |
| `chat_messages` | Chat history with citations |

Plus: `pgcrypto` extension, 5 indexes, and an auto-update trigger for session activity.

---

## Step 5: Verify

Inside `psql`:

```sql
\dt          -- List all tables
\di          -- List all indexes
\d users     -- Describe the users table
\q           -- Exit psql
```

Expected output:
```
 Schema |     Name      | Type  
--------+---------------+-------
 public | chat_messages | table 
 public | documents     | table 
 public | sessions      | table 
 public | users         | table 
```

---

## Step 6: Set DATABASE_URL on Your Backend Service

1. Go to your **backend web service** on Render → **Environment** → **Environment Variables**
2. Set `DATABASE_URL` to the **Internal Database URL** (faster, same-region only)

> **⚠️ Important:** Use the **Internal** URL for the backend service, not the External one.

---

## Quick Reference

| Command | Description |
|---------|-------------|
| `\dt` | List tables |
| `\di` | List indexes |
| `\d <table>` | Describe a table |
| `SELECT * FROM users;` | View all users |
| `\q` | Quit psql |

---

## TL;DR

```powershell
# Run schema and verify in two commands:
psql "YOUR_EXTERNAL_DATABASE_URL" -f "backend/database/schema.sql"
psql "YOUR_EXTERNAL_DATABASE_URL" -c "\dt"
```
