CREATE TABLE
    accounts (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID NOT NULL REFERENCES users (id),
        name TEXT NOT NULL,
        type TEXT NOT NULL CHECK (
            type IN (
                'asset',
                'liability',
                'equity',
                'income',
                'expense'
            )
        ),
        currency TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
        UNIQUE (user_id, name),
        UNIQUE (id, currency)
    );

CREATE INDEX accounts_user_id_idx ON accounts (user_id);

CREATE TABLE
    journal_transactions (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID NOT NULL REFERENCES users (id),
        description TEXT NOT NULL,
        occurred_at TIMESTAMPTZ NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
    );

CREATE INDEX journal_transactions_user_id_idx ON journal_transactions (user_id);
CREATE INDEX journal_transactions_occurred_at_idx ON journal_transactions (occurred_at);

CREATE TABLE postings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES journal_transactions(id),
    account_id UUID NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor <> 0),
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (account_id, currency) REFERENCES accounts(id, currency)
);

CREATE INDEX postings_transaction_id_idx ON postings (transaction_id);
CREATE INDEX postings_account_id_idx ON postings (account_id);

CREATE FUNCTION check_transaction_balance() RETURNS TRIGGER AS $$
DECLARE
    tx_id UUID;
    total BIGINT;
    distinct_currencies INTEGER;
    posting_count INTEGER;
BEGIN
    IF TG_TABLE_NAME = 'postings' THEN
        IF TG_OP = 'DELETE' THEN
            tx_id := OLD.transaction_id;
        ELSE
            tx_id := NEW.transaction_id;
        END IF;
    ELSE
        tx_id := NEW.id;
    END IF;

    SELECT COALESCE(SUM(amount_minor), 0),
            COUNT(DISTINCT currency),
            COUNT(*)
    INTO total, distinct_currencies, posting_count
    FROM postings
    WHERE transaction_id = tx_id;

    IF posting_count < 2 THEN
        RAISE EXCEPTION 'transaction % has % postings; minimum is 2', tx_id, posting_count;
    END IF;

    IF distinct_currencies > 1 THEN
        RAISE EXCEPTION 'transaction % has % distinct currencies', tx_id, distinct_currencies;
    END IF;

    IF total <> 0 THEN
        RAISE EXCEPTION 'transaction % does not balance: sum is %', tx_id, total;
    END IF;

    RETURN NULL;
END
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER check_balance_after_transaction
    AFTER INSERT ON journal_transactions
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION check_transaction_balance();

CREATE CONSTRAINT TRIGGER check_balance_after_posting
    AFTER INSERT OR UPDATE OR DELETE ON postings
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION check_transaction_balance();

CREATE FUNCTION deny_update_delete() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'table % is append-only; use compensating transactions', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER no_update_journal_transactions
    BEFORE UPDATE OR DELETE ON journal_transactions
    FOR EACH ROW
    EXECUTE FUNCTION deny_update_delete();

CREATE TRIGGER no_update_postings
    BEFORE UPDATE OR DELETE ON postings
    FOR EACH ROW
    EXECUTE FUNCTION deny_update_delete();

