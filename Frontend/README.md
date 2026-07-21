# Ledger — Expense Chatbot frontend

A Svelte + Vite frontend for the Expense_chatbot backend, wired to your
`AuthService`, `UserService`, and `ExpenseService` over ConnectRPC.

## Stack

- **Svelte 5** + **Vite**
- **@connectrpc/connect-web** — talks to the Go backend using the same
  `.proto` definitions as the server (copied into `proto/`, generated
  code lives in `src/gen/`)
- **svelte-spa-router** — hash-based client routing (`/#/chat`, `/#/expenses`, …)

## Getting started

```bash
npm install
cp .env.example .env
# edit .env if your backend isn't on http://localhost:8080
npm run dev
```

The dev server runs on Vite's default port (check the terminal output).
Make sure your Go backend is running and reachable at the URL in `.env`,
and that it has CORS enabled for that origin if backend and frontend run
on different ports.

## Regenerating API code after a proto change

If the backend's `.proto` files change, update the copies in `proto/expense/v1/`
to match, then regenerate:

```bash
npx buf generate proto
```

This regenerates everything under `src/gen/` using `protoc-gen-es` and
`protoc-gen-connect-es` (already installed as dev dependencies).

## Project layout

```
proto/expense/v1/       — copies of the backend's .proto files
src/gen/                — generated TS types + Connect clients (do not hand-edit)
src/lib/api/            — transport (auth header interceptor) + typed clients
src/lib/stores/         — auth state (persisted to localStorage) + toasts
src/lib/utils/format.ts — money/date formatting, currency preference
src/lib/components/     — Sidebar, AppShell, ToastStack
src/pages/              — one file per route (Login, Chat, Expenses, Budgets, …)
src/App.svelte          — router + auth guards
```

## Auth flow

- `Login`/`Register` call `AuthService`, store the returned JWT + user
  in `localStorage` via the `auth` store.
- Every subsequent Connect call attaches `Authorization: Bearer <token>`
  automatically (see `src/lib/api/transport.ts`).
- A `16 Unauthenticated` response clears local auth and the router sends
  you back to `/login`.

## Pages implemented

Every RPC across all three services has a corresponding UI:

| Page | Covers |
|---|---|
| Chat | `SendChatMessage`, `GetChatHistory` |
| Expenses | `ListExpenses` (+ date filter, pagination), `UpdateExpense`, `DeleteExpense` |
| Categories | `ListCategories`, `CreateCategory`, `UpdateCategory`, `DeleteCategory` |
| Budgets | `ListBudgets`, `CreateBudget`, `UpdateBudget`, `DeleteBudget` |
| Recurring | `ListRecurringExpenses`, `CreateRecurringExpense`, `UpdateRecurringExpense`, `DeleteRecurringExpense` |
| Insights | `GetMonthlyInsights` |
| Export | `ExportExpensesCSV`, `ExportExpensesPDF` |
| Settings | `GetProfile`, `UpdateProfile`, `UpdatePassword`, `DeleteProfile` |

`CreateExpense`, `GetExpense`, `ParseExpenseText`, and `QuickAddExpense`
aren't wired to their own UI — per the backend, the chatbot's NLP path
(`SendChatMessage`) is the intended way to create expenses.

## Design

A "ledger/receipt" visual direction: warm paper background, deep ink
text, a gold accent for primary actions, and dotted "receipt-row" line
items (see `.receipt-row` in `src/app.css`) used across Expenses,
Budgets, Recurring, and Insights as the one recurring signature motif.
