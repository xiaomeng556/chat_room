import { getToken } from './auth'

export const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? 'http://localhost:8888'

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {}
): Promise<T> {
  const url = new URL(path, API_BASE).toString()
  const headers = new Headers(init.headers ?? {})
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const res = await fetch(url, { ...init, headers })
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const data = await res.json()
      if (data?.error) msg = String(data.error)
    } catch {}
    throw new Error(msg)
  }

  return (await res.json()) as T
}

export function wsUrl(path: string, token?: string): string {
  const base = new URL(API_BASE)
  const protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  const u = new URL(path, `${protocol}//${base.host}`)
  if (token) u.searchParams.set('token', token)
  return u.toString()
}

