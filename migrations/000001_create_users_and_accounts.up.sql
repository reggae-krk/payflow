CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    currency CHAR(3) NOT NULL DEFAULT 'PLN',
    balance_minor BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT accounts_balance_non_negative
        CHECK (balance_minor >= 0),

    CONSTRAINT accounts_currency_supported
        CHECK (currency IN ('PLN')),

    CONSTRAINT accounts_user_currency_unique
        UNIQUE (user_id, currency)
);

CREATE INDEX idx_accounts_user_id
    ON accounts(user_id);