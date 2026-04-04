import React, { createContext, useContext, useMemo, useState } from 'react'

export type Role = 'admin' | 'operator' | 'readonly'

type AuthState = {
  role: Role
  setRole: (r: Role) => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [role, setRole] = useState<Role>('readonly')
  const value = useMemo(() => ({ role, setRole }), [role])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('AuthContext not initialized')
  return ctx
}
