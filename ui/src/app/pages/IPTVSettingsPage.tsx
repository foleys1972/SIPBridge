import { useEffect, useMemo, useState } from 'react'
import yaml from 'js-yaml'
import { apiFetch, apiPostJson, apiPutText } from '../api/client'
import type { ConfigStatus, IPTVSource, IPTVSubscriptionStatus, RootConfig } from '../api/types'

function cloneConfig(c: RootConfig): RootConfig {
  if (typeof structuredClone === 'function') return structuredClone(c) as RootConfig
  return JSON.parse(JSON.stringify(c)) as RootConfig
}

export default function IPTVSettingsPage() {
  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [draft, setDraft] = useState<RootConfig | null>(null)
  const [cfgStatus, setCfgStatus] = useState<ConfigStatus | null>(null)
  const [subs, setSubs] = useState<Record<string, boolean>>({})
  const [status, setStatus] = useState('')
  const [err, setErr] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [refreshMs, setRefreshMs] = useState(3000)
  const [liveStats, setLiveStats] = useState<IPTVSubscriptionStatus[]>([])
  const ffmpegDiag = useMemo(() => {
    const withDiag = liveStats.filter((s) => s.ffmpeg_found || s.ffmpeg_error || s.ffmpeg_path)
    if (!withDiag.length) {
      return null
    }
    const found = withDiag.find((s) => s.ffmpeg_found && s.ffmpeg_path)
    if (found) {
      return { ok: true, message: `FFmpeg resolved: ${found.ffmpeg_path}` }
    }
    const firstErr = withDiag.find((s) => s.ffmpeg_error)
    return { ok: false, message: firstErr?.ffmpeg_error ?? 'FFmpeg not found for one or more IPTV sources.' }
  }, [liveStats])

  const canApply = useMemo(() => Boolean(draft?.apiVersion && draft?.kind && draft?.metadata?.name), [draft])

  async function refresh() {
    const [c, st, subRes] = await Promise.all([
      apiFetch<RootConfig>('/v1/config'),
      apiFetch<ConfigStatus>('/v1/config/status').catch(() => null),
      apiFetch<{ subscriptions: IPTVSubscriptionStatus[] }>('/v1/iptv/subscriptions').catch(() => ({ subscriptions: [] })),
    ])
    setCfg(c)
    setDraft(cloneConfig(c))
    if (st) setCfgStatus(st)
    const map: Record<string, boolean> = {}
    for (const s of subRes.subscriptions ?? []) map[s.source_id] = s.running
    setSubs(map)
    setLiveStats(subRes.subscriptions ?? [])
  }

  useEffect(() => {
    refresh().catch((e) => setErr(e instanceof Error ? e.message : String(e)))
  }, [])

  useEffect(() => {
    if (!autoRefresh) return
    const t = setInterval(() => {
      refresh().catch((e) => setErr(e instanceof Error ? e.message : String(e)))
    }, refreshMs)
    return () => clearInterval(t)
  }, [autoRefresh, refreshMs])

  function updateSource(idx: number, next: IPTVSource) {
    setDraft((d) => {
      if (!d) return d
      const iptvSources = [...(d.spec.iptvSources ?? [])]
      iptvSources[idx] = next
      return { ...d, spec: { ...d.spec, iptvSources } }
    })
  }

  function addSource() {
    setDraft((d) => {
      if (!d) return d
      const iptvSources = [...(d.spec.iptvSources ?? [])]
      iptvSources.push({
        id: `iptv-${String(iptvSources.length + 1).padStart(3, '0')}`,
        name: 'New IPTV source',
        multicast_ip: '239.1.1.1',
        port: 5004,
        payload_type: 0,
        extract_audio_from_video: true,
        jitter_buffer_ms: 120,
        enabled: true,
      })
      return { ...d, spec: { ...d.spec, iptvSources } }
    })
  }

  function removeSource(idx: number) {
    setDraft((d) => {
      if (!d) return d
      const iptvSources = [...(d.spec.iptvSources ?? [])]
      iptvSources.splice(idx, 1)
      return { ...d, spec: { ...d.spec, iptvSources } }
    })
  }

  async function apply() {
    setStatus('Applying...')
    setErr('')
    try {
      if (cfgStatus?.config_read_only) throw new Error('Config is read-only (CONFIG_HTTP_URL).')
      if (!draft) throw new Error('No draft config loaded')
      const bodyText = yaml.dump(draft, { noRefs: true, lineWidth: 120 })
      await apiPutText('/v1/config', bodyText)
      setStatus('Applied')
      await refresh()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  async function startSub(sourceID: string) {
    setErr('')
    try {
      await apiPostJson('/v1/iptv/subscriptions', { source_id: sourceID })
      await refresh()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  async function stopSub(sourceID: string) {
    setErr('')
    try {
      await apiPostJson('/v1/iptv/subscriptions/stop', { source_id: sourceID })
      await refresh()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div>
      <div className="rounded-2xl border border-slate-800 bg-slate-900/80 px-5 py-4 text-sm text-slate-300 shadow-sm shadow-slate-950/30">
        Configure multicast IPTV RTP feeds and attach them to conference/hoot groups under Conference groups.
      </div>

      <section className="mt-6 rounded-[24px] border border-slate-800 bg-slate-950 p-5 shadow-sm shadow-slate-950/20">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="text-sm font-semibold text-slate-100">IPTV Sources</div>
          <button
            className="rounded-2xl border border-slate-800 bg-slate-900 px-4 py-2 text-sm font-semibold text-slate-100 transition hover:bg-slate-800"
            onClick={() => addSource()}
          >
            Add source
          </button>
        </div>

        <div className="mt-4 grid grid-cols-1 gap-4">
          {(draft?.spec.iptvSources ?? []).map((src, idx) => (
            <div key={`${src.id}-${idx}`} className="rounded-lg border border-slate-800 bg-slate-950 p-3">
              <div className="grid grid-cols-1 gap-2 md:grid-cols-12">
                <input
                  className="md:col-span-2 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                  value={src.id}
                  onChange={(e) => updateSource(idx, { ...src, id: e.target.value })}
                  placeholder="iptv-001"
                />
                <input
                  className="md:col-span-3 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
                  value={src.name ?? ''}
                  onChange={(e) => updateSource(idx, { ...src, name: e.target.value })}
                  placeholder="Name"
                />
                <input
                  className="md:col-span-3 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                  value={src.multicast_ip}
                  onChange={(e) => updateSource(idx, { ...src, multicast_ip: e.target.value })}
                  placeholder="239.1.1.1"
                />
                <input
                  className="md:col-span-1 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                  value={String(src.port ?? 5004)}
                  onChange={(e) => updateSource(idx, { ...src, port: Number(e.target.value) || 0 })}
                />
                <input
                  className="md:col-span-1 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                  value={String(src.payload_type ?? 0)}
                  onChange={(e) => updateSource(idx, { ...src, payload_type: Number(e.target.value) || 0 })}
                />
                <label className="md:col-span-2 flex items-center gap-2 text-xs text-slate-300">
                  <input
                    type="checkbox"
                    checked={Boolean(src.extract_audio_from_video)}
                    onChange={(e) => updateSource(idx, { ...src, extract_audio_from_video: e.target.checked })}
                  />
                  Extract audio from video (ffmpeg)
                </label>
                <input
                  className="md:col-span-1 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                  value={String(src.jitter_buffer_ms ?? 120)}
                  onChange={(e) => updateSource(idx, { ...src, jitter_buffer_ms: Number(e.target.value) || 0 })}
                  title="Jitter buffer ms"
                />
                <label className="md:col-span-1 flex items-center gap-2 text-xs text-slate-300">
                  <input
                    type="checkbox"
                    checked={Boolean(src.enabled)}
                    onChange={(e) => updateSource(idx, { ...src, enabled: e.target.checked })}
                  />
                  On
                </label>
                <button
                  className="md:col-span-1 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900"
                  onClick={() => removeSource(idx)}
                >
                  Remove
                </button>
                <div className="md:col-span-12 flex items-center gap-2">
                  <span className={`rounded px-2 py-0.5 text-xs ${subs[src.id] ? 'bg-emerald-900/40 text-emerald-200' : 'bg-slate-800 text-slate-300'}`}>
                    {subs[src.id] ? 'Running' : 'Stopped'}
                  </span>
                  <button
                    className="rounded-md border border-emerald-800 bg-emerald-950/30 px-2 py-1 text-xs text-emerald-200 hover:bg-emerald-900/30"
                    onClick={() => startSub(src.id)}
                    disabled={!src.id}
                  >
                    Start subscription
                  </button>
                  <button
                    className="rounded-md border border-rose-800 bg-rose-950/30 px-2 py-1 text-xs text-rose-200 hover:bg-rose-900/30"
                    onClick={() => stopSub(src.id)}
                    disabled={!src.id}
                  >
                    Stop subscription
                  </button>
                </div>
              </div>
            </div>
          ))}
          {!(draft?.spec.iptvSources?.length ?? 0) ? (
            <div className="rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-500">No IPTV sources configured.</div>
          ) : null}
        </div>
      </section>

      <section className="mt-6 rounded-[24px] border border-slate-800 bg-slate-950 p-5 shadow-sm shadow-slate-950/20">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="text-sm font-semibold text-slate-100">IPTV Health</div>
          <div className="flex items-center gap-3">
            <label className="flex items-center gap-2 text-xs text-slate-300">
              <input type="checkbox" checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} />
              Auto refresh
            </label>
            <select
              className="rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
              value={String(refreshMs)}
              onChange={(e) => setRefreshMs(Number(e.target.value) || 3000)}
              disabled={!autoRefresh}
            >
              <option value="1000">1s</option>
              <option value="3000">3s</option>
              <option value="5000">5s</option>
              <option value="10000">10s</option>
            </select>
          </div>
        </div>
        {ffmpegDiag ? (
          <div
            className={`mt-3 rounded-lg border px-3 py-2 text-xs ${
              ffmpegDiag.ok
                ? 'border-emerald-800/60 bg-emerald-950/30 text-emerald-200'
                : 'border-rose-800/60 bg-rose-950/30 text-rose-200'
            }`}
          >
            {ffmpegDiag.message}
          </div>
        ) : null}
        <div className="mt-3 overflow-hidden rounded-lg border border-slate-800">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-xs text-slate-400">
              <tr>
                <th className="px-3 py-2">Source</th>
                <th className="px-3 py-2">State</th>
                <th className="px-3 py-2">FFmpeg</th>
                <th className="px-3 py-2">Audio packets</th>
                <th className="px-3 py-2">Dropped packets</th>
                <th className="px-3 py-2">Last audio</th>
                <th className="px-3 py-2">Started</th>
              </tr>
            </thead>
            <tbody>
              {liveStats.map((row) => (
                <tr key={row.source_id} className="border-t border-slate-800">
                  <td className="px-3 py-2 font-mono text-xs text-slate-200">{row.source_id}</td>
                  <td className="px-3 py-2">
                    <span className={`rounded px-2 py-0.5 text-xs ${row.running ? 'bg-emerald-900/40 text-emerald-200' : 'bg-slate-800 text-slate-300'}`}>
                      {row.running ? 'Running' : 'Stopped'}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-xs">
                    {row.ffmpeg_found ? (
                      <span className="text-emerald-300" title={row.ffmpeg_path ?? ''}>{row.ffmpeg_path ?? 'Found'}</span>
                    ) : row.ffmpeg_error ? (
                      <span className="text-rose-300" title={row.ffmpeg_error}>Not found</span>
                    ) : (
                      <span className="text-slate-500">N/A</span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-slate-300">{row.audio_packets ?? 0}</td>
                  <td className="px-3 py-2 text-slate-300">{row.dropped_packets ?? 0}</td>
                  <td className="px-3 py-2 text-xs text-slate-400">{row.last_audio_at ? new Date(row.last_audio_at).toLocaleString() : '—'}</td>
                  <td className="px-3 py-2 text-xs text-slate-400">{row.started_at ? new Date(row.started_at).toLocaleString() : '—'}</td>
                </tr>
              ))}
              {!liveStats.length ? (
                <tr><td className="px-3 py-3 text-slate-500" colSpan={7}>No IPTV subscriptions yet.</td></tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      {status ? <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900 p-3 text-sm text-slate-200">{status}</div> : null}
      {err ? <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div> : null}

      <div className="mt-6 flex items-center justify-between gap-3">
        <div className="text-xs text-slate-500">
          Loaded: <span className="font-mono">{cfg?.metadata?.name ?? '—'}</span>
        </div>
        <div className="flex items-center gap-3">
          <button
            className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm text-slate-200 hover:bg-slate-900"
            onClick={() => refresh().catch((e) => setErr(e instanceof Error ? e.message : String(e)))}
          >
            Refresh
          </button>
          <button
            className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50"
            disabled={!canApply || Boolean(cfgStatus?.config_read_only)}
            onClick={() => apply()}
          >
            Apply
          </button>
        </div>
      </div>
    </div>
  )
}
