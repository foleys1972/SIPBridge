import { useCallback, useEffect, useMemo, useState } from 'react'
import { apiDelete, apiFetch, apiPutJson } from '../api/client'
import type { ConfigStatus, DatabaseSettingsResponse, DatabaseSpec, PostgresSpec } from '../api/types'

const sslModes = ['disable', 'allow', 'prefer', 'require', 'verify-ca', 'verify-full'] as const

function defaultPostgres(): PostgresSpec {
  return {
    host: '',
    port: 5432,
    user: '',
    database: '',
    ssl_mode: 'require',
    password_env_var: 'POSTGRES_PASSWORD',
    schema: 'public',
  }
}

function normalizeSaved(s: DatabaseSpec | null): { storage: DatabaseSpec['config_storage']; pg: PostgresSpec } {
  if (!s) {
    return { storage: 'yaml', pg: defaultPostgres() }
  }
  const storage = s.config_storage ?? 'yaml'
  const p = s.postgres
  const pg: PostgresSpec = p
    ? {
        host: p.host ?? '',
        port: p.port > 0 ? p.port : 5432,
        user: p.user ?? '',
        database: p.database ?? '',
        ssl_mode: p.ssl_mode || 'require',
        password_env_var: p.password_env_var || 'POSTGRES_PASSWORD',
        schema: p.schema || 'public',
      }
    : defaultPostgres()
  return { storage, pg }
}

