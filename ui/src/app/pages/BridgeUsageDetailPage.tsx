import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, PhoneOff, RefreshCw, Unplug } from 'lucide-react'
import { apiFetch, apiPostJson } from '../api/client'
import type { BridgeCallInfo, BridgeDetailResponse } from '../api/types'
import { employeeIdFromCall } from '../lib/employeeId'
import { useAuth } from '../auth/AuthContext'
import Badge from '../components/Badge'

export default function BridgeUsageDetailPage() {
  const { bridgeId } = useParams<{ bridgeId: string }>()
  const { role } = useAuth()
  const canAct = role === 'admin' || role === 'operator'

  const [data, setData] = useState<BridgeDetailResponse | null>(null)
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!bridgeId) return
    const d = await apiFetch<BridgeDetailResponse>(`/v1/bridges/${encodeURIComponent(bridgeId)}`)
    setData(d)
    setErr('')
  }, [bridgeId])

  useEffect(() => {
    if (!bridgeId) return
    let alive = true
    setLoading(true)
    load()
      .catch((e) => {
        if (!alive) return
        setErr(e instanceof Error ? e.message : String(e))
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    const t = setInterval(() => {
      load().catch(() => {})
    }, 2000)
    return () => {
      alive = false
      clearInterval(t)
    }
  }, [bridgeId, load])

  async function disconnect(c: BridgeCallInfo) {
    if (!bridgeId || !canAct) return
    setStatus('Disconnecting…')
    setErr('')
    try {
      await apiPostJson<{ ok: boolean }>(`/v1/bridges/${encodeURIComponent(bridgeId)}/calls/drop`, {
        call_id: c.call_id,
        from_tag: c.from_tag,
      })
      setStatus('Disconnected.')
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  async function resetBridge() {
    if (!bridgeId || !canAct) return
    if (!confirm('Disconnect all participants on this bridge?')) return
    setStatus('Resetting bridge…')
    setErr('')
    try {
      await apiPostJson<{ ok: boolean }>(`/v1/bridges/${encodeURIComponent(bridgeId)}/reset`)
      setStatus('Bridge cleared.')
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  if (!bridgeId) {
    return <div className="text-slate-400">Missing bridge id.</div>
  }

  if (loading && !data) {
    return <div className="text-sm text-slate-400">Loading bridge…</div>
  }

  if (err && !data) {
    return (
      <div className="space-y-3">
        <Link to="/usage" className="inline-flex items-center gap-1 text-sm text-sky-400 hover:text-sky-300">
          <ArrowLeft size={16} /> Back to realtime usage
        </Link>
        <div className="rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
      </div>
    )
  }

  const b = data?.bridge

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Link to="/usage" className="inline-flex items-center gap-1 text-sm text-sky-400 hover:text-sky-300">
            <ArrowLeft size={16} /> Back to realtime usage
          </Link>
          <h1 className="mt-2 text-xl font-semibold text-slate-100">{b?.name ?? bridgeId}</h1>
          <div className="mt-1 font-mono text-xs text-slate-500">{bridgeId}</div>
          {b?.type ? (
            <div className="mt-1 text-xs text-slate-400">
              Type: <span className="text-slate-300">{b.type}</span>
            </div>
          ) : null}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone={(data?.calls?.length ?? 0) > 0 ? 'green' : 'slate'}>
            {data?.calls?.length ?? 0} active
          </Badge>
          {canAct ? (
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100 hover:bg-amber-950/70"
              onClick={() => resetBridge()}
            >
              <RefreshCw size={16} /> Reset bridge
            </button>
          ) : (
            <span className="text-xs text-slate-500">Switch role to operator/admin to reset or disconnect.</span>
          )}
        </div>
      </div>

      {err ? (
        <div className="rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
      ) : null}
      {status ? (
        <div className="rounded-lg border border-slate-800 bg-slate-900/50 px-3 py-2 text-sm text-slate-300">{status}</div>
      ) : null}

      <section className="rounded-xl border border-slate-800 bg-slate-950/60 p-4">
        <h2 className="text-sm font-semibold text-slate-200">Configured participants</h2>
        <p className="mt-1 text-xs text-slate-500">From config (expected endpoints). Active calls may include IVR joins.</p>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[640px] text-left text-sm">
            <thead className="border-b border-slate-800 text-xs uppercase text-slate-500">
              <tr>
                <th className="py-2 pr-3">ID</th>
                <th className="py-2 pr-3">Name</th>
                <th className="py-2 pr-3">SIP URI</th>
                <th className="py-2 pr-3">Pair / End</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/80 text-slate-300">
              {(b?.participants ?? []).length === 0 ? (
                <tr>
                  <td colSpan={4} className="py-4 text-slate-500">
                    No participants in config.
                  </td>
                </tr>
              ) : (
                (b?.participants ?? []).map((p) => (
                  <tr key={p.id}>
                    <td className="py-2 pr-3 font-mono text-xs">{p.id}</td>
                    <td className="py-2 pr-3">{p.display_name}</td>
                    <td className="py-2 pr-3 font-mono text-xs">{p.sip_uri}</td>
                    <td className="py-2 pr-3 text-xs">
                      {p.pair_id ?? '—'} / {p.end ?? '—'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="rounded-xl border border-slate-800 bg-slate-950/60 p-4">
        <h2 className="text-sm font-semibold text-slate-200">Active calls</h2>
        <p className="mt-1 text-xs text-slate-500">Live SIP legs on this bridge. Disconnect sends BYE to the participant.</p>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[900px] text-left text-sm">
            <thead className="border-b border-slate-800 text-xs uppercase text-slate-500">
              <tr>
                <th className="py-2 pr-3">Employee ID</th>
                <th className="py-2 pr-3">Name</th>
                <th className="py-2 pr-3">PIN</th>
                <th className="py-2 pr-3">From</th>
                <th className="py-2 pr-3">Remote</th>
                <th className="py-2 pr-3">Call-ID</th>
                <th className="py-2 pr-3">Since</th>
                <th className="py-2 pr-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/80 text-slate-300">
              {(data?.calls ?? []).length === 0 ? (
                <tr>
                  <td colSpan={8} className="py-4 text-slate-500">
                    No active calls.
                  </td>
                </tr>
              ) : (
                (data?.calls ?? []).map((c) => (
                  <tr key={`${c.call_id}-${c.from_tag}`}>
                    <td className="py-2 pr-3 font-mono text-xs text-slate-200">{employeeIdFromCall(c) || '—'}</td>
                    <td className="py-2 pr-3 text-slate-300">{c.display_name || '—'}</td>
                    <td className="py-2 pr-3 font-mono text-xs tracking-widest text-slate-500">{c.pin_masked || '—'}</td>
                    <td className="max-w-[200px] truncate py-2 pr-3 font-mono text-xs" title={c.from_uri}>
                      {c.from_uri || '—'}
                    </td>
                    <td className="py-2 pr-3 font-mono text-xs">{c.remote_addr || '—'}</td>
                    <td className="max-w-[180px] truncate py-2 pr-3 font-mono text-[10px] text-slate-400" title={c.call_id}>
                      {c.call_id}
                    </td>
                    <td className="py-2 pr-3 text-xs text-slate-400">
                      {c.created_at ? new Date(c.created_at).toLocaleString() : '—'}
                    </td>
                    <td className="py-2 pr-3 text-right">
                      {canAct ? (
                        <button
                          type="button"
                          className="inline-flex items-center gap-1 rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900"
                          onClick={() => disconnect(c)}
                        >
                          <Unplug size={14} /> Disconnect
                        </button>
                      ) : (
                        <span className="text-xs text-slate-600">—</span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <div className="rounded-lg border border-slate-800 bg-slate-900/30 p-3 text-xs text-slate-500">
        <PhoneOff className="mb-1 inline" size={14} /> Reset bridge disconnects every active call on this bridge. Individual disconnect
        affects one leg only.
      </div>
    </div>
  )
}
