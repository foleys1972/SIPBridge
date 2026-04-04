import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch } from '../api/client'
import type { Bridge, BridgeCallInfo, ConferenceGroup, RootConfig } from '../api/types'
import Badge from '../components/Badge'

type CallsResponse = { bridge_id: string; calls: BridgeCallInfo[] }

function countEndpointsInGroup(g: ConferenceGroup) {
  return (g.sideA?.length ?? 0) + (g.sideB?.length ?? 0)
}

export default function BridgesLinesPage() {
  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [liveByBridge, setLiveByBridge] = useState<Record<string, number>>({})
  const [err, setErr] = useState<string>('')

  const bridges = useMemo<Bridge[]>(() => cfg?.spec.bridges ?? [], [cfg])
  const groups = useMemo<ConferenceGroup[]>(() => cfg?.spec.conferenceGroups ?? [], [cfg])

  useEffect(() => {
    let alive = true
    async function load() {
      try {
        const c = await apiFetch<RootConfig>('/v1/config')
        if (!alive) return
        setCfg(c)

        const bridges = c.spec.bridges ?? []
        const next: Record<string, number> = {}
        await Promise.all(
          bridges.map(async (b) => {
            try {
              const cr = await apiFetch<CallsResponse>(`/v1/bridges/${encodeURIComponent(b.id)}/calls`)
              next[b.id] = cr.calls?.length ?? 0
            } catch {
              next[b.id] = 0
            }
          }),
        )
        if (!alive) return
        setLiveByBridge(next)

        setErr('')
      } catch (e) {
        setErr(e instanceof Error ? e.message : String(e))
      }
    }
    load()
    const t = setInterval(load, 3000)
    return () => {
      alive = false
      clearInterval(t)
    }
  }, [])

  return (
    <div>
      <div className="text-xl font-semibold">Bridges & Lines</div>
      <div className="mt-1 text-sm text-slate-400">
        Inventory view derived from config (bridges, conference line groups, endpoints).
      </div>

      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">
          {err}
        </div>
      ) : null}

      <div className="mt-6 grid grid-cols-1 gap-4 xl:grid-cols-2">
        <section className="rounded-xl border border-slate-800 bg-slate-950 p-4">
          <div className="flex items-center justify-between">
            <div className="text-sm font-semibold">Conference Groups</div>
            <Badge tone={groups.length ? 'blue' : 'amber'}>{groups.length} groups</Badge>
          </div>
          <div className="mt-3 overflow-hidden rounded-lg border border-slate-800">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-900 text-xs text-slate-400">
                <tr>
                  <th className="px-3 py-2">ID</th>
                  <th className="px-3 py-2">Name</th>
                  <th className="px-3 py-2">Endpoints</th>
                  <th className="px-3 py-2">Ring Timeout</th>
                </tr>
              </thead>
              <tbody>
                {groups.map((g) => (
                  <tr key={g.id} className="border-t border-slate-800">
                    <td className="px-3 py-2 font-mono text-xs text-slate-300">{g.id}</td>
                    <td className="px-3 py-2 text-slate-200">{g.name}</td>
                    <td className="px-3 py-2 text-slate-300">{countEndpointsInGroup(g)}</td>
                    <td className="px-3 py-2 text-slate-300">{g.ring_timeout_seconds}s</td>
                  </tr>
                ))}
                {!groups.length ? (
                  <tr>
                    <td className="px-3 py-3 text-slate-500" colSpan={4}>
                      No conference groups in config.
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </section>

        <section className="rounded-xl border border-slate-800 bg-slate-950 p-4">
          <div className="flex items-center justify-between">
            <div className="text-sm font-semibold">Bridges</div>
            <Badge tone={bridges.length ? 'blue' : 'amber'}>{bridges.length} bridges</Badge>
          </div>
          <div className="mt-3 overflow-hidden rounded-lg border border-slate-800">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-900 text-xs text-slate-400">
                <tr>
                  <th className="px-3 py-2">ID</th>
                  <th className="px-3 py-2">Name</th>
                  <th className="px-3 py-2">Configured</th>
                  <th className="px-3 py-2">Live</th>
                </tr>
              </thead>
              <tbody>
                {bridges.map((b) => (
                  <tr key={b.id} className="border-t border-slate-800">
                    <td className="px-3 py-2 font-mono text-xs text-slate-300">
                      <Link className="text-slate-200 hover:text-slate-50" to={`/bridges/${encodeURIComponent(b.id)}`}>
                        {b.id}
                      </Link>
                    </td>
                    <td className="px-3 py-2 text-slate-200">
                      <Link className="hover:text-slate-50" to={`/bridges/${encodeURIComponent(b.id)}`}>
                        {b.name}
                      </Link>
                    </td>
                    <td className="px-3 py-2 text-slate-300">{b.participants?.length ?? 0}</td>
                    <td className="px-3 py-2 text-slate-300">{liveByBridge[b.id] ?? 0}</td>
                  </tr>
                ))}
                {!bridges.length ? (
                  <tr>
                    <td className="px-3 py-3 text-slate-500" colSpan={4}>
                      No bridges in config.
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="text-sm font-semibold">Routes</div>
        <div className="mt-2 overflow-hidden rounded-lg border border-slate-800">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-xs text-slate-400">
              <tr>
                <th className="px-3 py-2">Match User</th>
                <th className="px-3 py-2">Target Kind</th>
                <th className="px-3 py-2">Target ID</th>
              </tr>
            </thead>
            <tbody>
              {(cfg?.spec.routes ?? []).map((r) => (
                <tr key={`${r.match_user}-${r.target_kind}-${r.target_id}`} className="border-t border-slate-800">
                  <td className="px-3 py-2 font-mono text-xs text-slate-300">{r.match_user}</td>
                  <td className="px-3 py-2 text-slate-200">{r.target_kind}</td>
                  <td className="px-3 py-2 font-mono text-xs text-slate-300">{r.target_id}</td>
                </tr>
              ))}
              {!(cfg?.spec.routes?.length ?? 0) ? (
                <tr>
                  <td className="px-3 py-3 text-slate-500" colSpan={3}>
                    No routes in config.
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
