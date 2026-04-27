import { useCallback, useEffect, useMemo, useState } from 'react'
import yaml from 'js-yaml'
import { apiFetch, apiPutText } from '../api/client'
import type { AuthRole, ConfigStatus, LocalAuthUser, RootConfig } from '../api/types'

function cloneConfig(c: RootConfig): RootConfig {
  if (typeof structuredClone === 'function') return structuredClone(c) as RootConfig
  return JSON.parse(JSON.stringify(c)) as RootConfig
}

const roleOptions: AuthRole[] = ['admin', 'operator', 'readonly']

export default function AuthUsersPage() {
  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [draft, setDraft] = useState<RootConfig | null>(null)
  const [cfgStatus, setCfgStatus] = useState<ConfigStatus | null>(null)
  const [status, setStatus] = useState('')
  const [err, setErr] = useState('')
  const [passwordEdits, setPasswordEdits] = useState<Record<number, string>>({})

  const localUsers = useMemo(() => draft?.spec.auth?.local?.users ?? [], [draft])
  const authEnabled = Boolean(draft?.spec.auth?.enabled)
  const localEnabled = Boolean(draft?.spec.auth?.local?.enabled)

  const load = useCallback(async () => {
    const [c, st] = await Promise.all([
      apiFetch<RootConfig>('/v1/config'),
      apiFetch<ConfigStatus>('/v1/config/status').catch(() => null),
    ])
    setCfg(c)
    setDraft(cloneConfig(c))
    setPasswordEdits({})
    if (st) setCfgStatus(st)
  }, [])

  useEffect(() => {
    load().catch((e) => setErr(e instanceof Error ? e.message : String(e)))
  }, [load])

  function ensureAuth(d: RootConfig): RootConfig {
    return {
      ...d,
      spec: {
        ...d.spec,
        auth: {
          enabled: d.spec.auth?.enabled ?? true,
          session_ttl_minutes: d.spec.auth?.session_ttl_minutes ?? 480,
          local: {
            enabled: d.spec.auth?.local?.enabled ?? true,
            users: d.spec.auth?.local?.users ?? [],
          },
          adlds: d.spec.auth?.adlds ?? { enabled: false },
        },
      },
    }
  }

  function updateRoot(next: (d: RootConfig) => RootConfig) {
    setDraft((d) => {
      if (!d) return d
      const withAuth = ensureAuth(d)
      return next(withAuth)
    })
  }

  function updateUser(idx: number, next: LocalAuthUser) {
    updateRoot((d) => {
      const users = [...(d.spec.auth?.local?.users ?? [])]
      users[idx] = next
      return {
        ...d,
        spec: {
          ...d.spec,
          auth: {
            ...d.spec.auth!,
            local: {
              ...d.spec.auth!.local!,
              users,
            },
          },
        },
      }
    })
  }

  function addUser() {
    updateRoot((d) => {
      const users = [...(d.spec.auth?.local?.users ?? [])]
      users.push({ username: '', password: '', role: 'readonly' })
      return {
        ...d,
        spec: {
          ...d.spec,
          auth: {
            ...d.spec.auth!,
            local: {
              ...d.spec.auth!.local!,
              users,
            },
          },
        },
      }
    })
    setPasswordEdits((prev) => ({ ...prev, [localUsers.length]: '' }))
  }

  function removeUser(idx: number) {
    updateRoot((d) => {
      const users = [...(d.spec.auth?.local?.users ?? [])]
      users.splice(idx, 1)
      return {
        ...d,
        spec: {
          ...d.spec,
          auth: {
            ...d.spec.auth!,
            local: {
              ...d.spec.auth!.local!,
              users,
            },
          },
        },
      }
    })
    setPasswordEdits((prev) => {
      const next: Record<number, string> = {}
      for (const [k, v] of Object.entries(prev)) {
        const n = Number(k)
        if (Number.isNaN(n) || n === idx) continue
        next[n > idx ? n - 1 : n] = v
      }
      return next
    })
  }

  async function save() {
    setStatus('Saving...')
    setErr('')
    try {
      if (cfgStatus?.config_read_only) throw new Error('Config is read-only (CONFIG_HTTP_URL).')
      if (!draft) throw new Error('No config loaded')
      const users = [...(draft.spec.auth?.local?.users ?? [])]
      const mergedUsers = users.map((u, idx) => {
        const newPw = (passwordEdits[idx] ?? '').trim()
        if (newPw) return { ...u, password: newPw }
        return u
      })
      const invalid = mergedUsers.some((u) => !u.username.trim() || !u.password || !u.role)
      if (invalid) throw new Error('All local auth users require username, password, and role.')
      const toSave: RootConfig = {
        ...draft,
        spec: {
          ...draft.spec,
          auth: {
            ...draft.spec.auth!,
            local: {
              ...draft.spec.auth!.local!,
              users: mergedUsers,
            },
          },
        },
      }
      const body = yaml.dump(toSave, { noRefs: true, lineWidth: 120 })
      await apiPutText('/v1/config', body)
      setStatus('Saved.')
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div>
      <div className="text-sm font-semibold text-slate-200">Auth Users</div>
      <p className="mt-1 text-sm text-slate-400">
        Create and manage local login accounts for admin/operator/readonly access. AD LDS users are controlled in your directory.
      </p>

      <div className="mt-4 flex flex-wrap items-center gap-4">
        <label className="flex items-center gap-2 text-sm text-slate-300">
          <input type="checkbox" checked={authEnabled} onChange={(e) => updateRoot((d) => ({ ...d, spec: { ...d.spec, auth: { ...d.spec.auth!, enabled: e.target.checked } } }))} />
          Enable authentication
        </label>
        <label className="flex items-center gap-2 text-sm text-slate-300">
          <input type="checkbox" checked={localEnabled} onChange={(e) => updateRoot((d) => ({ ...d, spec: { ...d.spec, auth: { ...d.spec.auth!, local: { ...d.spec.auth!.local!, enabled: e.target.checked } } } }))} />
          Enable local accounts
        </label>
      </div>

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="flex items-center justify-between">
          <div className="text-sm font-semibold">Local users</div>
          <button className="rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-200 hover:bg-slate-800" onClick={() => addUser()}>
            Add user
          </button>
        </div>
        <div className="mt-4 overflow-hidden rounded-lg border border-slate-800">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-xs text-slate-400">
              <tr>
                <th className="px-3 py-2">Username</th>
                <th className="px-3 py-2">Password</th>
                <th className="px-3 py-2">Role</th>
                <th className="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {localUsers.map((u, i) => (
                <tr key={`${u.username}-${i}`} className="border-t border-slate-800">
                  <td className="px-3 py-2">
                    <input className="w-40 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={u.username} onChange={(e) => updateUser(i, { ...u, username: e.target.value })} placeholder="admin" />
                  </td>
                  <td className="px-3 py-2">
                    <input
                      type="password"
                      className="w-44 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                      value={passwordEdits[i] ?? ''}
                      onChange={(e) => setPasswordEdits((prev) => ({ ...prev, [i]: e.target.value }))}
                      placeholder="Leave blank to keep existing"
                      autoComplete="new-password"
                    />
                  </td>
                  <td className="px-3 py-2">
                    <select className="w-32 rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100" value={u.role} onChange={(e) => updateUser(i, { ...u, role: e.target.value as AuthRole })}>
                      {roleOptions.map((r) => (
                        <option key={r} value={r}>{r}</option>
                      ))}
                    </select>
                  </td>
                  <td className="px-3 py-2 text-right">
                    <button className="rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900" onClick={() => removeUser(i)}>Remove</button>
                  </td>
                </tr>
              ))}
              {!localUsers.length ? (
                <tr>
                  <td className="px-3 py-3 text-slate-500" colSpan={4}>No local auth users configured.</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <div className="mt-6 flex items-center gap-3">
        <button className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm text-slate-200 hover:bg-slate-900" onClick={() => load().catch((e) => setErr(e instanceof Error ? e.message : String(e)))}>
          Refresh
        </button>
        <button className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50" disabled={Boolean(cfgStatus?.config_read_only)} onClick={() => save()}>
          Save auth users
        </button>
        {status ? <span className="text-sm text-slate-400">{status}</span> : null}
      </div>
      {cfgStatus?.config_read_only ? <div className="mt-4 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">Read-only config (CONFIG_HTTP_URL). Save is disabled.</div> : null}
      {err ? <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div> : null}
      <div className="mt-4 text-xs text-slate-500">Loaded config: <span className="font-mono">{cfg?.metadata?.name ?? '—'}</span></div>
    </div>
  )
}
