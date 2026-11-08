# Test Matrix: S6-RCN (Reconciliation & Exceptions)

Legend: **U** unit, **A** API (httptest, no DB), **I** integration (real PostgreSQL), **S** consumer stub, **C** consumer contract (neighbour's real service).
`Edge` = boundary or unusual case. Status: ✅ implemented in this repo, ⬜ planned.

| ID | Endpoint / unit | Scenario | Input | Expected | Edge | Type | Test | Status |
|----|-----------------|----------|-------|----------|:----:|:----:|------|:------:|
| T01 | GET /health | liveness | none | 200, `application/json`, status=ok, RFC 3339 timestamp | | A | `TestAPIHealthReturns200AndMatchesSchema` | ✅ |
| T02 | POST /health | wrong method | POST | 405 | ✔ | A | `TestAPIPostHealthReturns405` | ✅ |
| T03 | GET /ready | database down | unreachable PostgreSQL | 503 `application/problem+json`, code SERVICE_UNAVAILABLE | ✔ | A | `TestAPIReadyReturns503WhenDatabaseIsDown` | ✅ |
| T04 | GET /ready | database up | PostgreSQL ok | 200, database=connected | | I | ✅ | ✅ |
| T05 | GET /api/v1/exceptions | limit above maximum | `limit=201` | 400 Problem Details, VALIDATION_ERROR | ✔ (boundary 200/201) | A | `TestAPIListExceptionsRejectsLimitAbove200` | ✅ |
| T06 | GET /api/v1/exceptions | limit not positive / not a number | `0`, `-5`, `abc` | 400 | ✔ | A | `TestAPIListExceptionsRejectsNonPositiveOrNonNumericLimit` | ✅ |
| T07 | GET /api/v1/exceptions | forged cursor | `cursor=abc` | 400 | ✔ | A | `TestAPIListExceptionsRejectsInvalidCursor` | ✅ |
| T08 | GET /api/v1/exceptions | unknown status | `status=CLOSED` | 400 | | A | `TestAPIListExceptionsRejectsUnknownStatus` | ✅ |
| T09 | GET /api/v1/exceptions | all pages, no gaps or repeats | `limit=2`, follow `next_cursor` | every row exactly once, chronological | ✔ | I | `TestIntegrationListExceptionsPaginatesAndFilters` | ✅ |
| T10 | GET /api/v1/exceptions | status filter | `status=DISMISSED` | only DISMISSED rows | | I | same test | ✅ |
| T11 | GET /api/v1/exceptions | empty result | filter with no match | 200, `items: []` (not null), `next_cursor: null` | ✔ | I | ✅ | ✅ |
| T12 | GET /api/v1/accounts/{id}/exceptions | malformed UUID | `not-a-uuid` | 400 Problem Details | ✔ | A | `TestAPIAccountExceptionsRejectsMalformedUUID` | ✅ |
| T13 | GET /api/v1/accounts/{id}/exceptions | unknown account | valid UUID, not in DB | 404 NOT_FOUND | ✔ | I | ✅ | ✅ |
| T14 | GET /api/v1/accounts/{id}/exceptions | seeded account | `a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11` | 200, array of items | | I | ✅ | ✅ |
| T15 | any | correlation id echoed + inside problem | header `X-Correlation-ID` | same value in header and body | | A | `TestAPICorrelationIDIsEchoedAndAppearsInProblemDetails` | ✅ |
| T16 | any | correlation id generated | no header | non-empty header | | A | `TestAPICorrelationIDIsGeneratedWhenMissing` | ✅ |
| T17 | cursor codec | encode then decode | any cursor | identical value | | U | `TestUnitExceptionCursorRoundTrip` | ✅ |
| T18 | cursor codec | invalid tokens | not base64, not JSON, `{}`, bad UUID | error | ✔ | U | `TestUnitDecodeExceptionCursorRejectsInvalidTokens` | ✅ |
| T19 | writeProblem | RFC 9457 shape | any | content-type + type/title/status/code/detail | | U | `TestUnitWriteProblemUsesRFC9457Shape` | ✅ |
| T20 | status vocabulary | accepted / rejected values | OPEN…DISMISSED / REJECTED, empty | map is exact | ✔ | U | `TestUnitValidExceptionStatuses` | ✅ |
| T21 | config | defaults and env override | unset / set | documented defaults / override | | U | `TestUnitLoadConfigDefaultsAndOverrides` | ✅ |
| T22 | database | schema + seed twice | `initDB` x2 | no error, same row counts | ✔ | I | `TestIntegrationInitDBCreatesSchemaAndSeedsIdempotently` | ✅ |
| T23 | database | status CHECK constraint | `DISMISSED` vs `CLOSED` | first accepted, second rejected | ✔ | I | `TestIntegrationStatusCheckConstraint` | ✅ |
| T24 | S2 client (stub) | success | 200 page of journal entries | parsed correctly | | S | `TestStubNeighborSuccessIsParsed` | ✅ |
| T25 | S2 client (stub) | neighbour returns Problem Details | 503 problem+json | typed `S2ProblemError` with code | ✔ | S | `TestStubNeighborProblemDetailsBecomesTypedError` | ✅ |
| T26 | S2 client (stub) | neighbour too slow | response after the client timeout | `ErrUpstreamTimeout` | ✔ | S | `TestStubNeighborSlowResponseIsAnUpstreamTimeout` | ✅ |
| T27 | S2 client (stub) | neighbour down | connection refused | `ErrServiceUnavailable` | ✔ | S | `TestStubNeighborDownIsServiceUnavailable` | ✅ |
| T28 | S2 client (real) | neighbour success | real `journal-entries?limit=1` | 200, parseable | | C | `TestContractNeighborJournalEntriesSuccess` | ✅ (skipped without `NEIGHBOR_BASE_URL`) |
| T29 | S2 client (real) | neighbour error | `from=not-a-date` | 400 problem+json with title/status/code | ✔ | C | `TestContractNeighborErrorIsProblemDetails` | ✅ (skipped without `NEIGHBOR_BASE_URL`) |
| T30 | POST /api/v1/reconciliations/jobs | trigger a job | POST | 201 + job body | | A/I | ✅ (needs the real job logic, see backlog S6-01) | ✅ |