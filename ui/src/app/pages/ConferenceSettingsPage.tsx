import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import yaml from 'js-yaml'
import { apiFetch, apiPutText } from '../api/client'
import type { ConferenceGroup, ConfigStatus, Endpoint, IPTVSource, RootConfig } from '../api/types'

export default function ConferenceSettingsPage() {
  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [draft, setDraft] = useState<RootConfig | null>(null)
  const [cfgStatus, setCfgStatus] = useState<ConfigStatus | null>(null)
  const [status, setStatus] = useState('')
  const [err, setErr] = useState('')

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
        setErr('Config is read-only (CONFIG_HTTP_URL).')
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

  function updateGroup(idx: number, next: ConferenceGroup) {
    setDraft((d) => {
      if (!d) return d
      const conferenceGroups = [...(d.spec.conferenceGroups ?? [])]
      conferenceGroups[idx] = next
      return { ...d, spec: { ...d.spec, conferenceGroups } }
    })
  }

  function addGroup() {
    setDraft((d) => {
      if (!d) return d
      const conferenceGroups = [...(d.spec.conferenceGroups ?? [])]
      conferenceGroups.push({
        id: `conf-${String(conferenceGroups.length + 1).padStart(3, '0')}`,
        name: 'New Conference Group',
        type: 'mrd',
        ring_timeout_seconds: 30,
        ddi_access_enabled: false,
        ddi_access_number: '',
        recording_enabled: false,
        sideA: [],
        sideB: [],
      })
      return { ...d, spec: { ...d.spec, conferenceGroups } }
    })
  }

  function removeGroup(idx: number) {
    setDraft((d) => {
      if (!d) return d
      const conferenceGroups = [...(d.spec.conferenceGroups ?? [])]
      conferenceGroups.splice(idx, 1)
      return { ...d, spec: { ...d.spec, conferenceGroups } }
    })
  }

  function toggleIPTVSource(groupIdx: number, sourceID: string, checked: boolean) {
    setDraft((d) => {
      if (!d) return d
      const conferenceGroups = [...(d.spec.conferenceGroups ?? [])]
      const g = conferenceGroups[groupIdx]
      const selected = new Set(g.iptv_source_ids ?? [])
      if (checked) selected.add(sourceID)
      else selected.delete(sourceID)
      conferenceGroups[groupIdx] = { ...g, iptv_source_ids: Array.from(selected) }
      return { ...d, spec: { ...d.spec, conferenceGroups } }
    })
  }

  function updateEndpoint(groupIdx: number, side: 'A' | 'B', epIdx: number, next: Endpoint) {
    setDraft((d) => {
      if (!d) return d
      const conferenceGroups = [...(d.spec.conferenceGroups ?? [])]
      const g = conferenceGroups[groupIdx]
      const sideKey = side === 'A' ? 'sideA' : 'sideB'
      const endpoints = [...(((g as unknown) as Record<string, Endpoint[]>)[sideKey] ?? [])]
      endpoints[epIdx] = next
      conferenceGroups[groupIdx] = { ...g, [sideKey]: endpoints } as ConferenceGroup
      return { ...d, spec: { ...d.spec, conferenceGroups } }
    })
  }

  function addEndpoint(groupIdx: number, side: 'A' | 'B') {
    setDraft((d) => {
      if (!d) return d
      const conferenceGroups = [...(d.spec.conferenceGroups ?? [])]
      const g = conferenceGroups[groupIdx]
      const sideKey = side === 'A' ? 'sideA' : 'sideB'
      const endpoints = [...(((g as unknown) as Record<string, Endpoint[]>)[sideKey] ?? [])]
      endpoints.push({
        id: `${side.toLowerCase()}-${endpoints.length + 1}`,
        display_name: '',
        sip_uri: '',
        location: '',
        linked_user_id: '',
        linked_device_id: '',
      })
      conferenceGroups[groupIdx] = { ...g, [sideKey]: endpoints } as ConferenceGroup
      return { ...d, spec: { ...d.spec, conferenceGroups } }
    })
  }

  function removeEndpoint(groupIdx: number, side: 'A' | 'B', epIdx: number) {
    setDraft((d) => {
      if (!d) return d
      const conferenceGroups = [...(d.spec.conferenceGroups ?? [])]
      const g = conferenceGroups[groupIdx]
      const sideKey = side === 'A' ? 'sideA' : 'sideB'
      const endpoints = [...(((g as unknown) as Record<string, Endpoint[]>)[sideKey] ?? [])]
      endpoints.splice(epIdx, 1)
      conferenceGroups[groupIdx] = { ...g, [sideKey]: endpoints } as ConferenceGroup
      return { ...d, spec: { ...d.spec, conferenceGroups } }
    })
  }

  return (
    <div>
      <div className="rounded-2xl border border-slate-800 bg-slate-900/80 px-5 py-4 text-sm text-slate-300 shadow-sm shadow-slate-950/30">
        Conference line groups (MRD/ARD/HOOT) can be managed here.{' '}
        <Link className="text-sky-400 underline hover:text-sky-300" to="/usage">
          Realtime Usage
        </Link>{' '}
        shows active sessions per group. Enable SIPREC per group when global recording is configured under{' '}
        <Link className="text-sky-400 underline hover:text-sky-300" to="/settings/recording">
          Settings → Recording
        </Link>
        . Link endpoints to user devices for CTI metadata. User access is managed under{' '}
        <Link className="text-sky-400 underline hover:text-sky-300" to="/settings/users">
          Users
        </Link>
        .
      </div>

      {cfgStatus?.config_read_only ? (
        <div className="mt-3 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">
          <strong>Read-only mode:</strong> apply is disabled while CONFIG_HTTP_URL is set.
        </div>
      ) : null}

      <section className="mt-6 rounded-[24px] border border-slate-800 bg-slate-950 p-5 shadow-sm shadow-slate-950/20">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="text-sm font-semibold text-slate-100">Conference Line Groups</div>
          <button
            className="rounded-2xl border border-slate-800 bg-slate-900 px-4 py-2 text-sm font-semibold text-slate-100 transition hover:bg-slate-800"
            onClick={() => addGroup()}
          >
            Add group
          </button>
        </div>

        <div className="mt-4 grid grid-cols-1 gap-4">
          {(draft?.spec.conferenceGroups ?? []).map((g, idx) => (
            <div key={g.id} className="rounded-lg border border-slate-800 bg-slate-950 p-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-3">
                  <div className="font-mono text-xs text-slate-400">{g.id}</div>
                  <input
                    className="w-56 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-sm text-slate-100"
                    value={g.name ?? ''}
                    onChange={(e) => updateGroup(idx, { ...g, name: e.target.value })}
                    placeholder="Name"
                  />
                  <select
                    className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-sm text-slate-100"
                    value={((g.type as string) || 'mrd') as string}
                    onChange={(e) => updateGroup(idx, { ...g, type: e.target.value as ConferenceGroup['type'] })}
                  >
                    <option value="mrd">MRD</option>
                    <option value="ard">ARD</option>
                    <option value="hoot">HOOT</option>
                  </select>
                </div>

                <div className="flex flex-wrap items-center gap-2">
                  <label className="flex items-center gap-2 text-xs text-slate-300">
                    <input
                      type="checkbox"
                      checked={Boolean(g.ddi_access_enabled)}
                      onChange={(e) => updateGroup(idx, { ...g, ddi_access_enabled: e.target.checked })}
                    />
                    DDI access
                  </label>
                  <input
                    className="w-32 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                    value={g.ddi_access_number ?? ''}
                    onChange={(e) => updateGroup(idx, { ...g, ddi_access_number: e.target.value.replace(/\D+/g, '') })}
                    placeholder="Conf #"
                    disabled={!g.ddi_access_enabled}
                  />
                  <input
                    className="w-28 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                    value={String(g.ring_timeout_seconds ?? 30)}
                    onChange={(e) => updateGroup(idx, { ...g, ring_timeout_seconds: Number(e.target.value) || 0 })}
                    min={0}
                  />
                  <div className="text-xs text-slate-500">timeout (s)</div>
                  <label
                    className="flex items-center gap-2 text-xs text-slate-300"
                    title="When on, SIPREC is forked for this conference while global recording is enabled. Independent of per-user settings."
                  >
                    <input
                      type="checkbox"
                      checked={Boolean(g.recording_enabled)}
                      onChange={(e) => updateGroup(idx, { ...g, recording_enabled: e.target.checked })}
                    />
                    Record conference
                  </label>
                  <button
                    className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900"
                    onClick={() => removeGroup(idx)}
                  >
                    Remove
                  </button>
                </div>
              </div>

              <div className="mt-3 rounded-md border border-slate-800/80 bg-slate-950/60 p-2">
                <div className="text-xs font-semibold text-slate-300">IPTV multicast feeds</div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {(draft?.spec.iptvSources ?? []).map((src: IPTVSource) => (
                    <label key={src.id} className="flex items-center gap-2 rounded-md border border-slate-800 px-2 py-1 text-xs text-slate-300">
                      <input
                        type="checkbox"
                        checked={Boolean((g.iptv_source_ids ?? []).includes(src.id))}
                        onChange={(e) => toggleIPTVSource(idx, src.id, e.target.checked)}
                      />
                      <span className="font-mono">{src.id}</span>
                      <span className="text-slate-500">{src.name ?? src.multicast_ip}</span>
                    </label>
                  ))}
                  {!(draft?.spec.iptvSources?.length ?? 0) ? (
                    <div className="text-xs text-slate-500">No IPTV sources configured in Settings - IPTV.</div>
                  ) : null}
                </div>
              </div>

              <div className="mt-4 grid grid-cols-1 gap-4 xl:grid-cols-2">
                <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
                  <div className="flex items-center justify-between">
                    <div className="text-sm font-semibold">Side A</div>
                    <button
                      className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900"
                      onClick={() => addEndpoint(idx, 'A')}
                    >
                      Add
                    </button>
                  </div>
                  {(g.sideA ?? []).map((ep, epIdx) => (
                    <div key={`${g.id}-A-${epIdx}`} className="mt-2 rounded-md border border-slate-800/80 bg-slate-950/50 p-2">
                      <div className="grid grid-cols-12 gap-2">
                        <input
                          className="col-span-3 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={ep.id}
                          onChange={(e) => updateEndpoint(idx, 'A', epIdx, { ...ep, id: e.target.value })}
                          placeholder="a-1"
                        />
                        <input
                          className="col-span-4 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
                          value={ep.display_name}
                          onChange={(e) => updateEndpoint(idx, 'A', epIdx, { ...ep, display_name: e.target.value })}
                          placeholder="A-1"
                        />
                        <input
                          className="col-span-4 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={ep.sip_uri}
                          onChange={(e) => updateEndpoint(idx, 'A', epIdx, { ...ep, sip_uri: e.target.value })}
                          placeholder="sip:desk-a1@domain"
                        />
                        <input
                          className="col-span-1 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
                          value={ep.location ?? ''}
                          onChange={(e) => updateEndpoint(idx, 'A', epIdx, { ...ep, location: e.target.value })}
                          placeholder="LDN"
                        />
                      </div>
                      <div className="mt-2 grid grid-cols-1 gap-2 md:grid-cols-2">
                        <input
                          className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-[10px] text-slate-100"
                          value={ep.linked_user_id ?? ''}
                          onChange={(e) => updateEndpoint(idx, 'A', epIdx, { ...ep, linked_user_id: e.target.value })}
                          placeholder="Linked employee id (user id)"
                        />
                        <input
                          className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-[10px] text-slate-100"
                          value={ep.linked_device_id ?? ''}
                          onChange={(e) => updateEndpoint(idx, 'A', epIdx, { ...ep, linked_device_id: e.target.value })}
                          placeholder="Linked device id (from Users)"
                        />
                      </div>
                      <button
                        type="button"
                        className="mt-2 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900"
                        onClick={() => removeEndpoint(idx, 'A', epIdx)}
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                </div>

                <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
                  <div className="flex items-center justify-between">
                    <div className="text-sm font-semibold">Side B</div>
                    <button
                      className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900"
                      onClick={() => addEndpoint(idx, 'B')}
                    >
                      Add
                    </button>
                  </div>
                  {(g.sideB ?? []).map((ep, epIdx) => (
                    <div key={`${g.id}-B-${epIdx}`} className="mt-2 rounded-md border border-slate-800/80 bg-slate-950/50 p-2">
                      <div className="grid grid-cols-12 gap-2">
                        <input
                          className="col-span-3 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={ep.id}
                          onChange={(e) => updateEndpoint(idx, 'B', epIdx, { ...ep, id: e.target.value })}
                          placeholder="b-1"
                        />
                        <input
                          className="col-span-4 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
                          value={ep.display_name}
                          onChange={(e) => updateEndpoint(idx, 'B', epIdx, { ...ep, display_name: e.target.value })}
                          placeholder="B-1"
                        />
                        <input
                          className="col-span-4 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={ep.sip_uri}
                          onChange={(e) => updateEndpoint(idx, 'B', epIdx, { ...ep, sip_uri: e.target.value })}
                          placeholder="sip:desk-b1@domain"
                        />
                        <input
                          className="col-span-1 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
                          value={ep.location ?? ''}
                          onChange={(e) => updateEndpoint(idx, 'B', epIdx, { ...ep, location: e.target.value })}
                          placeholder="NYC"
                        />
                      </div>
                      <div className="mt-2 grid grid-cols-1 gap-2 md:grid-cols-2">
                        <input
                          className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-[10px] text-slate-100"
                          value={ep.linked_user_id ?? ''}
                          onChange={(e) => updateEndpoint(idx, 'B', epIdx, { ...ep, linked_user_id: e.target.value })}
                          placeholder="Linked employee id (user id)"
                        />
                        <input
                          className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-[10px] text-slate-100"
                          value={ep.linked_device_id ?? ''}
                          onChange={(e) => updateEndpoint(idx, 'B', epIdx, { ...ep, linked_device_id: e.target.value })}
                          placeholder="Linked device id (from Users)"
                        />
                      </div>
                      <button
                        type="button"
                        className="mt-2 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900"
                        onClick={() => removeEndpoint(idx, 'B', epIdx)}
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          ))}
          {!(draft?.spec.conferenceGroups?.length ?? 0) ? (
            <div className="rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-500">No conference groups.</div>
          ) : null}
        </div>
      </section>

      {status ? (
        <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900 p-3 text-sm text-slate-200">{status}</div>
      ) : null}
      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
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
    </div>
  )
}
