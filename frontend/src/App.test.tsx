import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import { api } from './api/client';

// The page talks to the backend only through the generated client, so we replace it.
vi.mock('./api/client', () => ({
  api: { GET: vi.fn(), POST: vi.fn() },
}));

const get = api.GET as unknown as ReturnType<typeof vi.fn>;
const ok = { ok: true, status: 200, statusText: 'OK' };

function mockBackend(listResult: unknown) {
  get.mockImplementation(async (path: string) => {
    if (path === '/health') {
      return { data: { status: 'ok', service: 'S6-RCN', version: '1.0.0', timestamp: '2026-11-08T10:00:00Z' }, response: ok };
    }
    if (path === '/ready') {
      return { data: { status: 'ready', service: 'S6-RCN', database: 'connected', timestamp: '2026-11-08T10:00:00Z' }, response: ok };
    }
    if (path === '/api/v1/exceptions') return listResult;
    throw new Error(`unexpected request to ${path}`);
  });
}

describe('App (component tests)', () => {
  beforeEach(() => {
    get.mockReset();
  });

  it('shows the exceptions returned by the API', async () => {
    mockBackend({
      data: {
        items: [
          {
            id: 'e1111111-1111-1111-1111-111111111111',
            job_id: '0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11',
            transaction_reference: 'f1111111-1111-1111-1111-111111111111',
            type: 'AMOUNT_MISMATCH',
            status: 'OPEN',
            ledger_amount: { amount: 25000000, currency: 'IRR' },
            gateway_amount: { amount: 24500000, currency: 'IRR' },
            difference_amount: { amount: 500000, currency: 'IRR' },
            description: 'AMOUNT_MISMATCH: expected 25000000 IRR, actual 24500000',
            resolution_notes: null,
            resolved_by: null,
            created_at: '2026-11-08T08:00:00Z',
          },
        ],
        next_cursor: null,
      },
      response: ok,
    });

    render(<App />);

    // Adjust this matcher if App prints the reference or type differently.
    //expect(await screen.findByText(/AMOUNT_MISMATCH/)).toBeInTheDocument();
    expect(await screen.findByText('f1111111-1111-1111-1111-111111111111')).toBeInTheDocument();
    expect(screen.getByText('AMOUNT_MISMATCH')).toBeInTheDocument();
  });

  it('shows a readable message when the API answers with Problem Details', async () => {
    mockBackend({
      error: {
        type: 'https://errors.labbank.example/validation-error',
        title: 'Invalid limit',
        status: 400,
        code: 'VALIDATION_ERROR',
        detail: 'limit must not exceed 200',
        correlation_id: 'abc-123',
      },
      response: { ok: false, status: 400, statusText: 'Bad Request' },
    });

    render(<App />);

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('limit must not exceed 200');
    expect(alert).toHaveTextContent('abc-123');
  });
});
