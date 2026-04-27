import { useState } from 'react'
import { useAuth } from './AuthContext'

export default function LoginGate({ children }: { children: JSX.Element }) {
  const { authEnabled, loading, user, login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  if (loading) {
    return <div className="p-8 text-sm text-slate-400">Loading authentication...</div>
  }
  if (!authEnabled || user) return children

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setErr('')
    try {
      await login(username, password)
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : String(ex))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="mx-auto max-w-md px-6 py-16">
        <div className="rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-xl shadow-slate-950/40">
          <div className="text-lg font-semibold">Sign in</div>
          <div className="mt-1 text-sm text-slate-400">Use local account or AD LDS credentials.</div>
          <form className="mt-4 grid gap-3" onSubmit={onSubmit}>
            <input className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="Username" />
            <input type="password" className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Password" />
            <button disabled={submitting} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50">
              {submitting ? 'Signing in...' : 'Sign in'}
            </button>
          </form>
          {err ? <div className="mt-3 rounded-md border border-rose-900/70 bg-rose-950 px-3 py-2 text-sm text-rose-200">{err}</div> : null}
        </div>
      </div>
    </div>
  )
}