export default function DatabaseSettingsPage() {
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')
  const [cfgStatus, setCfgStatus] = useState<ConfigStatus | null>(null)
  const [apiNote, setApiNote] = useState('')
  const [envInfo, setEnvInfo] = useState<DatabaseSettingsResponse['env'] | null>(null)
  const [storage, setStorage] = useState<DatabaseSpec['config_storage']>('yaml')
  const [pg, setPg] = useState<PostgresSpec>(() => defaultPostgres())

  const load = useCallback(async () => {
    setLoading(true)
    setErr('')
    const [data, st] = await Promise.all([
      apiFetch<DatabaseSettingsResponse>('/v1/settings/database'),
      apiFetch<ConfigStatus>('/v1/config/status').catch(() => null),
    ])
    setApiNote(data.note ?? '')
    setEnvInfo(data.env ?? null)
    const n = normalizeSaved(data.saved)
    setStorage(n.storage)
    setPg(n.pg)
    if (st) setCfgStatus(st)
    setLoading(false)
  }, [])

  useEffect(() => {
    load().catch((e) => {
      setErr(e instanceof Error ? e.message : String(e))
      setLoading(false)
    })
  }, [load])

  const bodyToSave = useMemo((): DatabaseSpec => {
    const out: DatabaseSpec = { config_storage: storage }
    if (storage === 'postgres') {
      out.postgres = {
        host: pg.host.trim(),
        port: Number(pg.port) || 5432,
        user: pg.user.trim(),
        database: pg.database.trim(),
        ssl_mode: (pg.ssl_mode || 'require').trim(),
        password_env_var: (pg.password_env_var || 'POSTGRES_PASSWORD').trim() || 'POSTGRES_PASSWORD',
        schema: (pg.schema || 'public').trim() || 'public',
      }
    }
    return out
  }, [storage, pg])

  async function save() {
    setErr('')
    setStatus('Saving…')
    try {
      if (cfgStatus?.config_read_only) {
        setErr('Config is read-only (CONFIG_HTTP_URL).')
        setStatus('')
        return
      }
      await apiPutJson('/v1/settings/database', bodyToSave)
      setStatus('Saved.')
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  async function clearSaved() {
    if (!window.confirm('Remove saved database settings from config?')) return
    setErr('')
    try {
      await apiDelete('/v1/settings/database')
      setStatus('Cleared.')
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  if (loading) {
    return <div className="text-sm text-slate-400">Loading…</div>
  }

  return (
    <div>
      <div className="text-sm font-semibold text-slate-200">Configuration storage</div>
      <p className="mt-1 text-sm text-slate-400">
        Choose how this deployment is intended to store SIPBridge configuration. Values are persisted in{' '}
        <span className="font-mono text-slate-300">spec.database</span> in your YAML file. Database passwords must not be
        stored here—use the environment variable named below or <span className="font-mono">DATABASE_URL</span> on the
        process.
      </p>

      {envInfo ? (
        <div className="mt-3 flex flex-wrap gap-4 text-xs text-slate-500">
          <span>
            <span className="text-slate-400">CONFIG_HTTP_URL</span> active:{' '}
            {envInfo.config_http_url_set ? 'yes' : 'no'}
          </span>
          <span>
            <span className="text-slate-400">DATABASE_URL</span> set: {envInfo.database_url_set ? 'yes' : 'no'}
          </span>
        </div>
      ) : null}

      {cfgStatus?.config_read_only ? (
        <div className="mt-4 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">
          Read-only config (CONFIG_HTTP_URL). Changes cannot be applied from this UI.
        </div>
      ) : null}

      <div className="mt-6 space-y-4">
        <label className="flex flex-col gap-2 text-sm text-slate-300">
          <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Config source</span>
          <select
            className="max-w-md rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-slate-100"
            value={storage}
            onChange={(e) => setStorage(e.target.value as DatabaseSpec['config_storage'])}
            disabled={Boolean(cfgStatus?.config_read_only)}
          >
            <option value="yaml">Local YAML file (CONFIG_PATH)</option>
            <option value="http">Remote HTTP URL (CONFIG_HTTP_URL)</option>
            <option value="postgres">PostgreSQL (enterprise — connection metadata)</option>
          </select>
        </label>

        {storage === 'http' ? (
          <div className="rounded-lg border border-slate-800 bg-slate-900/50 p-3 text-sm text-slate-400">
            Point all nodes at the same YAML using <span className="font-mono text-slate-300">CONFIG_HTTP_URL</span> in the
            environment. The API will not write to disk while that variable is set.
          </div>
        ) : null}

        {storage === 'postgres' ? (
          <div className="grid max-w-2xl grid-cols-1 gap-4 rounded-xl border border-slate-800 bg-slate-950 p-4 md:grid-cols-2">
            <label className="flex flex-col gap-1 text-xs text-slate-400 md:col-span-2">
              Host
              <input
                className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                value={pg.host}
                onChange={(e) => setPg((p) => ({ ...p, host: e.target.value }))}
                placeholder="db.internal.example.com"
                disabled={Boolean(cfgStatus?.config_read_only)}
              />
            </label>
            <label className="flex flex-col gap-1 text-xs text-slate-400">
              Port
              <input
                type="number"
                className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                value={pg.port}
                onChange={(e) => setPg((p) => ({ ...p, port: Number(e.target.value) || 0 }))}
                min={1}
                max={65535}
                disabled={Boolean(cfgStatus?.config_read_only)}
              />
            </label>
            <label className="flex flex-col gap-1 text-xs text-slate-400">
              Database name
              <input
                className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                value={pg.database}
                onChange={(e) => setPg((p) => ({ ...p, database: e.target.value }))}
                placeholder="sipbridge"
                disabled={Boolean(cfgStatus?.config_read_only)}
              />
            </label>
            <label className="flex flex-col gap-1 text-xs text-slate-400">
              User
              <input
                className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                value={pg.user}
                onChange={(e) => setPg((p) => ({ ...p, user: e.target.value }))}
                placeholder="sipbridge_app"
                disabled={Boolean(cfgStatus?.config_read_only)}
              />
            </label>
            <label className="flex flex-col gap-1 text-xs text-slate-400">
              SSL mode
              <select
                className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 text-sm text-slate-100"
                value={pg.ssl_mode || 'require'}
                onChange={(e) => setPg((p) => ({ ...p, ssl_mode: e.target.value }))}
                disabled={Boolean(cfgStatus?.config_read_only)}
              >
                {sslModes.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex flex-col gap-1 text-xs text-slate-400">
              Password env var
              <input
                className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                value={pg.password_env_var ?? ''}
                onChange={(e) => setPg((p) => ({ ...p, password_env_var: e.target.value }))}
                placeholder="POSTGRES_PASSWORD"
                disabled={Boolean(cfgStatus?.config_read_only)}
              />
            </label>
            <label className="flex flex-col gap-1 text-xs text-slate-400">
              Schema (optional)
              <input
                className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                value={pg.schema ?? ''}
                onChange={(e) => setPg((p) => ({ ...p, schema: e.target.value }))}
                placeholder="public"
                disabled={Boolean(cfgStatus?.config_read_only)}
              />
            </label>
            <div className="md:col-span-2 rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-xs text-slate-500">
              Runtime wiring: set <span className="font-mono">DATABASE_URL</span> or{' '}
              <span className="font-mono">POSTGRES_*</span> / the password variable above on the sipbridge service. This
              screen records the intended connection for operators; the binary does not require PostgreSQL yet unless you
              enable it in deployment.
            </div>
          </div>
        ) : null}

        {storage === 'yaml' ? (
          <div className="rounded-lg border border-slate-800 bg-slate-900/50 p-3 text-sm text-slate-400">
            Default: configuration is read from <span className="font-mono text-slate-300">CONFIG_PATH</span> (typically{' '}
            <span className="font-mono">config.yaml</span>).
          </div>
        ) : null}
      </div>

      <div className="mt-8 flex flex-wrap items-center gap-3">
        <button
          type="button"
          className="rounded-md bg-sky-600 px-4 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50"
          disabled={Boolean(cfgStatus?.config_read_only)}
          onClick={() => save()}
        >
          Save
        </button>
        <button
          type="button"
          className="rounded-md border border-slate-700 bg-slate-900 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800 disabled:opacity-50"
          disabled={Boolean(cfgStatus?.config_read_only)}
          onClick={() => clearSaved()}
        >
          Clear saved database block
        </button>
        {status ? <span className="text-sm text-slate-400">{status}</span> : null}
      </div>

      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
      ) : null}

      <div className="mt-8 rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-xs text-slate-500">
        {apiNote}
      </div>
    </div>
  )
}
