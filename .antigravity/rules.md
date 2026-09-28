# Project Engineering Rules

## Core Tech Stack
- **Language**: Go (latest stable conventions, idiomatic code).
- **Database Access**: Plain SQL only (`database/sql` or `pgx`/`sqlx`).
- **ORM Policy**: Strictly NO ORMs (no GORM, ent, etc.). Write raw, explicit SQL queries.

## Database & SQL Guidelines
- **Parameterization**: Always use parameterized queries (`$1`, `?`) to prevent SQL injection. Never concatenate strings into queries.
- **Resource Management**: Always check errors on `rows.Err()` and immediately `defer rows.Close()` after acquiring cursors.
- **Context Awareness**: Use context-aware methods (`QueryContext`, `ExecContext`, `BeginTx`) with request-scoped timeouts or cancellations.
- **Transactions**: Explicitly handle transaction commit and rollback with a safe `defer tx.Rollback()` pattern.
- **SQL Formatting**: Write SQL keywords in UPPERCASE (`SELECT`, `WHERE`, `JOIN`) and keep queries readable across multiple lines.

## Go Code Standards
- **Error Handling**: Follow standard Go error wrapping using `fmt.Errorf("failed to do x: %w", err)`. Never discard errors silently.
- **Simplicity**: Prefer the standard library where feasible. Avoid heavy third-party dependencies unless strictly necessary.
- **Struct Tags**: Use explicit struct tags for database scanning and JSON serialization (`db:"column_name"` `json:"field_name"`).

## Response & Communication Style
- Be concise, direct, and to the point.
- Avoid conversational filler, introductory remarks, or summaries.
- Prefer structured bullet points over long prose explanations.
- Output code snippets fully ready to run or insert.