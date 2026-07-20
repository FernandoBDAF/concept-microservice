// Thin fetch client for controld. Every request carries the Bearer token
// when one is configured; EventSource can't send headers, so the stream URL
// appends ?token= instead (contract note). All callers get typed errors so a
// 401 can raise the token banner and a 501/network error can degrade cleanly.

export const CONTROLD_BASE =
  process.env.NEXT_PUBLIC_CONTROLD_URL ?? "http://127.0.0.1:4900";

const TOKEN_KEY = "controld_token";

export function getToken(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(TOKEN_KEY) ?? "";
}

export function setToken(token: string): void {
  if (typeof window === "undefined") return;
  if (token) window.localStorage.setItem(TOKEN_KEY, token);
  else window.localStorage.removeItem(TOKEN_KEY);
}

export class ApiError extends Error {
  status: number;
  // Structured ids some 409 bodies carry: the action already running for a
  // (system,target) pair (engine), or the session already open (sessions).
  runningId?: string;
  openId?: string;
  constructor(
    message: string,
    status: number,
    ids?: { runningId?: string; openId?: string }
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.runningId = ids?.runningId;
    this.openId = ids?.openId;
  }
}

function authHeaders(extra?: Record<string, string>): Record<string, string> {
  const token = getToken();
  return {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(extra ?? {}),
  };
}

async function toApiError(res: Response): Promise<ApiError> {
  let message = `controld returned ${res.status}`;
  let runningId: string | undefined;
  let openId: string | undefined;
  try {
    const body = await res.json();
    if (body && typeof body.error === "string") message = body.error;
    if (body && typeof body.running_id === "string") runningId = body.running_id;
    if (body && typeof body.open_id === "string") openId = body.open_id;
  } catch {
    /* body may be empty or non-JSON */
  }
  return new ApiError(message, res.status, { runningId, openId });
}

export async function getJSON<T>(
  path: string,
  opts?: { signal?: AbortSignal }
): Promise<T> {
  const res = await fetch(`${CONTROLD_BASE}${path}`, {
    cache: "no-store",
    headers: authHeaders(),
    signal: opts?.signal,
  });
  if (!res.ok) {
    throw await toApiError(res);
  }
  return (await res.json()) as T;
}

export async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${CONTROLD_BASE}${path}`, {
    method: "POST",
    cache: "no-store",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw await toApiError(res);
  }
  // 204 (outcome) has no body.
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function patchJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${CONTROLD_BASE}${path}`, {
    method: "PATCH",
    cache: "no-store",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw await toApiError(res);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function getText(path: string): Promise<string> {
  const res = await fetch(`${CONTROLD_BASE}${path}`, {
    cache: "no-store",
    headers: authHeaders(),
  });
  if (!res.ok) {
    throw await toApiError(res);
  }
  return await res.text();
}

// SSE endpoint URL with the token folded into the query string, since
// EventSource cannot set an Authorization header.
export function streamUrl(id: string): string {
  const token = getToken();
  const q = token ? `?token=${encodeURIComponent(token)}` : "";
  return `${CONTROLD_BASE}/api/actions/${encodeURIComponent(id)}/stream${q}`;
}
