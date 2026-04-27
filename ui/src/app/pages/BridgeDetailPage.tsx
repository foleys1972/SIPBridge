import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiFetch, apiPostText } from '../api/client'
import type { Bridge, BridgeCallInfo, RootConfig } from '../api/types'
import { employeeIdFromCall } from '../lib/employeeId'
import { useAuth } from '../auth/AuthContext'
import Badge from '../components/Badge'

type CallsResponse = { bridge_id: string; calls: BridgeCallInfo[] }

export default function BridgeDetailPage() {
  const params = useParams()
  const bridgeId = params.bridgeId ?? ''
  const { role } = useAuth()
  const canAct = role === 'admin' || role === 'operator'

  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [calls, setCalls] = useState<BridgeCallInfo[]>([])
  const [status, setStatus] = useState<string>('')
  const [err, setErr] = useState<string>('')

  const bridge = useMemo<Bridge | null>(() => {
    if (!cfg) return null
    return (cfg.spec.bridges ?? []).find((b) => b.id === bridgeId) ?? null
  }, [cfg, bridgeId])

  useEffect(() => {
    let alive = true
    async function load() {
      try {
        const c = await apiFetch<RootConfig>('/v1/config')
        const cr = await apiFetch<CallsResponse>(`/v1/bridges/${encodeURIComponent(bridgeId)}/calls`)
        if (!alive) return
        setCfg(c)
        setCalls(cr.calls ?? [])
        setErr('')
      } catch (e) {
        if (!alive) return
        setErr(e instanceof Error ? e.message : String(e))
      }
    }
    if (!bridgeId) return
    load()
    const t = setInterval(load, 2000)
    return () => {
      alive = false
      clearInterval(t)
    }
  }, [bridgeId])

  async function drop(call: BridgeCallInfo) {
    setStatus('Dropping...')
    setErr('')
    try {
      await apiPostText(`/v1/bridges/${encodeURIComponent(bridgeId)}/calls/drop`, JSON.stringify({ call_id: call.call_id, from_tag: call.from_tag }))
      const cr = await apiFetch<CallsResponse>(`/v1/bridges/${encodeURIComponent(bridgeId)}/calls`)
      setCalls(cr.calls ?? [])
      setStatus('')
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
      setStatus('')
    }
  }

  return (
    <div>
      <div className="flex items-start justify-between">
        <div>
          <div className="text-xl font-semibold">Bridge</div>
          <div className="mt-1 text-sm text-slate-400">
            <Link className="text-slate-300 hover:text-slate-100" to="/bridges">
              Bridges & Lines
            </Link>
            <span className="mx-2 text-slate-600">/</span>
            <span className="font-mono text-xs text-slate-300">{bridgeId}</span>
          </div>
        </div>
        <Badge tone={calls.length ? 'blue' : 'amber'}>{calls.length} active</Badge>
      </div>

      {bridge ? (
        <div className="mt-4 rounded-xl border border-slate-800 bg-slate-950 p-4">
          <div className="text-sm font-semibold">Details</div>
          <div className="mt-2 grid grid-cols-1 gap-2 text-sm md:grid-cols-2">
            <div>
              <div className="text-xs text-slate-500">Name</div>
              <div className="text-slate-200">{bridge.name}</div>
            </div>
            <div>
              <div className="text-xs text-slate-500">Configured participants</div>
              <div className="text-slate-200">{bridge.participants?.length ?? 0}</div>
            </div>
            <div>
              <div className="text-xs text-slate-500">Record bridge calls</div>
              <div className="text-slate-200">
                {bridge.recording_enabled === false ? 'Off' : 'On'}{' '}
                <span className="text-xs text-slate-500">(edit under Configuration → Bridges)</span>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
      ) : null}

      {status ? <div className="mt-4 text-sm text-slate-400">{status}</div> : null}

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="text-sm font-semibold">Active Connections</div>
          {!canAct ? (
            <span className="text-xs text-slate-500">Switch role to operator or admin to drop calls.</span>
          ) : null}
        </div>
        <div className="mt-3 overflow-x-auto rounded-lg border border-slate-800">
          <table className="w-full min-w-[900px] text-left text-sm">
            <thead className="bg-slate-900 text-xs text-slate-400">
              <tr>
                <th className="px-3 py-2">Employee ID</th>
                <th className="px-3 py-2">Name</th>
                <th className="px-3 py-2">PIN</th>
                <th className="px-3 py-2">From</th>
                <th className="px-3 py-2">Remote</th>
                <th className="px-3 py-2">Call-ID</th>
                <th className="px-3 py-2">Started</th>
                <th className="sticky right-0 z-10 bg-slate-900 px-3 py-2 text-right shadow-[-8px_0_12px_-4px_rgba(0,0,0,0.4)]">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody>
              {calls.map((c) => (
                <tr key={`${c.call_id}|${c.from_tag}`} className="border-t border-slate-800">
                  <td className="px-3 py-2 font-mono text-xs text-slate-200">
                    {employeeIdFromCall(c) || '—'}
                  </td>
                  <td className="px-3 py-2 text-slate-300">{c.display_name || '—'}</td>
                  <td className="px-3 py-2 font-mono text-xs tracking-widest text-slate-400">{c.pin_masked || '—'}</td>
                  <td className="px-3 py-2 font-mono text-xs text-slate-200">{c.from_uri}</td>
                  <td className="px-3 py-2 font-mono text-xs text-slate-300">{c.remote_addr}</td>
                  <td className="px-3 py-2 font-mono text-xs text-slate-300">{c.call_id}</td>
                  <td className="px-3 py-2 text-slate-300">{new Date(c.created_at).toLocaleString()}</td>
                  <td className="sticky right-0 z-10 border-l border-slate-800/80 bg-slate-950 px-3 py-2 text-right">
                    {canAct ? (
                      <button
                        type="button"
                        className="rounded-md border border-rose-900/60 bg-rose-950 px-2 py-1 text-xs text-rose-200 hover:bg-rose-900/30"
                        onClick={() => drop(c)}
                      >
                        Drop
                      </button>
                    ) : (
                      <span className="text-xs text-slate-600">—</span>
                    )}
                  </td>
                </tr>
              ))}
              {!calls.length ? (
                <tr>
                  <td className="px-3 py-3 text-slate-500" colSpan={8}>
                    No active calls.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
