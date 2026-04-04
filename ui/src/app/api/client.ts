export type HttpMethod = 'GET' | 'POST' | 'PUT'

export class ApiError extends Error {
  status: number
  body: string
  constructor(message: string, status: number, body: string) {
    super(message)
    this.status = status
    this.body = body
  }
}

function baseUrl() {
  const b = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'
  return b.replace(/\/$/, '')
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const url = `${baseUrl()}${path.startsWith('/') ? '' : '/'}${path}`
  const resp = await fetch(url, {
    ...init,
    headers: {
      ...(init?.headers ?? {}),
    },
  })

  const text = await resp.text()
  if (!resp.ok) {
    throw new ApiError(`HTTP ${resp.status}`, resp.status, text)
  }
  if (!text) return {} as T
  return JSON.parse(text) as T
}

export async function apiPutText<T>(path: string, bodyText: string): Promise<T> {
  return apiFetch<T>(path, {
    method: 'PUT',
    headers: {
      'Content-Type': 'text/plain',
    },
    body: bodyText,
  })
}

export async function apiPostText<T>(path: string, bodyText: string): Promise<T> {
  return apiFetch<T>(path, {
    method: 'POST',
    headers: {
      'Content-Type': 'text/plain',
    },
    body: bodyText,
  })
}

export async function apiPutJson<T>(path: string, body: unknown): Promise<T> {
  return apiFetch<T>(path, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  })
}

export async function apiDelete<T>(path: string): Promise<T> {
  return apiFetch<T>(path, { method: 'DELETE' })
}

export async function apiPostJson<T>(path: string, body?: unknown): Promise<T> {
  return apiFetch<T>(path, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: body === undefined ? '{}' : JSON.stringify(body),
  })
}
