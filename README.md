<p align="center">
  <h1 align="center">🪙 Ledger — AI Expense Tracker</h1>
  <p align="center"><strong>AI-Powered Conversational Expense Chatbot & Financial Ledger</strong></p>
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
  <a href="https://svelte.dev/"><img src="https://img.shields.io/badge/Svelte-5-FF3E00?style=flat-square&logo=svelte" alt="Svelte Version"></a>
  <a href="https://connectrpc.com/"><img src="https://img.shields.io/badge/ConnectRPC-v1-000000?style=flat-square" alt="ConnectRPC"></a>
  <a href="https://www.postgresql.org/"><img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat-square&logo=postgresql" alt="PostgreSQL"></a>
  <a href="https://www.docker.com/"><img src="https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat-square&logo=docker" alt="Docker"></a>
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License">
</p>

<p align="center">
  <i>A production-ready financial tracking platform that combines natural language chatbot logging, timezone-aware budgeting, and multi-currency tracking into a unified paper-ledger design.</i>
</p>

<p align="center">
  <a href="https://expense-tracker-frontend-8cfy.onrender.com">🚀 Live Application Demo</a>
</p>

---

## 🌟 Overview

**Ledger** is a secure, personal financial management platform built to replace tedious manual form entry with a conversational AI interface. Powered by an internal NLP engine, users can log expenses naturally (e.g. `"coffee 120"` or `"spent 500 on office"`) in plain text. The system automatically parses the amount, detects currency tags, resolves custom or system categories case-insensitively, and commits the records directly to a database.

Unlike generic spreadsheet trackers, Ledger runs a timezone-aware ecosystem supporting global category boundaries, recurring scheduler tasks, and detailed financial health metrics (On Track, Warning, Exceeded) adjusted to the user's home location.

> **Why build this?**  
> To demonstrate a robust full-stack architecture using statically compiled Go services, schema-driven databases (Ent ORM), type-safe schema synchronization via ConnectRPC (Protobuf), and a reactive frontend (Svelte 5) working in absolute timezone-alignment.

---

## ✨ Key Features

| Feature | Description |
| :--- | :--- |
| **🤖 AI Chatbot Logging** | Conversational expense logging using custom NLP parsing that identifies amount, title, date, currency, and maps categories. |
| **💰 Multi-Currency support** | Fully localized default support for `INR` (₹) with active conversions/checks for `USD`, `EUR`, and `GBP`. |
| **📊 Timezone-Aware Budgets** | Set monthly, weekly, or yearly budgets (category-specific or overall) in your local timezone (`Asia/Kolkata`) with inclusive date boundaries. |
| **⏰ Recurring Scheduler** | Background scheduler checking active subscriptions and recurring bills (daily, weekly, monthly) and logging them on their run dates. |
| **📈 Insights & Analytics** | Dynamic analytics parsing month-over-month spending increases, spikes, and budget compliance warnings. |
| **📄 PDF & CSV Exports** | On-demand generation of tabular CSV sheets or styled PDF receipts. |
| **🌗 Dual Theme Engine** | Premium paper-ledger aesthetic supporting high-contrast Light and Dark mode options. |

---

## 📐 Architecture

Ledger follows a strict separation of concerns, routing typed RPC calls over HTTP/2 (or HTTP/1.1 fallback) via ConnectRPC.

```mermaid
graph TD
    %% Client Layer
    subgraph client_layer ["Client Layer"]
        Browser[User Browser]
        Svelte[Svelte 5 Frontend]
    end

    %% Application Layer
    subgraph app_layer ["Application Layer"]
        GoServer[Go Backend Server]
        Auth[JWT Auth Interceptor]
        NLPEngine[Go NLP Parser]
        Scheduler[Background Job Scheduler]
    end

    %% Database Layer
    subgraph data_layer ["Data Layer"]
        DB[(PostgreSQL Database)]
    end

    %% Relationships
    Browser -->|HTTP/HTTPS| Svelte
    Svelte -->|ConnectRPC + JWT| GoServer
    GoServer --> Auth
    GoServer -->|Parse Text| NLPEngine
    GoServer -->|Ent ORM / sql.DB| DB
    Scheduler -->|Poll due jobs| DB
    Scheduler -->|Log recurring expenses| DB
```

---

## 🚀 Quick Start

