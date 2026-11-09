import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ProblemAlert } from './ProblemAlert';

describe('ProblemAlert (component test)', () => {
  it('shows title, detail, code and correlation id of a Problem Details error', () => {
    render(
      <ProblemAlert
        problem={{
          title: 'Invalid limit',
          detail: 'limit must not exceed 200',
          code: 'VALIDATION_ERROR',
          status: 400,
          correlation_id: 'abc-123',
        }}
      />,
    );

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Invalid limit');
    expect(alert).toHaveTextContent('limit must not exceed 200');
    expect(alert).toHaveTextContent('VALIDATION_ERROR');
    expect(alert).toHaveTextContent('abc-123');
  });

  it('renders nothing when there is no error', () => {
    const { container } = render(<ProblemAlert problem={null} />);
    expect(container).toBeEmptyDOMElement();
  });
});
