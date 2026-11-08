// Shows an RFC 9457 Problem Details error (title, detail, code, correlation id).
export interface Problem {
  title?: string;
  detail?: string;
  code?: string;
  status?: number;
  correlation_id?: string;
}

export function ProblemAlert({ problem }: { problem: Problem | null }) {
  if (!problem) return null;
  return (
    <div role="alert" className="problem-alert">
      <strong>{problem.title ?? 'Request failed'}</strong>
      {problem.detail && <p>{problem.detail}</p>}
      <small>
        {problem.code && <span>Code: {problem.code}</span>}
        {problem.correlation_id && <span> · Correlation ID: {problem.correlation_id}</span>}
      </small>
    </div>
  );
}
