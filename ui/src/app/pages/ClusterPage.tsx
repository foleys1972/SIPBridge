import { useCallback, useEffect, useState } from 'react'
import { apiDelete, apiFetch, apiPutJson } from '../api/client'
import type {
  ClusterSettingsResponse,
  ClusterSpec,
  ClusterSummaryResponse,
} from '../api/types'

type CapacityRow = {
  active_dialogs?: number
  max_concurrent_calls?: number
  soft_max_concurrent_calls?: number
  load_ratio?: number
  soft_load_ratio?: number
  accepting_new_calls?: boolean
  soft_overloaded?: boolean
  hard_overloaded?: boolean
  local_instance_id?: string
}

function numOrDash(v: unknown): string {
  if (typeof v === 'number' && !Number.isNaN(v)) return String(v)
  return '—'
}

function boolOrDash(v: unknown): string {
  if (typeof v === 'boolean') return v ? 'yes' : 'no'
  return '—'
}

function emptyClusterForm(saved: ClusterSpec | null, effective: ClusterSettingsResponse['effective']): ClusterSpec {
  const s = saved ?? {}
  return {
    max_concurrent_calls: s.max_concurrent_calls ?? effective.max_concurrent_calls ?? 0,
    soft_max_concurrent_calls: s.soft_max_concurrent_calls ?? effective.soft_max_concurrent_calls ?? 0,
    overflow_redirect_enabled: s.overflow_redirect_enabled ?? effective.overflow_redirect_enabled ?? false,
    overflow_redirect_sip_uri: s.overflow_redirect_sip_uri ?? effective.overflow_redirect_sip_uri ?? '',
  }
}

