# PayFlow System Architecture

## 1. High-Level Overview
PayFlow is a financial ledger and payment flow system written in Go. It provides user management, multi-currency account handling, double-entry transaction ledgering, fund transfers, and transactional financial reporting. The core stack consists of Go 1.26, Gin for REST APIs, gRPC with Protocol Buffers for reporting microservices, PostgreSQL with `pgx/v5` (`pgxpool`), JWT authentication, and Docker/Testcontainers for deployment and integration testing.

## 2. Directory Layout
* `cmd/`: Application entry points.
  * `cmd/api/`: REST API HTTP server (`main.go`, `routes.go`).
* `proto/`: gRPC protobuf definitions and server entry points.
  * `proto/reporting/`: gRPC reporting service entry point (`main.go`) and generated protobuf files (`pb/`, `v1/`).
* `internal/`: Private application code.
  * `app/`: Cross-domain orchestration workflows (e.g., user registration with default wallet initialization).
  * `auth/`: Authentication middleware (`RequireAuth`) and JWT token creation/validation.
  * `config/`: Environment configuration management. Split into `Load()` (requires `JWT_SECRET`, used by `cmd/api`) and `LoadDBConfig()` (database connection settings only, used by `proto/reporting` so it does not depend on a secret it never uses).
  * `db/`: Database abstractions (`Querier` interface unifying `pgx.Tx` and `pgxpool.Pool`) and Postgres error code helpers (`IsUniqueViolation`, `IsForeignKeyViolation`, `IsCheckViolation`).
  * `reporting/`: gRPC reporting service implementation for account summaries and aggregations.
  * `users/`: User domain logic, entity models, password hashing (bcrypt), input validation, repository, and HTTP handlers.
  * `wallet/`: Core financial domain logic (accounts, transfers, double-entry ledger entries, currencies), transactional services, repositories, and HTTP handlers.
  * `testhelpers/`: Test infrastructure and Postgres Testcontainers setup.
* `migrations/`: SQL migration files (`golang-migrate`) defining database tables, foreign keys, and non-negative balance constraints.
* `docs/`: System documentation, DBML definitions, and entity-relationship diagrams (ERD).

## 3. Data Flow
* **HTTP REST Request Flow (e.g., Fund Transfers / Deposits / Withdrawals)**:
  1. **Client Request**: Client sends an HTTP request with a JSON payload and Bearer JWT token to `cmd/api`.
  2. **Router & Middleware**: `Gin` router passes the request to `auth.RequireAuth`, which verifies the JWT token, extracts `userID`, and injects it into the context.
  3. **Handler Layer**: `wallet.Handler` parses request parameters/body (`ShouldBindJSON`), extracts `userID`, and calls `wallet.AccountService`.
  4. **Service Layer**: `wallet.service` validates request logic, opens a Postgres transaction (`pool.Begin(ctx)`), verifies account ownership, and adjusts balances through a single atomic `UPDATE accounts SET balance_minor = balance_minor + $1 WHERE id = $2 AND balance_minor + $1 >= 0` statement per account (source, then destination for transfers). This statement is both the balance check and the update in one round trip — it does not use a separate `SELECT ... FOR UPDATE` step, and account rows are **not** explicitly locked in ascending ID order. See "Known limitation" below.
  5. **Repository Layer**: Repositories (`AccountRepository`, `TransferRepository`, `LedgerRepository`) execute raw parameterized SQL using the `db.Querier` interface (bound to `pgx.Tx`).
  6. **Database Execution**: PostgreSQL executes raw SQL queries and enforces schema constraints (e.g., non-negative balances via `CHECK`).
  7. **Response Flow**: Repositories map rows to domain structs; the service commits the transaction; the handler serializes domain results into HTTP JSON responses.
* **gRPC Request Flow (e.g., Account Summary Reporting)**:
  1. **Client Request**: gRPC client invokes `GetAccountSummary` on port 50051.
  2. **gRPC Server**: `reporting.server` receives the request.
  3. **Database Aggregation**: Executes parameterized aggregate SQL queries directly via the `db.Querier` interface (in production wired to a `*pgxpool.Pool`; in unit tests wired to a fake implementation) to compute deposits, withdrawals, and transfers in/out.
  4. **Response Flow**: Query results are scanned into protobuf response structs (`pb.GetAccountSummaryResponse`) and returned to the client.

## 4. Core Design Patterns
* **Dependency Injection & Interface Segregation**: Components depend on explicit Go interfaces (`UserHandler`, `AccountService`, `AccountRepository`, `Querier`) passed via constructor functions (`NewHandler`, `NewService`, `NewAccountRepository`), ensuring decoupled design and testability.
* **Querier Pattern (Unified DB/Tx Operations)**: The `db.Querier` interface abstracts both `*pgxpool.Pool` and `pgx.Tx` behind the same method set (`Exec`, `Query`, `QueryRow`), allowing repositories to run either inside a transaction or directly against the pool without code duplication. The Reporting Service reuses this same interface to swap a real pool for a fake in unit tests, without needing a running database.
* **Double-Entry Ledger Pattern**: Financial operations append immutable `LedgerEntry` records (debit/credit) alongside account balance updates for auditability.
* **Atomic Compare-and-Update Balance Guard**: Instead of a separate read-then-check-then-write sequence, balance changes go through a single `UPDATE ... WHERE balance_minor + $1 >= 0` statement. PostgreSQL evaluates the `WHERE` clause and applies the update atomically for that row, so a concurrent transaction cannot observe a stale balance between check and write for the *same* account.
* **Domain Error Wrapping & Translation**: Low-level database errors (`pgconn.PgError`) are inspected via helper functions (`IsUniqueViolation`, `IsForeignKeyViolation`, `IsCheckViolation`) and translated into domain errors (`ErrUserNotFound`, `ErrDuplicateTransfer`, `ErrInsufficientFunds`, `ErrForbidden`). Note: `ErrUserNotFound` is defined in the `wallet` package (raised when creating an account for a non-existent user violates a foreign key), not in `users`.

## 5. Known Limitation: Transfer Lock Ordering
The atomic per-row `UPDATE` guard (see section 4) prevents a lost update *within a single account's balance*, but `Transfer()` currently updates the source account and then the destination account in the order given by the request, **not** sorted by ascending account ID. Two concurrent, opposite-direction transfers (e.g. transaction A transferring account 1 -> account 2, and transaction B transferring account 2 -> account 1) can each acquire an implicit row lock on one account via their `UPDATE` and then block waiting for the other — a classic circular-wait deadlock. `docs/erd.md` and an earlier commit message describe a fixed ascending-ID lock order as the intended mitigation, but this is not reflected in the current `Transfer()` implementation. This should either be fixed (sort source/destination by ID before issuing the two `UPDATE` statements) or explicitly re-scoped in the documentation and validated with a concurrent-transfer test before being considered resolved.

## 6. Configuration & Environment
* **Configuration Loading**: Environment variables are parsed in `internal/config` using `os.LookupEnv` with default fallbacks for local development. `cmd/api` calls `Load()`, which additionally requires `JWT_SECRET` and panics if it is missing. `proto/reporting` calls `LoadDBConfig()`, which only reads database connection settings and has no dependency on `JWT_SECRET`.
* **Database Connection Pooling**: Database connections are managed by `pgxpool.Pool` (`github.com/jackc/pgx/v5/pgxpool`). The pool is instantiated at startup in `cmd/api/main.go` and `proto/reporting/main.go` using `cfg.ConnString()`, providing connection lifecycle management, health checking (`pool.Ping`), and transaction creation (`pool.Begin(ctx)`).