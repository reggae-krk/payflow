CREATE TABLE transfers (
    id BIGSERIAL PRIMARY KEY,
    idempotency_key TEXT UNIQUE,
    source_account_id BIGINT NOT NULL REFERENCES accounts(id),
    destination_account_id BIGINT NOT NULL REFERENCES accounts(id),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'completed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    transfer_id BIGINT REFERENCES transfers(id),
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    operation_type TEXT,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('credit', 'debit')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_entries_account_created_at
    ON ledger_entries (account_id, created_at);

CREATE INDEX idx_ledger_entries_transfer_id
    ON ledger_entries (transfer_id);