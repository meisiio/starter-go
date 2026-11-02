-- ============================================================================
-- LabBank Service 6 (S6-RCN) - PostgreSQL Schema & Seed Migration Script
-- Jira Task: KAN-10 (Design PostgreSQL Schema and Seed Data for Reconciliation & Exceptions)
-- ============================================================================

-- 1. Table: reconciliation_jobs
CREATE TABLE IF NOT EXISTS reconciliation_jobs (
    id UUID PRIMARY KEY,
    business_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('PENDING', 'IN_PROGRESS', 'COMPLETED', 'FAILED')),
    total_checked INT NOT NULL DEFAULT 0,
    matched_count INT NOT NULL DEFAULT 0,
    discrepancy_count INT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT
);

-- 2. Table: bank_accounts (known bank accounts for validation & consumer query verification)
CREATE TABLE IF NOT EXISTS bank_accounts (
    account_id UUID PRIMARY KEY,
    account_number VARCHAR(50) NOT NULL UNIQUE,
    iban VARCHAR(34) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Table: exceptions
CREATE TABLE IF NOT EXISTS exceptions (
    id UUID PRIMARY KEY,
    job_id UUID REFERENCES reconciliation_jobs(id) ON DELETE SET NULL,
    account_id UUID NOT NULL,
    reference_id UUID NOT NULL,
    entry_id UUID,
    discrepancy_type VARCHAR(40) NOT NULL CHECK (discrepancy_type IN ('AMOUNT_MISMATCH', 'MISSING_IN_LEDGER', 'MISSING_IN_SETTLEMENT', 'STATUS_CONFLICT')),
    expected_amount BIGINT NOT NULL,
    actual_amount BIGINT NOT NULL,
    difference_amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'IRR',
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'UNDER_REVIEW', 'RESOLVED', 'DISMISSED')),
    resolution_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Indexes & Constraints (Fast account lookups and cursor pagination)
CREATE INDEX IF NOT EXISTS idx_exceptions_account_id ON exceptions(account_id);
CREATE INDEX IF NOT EXISTS idx_exceptions_status_created ON exceptions(status, created_at DESC);

-- ============================================================================
-- Seed Data for Team 1 (Account - Consumer) automated verification
-- ============================================================================

-- Known test bank accounts
INSERT INTO bank_accounts (account_id, account_number, iban, created_at)
VALUES 
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'AC-1001-2026', 'IR120120000000000000000001', NOW() - INTERVAL '30 days'),
    ('b1ffcd00-0d1c-4fe9-cc7e-7cc0ce491b22', 'AC-1002-2026', 'IR120120000000000000000002', NOW() - INTERVAL '30 days')
ON CONFLICT (account_id) DO NOTHING;

-- Initial reconciliation audit run
INSERT INTO reconciliation_jobs (id, business_date, status, total_checked, matched_count, discrepancy_count, started_at, completed_at, error_message)
VALUES 
    ('0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11', '2026-10-31', 'COMPLETED', 1422, 1420, 2, NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour', NULL)
ON CONFLICT (id) DO NOTHING;

-- Sample discrepancy exceptions associated with Account a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11
INSERT INTO exceptions (
    id,
    job_id,
    account_id,
    reference_id,
    entry_id,
    discrepancy_type,
    expected_amount,
    actual_amount,
    difference_amount,
    currency,
    status,
    resolution_notes,
    created_at,
    updated_at
)
VALUES 
    (
        'e1111111-1111-1111-1111-111111111111',
        '0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11',
        'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
        'f1111111-1111-1111-1111-111111111111',
        '0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11',
        'AMOUNT_MISMATCH',
        25000000,
        24500000,
        500000,
        'IRR',
        'OPEN',
        'Settlement fee calculation discrepancy between S2 ledger (25,000,000 IRR) and S5 gateway settlement report (24,500,000 IRR)',
        NOW() - INTERVAL '90 minutes',
        NOW() - INTERVAL '90 minutes'
    ),
    (
        'e2222222-2222-2222-2222-222222222222',
        '0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11',
        'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
        'f2222222-2222-2222-2222-222222222222',
        '0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c22',
        'MISSING_IN_SETTLEMENT',
        10000000,
        0,
        10000000,
        'IRR',
        'UNDER_REVIEW',
        'Transfer recorded in S2 ledger but missing in external S5 clearing report',
        NOW() - INTERVAL '60 minutes',
        NOW() - INTERVAL '60 minutes'
    )
ON CONFLICT (id) DO NOTHING;
