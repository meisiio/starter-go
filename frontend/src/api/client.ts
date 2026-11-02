import createClient from "openapi-fetch";
import type { paths } from "./schema";

// The paths in openapi/reconciliation.yaml already start with /api/v1 (plus /health and /ready),
// so the base URL is empty. The Vite dev server proxies them to http://localhost:8086.
export const api = createClient<paths>({ baseUrl: "" });
