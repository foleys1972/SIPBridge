import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiFetch, apiPostJson, apiPutJson } from '../api/client'
import type { RecordingSettingsResponse, RootConfig, UserDetailResponse, UserDevice } from '../api/types'

type CTIRow = { key: string; value: string }

function ctiToRows(cti?: Record<string, string>): CTIRow[] {
  if (!cti || Object.keys(cti).length === 0) return [{ key: '', value: '' }]
  return Object.entries(cti).map(([key, value]) => ({ key, value: value ?? '' }))
}

function rowsToCTI(rows: CTIRow[]): Record<string, string> | undefined {
  const out: Record<string, string> = {}
  for (const r of rows) {
    const k = r.key.trim()
    if (k) out[k] = r.value
  }
  return Object.keys(out).length ? out : undefined
}

export default function UserDetailPage() {
  const { userId } = useParams<{ userId: string }>()
  const id = userId ? decodeURIComponent(userId) : ''

  const [cfgStatus, setCfgStatus] = useState<{ config_read_only?: boolean } | null>(null)
  const [detail, setDetail] = useState<UserDetailResponse['user'] | null>(null)
  const [bridges, setBridges] = useState<{ id: string; name: string }[]>([])
  const [groups, setGroups] = useState<{ id: string; name: string }[]>([])
  const [displayName, setDisplayName] = useState('')
  const [region, setRegion] = useState('')
  const [allowedBridges, setAllowedBridges] = useState<Set<string>>(new Set())
  const [allowedGroups, setAllowedGroups] = useState<Set<string>>(new Set())
  const [newPin, setNewPin] = useState('')
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')
  const [resetMsg, setResetMsg] = useState('')
  const [recordingOptIn, setRecordingOptIn] = useState(false)
  const [devices, setDevices] = useState<UserDevice[]>([])
  const [ctiRowsByIdx, setCtiRowsByIdx] = useState<CTIRow[][]>([])
  const [recordingRegionKeys, setRecordingRegionKeys] = useState<string[]>([])
  const [regionUseList, setRegionUseList] = useState(false)

  async function refresh() {
    if (!id) return
    setErr('')
    try {
      const [u, cfg, st, rec] = await Promise.all([
        apiFetch<UserDetailResponse>(`/v1/users/${encodeURIComponent(id)}`),
        apiFetch<RootConfig>('/v1/config'),
        apiFetch<{ config_read_only?: boolean }>('/v1/config/status').catch(() => null),
        apiFetch<RecordingSettingsResponse>('/v1/settings/recording').catch(() => null),
      ])
      setDetail(u.user)
      setDisplayName(u.user.display_name ?? '')
      setRegion(u.user.region ?? '')
      setAllowedBridges(new Set(u.user.allowed_bridge_ids ?? []))
      setAllowedGroups(new Set(u.user.allowed_conference_group_ids ?? []))
      setRecordingOptIn(Boolean(u.user.recording_opt_in))
      const devs = [...(u.user.devices ?? [])]
      setDevices(devs)
      setCtiRowsByIdx(devs.map((d) => ctiToRows(d.cti)))
      const keys = Object.keys(rec?.saved?.siprec?.region_to_trunk ?? {})
      setRecordingRegionKeys(keys)
      const ur = u.user.region ?? ''
      if (keys.length > 0 && keys.includes(ur)) {
        setRegionUseList(true)
      } else {
        setRegionUseList(false)
      }
      setBridges((cfg.spec.bridges ?? []).map((b) => ({ id: b.id, name: b.name })))
      setGroups((cfg.spec.conferenceGroups ?? []).map((g) => ({ id: g.id, name: g.name ?? g.id })))
      if (st) setCfgStatus(st)
    } catch (e) {
      setDetail(null)
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  useEffect(() => {
    refresh()
  }, [id])

  async function save() {
    if (!id || !detail) return
    setErr('')
    setStatus('Saving…')
    try {
      const mergedDevices: UserDevice[] = devices.map((d, i) => {
        const cti = rowsToCTI(ctiRowsByIdx[i] ?? [{ key: '', value: '' }])
        return { ...d, cti }
      })
      const body: Record<string, unknown> = {
        display_name: displayName,
        region,
        allowed_bridge_ids: Array.from(allowedBridges),
        allowed_conference_group_ids: Array.from(allowedGroups),
        recording_opt_in: recordingOptIn,
        devices: mergedDevices,
      }
      const pin = newPin.replace(/\D+/g, '')
      if (pin) {
        body.participant_id = pin
      }
      await apiPutJson(`/v1/users/${encodeURIComponent(id)}`, body)
      setNewPin('')
      setStatus('Saved')
      await refresh()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  async function resetPin() {
    if (!id) return
    setErr('')
    setResetMsg('')
    try {
      const res = await apiPostJson<{ ok: boolean; new_pin?: string; error?: string }>(
        `/v1/users/${encodeURIComponent(id)}/reset-pin`,
        {},
      )
      if (res.new_pin) {
        setResetMsg(`New PIN (copy now; it will not be shown again): ${res.new_pin}`)
      }
      await refresh()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  if (!id) {
    return <div className="text-sm text-slate-400">Missing employee ID in URL.</div>
  }

  return (
    <div>
      <div className="flex flex-wrap items-center gap-3">
        <Link className="text-sm text-sky-400 hover:text-sky-300" to="/settings/users">
          ← Users
        </Link>
      </div>

      <div className="mt-4 text-lg font-semibold text-slate-100">{detail?.display_name || id}</div>
      <div className="mt-2">
        <div className="text-xs font-medium uppercase tracking-wide text-slate-500">Employee ID</div>
        <div className="font-mono text-sm text-slate-200">{detail?.employee_id ?? detail?.id ?? id}</div>
        <p className="mt-1 text-xs text-slate-500">Fixed when the user was created; it matches HR records and appears on live calls and MI.</p>
      </div>

      {cfgStatus?.config_read_only ? (
        <div className="mt-4 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">
          Read-only config (CONFIG_HTTP_URL). User edits are disabled at the API.
        </div>
      ) : null}

      <div className="mt-6 grid max-w-xl grid-cols-1 gap-4">
        <label className="flex flex-col gap-1 text-xs text-slate-400">
          Display name
          <input
            className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 text-sm text-slate-100"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
          />
        </label>
        <div>
          <div className="text-xs font-semibold text-slate-300">Region (routing &amp; recording trunk)</div>
          <p className="mt-1 text-xs text-slate-500">
            Used for endpoint ordering and to pick a SIPREC trunk when region mappings exist under Settings → Recording.
          </p>
          {recordingRegionKeys.length > 0 ? (
            <div className="mt-2 space-y-2">
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  className={`rounded-md border px-3 py-1.5 text-xs ${
                    regionUseList
                      ? 'border-sky-600 bg-sky-950/50 text-sky-100'
                      : 'border-slate-700 bg-slate-900 text-slate-300 hover:bg-slate-800'
                  }`}
                  onClick={() => {
                    setRegionUseList(true)
                    if (recordingRegionKeys.length > 0 && !recordingRegionKeys.includes(region)) {
                      setRegion(recordingRegionKeys[0])
                    }
                  }}
                >
                  Choose mapped region
                </button>
                <button
                  type="button"
                  className={`rounded-md border px-3 py-1.5 text-xs ${
                    !regionUseList
                      ? 'border-sky-600 bg-sky-950/50 text-sky-100'
                      : 'border-slate-700 bg-slate-900 text-slate-300 hover:bg-slate-800'
                  }`}
                  onClick={() => setRegionUseList(false)}
                >
                  Custom label
                </button>
              </div>
              {regionUseList ? (
                <select
                  className="w-full max-w-md rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                  value={recordingRegionKeys.includes(region) ? region : recordingRegionKeys[0]}
                  onChange={(e) => setRegion(e.target.value)}
                >
                  {recordingRegionKeys.map((k) => (
                    <option key={k} value={k}>
                      {k}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  className="w-full max-w-md rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
                  value={region}
                  onChange={(e) => setRegion(e.target.value)}
                  placeholder="e.g. EMEA, LDN"
                />
              )}
            </div>
          ) : (
            <input
              className="mt-2 w-full max-w-md rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
              value={region}
              onChange={(e) => setRegion(e.target.value)}
              placeholder="EMEA"
            />
          )}
        </div>

        <label className="flex items-center gap-2 text-sm text-slate-300">
          <input
            type="checkbox"
            checked={recordingOptIn}
            onChange={(e) => setRecordingOptIn(e.target.checked)}
            disabled={Boolean(cfgStatus?.config_read_only)}
          />
          Eligible for recording (PIN dial-in): include employee id and metadata when global recording is on
        </label>

        <div>
          <div className="text-xs font-semibold text-slate-300">Devices (DDI / mobile / private wire)</div>
          <p className="mt-1 text-xs text-slate-500">
            Used for SIPREC CTI when a conference endpoint links to this user and device. Add one row per instrument.
          </p>
          <div className="mt-2 space-y-3">
            {devices.map((d, i) => (
              <div key={i} className="rounded-lg border border-slate-800 bg-slate-950 p-3">
                <div className="grid grid-cols-1 gap-2 md:grid-cols-4">
                  <label className="flex flex-col gap-1 text-[11px] text-slate-500">
                    Device id
                    <input
                      className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                      value={d.id}
                      onChange={(e) => {
                        const next = [...devices]
                        next[i] = { ...d, id: e.target.value }
                        setDevices(next)
                      }}
                      disabled={Boolean(cfgStatus?.config_read_only)}
                    />
                  </label>
                  <label className="flex flex-col gap-1 text-[11px] text-slate-500">
                    Kind
                    <select
                      className="rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100"
                      value={d.kind}
                      onChange={(e) => {
                        const next = [...devices]
                        next[i] = { ...d, kind: e.target.value as UserDevice['kind'] }
                        setDevices(next)
                      }}
                      disabled={Boolean(cfgStatus?.config_read_only)}
                    >
                      <option value="ddi">DDI</option>
                      <option value="mobile">Mobile</option>
                      <option value="private_wire">Private wire</option>
                    </select>
                  </label>
                  <label className="flex flex-col gap-1 text-[11px] text-slate-500 md:col-span-2">
                    Address / number
                    <input
                      className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                      value={d.address ?? ''}
                      onChange={(e) => {
                        const next = [...devices]
                        next[i] = { ...d, address: e.target.value }
                        setDevices(next)
                      }}
                      placeholder="+4420..."
                      disabled={Boolean(cfgStatus?.config_read_only)}
                    />
                  </label>
                </div>
                <div className="mt-2">
                  <div className="text-[11px] text-slate-500">CTI (metadata keys for recorder)</div>
                  <div className="mt-1 space-y-2">
                    {(ctiRowsByIdx[i] ?? [{ key: '', value: '' }]).map((row, ri) => (
                      <div key={ri} className="flex flex-wrap items-center gap-2">
                        <input
                          className="min-w-[120px] flex-1 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-[11px] text-slate-100"
                          placeholder="Key"
                          value={row.key}
                          onChange={(e) => {
                            const rows = [...(ctiRowsByIdx[i] ?? [{ key: '', value: '' }])]
                            rows[ri] = { ...row, key: e.target.value }
                            const next = [...ctiRowsByIdx]
                            next[i] = rows
                            setCtiRowsByIdx(next)
                          }}
                          disabled={Boolean(cfgStatus?.config_read_only)}
                        />
                        <input
                          className="min-w-[120px] flex-1 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-[11px] text-slate-100"
                          placeholder="Value"
                          value={row.value}
                          onChange={(e) => {
                            const rows = [...(ctiRowsByIdx[i] ?? [{ key: '', value: '' }])]
                            rows[ri] = { ...row, value: e.target.value }
                            const next = [...ctiRowsByIdx]
                            next[i] = rows
                            setCtiRowsByIdx(next)
                          }}
                          disabled={Boolean(cfgStatus?.config_read_only)}
                        />
                        <button
                          type="button"
                          className="text-[11px] text-slate-500 hover:text-rose-300"
                          disabled={Boolean(cfgStatus?.config_read_only)}
                          onClick={() => {
                            const rows = (ctiRowsByIdx[i] ?? [{ key: '', value: '' }]).filter((_, j) => j !== ri)
                            const next = [...ctiRowsByIdx]
                            next[i] = rows.length ? rows : [{ key: '', value: '' }]
                            setCtiRowsByIdx(next)
                          }}
                        >
                          Remove
                        </button>
                      </div>
                    ))}
                  </div>
                  <button
                    type="button"
                    className="mt-1 text-xs text-sky-400 hover:text-sky-300"
                    disabled={Boolean(cfgStatus?.config_read_only)}
                    onClick={() => {
                      const rows = [...(ctiRowsByIdx[i] ?? [{ key: '', value: '' }]), { key: '', value: '' }]
                      const next = [...ctiRowsByIdx]
                      next[i] = rows
                      setCtiRowsByIdx(next)
                    }}
                  >
                    Add CTI field
                  </button>
                </div>
                <button
                  type="button"
                  className="mt-2 text-xs text-rose-300 hover:text-rose-200"
                  disabled={Boolean(cfgStatus?.config_read_only)}
                  onClick={() => {
                    setDevices(devices.filter((_, j) => j !== i))
                    setCtiRowsByIdx(ctiRowsByIdx.filter((_, j) => j !== i))
                  }}
                >
                  Remove device
                </button>
              </div>
            ))}
          </div>
          <button
            type="button"
            className="mt-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-50"
            disabled={Boolean(cfgStatus?.config_read_only)}
            onClick={() => {
              setDevices([...devices, { id: `dev-${devices.length + 1}`, kind: 'ddi', address: '', cti: {} }])
              setCtiRowsByIdx([...ctiRowsByIdx, [{ key: '', value: '' }]])
            }}
          >
            Add device
          </button>
        </div>

        <div>
          <div className="text-xs text-slate-400">PIN</div>
          <div className="mt-1 flex flex-wrap items-center gap-3">
            <span className="rounded-md border border-slate-800 bg-slate-900 px-3 py-2 font-mono text-sm tracking-widest text-slate-200">
              {detail?.pin_masked || (detail?.pin_set ? '••••••' : '—')}
            </span>
            <button
              type="button"
              disabled={Boolean(cfgStatus?.config_read_only)}
              className="rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 hover:bg-slate-800 disabled:opacity-50"
              onClick={() => resetPin()}
            >
              Reset PIN
            </button>
          </div>
          <p className="mt-2 text-xs text-slate-500">Set a new PIN below, or use Reset to generate a random 6-digit PIN.</p>
          <input
            className="mt-2 w-48 rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
            value={newPin}
            onChange={(e) => setNewPin(e.target.value.replace(/\D+/g, ''))}
            placeholder="New PIN (digits)"
            inputMode="numeric"
            disabled={Boolean(cfgStatus?.config_read_only)}
          />
        </div>

        <div>
          <div className="text-xs font-semibold text-slate-300">Allowed bridges</div>
          <div className="mt-2 flex flex-wrap gap-2">
            {bridges.map((b) => {
              const checked = allowedBridges.has(b.id)
              return (
                <label
                  key={b.id}
                  className="flex items-center gap-2 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200"
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    disabled={Boolean(cfgStatus?.config_read_only)}
                    onChange={(e) => {
                      const next = new Set(allowedBridges)
                      if (e.target.checked) next.add(b.id)
                      else next.delete(b.id)
                      setAllowedBridges(next)
                    }}
                  />
                  <span className="font-mono">{b.id}</span>
                  <span className="text-slate-500">{b.name}</span>
                </label>
              )
            })}
            {bridges.length === 0 ? <span className="text-xs text-slate-500">No bridges in config.</span> : null}
          </div>
        </div>

        <div>
          <div className="text-xs font-semibold text-slate-300">Allowed conference groups</div>
          <div className="mt-2 flex flex-wrap gap-2">
            {groups.map((g) => {
              const checked = allowedGroups.has(g.id)
              return (
                <label
                  key={g.id}
                  className="flex items-center gap-2 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200"
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    disabled={Boolean(cfgStatus?.config_read_only)}
                    onChange={(e) => {
                      const next = new Set(allowedGroups)
                      if (e.target.checked) next.add(g.id)
                      else next.delete(g.id)
                      setAllowedGroups(next)
                    }}
                  />
                  <span className="font-mono">{g.id}</span>
                  <span className="text-slate-500">{g.name}</span>
                </label>
              )
            })}
            {groups.length === 0 ? <span className="text-xs text-slate-500">No conference groups in config.</span> : null}
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            disabled={Boolean(cfgStatus?.config_read_only)}
            className="rounded-md bg-sky-600 px-4 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50"
            onClick={() => save()}
          >
            Save
          </button>
          {status ? <span className="text-sm text-slate-400">{status}</span> : null}
        </div>
      </div>

      {resetMsg ? (
        <div className="mt-4 rounded-lg border border-sky-900 bg-sky-950/40 px-3 py-2 text-sm text-sky-100">{resetMsg}</div>
      ) : null}
      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
      ) : null}
    </div>
  )
}
