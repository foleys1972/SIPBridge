import { useCallback, useEffect, useState } from 'react'
import { apiDelete, apiFetch, apiPutJson } from '../api/client'
import type {
  ConfigStatus,
  RecordingSettingsResponse,
  RecordingSpec,
  RecordingTrunkEntry,
  SIPRECIntegrationSpec,
} from '../api/types'

function emptySiprec(): SIPRECIntegrationSpec {
  return {
    enabled: false,
    recorder_sip_uri: '',
    recording_trunk_sip_uri: '',
    metadata_namespace: '',
    trunks: [],
    default_trunk_id: '',
    region_to_trunk: {},
  }
}

function normalizeSaved(s: RecordingSpec | null): RecordingSpec {
  if (!s) {
    return { global_enabled: false, siprec: emptySiprec() }
  }
  const sip = s.siprec
  if (!sip) {
    return { global_enabled: Boolean(s.global_enabled), siprec: emptySiprec() }
  }
  const legacyRecorder = (sip.recorder_sip_uri ?? '').trim()
  const hasTrunks = (sip.trunks?.length ?? 0) > 0
  if (legacyRecorder && !hasTrunks) {
    const trunks: RecordingTrunkEntry[] = [
      {
        id: 'default',
        label: 'Default',
        recorder_sip_uri: sip.recorder_sip_uri ?? '',
        recording_trunk_sip_uri: sip.recording_trunk_sip_uri ?? '',
      },
    ]
    return {
      global_enabled: Boolean(s.global_enabled),
      siprec: {
        enabled: Boolean(sip.enabled),
        recorder_sip_uri: '',
        recording_trunk_sip_uri: '',
        metadata_namespace: sip.metadata_namespace ?? '',
        trunks,
        default_trunk_id: 'default',
        region_to_trunk: { ...(sip.region_to_trunk ?? {}) },
      },
    }
  }
  return {
    global_enabled: Boolean(s.global_enabled),
    siprec: {
      enabled: Boolean(sip.enabled),
      recorder_sip_uri: sip.recorder_sip_uri ?? '',
      recording_trunk_sip_uri: sip.recording_trunk_sip_uri ?? '',
      metadata_namespace: sip.metadata_namespace ?? '',
      trunks: sip.trunks?.length
        ? sip.trunks.map((t) => ({
            id: t.id,
            label: t.label ?? '',
            recorder_sip_uri: t.recorder_sip_uri ?? '',
            recording_trunk_sip_uri: t.recording_trunk_sip_uri ?? '',
          }))
        : [],
      default_trunk_id: sip.default_trunk_id ?? '',
      region_to_trunk: { ...(sip.region_to_trunk ?? {}) },
    },
  }
}

function siprecForSave(s: RecordingSpec): RecordingSpec {
  const sip = s.siprec ?? emptySiprec()
  const trunks = (sip.trunks ?? []).filter((t) => (t.id ?? '').trim() !== '')
  if (trunks.length > 0) {
    return {
      global_enabled: s.global_enabled,
      siprec: {
        enabled: sip.enabled,
        metadata_namespace: sip.metadata_namespace,
        trunks,
        default_trunk_id: sip.default_trunk_id,
        region_to_trunk: sip.region_to_trunk,
      },
    }
  }
  return {
    global_enabled: s.global_enabled,
    siprec: {
      enabled: sip.enabled,
      recorder_sip_uri: sip.recorder_sip_uri,
      recording_trunk_sip_uri: sip.recording_trunk_sip_uri,
      metadata_namespace: sip.metadata_namespace,
    },
  }
}

type RegionRow = { regionLabel: string; trunkId: string }

function regionMapToRows(m: Record<string, string> | undefined): RegionRow[] {
  if (!m || Object.keys(m).length === 0) return []
  return Object.entries(m).map(([regionLabel, trunkId]) => ({ regionLabel, trunkId }))
}

function rowsToRegionMap(rows: RegionRow[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const r of rows) {
    const k = r.regionLabel.trim()
    if (k) out[k] = r.trunkId.trim()
  }
  return out
}

