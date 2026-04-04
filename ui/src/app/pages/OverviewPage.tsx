import { useEffect, useMemo, useState } from 'react'
import { apiFetch } from '../api/client'
import type { Healthz, RootConfig, SIPStats } from '../api/types'
import Badge from '../components/Badge'
import KpiCard from '../components/KpiCard'

export default function OverviewPage() {
  const [health, setHealth] = useState<Healthz | null>(null)
  const [stats, setStats] = useState<SIPStats | null>(null)
  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [err, setErr] = useState<string>('')

  const lastPacket = useMemo(() => {
    if (!stats?.last_packet_at) return '—'
    const d = new Date(stats.last_packet_at)
    if (Number.isNaN(d.getTime())) return stats.last_packet_at
    return d.toLocaleString()
  }, [stats?.last_packet_at])

  useEffect(() => {
    let alive = true
    async function load() {
      try {
        const [h, s, c] = await Promise.all([
          apiFetch<Healthz>('/healthz'),
          apiFetch<SIPStats>('/v1/sip/stats'),
          apiFetch<RootConfig>('/v1/config'),
        ])
        if (!alive) return
        setHealth(h)
        setStats(s)
        setCfg(c)
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
          <div className="text-xl font-semibold">Overview</div>
          <div className="mt-1 text-sm text-slate-400">
            Health, live SIP counters, and a high-level inventory view.
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Badge tone={health?.ok ? 'green' : 'red'}>
            API {health?.ok ? 'OK' : 'DOWN'}
          </Badge>
          <Badge tone={stats?.started ? 'green' : 'amber'}>
            SIP {stats?.started ? 'LISTENING' : 'STOPPED'}
          </Badge>
        </div>
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
        <KpiCard
          title="Routes"
          value={cfg?.spec.routes?.length ?? '—'}
          hint={
            <span>
              Bridges: {cfg?.spec.bridges?.length ?? '—'} | Conf groups:{' '}
              {cfg?.spec.conferenceGroups?.length ?? '—'}
            </span>
          }
        />
      </div>

      <div className="mt-8 grid grid-cols-1 gap-4 xl:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-950 p-4">
          <div className="text-sm font-semibold">What’s next</div>
          <div className="mt-2 text-sm text-slate-400">
            MI data and realtime usage will be backed by additional endpoints.
            RBAC is currently client-side scaffolding.
          </div>
        </div>
        <div className="rounded-xl border border-slate-800 bg-slate-950 p-4">
          <div className="text-sm font-semibold">Config version</div>
          <div className="mt-2 text-sm text-slate-300">
            <span className="font-mono">{cfg?.apiVersion ?? '—'}</span>
          </div>
          <div className="mt-1 text-xs text-slate-500">
            kind: <span className="font-mono">{cfg?.kind ?? '—'}</span>
          </div>
        </div>
      </div>
    </div>
  )
}