### Prerequisites
- **Go 1.25+** (for Go backend compilation)
- **Node.js 18+** & **npm** (for Frontend dev server)
- **PostgreSQL** instance running locally

### Installation

#### 1️⃣ Clone the Repository
```bash
git clone https://github.com/sarthak733/Expense_chatbot.git
cd Expense_chatbot
```

#### 2️⃣ Setup & Run Go Backend
Navigate to the server directory and configure your local environments:
```bash
cd expense-server
cp .env.example .env
# Edit .env with your PostgreSQL credentials:
# DATABASE_URL=postgres://postgres:password@localhost:5432/expense_tracker
# JWT_SECRET=your_32_byte_secret_key
```
Run the server (it automatically compiles, applies database schema migrations, and starts the scheduler):
```bash
go run ./cmd/server
```
*Backend is live at `http://localhost:8080`.*

#### 3️⃣ Setup & Run Svelte Frontend
Open a new terminal window in the root directory:
```bash
cd Frontend
npm install
cp .env.example .env
# Edit .env if your backend runs on a custom port
```
Start the development server:
```bash
npm run dev
```
*Frontend is live at `http://localhost:5173`.*

---

## 🛠️ Tech Stack

### Frontend
- **Svelte 5** — Declarative UI binding
- **Vite** — Fast bundle builder
- **ConnectRPC Web** — Type-safe Protobuf clients
- **Vanilla CSS** — Custom HSL paper-texture visual language

### Backend
- **Go (Golang)** — Concurrent, statically compiled server
- **ConnectRPC** — Dual-compatibility gRPC and HTTP API framework
- **Ent ORM** — Graph-based database entities
- **Time/Tzdata** — Bundled time databases for strict IST (`Asia/Kolkata`) compliance

### Infrastructure & DevOps
- **PostgreSQL** — Core database
- **Docker** — Multi-stage container builds
- **Render Blueprints** — blueprint-managed deployments (`render.yaml`)

---

## 📁 Project Structure

```
Expense_chatbot/
├── expense-server/              # Go Backend Application
│   ├── cmd/
│   │   └── server/main.go       # Server entry point, timezone setup, database seeders
│   ├── ent/                     # Ent ORM auto-generated schemas
│   ├── gen/                     # ConnectRPC generated Go files
│   ├── internal/
│   │   ├── config/              # Server configuration loaders
│   │   ├── database/            # Database initialization and migration run
│   │   ├── handler/             # ConnectRPC RPC endpoints (Expenses, Budgets, Profile)
│   │   ├── middleware/          # JWT authentication, CORS, logging, recovery
│   │   ├── nlp/                 # Parser, keywords categorizer, currency extractor
│   │   └── scheduler/           # Background recurring expense cron job
│   ├── migrations/              # Versioned SQL migration files
│   ├── proto/                   # API Protocol Buffers (.proto files)
│   ├── tests/
│   │   └── integration_test.go  # Core end-to-end integration test suites
│   ├── Dockerfile               # Multi-stage Alpine container configuration
│   └── go.mod                   # Go dependencies definition
│
├── Frontend/                    # Svelte Client Application
│   ├── proto/                   # Protobuf definitions
│   ├── public/                  # Global static assets
│   ├── src/
│   │   ├── gen/                 # ConnectRPC Web generated TS files
│   │   ├── lib/
│   │   │   ├── components/      # Reusable UI elements (Shell, Sidebars, ToastStack)
│   │   │   ├── stores/          # Auth context, Svelte stores, themes
│   │   │   └── api/             # Connect client instances & request interceptors
│   │   ├── pages/               # Route pages (Login, Budgets, Chat, Expenses, Settings)
│   │   ├── App.svelte           # Client router and guest/auth redirects
│   │   ├── app.css              # Color tokens, paper visual design
│   │   └── main.ts              # Frontend mounting and startup theme config
│   ├── package.json             # Frontend npm dependencies
│   └── vite.config.ts           # Vite compile configuration
│
├── render.yaml                  # Managed infrastructure blueprint
├── deploymen.readme             # Step-by-step staging setup documentation
└── requirements.txt             # Placeholder requirements documentation
```

---

## 🧪 Testing

The backend includes a comprehensive test suite covering the entire API lifecycle.

To run the integration tests locally:
```bash
cd expense-server
go test -v ./tests/integration_test.go
```
*Note: Make sure your PostgreSQL database is available for the test runner to clean, migrate, and assert.*
