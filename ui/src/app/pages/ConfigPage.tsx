import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import yaml from 'js-yaml'
import { apiFetch, apiPutText } from '../api/client'
import type { Bridge, ConfigStatus, IVRConfig, RootConfig, Route } from '../api/types'

export default function ConfigPage() {
  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [draft, setDraft] = useState<RootConfig | null>(null)
  const [cfgStatus, setCfgStatus] = useState<ConfigStatus | null>(null)
  const [status, setStatus] = useState<string>('')
  const [err, setErr] = useState<string>('')

  const canApply = useMemo(() => {
    if (!draft) return false
    return Boolean(draft.apiVersion && draft.kind && draft.metadata?.name)
  }, [draft])

  function cloneConfig(c: RootConfig): RootConfig {
    if (typeof structuredClone === 'function') {
      return structuredClone(c) as RootConfig
    }
    return JSON.parse(JSON.stringify(c)) as RootConfig
  }

  async function refresh() {
    const [c, st] = await Promise.all([
      apiFetch<RootConfig>('/v1/config'),
      apiFetch<ConfigStatus>('/v1/config/status').catch(() => null),
    ])
    setCfg(c)
    setDraft(cloneConfig(c))
    if (st) setCfgStatus(st)
  }

  useEffect(() => {
    refresh().catch((e) => setErr(e instanceof Error ? e.message : String(e)))
  }, [])

  async function apply() {
    setStatus('Applying...')
    setErr('')
    try {
      if (cfgStatus?.config_read_only) {
        setErr('Config is read-only (CONFIG_HTTP_URL). Update the upstream YAML or disable HTTP config.')
        setStatus('')
        return
      }
      if (!draft) {
        setErr('No draft config loaded')
        setStatus('')
        return
      }
      const bodyText = yaml.dump(draft, { noRefs: true, lineWidth: 120 })
      await apiPutText('/v1/config', bodyText)
      setStatus('Applied')
      await refresh()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  function updateRoute(idx: number, next: Route) {
    setDraft((d) => {
      if (!d) return d
      const routes = [...(d.spec.routes ?? [])]
      routes[idx] = next
      return { ...d, spec: { ...d.spec, routes } }
    })
  }

  function addRoute() {
    setDraft((d) => {
      if (!d) return d
      const routes = [...(d.spec.routes ?? [])]
      routes.push({ match_user: '', target_kind: 'conferenceGroup', target_id: '' })
      return { ...d, spec: { ...d.spec, routes } }
    })
  }

  function removeRoute(idx: number) {
    setDraft((d) => {
      if (!d) return d
      const routes = [...(d.spec.routes ?? [])]
      routes.splice(idx, 1)
      return { ...d, spec: { ...d.spec, routes } }
    })
  }

  function updateBridge(idx: number, next: Bridge) {
    setDraft((d) => {
      if (!d) return d
      const bridges = [...(d.spec.bridges ?? [])]
      bridges[idx] = next
      return { ...d, spec: { ...d.spec, bridges } }
    })
  }

  function updateIVR(next: IVRConfig) {
    setDraft((d) => {
      if (!d) return d
      return { ...d, spec: { ...d.spec, ivr: { ...(d.spec.ivr ?? ({} as IVRConfig)), ...next } } }
    })
  }

  function addBridge() {
    setDraft((d) => {
      if (!d) return d
      const bridges = [...(d.spec.bridges ?? [])]
      bridges.push({
        id: `bridge-${String(bridges.length + 1).padStart(3, '0')}`,
        name: 'New Bridge',
        participants: [],
        recording_enabled: true
      })
      return { ...d, spec: { ...d.spec, bridges } }
    })
  }

  function removeBridge(idx: number) {
    setDraft((d) => {
      if (!d) return d
      const bridges = [...(d.spec.bridges ?? [])]
      bridges.splice(idx, 1)
      return { ...d, spec: { ...d.spec, bridges } }
    })
  }

  return (
    <div>
      <div className="text-xl font-semibold">Configuration</div>
      <div className="mt-1 text-sm text-slate-400">
        Drive configuration via forms and tables. Changes are validated server-side and applied via{' '}
        <span className="font-mono">PUT /v1/config</span>.
      </div>
      {cfgStatus?.config_read_only ? (
        <div className="mt-3 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">
          <strong>Read-only mode:</strong> this process loads shared config from{' '}
          <span className="font-mono text-amber-50">{cfgStatus.config_http_url || 'CONFIG_HTTP_URL'}</span>.
          Apply from the UI is disabled; publish YAML to that URL (GitOps, object store, or config service). Poll interval:{' '}
          {cfgStatus.config_http_poll_sec || 0}s.
        </div>
      ) : null}
      <div className="mt-3 rounded-lg border border-slate-800 bg-slate-900/40 px-3 py-2 text-sm text-slate-300">
        <strong className="text-slate-200">SBC &amp; SIP connectivity</strong> (TLS, proxy, Oracle/AudioCodes) is configured in a{' '}
        <Link className="text-sky-400 underline hover:text-sky-300" to="/setup/sbc">
          separate guided setup
        </Link>
        . It saves <span className="font-mono text-slate-400">spec.sipStack</span> in this file; restart sipbridge after changes.
      </div>

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="text-sm font-semibold">IVR</div>
        <div className="mt-1 text-xs text-slate-500">Main incoming entry and where to manage dial-in users.</div>

        <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
            <div className="text-xs font-semibold text-slate-300">Main incoming route (IVR entry user)</div>
            <div className="mt-2 flex items-center gap-3">
              <input
                className="w-40 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                value={draft?.spec.ivr?.entry_user ?? ''}
                onChange={(e) => updateIVR({ entry_user: e.target.value })}
                placeholder="9000"
              />
              <div className="text-xs text-slate-500">DDI/SIP trunk should target this user (e.g. sip:9000@host).</div>
            </div>
          </div>

          <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
            <div className="text-xs font-semibold text-slate-300">Dial-in users</div>
            <p className="mt-1 text-[11px] text-slate-500">
              Each dial-in user’s <span className="text-slate-400">employee ID</span> (bank HR id), masked PINs, and
              bridge/conference access are edited under{' '}
              <Link className="text-sky-400 underline hover:text-sky-300" to="/settings/users">
                Settings → Users
              </Link>
              . Region preference for routing still comes from each user record.
            </p>
          </div>
        </div>
      </section>

      {status ? (
        <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900 p-3 text-sm text-slate-200">
          {status}
        </div>
      ) : null}
      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">
          {err}
        </div>
      ) : null}

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

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="flex items-center justify-between">
          <div className="text-sm font-semibold">Routes</div>
          <button
            className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm text-slate-200 hover:bg-slate-900"
            onClick={() => addRoute()}
          >
            Add route
          </button>
        </div>
        <div className="mt-3 overflow-hidden rounded-lg border border-slate-800">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-xs text-slate-400">
              <tr>
                <th className="px-3 py-2">Match user</th>
                <th className="px-3 py-2">Target kind</th>
                <th className="px-3 py-2">Target id</th>
                <th className="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {(draft?.spec.routes ?? []).map((r, idx) => (
                <tr key={idx} className="border-t border-slate-800">
                  <td className="px-3 py-2">
                    <input
                      className="w-full rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                      value={r.match_user}
                      onChange={(e) => updateRoute(idx, { ...r, match_user: e.target.value })}
                      placeholder="1000"
                    />
                  </td>
                  <td className="px-3 py-2">
                    <select
                      className="w-full rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
                      value={r.target_kind}
                      onChange={(e) => updateRoute(idx, { ...r, target_kind: e.target.value })}
                    >
                      <option value="conferenceGroup">conferenceGroup</option>
                      <option value="bridge">bridge</option>
                      <option value="hootGroup">hootGroup</option>
                    </select>
                  </td>
                  <td className="px-3 py-2">
                    <input
                      className="w-full rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                      value={r.target_id}
                      onChange={(e) => updateRoute(idx, { ...r, target_id: e.target.value })}
                      placeholder="conf-001"
                    />
                  </td>
                  <td className="px-3 py-2 text-right">
                    <button
                      className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900"
                      onClick={() => removeRoute(idx)}
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
              {!(draft?.spec.routes?.length ?? 0) ? (
                <tr>
                  <td className="px-3 py-3 text-slate-500" colSpan={4}>
                    No routes.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="text-sm font-semibold">Conference line groups</div>
        <p className="mt-1 text-sm text-slate-400">
          Edit MRD/ARD/HOOT groups, side endpoints, and DDI access numbers in{' '}
          <Link className="text-sky-400 underline hover:text-sky-300" to="/settings/conference">
            Settings → Conference groups
          </Link>
          .
        </p>
      </section>

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="flex items-center justify-between">
          <div className="text-sm font-semibold">Bridges</div>
          <button
            className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm text-slate-200 hover:bg-slate-900"
            onClick={() => addBridge()}
          >
            Add bridge
          </button>
        </div>
        <div className="mt-4 grid grid-cols-1 gap-3">
          {(draft?.spec.bridges ?? []).map((b, idx) => (
            <div key={b.id} className="flex flex-wrap items-center gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3">
              <input
                className="w-44 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                value={b.id}
                onChange={(e) => updateBridge(idx, { ...b, id: e.target.value })}
              />
              <input
                className="w-56 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-sm text-slate-100"
                value={b.name}
                onChange={(e) => updateBridge(idx, { ...b, name: e.target.value })}
              />
              <label
                className="flex items-center gap-2 text-xs text-slate-300"
                title="When off, SIPREC is not forked for this bridge (global recording must still be on)."
              >
                <input
                  type="checkbox"
                  checked={b.recording_enabled !== false}
                  onChange={(e) => updateBridge(idx, { ...b, recording_enabled: e.target.checked })}
                />
                Record bridge calls
              </label>
              <label className="flex items-center gap-2 text-xs text-slate-300">
                <input
                  type="checkbox"
                  checked={Boolean(b.ddi_access_enabled)}
                  onChange={(e) => updateBridge(idx, { ...b, ddi_access_enabled: e.target.checked })}
                />
                DDI access
              </label>
              <input
                className="w-32 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                value={b.ddi_access_number ?? ''}
                onChange={(e) => updateBridge(idx, { ...b, ddi_access_number: e.target.value.replace(/\D+/g, '') })}
                placeholder="Bridge #"
                disabled={!b.ddi_access_enabled}
              />
              <div className="ml-auto flex items-center gap-2">
                <div className="text-xs text-slate-500">Participants: {b.participants?.length ?? 0}</div>
                <button
                  className="rounded-md border border-rose-900/60 bg-rose-950 px-3 py-2 text-sm text-rose-200 hover:bg-rose-900/30"
                  onClick={() => removeBridge(idx)}
                >
                  Remove
                </button>
              </div>
            </div>
          ))}
          {!(draft?.spec.bridges?.length ?? 0) ? (
            <div className="rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-500">No bridges.</div>
          ) : null}
        </div>
      </section>
    </div>
  )
}
