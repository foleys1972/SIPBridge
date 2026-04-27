import { useCallback, useEffect, useState } from 'react'
import { apiFetch } from '../api/client'
import type { RecorderDashboardSnapshot, ServiceDashboardResponse, ServiceDashboardRow } from '../api/types'

function statusBadge(status: string) {
  const s = status.toLowerCase()
  if (s === 'up') return 'bg-emerald-900/60 text-emerald-100 border-emerald-700'
  if (s === 'degraded') return 'bg-amber-900/50 text-amber-100 border-amber-700'
  if (s === 'down') return 'bg-rose-900/50 text-rose-100 border-rose-800'
  return 'bg-slate-800 text-slate-300 border-slate-700'
}

function ServiceTable({ rows, empty }: { rows: ServiceDashboardRow[]; empty?: string }) {
  if (!rows.length) {
    return <p className="text-sm text-slate-500">{empty ?? 'No rows.'}</p>
  }
  return (
    <div className="overflow-x-auto rounded-lg border border-slate-800">
      <table className="w-full min-w-[640px] text-left text-sm">
        <thead className="border-b border-slate-800 bg-slate-900/50 text-xs uppercase text-slate-500">
          <tr>
            <th className="px-3 py-2">Service</th>
            <th className="px-3 py-2">Status</th>
            <th className="px-3 py-2">Detail</th>
            <th className="px-3 py-2">Latency</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id} className="border-b border-slate-800/80 last:border-0">
              <td className="px-3 py-2.5 text-slate-200">{row.label}</td>
              <td className="px-3 py-2.5">
                <span
                  className={`inline-block rounded border px-2 py-0.5 text-xs font-medium capitalize ${statusBadge(row.status)}`}
                >
                  {row.status}
                </span>
              </td>
              <td className="max-w-md px-3 py-2.5 font-mono text-xs text-slate-400">{row.detail ?? '—'}</td>
              <td className="px-3 py-2.5 text-slate-500">{row.latency_ms != null ? `${row.latency_ms} ms` : '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function parseServicesFromPayload(payload: Record<string, unknown> | undefined): ServiceDashboardRow[] {
  if (!payload) return []
  const s = payload.services
  if (!Array.isArray(s)) return []
  return s.filter((x) => x && typeof x === 'object') as ServiceDashboardRow[]
}

function parseLogFromPayload(payload: Record<string, unknown> | undefined): { ts: string; level: string; message: string }[] {
  if (!payload) return []
  const l = payload.log
  if (!Array.isArray(l)) return []
  return l
    .filter((x) => x && typeof x === 'object')
    .map((e) => {
      const o = e as Record<string, unknown>
      return {
        ts: String(o.ts ?? ''),
        level: String(o.level ?? ''),
        message: String(o.message ?? ''),
      }
    })
}

export default function StackHealthPage() {
  const [data, setData] = useState<ServiceDashboardResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState('')
  const [auto, setAuto] = useState(true)

  const load = useCallback(async () => {
    setErr('')
    try {
      const d = await apiFetch<ServiceDashboardResponse>('/v1/dashboard/services')
      setData(d)
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load().catch(() => {})
  }, [load])

  useEffect(() => {
    if (!auto) return
    const t = window.setInterval(() => {
      load().catch(() => {})
    }, 15000)
    return () => window.clearInterval(t)
  }, [auto, load])

  if (loading && !data) {
    return <div className="text-sm text-slate-400">Loading stack checks…</div>
  }

  const rec = data?.recorder as RecorderDashboardSnapshot | null | undefined
  const recPayload = rec?.ok && rec.payload && typeof rec.payload === 'object' ? (rec.payload as Record<string, unknown>) : undefined
  const recServices = parseServicesFromPayload(recPayload)
  const recLog = parseLogFromPayload(recPayload)
  const bridgePayload =
    recPayload && recPayload.bridge && typeof recPayload.bridge === 'object'
      ? (recPayload.bridge as Record<string, unknown>)
      : undefined
  const bridgeOk = bridgePayload && bridgePayload.ok === true
  const bridgeInner = bridgePayload?.payload as Record<string, unknown> | undefined
  const bridgeServices = parseServicesFromPayload(bridgeInner)

  return (
    <div>
      <div className="text-xl font-semibold text-slate-100">Stack health</div>
      <p className="mt-1 text-sm text-slate-400">
        <strong className="text-slate-300">SIP Bridge</strong> probes run in this API process.
        Drachtio / rtpengine / PostgreSQL rows only appear when{' '}
        <span className="font-mono text-slate-500">spec.recording.siprec.enabled</span> is true.{' '}
        <strong className="text-slate-300">SIPREC recorder</strong> block is fetched from{' '}
        <span className="font-mono text-slate-500">SIPREC_RECORDER_BASE_URL</span> (default{' '}
        <span className="font-mono">http://127.0.0.1:3030</span>
        ); set to <span className="font-mono">none</span> to hide it.
      </p>

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <button
          type="button"
          className="rounded-md bg-sky-600 px-4 py-2 text-sm font-semibold text-white hover:bg-sky-500"
          onClick={() => {
            setLoading(true)
            load().finally(() => setLoading(false))
          }}
        >
          Refresh now
        </button>
        <label className="flex items-center gap-2 text-sm text-slate-300">
          <input type="checkbox" checked={auto} onChange={(e) => setAuto(e.target.checked)} />
          Auto-refresh every 15s
        </label>
        {data ? (
          <span className="text-sm text-slate-400">
            Bridge: {data.checked_at} — {data.summary_up}/{data.summary_total} up
            {rec?.ok ? ` · SIPREC fetch ${rec.latency_ms ?? '?'} ms` : rec && !rec.ok ? ` · SIPREC: ${rec.error}` : ''}
          </span>
        ) : null}
      </div>

      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
      ) : null}

      {data ? (
        <>
          <h3 className="mt-8 text-sm font-semibold uppercase tracking-wide text-slate-500">SIP Bridge (local)</h3>
          <div className="mt-2">
            <ServiceTable rows={data.services} />
          </div>

          <h3 className="mt-10 text-sm font-semibold uppercase tracking-wide text-slate-500">SIPREC recorder (Node)</h3>
          <p className="mt-1 text-xs text-slate-500">
            Pulled from the recorder&apos;s <span className="font-mono">/api/health/dashboard</span> (PostgreSQL, drachtio, rtpengine, optional SIP Bridge merge).
          </p>
          {!rec ? (
            <p className="mt-2 text-sm text-slate-500">Recorder block disabled (set SIPREC_RECORDER_BASE_URL=none to hide).</p>
          ) : !rec.ok ? (
            <div className="mt-2 rounded-lg border border-amber-900/50 bg-amber-950/30 p-3 text-sm text-amber-100">
              Could not load SIPREC dashboard: {rec.error ?? 'unknown'} ({rec.url})
            </div>
          ) : (
            <>
              <p className="mt-2 text-xs text-slate-500">
                {recPayload?.checked_at ? `Checked ${String(recPayload.checked_at)}` : ''}{' '}
                {recPayload?.summary_up != null && recPayload?.summary_total != null
                  ? ` — ${String(recPayload.summary_up)}/${String(recPayload.summary_total)} local checks up`
                  : ''}
              </p>
              <div className="mt-3">
                <ServiceTable rows={recServices} empty="No services in payload." />
              </div>

              {bridgeOk && bridgeServices.length > 0 ? (
                <>
                  <h4 className="mt-6 text-xs font-semibold uppercase tracking-wide text-slate-500">
                    SIP Bridge (via SIPREC merge)
                  </h4>
                  <p className="mt-1 text-xs text-slate-500">
                    Recorder fetched SIP Bridge from{' '}
                    <span className="font-mono">{String(bridgePayload?.source ?? '')}</span> (set SIPBRIDGE_DASHBOARD_URL on SIPREC).
                  </p>
                  <div className="mt-2">
                    <ServiceTable rows={bridgeServices} />
                  </div>
                </>
              ) : null}

              {recLog.length > 0 ? (
                <div className="mt-6">
                  <div className="text-sm font-semibold text-slate-200">SIPREC activity log</div>
                  <div className="mt-2 max-h-40 overflow-y-auto rounded-lg border border-slate-800 bg-slate-950 p-3 font-mono text-xs text-slate-400">
                    {[...recLog].reverse().map((e, i) => (
                      <div key={`${e.ts}-${i}`} className="border-b border-slate-800/50 py-1 last:border-0">
                        <span className="text-slate-500">{e.ts}</span> [{e.level}] {e.message}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}
            </>
          )}

          <div className="mt-10">
            <div className="text-sm font-semibold text-slate-200">SIP Bridge activity log</div>
            <p className="mt-1 text-xs text-slate-500">Recent refresh summaries (this API process).</p>
            <div className="mt-2 max-h-64 overflow-y-auto rounded-lg border border-slate-800 bg-slate-950 p-3 font-mono text-xs text-slate-400">
              {data.log.length === 0 ? (
                <span className="text-slate-600">No entries yet.</span>
              ) : (
                [...data.log].reverse().map((e, i) => (
                  <div key={`${e.ts}-${i}`} className="border-b border-slate-800/50 py-1 last:border-0">
                    <span className="text-slate-500">{e.ts}</span>{' '}
                    <span className={e.level === 'warn' ? 'text-amber-400' : 'text-slate-300'}>[{e.level}]</span> {e.message}
                  </div>
                ))
              )}
            </div>
          </div>

          {data.note ? <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-xs text-slate-500">{data.note}</div> : null}
        </>
      ) : null}
    </div>
  )
}
