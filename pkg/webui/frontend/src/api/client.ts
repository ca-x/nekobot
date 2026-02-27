const TOKEN_KEY = 'nekobot_token';

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const resp = await fetch(path, { ...options, headers });

  if (resp.status === 401) {
    clearToken();
    window.location.href = '/login';
    throw new ApiError(401, 'Session expired');
  }

  if (!resp.ok) {
    const body = await resp.text();
    throw new ApiError(resp.status, body || resp.statusText);
  }

  if (resp.status === 204 || resp.headers.get('content-length') === '0') {
    return undefined as T;
  }

  return resp.json();
}

export const api = {
  get: <T>(path: string) => apiFetch<T>(path),

  post: <T>(path: string, body?: unknown) =>
    apiFetch<T>(path, {
      method: 'POST',
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),

  put: <T>(path: string, body?: unknown) =>
    apiFetch<T>(path, {
      method: 'PUT',
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),

  delete: <T>(path: string) =>
    apiFetch<T>(path, { method: 'DELETE' }),
};
