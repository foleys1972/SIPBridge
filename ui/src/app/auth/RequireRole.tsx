import React from 'react'
import { useAuth, type Role } from './AuthContext'

export default function RequireRole({
  allow,
  children,
}: {
  allow: Role[]
  children: React.ReactNode
}) {
  const { role } = useAuth()
  if (!allow.includes(role)) {
    return (
      <div className="rounded-lg border border-slate-800 bg-slate-950 p-6">
        <div className="text-lg font-semibold">Access denied</div>
        <div className="mt-1 text-sm text-slate-400">
          Your role <span className="font-mono">{role}</span> cannot access this
          page.
        </div>
      </div>
    )
  }
  return <>{children}</>
}