export default function ClusterPage() {
  const [cap, setCap] = useState<CapacityRow | null>(null)
  const [clusterSettings, setClusterSettings] = useState<ClusterSettingsResponse | null>(null)
  const [form, setForm] = useState<ClusterSpec>({})
  const [summary, setSummary] = useState<ClusterSummaryResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [probing, setProbing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')
  const [cfgReadOnly, setCfgReadOnly] = useState(false)

  const loadLocal = useCallback(async () => {
    setErr('')
    const [c, cf, st] = await Promise.all([
      apiFetch<CapacityRow>('/v1/capacity'),
      apiFetch<ClusterSettingsResponse>('/v1/settings/cluster').catch(() => null),
      apiFetch<{ config_read_only?: boolean }>('/v1/config/status').catch(() => null),
    ])
    setCap(c)
    setClusterSettings(cf)
    if (cf) {
      setForm(emptyClusterForm(cf.saved, cf.effective))
    }
    setCfgReadOnly(Boolean(st?.config_read_only))
  }, [])

  useEffect(() => {
    setLoading(true)
    loadLocal()
      .catch((e) => setErr(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false))
  }, [loadLocal])

  async function probePeers() {
    setProbing(true)
    setErr('')
    try {
      const s = await apiFetch<ClusterSummaryResponse>('/v1/cluster/summary?probe=true')
      setSummary(s)
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setProbing(false)
    }
  }

  async function saveCluster() {
    setErr('')
    setSaving(true)
    setStatus('')
    try {
      if (cfgReadOnly) {
        setErr('Config is read-only (CONFIG_HTTP_URL).')
        setSaving(false)
        return
      }
      const body: ClusterSpec = {
        max_concurrent_calls: Number(form.max_concurrent_calls ?? 0),
        soft_max_concurrent_calls: Number(form.soft_max_concurrent_calls ?? 0),
        overflow_redirect_enabled: Boolean(form.overflow_redirect_enabled),
        overflow_redirect_sip_uri: (form.overflow_redirect_sip_uri ?? '').trim() || undefined,
      }
      await apiPutJson('/v1/settings/cluster', body)
      setStatus('Saved. Restart this process so new SIP dialogs use the updated limits.')
      await loadLocal()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  async function clearCluster() {
    if (!window.confirm('Remove saved cluster limits from config? Environment defaults apply after restart.')) return
    setErr('')
    try {
      await apiDelete('/v1/settings/cluster')
      setStatus('Cleared saved spec.cluster.')
      await loadLocal()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  const local = summary?.local ?? null
  const peers = summary?.peers ?? []
  const eff = clusterSettings?.effective

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold text-slate-100">Cluster &amp; capacity</h1>
        <p className="mt-1 text-sm text-slate-400">
          Put your SBC or carrier in front for signaling HA. Tune admission control below; values are stored in config and
          merged with environment at startup—restart after changes so the SIP stack applies new limits.
        </p>
      </div>

      {err ? (
        <div className="rounded-md border border-red-900 bg-red-950/40 px-3 py-2 text-sm text-red-200">{err}</div>
      ) : null}

      {loading ? (
        <div className="text-sm text-slate-400">Loading…</div>
      ) : (
        <>
          <section className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
            <div className="text-sm font-semibold text-slate-200">This node (live)</div>
            <dl className="mt-3 grid grid-cols-1 gap-2 text-sm sm:grid-cols-2 lg:grid-cols-3">
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Instance</dt>
                <dd className="font-mono text-slate-200">{cap?.local_instance_id || '—'}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Active dialogs</dt>
                <dd className="font-mono text-slate-200">{numOrDash(cap?.active_dialogs)}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Max concurrent</dt>
                <dd className="font-mono text-slate-200">{numOrDash(cap?.max_concurrent_calls)}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Soft max</dt>
                <dd className="font-mono text-slate-200">{numOrDash(cap?.soft_max_concurrent_calls)}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Load ratio</dt>
                <dd className="font-mono text-slate-200">
                  {typeof cap?.load_ratio === 'number' ? cap.load_ratio.toFixed(2) : '—'}
                </dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Accepting new calls</dt>
                <dd className="text-slate-200">{boolOrDash(cap?.accepting_new_calls)}</dd>
              </div>
            </dl>
          </section>

          <section className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
            <div className="text-sm font-semibold text-slate-200">Effective cluster limits (runtime)</div>
            <p className="mt-1 text-xs text-slate-500">
              What the SIP process is using now. After editing saved limits below, restart the process to align these values.
            </p>
            <dl className="mt-3 grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Max concurrent calls</dt>
                <dd className="font-mono text-slate-200">{eff ? numOrDash(eff.max_concurrent_calls) : '—'}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Soft max</dt>
                <dd className="font-mono text-slate-200">{eff ? numOrDash(eff.soft_max_concurrent_calls) : '—'}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-500">Overflow redirect</dt>
                <dd className="text-slate-200">{eff ? boolOrDash(eff.overflow_redirect_enabled) : '—'}</dd>
              </div>
              <div className="sm:col-span-2">
                <dt className="text-xs uppercase tracking-wide text-slate-500">Overflow SIP URI</dt>
                <dd className="break-all font-mono text-xs text-slate-300">{eff?.overflow_redirect_sip_uri || '—'}</dd>
              </div>
            </dl>
          </section>

          <section className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
            <div className="text-sm font-semibold text-slate-200">Saved cluster limits (config)</div>
            {clusterSettings?.note ? <p className="mt-1 text-xs text-slate-500">{clusterSettings.note}</p> : null}
            <div className="mt-4 grid max-w-xl grid-cols-1 gap-4">
              <label className="flex flex-col gap-1 text-xs text-slate-400">
                Max concurrent calls
                <input
                  type="number"
                  min={0}
                  className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                  value={form.max_concurrent_calls ?? ''}
                  onChange={(e) =>
                    setForm((f) => ({
                      ...f,
                      max_concurrent_calls: e.target.value === '' ? undefined : Number(e.target.value),
                    }))
                  }
                  disabled={cfgReadOnly}
                />
              </label>
              <label className="flex flex-col gap-1 text-xs text-slate-400">
                Soft max concurrent calls
                <input
                  type="number"
                  min={0}
                  className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                  value={form.soft_max_concurrent_calls ?? ''}
                  onChange={(e) =>
                    setForm((f) => ({
                      ...f,
                      soft_max_concurrent_calls: e.target.value === '' ? undefined : Number(e.target.value),
                    }))
                  }
                  disabled={cfgReadOnly}
                />
              </label>
              <label className="flex items-center gap-2 text-sm text-slate-300">
                <input
                  type="checkbox"
                  checked={Boolean(form.overflow_redirect_enabled)}
                  onChange={(e) => setForm((f) => ({ ...f, overflow_redirect_enabled: e.target.checked }))}
                  disabled={cfgReadOnly}
                />
                Overflow redirect enabled (302 to URI when at capacity)
              </label>
              <label className="flex flex-col gap-1 text-xs text-slate-400">
                Overflow redirect SIP URI
                <input
                  className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                  value={form.overflow_redirect_sip_uri ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, overflow_redirect_sip_uri: e.target.value }))}
                  placeholder="sip:overflow@sbc.example.com"
                  disabled={cfgReadOnly}
                />
              </label>
            </div>
            <div className="mt-6 flex flex-wrap gap-2">
              <button
                type="button"
                className="rounded-md bg-sky-600 px-4 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50"
                disabled={cfgReadOnly || saving}
                onClick={() => saveCluster()}
              >
                {saving ? 'Saving…' : 'Save cluster limits'}
              </button>
              <button
                type="button"
                className="rounded-md border border-slate-700 bg-slate-900 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800 disabled:opacity-50"
                disabled={cfgReadOnly}
                onClick={() => clearCluster()}
              >
                Clear saved cluster block
              </button>
            </div>
            {status ? <p className="mt-3 text-sm text-slate-400">{status}</p> : null}
          </section>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              className="rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-sm text-slate-100 hover:bg-slate-800 disabled:opacity-50"
              onClick={() => probePeers()}
              disabled={probing}
            >
              {probing ? 'Probing peers…' : 'Probe peers (capacity)'}
            </button>
            <button
              type="button"
              className="rounded-md border border-slate-700 px-3 py-1.5 text-sm text-slate-400 hover:bg-slate-900"
              onClick={() => {
                setSummary(null)
                loadLocal().catch(() => {})
              }}
            >
              Refresh local
            </button>
          </div>

          {local && typeof local === 'object' ? (
            <section className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
              <div className="text-sm font-semibold text-slate-200">Summary (local from last probe)</div>
              <dl className="mt-3 grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
                {'local_instance_id' in local ? (
                  <div>
                    <dt className="text-xs uppercase tracking-wide text-slate-500">Instance</dt>
                    <dd className="font-mono text-xs text-slate-300">{String(local.local_instance_id ?? '—')}</dd>
                  </div>
                ) : null}
                {'active_dialogs' in local ? (
                  <div>
                    <dt className="text-xs uppercase tracking-wide text-slate-500">Active dialogs</dt>
                    <dd className="font-mono text-xs text-slate-300">{numOrDash(local.active_dialogs)}</dd>
                  </div>
                ) : null}
              </dl>
              {'cluster_limits' in local && local.cluster_limits && typeof local.cluster_limits === 'object' ? (
                <div className="mt-4">
                  <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">Cluster limits (probe)</div>
                  <dl className="mt-2 grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
                    {Object.entries(local.cluster_limits as Record<string, unknown>).map(([k, v]) => (
                      <div key={k}>
                        <dt className="text-xs uppercase tracking-wide text-slate-500">{k.replace(/_/g, ' ')}</dt>
                        <dd className="font-mono text-xs text-slate-300 break-all">{String(v)}</dd>
                      </div>
                    ))}
                  </dl>
                </div>
              ) : null}
            </section>
          ) : null}

          {peers.length > 0 ? (
            <section className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
              <div className="text-sm font-semibold text-slate-200">Peers</div>
              <div className="mt-2 overflow-x-auto">
                <table className="min-w-full text-left text-xs">
                  <thead className="border-b border-slate-800 text-slate-500">
                    <tr>
                      <th className="py-1 pr-3">Id</th>
                      <th className="py-1 pr-3">Latency</th>
                      <th className="py-1 pr-3">Active</th>
                      <th className="py-1 pr-3">Load</th>
                      <th className="py-1 pr-3">Accepting</th>
                      <th className="py-1 pr-3">Error</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800 text-slate-300">
                    {peers.map((p, i) => {
                      const row = p as Record<string, unknown>
                      const c = row.capacity as Record<string, unknown> | undefined
                      return (
                        <tr key={i}>
                          <td className="py-2 pr-3 font-mono">{String(row.id ?? '')}</td>
                          <td className="py-2 pr-3">{String(row.capacity_latency_ms ?? '—')}</td>
                          <td className="py-2 pr-3">{c ? String(c.active_dialogs ?? '—') : '—'}</td>
                          <td className="py-2 pr-3">{c ? Number(c.load_ratio ?? 0).toFixed(2) : '—'}</td>
                          <td className="py-2 pr-3">{c ? String(c.accepting_new_calls ?? '—') : '—'}</td>
                          <td className="py-2 pr-3 text-red-300/90">{String(row.capacity_error ?? '')}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </section>
          ) : null}
        </>
      )}
    </div>
  )
}
