import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { apiFetch, apiPostJson } from '../api/client'

export type Role = 'admin' | 'operator' | 'readonly'

type AuthUser = {
  username: string
  provider: string
  role: Role
}

type AuthState = {
  role: Role
  user: AuthUser | null
  loading: boolean
  authEnabled: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [loading, setLoading] = useState(true)
  const [authEnabled, setAuthEnabled] = useState(false)

  useEffect(() => {
    let alive = true
    async function loadMe() {
      try {
        const me = await apiFetch<{ enabled: boolean; user?: AuthUser }>('/v1/auth/me')
        if (!alive) return
        setAuthEnabled(Boolean(me.enabled))
        setUser(me.user ?? null)
      } catch {
        if (!alive) return
        setUser(null)
      } finally {
        if (alive) setLoading(false)
      }
    }
    loadMe()
    return () => {
      alive = false
    }
  }, [])

  async function login(username: string, password: string) {
    const res = await apiPostJson<{ ok: boolean; token: string; user: AuthUser; error?: string }>('/v1/auth/login', { username, password })
    if (!res.ok) throw new Error(res.error ?? 'Login failed')
    localStorage.setItem('sipbridge.auth.token', res.token)
    setUser(res.user)
    setAuthEnabled(true)
  }

  function logout() {
    localStorage.removeItem('sipbridge.auth.token')
    setUser(null)
  }

  const role: Role = user?.role ?? 'readonly'
  const value = useMemo(() => ({ role, user, loading, authEnabled, login, logout }), [role, user, loading, authEnabled])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('AuthContext not initialized')
  return ctx
}
