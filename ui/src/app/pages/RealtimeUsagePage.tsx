import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Activity, ChevronRight } from 'lucide-react'
import { apiFetch } from '../api/client'
import type { BridgeListResponse, ConferenceGroupsUsageResponse, SIPStats } from '../api/types'
import Badge from '../components/Badge'
import KpiCard from '../components/KpiCard'

export default function RealtimeUsagePage() {
  const [stats, setStats] = useState<SIPStats | null>(null)
  const [bridges, setBridges] = useState<BridgeListResponse | null>(null)
  const [confUsage, setConfUsage] = useState<ConferenceGroupsUsageResponse | null>(null)
  const [err, setErr] = useState<string>('')

  const lastPacket = useMemo(() => {
    if (!stats?.last_packet_at) return '—'
    const d = new Date(stats.last_packet_at)
    if (Number.isNaN(d.getTime())) return stats.last_packet_at
    return d.toLocaleString()
  }, [stats?.last_packet_at])

  const totalActive = useMemo(() => {
    if (!bridges?.bridges?.length) return 0
    return bridges.bridges.reduce((s, b) => s + (b.active_calls ?? 0), 0)
  }, [bridges])

  const confSessionCount = confUsage?.sessions?.length ?? 0

  useEffect(() => {
    let alive = true
    async function load() {
      try {
        const [s, bl, cg] = await Promise.all([
          apiFetch<SIPStats>('/v1/sip/stats'),
          apiFetch<BridgeListResponse>('/v1/bridges'),
          apiFetch<ConferenceGroupsUsageResponse>('/v1/conference-groups/usage').catch(() => null),
        ])
        if (!alive) return
        setStats(s)
        setBridges(bl)
        if (cg) setConfUsage(cg)
        setErr('')
      } catch (e) {
        setErr(e instanceof Error ? e.message : String(e))
      }
    }
    load()
    const t = setInterval(load, 2000)
    return () => {
      alive = false
      clearInterval(t)
    }
  }, [])

  return (
    <div>
      <div className="flex items-start justify-between gap-6">
        <div>
          <div className="text-xl font-semibold">Realtime Usage</div>
          <div className="mt-1 text-sm text-slate-400">
            Live SIP counters, bridge activity, and conference line group sessions (polls every 2s). Open a bridge to manage
            participants or reset the room.
          </div>
        </div>
        <Badge tone={stats?.started ? 'green' : 'amber'}>
          {stats?.started ? 'live' : 'stopped'}
        </Badge>
      </div>

      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">
          {err}
        </div>
      ) : null}

      <div className="mt-6 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <KpiCard title="Packets Rx" value={stats?.packets_rx ?? '—'} />
        <KpiCard title="Bytes Rx" value={stats?.bytes_rx ?? '—'} />
        <KpiCard title="Last Packet" value={lastPacket} />
        <KpiCard title="Bridges (config)" value={bridges?.bridges?.length ?? '—'} hint="Active calls (all bridges)" />
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        <KpiCard title="Active calls (total)" value={totalActive} />
        <KpiCard
          title="Conference group sessions"
          value={confSessionCount}
          hint="IVR dial-in or direct INVITE fanouts"
        />
      </div>

      <section className="mt-8">
        <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-200">
          <Activity size={16} /> Conference line groups (live)
        </div>
        <div className="overflow-x-auto rounded-xl border border-slate-800 bg-slate-950/60">
          <table className="w-full min-w-[880px] text-left text-sm">
            <thead className="border-b border-slate-800 text-xs uppercase text-slate-500">
              <tr>
                <th className="px-4 py-3">Group id</th>
                <th className="px-4 py-3">Source</th>
                <th className="px-4 py-3">Phase</th>
                <th className="px-4 py-3">Side</th>
                <th className="px-4 py-3">Type</th>
                <th className="px-4 py-3">Fanout legs</th>
                <th className="px-4 py-3">Winner</th>
                <th className="px-4 py-3">Region</th>
                <th className="px-4 py-3">Remote</th>
                <th className="px-4 py-3">Since</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/80">
              {(confUsage?.sessions ?? []).length === 0 ? (
                <tr>
                  <td colSpan={10} className="px-4 py-8 text-center text-slate-500">
                    No active conference group sessions.
                  </td>
                </tr>
              ) : (
                (confUsage?.sessions ?? []).map((row, i) => (
                  <tr key={`${row.session_ref}-${i}`} className="text-slate-300 hover:bg-slate-900/40">
                    <td className="px-4 py-3 font-mono text-xs text-slate-200">{row.group_id}</td>
                    <td className="px-4 py-3 text-xs">{row.source}</td>
                    <td className="px-4 py-3 font-mono text-xs">{row.phase}</td>
                    <td className="px-4 py-3 text-xs">{row.caller_side || '—'}</td>
                    <td className="px-4 py-3 text-xs">{row.conference_group_type || '—'}</td>
                    <td className="px-4 py-3">{row.fanout_legs}</td>
                    <td className="px-4 py-3">{row.winner_established ? 'yes' : 'no'}</td>
                    <td className="px-4 py-3 font-mono text-xs text-slate-400">{row.preferred_region || '—'}</td>
                    <td className="px-4 py-3 font-mono text-xs text-slate-500">{row.remote_addr || '—'}</td>
                    <td className="px-4 py-3 text-xs text-slate-500">
                      {row.created_at ? new Date(row.created_at).toLocaleTimeString() : '—'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {confUsage?.by_group && Object.keys(confUsage.by_group).length > 0 ? (
          <div className="mt-3 flex flex-wrap gap-2 text-xs text-slate-500">
            <span className="text-slate-400">Active by group:</span>
            {Object.entries(confUsage.by_group).map(([gid, n]) => (
              <span key={gid} className="rounded border border-slate-800 bg-slate-900/60 px-2 py-0.5 font-mono">
                {gid} ({n})
              </span>
            ))}
          </div>
        ) : null}
      </section>

      <section className="mt-8">
        <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-200">
          <Activity size={16} /> Bridges
        </div>
        <div className="overflow-x-auto rounded-xl border border-slate-800 bg-slate-950/60">
          <table className="w-full min-w-[720px] text-left text-sm">
            <thead className="border-b border-slate-800 text-xs uppercase text-slate-500">
              <tr>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">ID</th>
                <th className="px-4 py-3">Type</th>
                <th className="px-4 py-3">Configured</th>
                <th className="px-4 py-3">Active calls</th>
                <th className="px-4 py-3 w-24" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/80">
              {(bridges?.bridges ?? []).length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-slate-500">
                    No bridges in config. Add bridges under <span className="font-mono text-slate-400">spec.bridges</span> in{' '}
                    <Link className="text-sky-400 underline" to="/config">
                      Config
                    </Link>
                    .
                  </td>
                </tr>
              ) : (
                (bridges?.bridges ?? []).map((b) => (
                  <tr key={b.id} className="text-slate-300 hover:bg-slate-900/40">
                    <td className="px-4 py-3 font-medium text-slate-100">{b.name}</td>
                    <td className="px-4 py-3 font-mono text-xs text-slate-400">{b.id}</td>
                    <td className="px-4 py-3 text-xs">{b.type ?? '—'}</td>
                    <td className="px-4 py-3 text-xs">{b.participants?.length ?? 0}</td>
                    <td className="px-4 py-3">
                      <Badge tone={b.active_calls > 0 ? 'green' : 'slate'}>{b.active_calls}</Badge>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Link
                        to={`/usage/bridge/${encodeURIComponent(b.id)}`}
                        className="inline-flex items-center gap-1 text-sky-400 hover:text-sky-300"
                      >
                        Open <ChevronRight size={16} />
                      </Link>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