export default function RecordingSettingsPage() {
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')
  const [note, setNote] = useState('')
  const [cfgStatus, setCfgStatus] = useState<ConfigStatus | null>(null)
  const [spec, setSpec] = useState<RecordingSpec>(() => normalizeSaved(null))
  const [regionRows, setRegionRows] = useState<RegionRow[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    setErr('')
    const [data, st] = await Promise.all([
      apiFetch<RecordingSettingsResponse>('/v1/settings/recording'),
      apiFetch<ConfigStatus>('/v1/config/status').catch(() => null),
    ])
    setNote(data.note ?? '')
    const n = normalizeSaved(data.saved)
    setSpec(n)
    setRegionRows(regionMapToRows(n.siprec?.region_to_trunk))
    if (st) setCfgStatus(st)
    setLoading(false)
  }, [])

  useEffect(() => {
    load().catch((e) => {
      setErr(e instanceof Error ? e.message : String(e))
      setLoading(false)
    })
  }, [load])

  async function save() {
    setErr('')
    setStatus('Saving…')
    try {
      if (cfgStatus?.config_read_only) {
        setErr('Config is read-only (CONFIG_HTTP_URL).')
        setStatus('')
        return
      }
      const sip = spec.siprec ?? emptySiprec()
      const merged: RecordingSpec = {
        ...spec,
        siprec: {
          ...sip,
          region_to_trunk: rowsToRegionMap(regionRows),
        },
      }
      await apiPutJson('/v1/settings/recording', siprecForSave(merged))
      setStatus('Saved.')
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  async function clearSaved() {
    if (!window.confirm('Remove saved recording settings from config?')) return
    setErr('')
    try {
      await apiDelete('/v1/settings/recording')
      setStatus('Cleared.')
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  if (loading) {
    return <div className="text-sm text-slate-400">Loading…</div>
  }

  const sip = spec.siprec ?? emptySiprec()
  const trunks = sip.trunks ?? []
  const useMultiTrunk = trunks.length > 0

  return (
    <div>
      <div className="text-sm font-semibold text-slate-200">Recording &amp; SIPREC</div>
      <p className="mt-1 text-sm text-slate-400">
        Configure one or more recording trunks so users in different regions (e.g. US vs EMEA) can be forked to the correct
        recorder. Map <span className="font-mono text-slate-300">User.region</span> labels to trunks on{' '}
        <span className="font-mono text-slate-300">Users</span>. Per-conference recording remains on Conference groups.
      </p>

      {cfgStatus?.config_read_only ? (
        <div className="mt-4 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">
          Read-only config (CONFIG_HTTP_URL). Changes cannot be applied from this UI.
        </div>
      ) : null}

      <div className="mt-6 space-y-4">
        <label className="flex items-center gap-2 text-sm text-slate-300">
          <input
            type="checkbox"
            checked={spec.global_enabled}
            onChange={(e) => setSpec((s) => ({ ...s, global_enabled: e.target.checked }))}
            disabled={Boolean(cfgStatus?.config_read_only)}
          />
          Global recording enabled (master switch for SIPREC metadata and future media fork)
        </label>

        <div className="rounded-xl border border-slate-800 bg-slate-950 p-4">
          <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">SIPREC integration</div>
          <label className="mt-3 flex items-center gap-2 text-sm text-slate-300">
            <input
              type="checkbox"
              checked={sip.enabled}
              onChange={(e) =>
                setSpec((s) => ({
                  ...s,
                  siprec: { ...(s.siprec ?? emptySiprec()), enabled: e.target.checked },
                }))
              }
              disabled={Boolean(cfgStatus?.config_read_only)}
            />
            Enable SIPREC toward recorder
          </label>

          <div className="mt-4 flex flex-wrap gap-2">
            <button
              type="button"
              className="rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-50"
              disabled={Boolean(cfgStatus?.config_read_only)}
              onClick={() => {
                setSpec((s) => {
                  const cur = s.siprec ?? emptySiprec()
                  if ((cur.trunks?.length ?? 0) > 0) return s
                  const legacyR = (cur.recorder_sip_uri ?? '').trim()
                  const legacyT = (cur.recording_trunk_sip_uri ?? '').trim()
                  const nextTrunks: RecordingTrunkEntry[] = [
                    {
                      id: 'us',
                      label: 'US',
                      recorder_sip_uri: legacyR || '',
                      recording_trunk_sip_uri: legacyT,
                    },
                  ]
                  return {
                    ...s,
                    siprec: {
                      ...cur,
                      recorder_sip_uri: '',
                      recording_trunk_sip_uri: '',
                      trunks: nextTrunks,
                      default_trunk_id: 'us',
                      region_to_trunk: cur.region_to_trunk ?? {},
                    },
                  }
                })
              }}
            >
              Use multiple recording trunks
            </button>
            {useMultiTrunk ? (
              <button
                type="button"
                className="rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-50"
                disabled={Boolean(cfgStatus?.config_read_only)}
                onClick={() => {
                  if (
                    !window.confirm(
                      'Collapse to a single legacy recorder URI? Region routing will be cleared.',
                    )
                  )
                    return
                  const cur = spec.siprec ?? emptySiprec()
                  const first = cur.trunks?.[0]
                  setSpec({
                    ...spec,
                    siprec: {
                      enabled: cur.enabled,
                      recorder_sip_uri: first?.recorder_sip_uri ?? '',
                      recording_trunk_sip_uri: first?.recording_trunk_sip_uri ?? '',
                      metadata_namespace: cur.metadata_namespace,
                    },
                  })
                  setRegionRows([])
                }}
              >
                Use single recorder only
              </button>
            ) : null}
          </div>

          {!useMultiTrunk ? (
            <>
              <label className="mt-3 flex flex-col gap-1 text-xs text-slate-400">
                Recorder SIP URI (RS)
                <input
                  className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                  value={sip.recorder_sip_uri ?? ''}
                  onChange={(e) =>
                    setSpec((s) => ({
                      ...s,
                      siprec: { ...(s.siprec ?? emptySiprec()), recorder_sip_uri: e.target.value },
                    }))
                  }
                  placeholder="sip:recorder@sbc.example.com"
                  disabled={Boolean(cfgStatus?.config_read_only)}
                />
              </label>
              <label className="mt-3 flex flex-col gap-1 text-xs text-slate-400">
                Recording trunk SIP URI (optional dedicated path)
                <input
                  className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                  value={sip.recording_trunk_sip_uri ?? ''}
                  onChange={(e) =>
                    setSpec((s) => ({
                      ...s,
                      siprec: { ...(s.siprec ?? emptySiprec()), recording_trunk_sip_uri: e.target.value },
                    }))
                  }
                  placeholder="sip:rec-trunk@sbc.example.com"
                  disabled={Boolean(cfgStatus?.config_read_only)}
                />
              </label>
            </>
          ) : (
            <div className="mt-4 space-y-3">
              <div className="text-xs text-slate-500">
                Each trunk is a recorder (and optional outbound trunk) for a geography or compliance zone. Users are routed by
                matching their region label to a trunk below.
              </div>
              {(trunks.length ? trunks : [{ id: '', label: '', recorder_sip_uri: '', recording_trunk_sip_uri: '' }]).map(
                (t, idx) => (
                  <div
                    key={idx}
                    className="rounded-lg border border-slate-800 bg-slate-900/40 p-3"
                  >
                    <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
                      <label className="flex flex-col gap-1 text-[11px] text-slate-500">
                        Trunk id
                        <input
                          className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={t.id}
                          onChange={(e) => {
                            const next = [...(sip.trunks ?? [])]
                            next[idx] = { ...t, id: e.target.value }
                            setSpec((s) => ({
                              ...s,
                              siprec: { ...(s.siprec ?? emptySiprec()), trunks: next },
                            }))
                          }}
                          placeholder="us"
                          disabled={Boolean(cfgStatus?.config_read_only)}
                        />
                      </label>
                      <label className="flex flex-col gap-1 text-[11px] text-slate-500">
                        Label (optional)
                        <input
                          className="rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
                          value={t.label ?? ''}
                          onChange={(e) => {
                            const next = [...(sip.trunks ?? [])]
                            next[idx] = { ...t, label: e.target.value }
                            setSpec((s) => ({
                              ...s,
                              siprec: { ...(s.siprec ?? emptySiprec()), trunks: next },
                            }))
                          }}
                          placeholder="United States"
                          disabled={Boolean(cfgStatus?.config_read_only)}
                        />
                      </label>
                      <label className="flex flex-col gap-1 text-[11px] text-slate-500 md:col-span-2">
                        Recorder SIP URI (RS)
                        <input
                          className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={t.recorder_sip_uri ?? ''}
                          onChange={(e) => {
                            const next = [...(sip.trunks ?? [])]
                            next[idx] = { ...t, recorder_sip_uri: e.target.value }
                            setSpec((s) => ({
                              ...s,
                              siprec: { ...(s.siprec ?? emptySiprec()), trunks: next },
                            }))
                          }}
                          disabled={Boolean(cfgStatus?.config_read_only)}
                        />
                      </label>
                      <label className="flex flex-col gap-1 text-[11px] text-slate-500 md:col-span-2">
                        Recording trunk SIP URI (optional)
                        <input
                          className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={t.recording_trunk_sip_uri ?? ''}
                          onChange={(e) => {
                            const next = [...(sip.trunks ?? [])]
                            next[idx] = { ...t, recording_trunk_sip_uri: e.target.value }
                            setSpec((s) => ({
                              ...s,
                              siprec: { ...(s.siprec ?? emptySiprec()), trunks: next },
                            }))
                          }}
                          disabled={Boolean(cfgStatus?.config_read_only)}
                        />
                      </label>
                    </div>
                    <button
                      type="button"
                      className="mt-2 text-xs text-rose-300 hover:text-rose-200 disabled:opacity-50"
                      disabled={Boolean(cfgStatus?.config_read_only)}
                      onClick={() => {
                        const next = [...(sip.trunks ?? [])].filter((_, j) => j !== idx)
                        setSpec((s) => ({
                          ...s,
                          siprec: { ...(s.siprec ?? emptySiprec()), trunks: next },
                        }))
                      }}
                    >
                      Remove trunk
                    </button>
                  </div>
                ),
              )}
              <button
                type="button"
                className="rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-50"
                disabled={Boolean(cfgStatus?.config_read_only)}
                onClick={() => {
                  const next = [...(sip.trunks ?? [])]
                  next.push({ id: `trunk-${next.length + 1}`, label: '', recorder_sip_uri: '', recording_trunk_sip_uri: '' })
                  setSpec((s) => ({
                    ...s,
                    siprec: { ...(s.siprec ?? emptySiprec()), trunks: next },
                  }))
                }}
              >
                Add trunk
              </button>

              <label className="mt-2 flex flex-col gap-1 text-xs text-slate-400">
                Default trunk (when user region does not match any mapping)
                <select
                  className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 text-sm text-slate-100"
                  value={sip.default_trunk_id ?? ''}
                  onChange={(e) =>
                    setSpec((s) => ({
                      ...s,
                      siprec: { ...(s.siprec ?? emptySiprec()), default_trunk_id: e.target.value },
                    }))
                  }
                  disabled={Boolean(cfgStatus?.config_read_only)}
                >
                  <option value="">— select —</option>
                  {trunks.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.id}
                      {t.label ? ` (${t.label})` : ''}
                    </option>
                  ))}
                </select>
              </label>

              <div className="mt-4">
                <div className="text-xs font-semibold text-slate-400">Region → trunk</div>
                <p className="mt-1 text-xs text-slate-500">
                  Keys must match the region chosen on each user (e.g. <span className="font-mono">US</span> routes to the US
                  trunk).
                </p>
                <div className="mt-2 space-y-2">
                  {regionRows.map((row, i) => (
                    <div key={i} className="flex flex-wrap items-end gap-2">
                      <label className="flex flex-col gap-1 text-[11px] text-slate-500">
                        User region label
                        <input
                          className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={row.regionLabel}
                          onChange={(e) => {
                            const next = [...regionRows]
                            next[i] = { ...row, regionLabel: e.target.value }
                            setRegionRows(next)
                          }}
                          placeholder="US"
                          disabled={Boolean(cfgStatus?.config_read_only)}
                        />
                      </label>
                      <span className="pb-2 text-slate-600">→</span>
                      <label className="flex flex-col gap-1 text-[11px] text-slate-500">
                        Trunk
                        <select
                          className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                          value={row.trunkId}
                          onChange={(e) => {
                            const next = [...regionRows]
                            next[i] = { ...row, trunkId: e.target.value }
                            setRegionRows(next)
                          }}
                          disabled={Boolean(cfgStatus?.config_read_only)}
                        >
                          <option value="">—</option>
                          {trunks.map((t) => (
                            <option key={t.id} value={t.id}>
                              {t.id}
                            </option>
                          ))}
                        </select>
                      </label>
                      <button
                        type="button"
                        className="rounded border border-slate-800 px-2 py-1 text-xs text-slate-400 hover:bg-slate-900"
                        disabled={Boolean(cfgStatus?.config_read_only)}
                        onClick={() => setRegionRows(regionRows.filter((_, j) => j !== i))}
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                </div>
                <button
                  type="button"
                  className="mt-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-50"
                  disabled={Boolean(cfgStatus?.config_read_only)}
                  onClick={() => setRegionRows([...regionRows, { regionLabel: '', trunkId: trunks[0]?.id ?? '' }])}
                >
                  Add region mapping
                </button>
              </div>
            </div>
          )}

          <label className="mt-3 flex flex-col gap-1 text-xs text-slate-400">
            Metadata namespace / label (optional)
            <input
              className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
              value={sip.metadata_namespace ?? ''}
              onChange={(e) =>
                setSpec((s) => ({
                  ...s,
                  siprec: { ...(s.siprec ?? emptySiprec()), metadata_namespace: e.target.value },
                }))
              }
              placeholder="urn:bank:cti"
              disabled={Boolean(cfgStatus?.config_read_only)}
            />
          </label>
        </div>
      </div>

      <div className="mt-8 flex flex-wrap items-center gap-3">
        <button
          type="button"
          className="rounded-md bg-sky-600 px-4 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50"
          disabled={Boolean(cfgStatus?.config_read_only)}
          onClick={() => save()}
        >
          Save
        </button>
        <button
          type="button"
          className="rounded-md border border-slate-700 bg-slate-900 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800 disabled:opacity-50"
          disabled={Boolean(cfgStatus?.config_read_only)}
          onClick={() => clearSaved()}
        >
          Clear saved recording block
        </button>
        {status ? <span className="text-sm text-slate-400">{status}</span> : null}
      </div>

      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
      ) : null}
      {note ? <div className="mt-6 rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-xs text-slate-500">{note}</div> : null}
    </div>
  )
}
