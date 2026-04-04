import { useCallback, useEffect, useState } from 'react'
import { apiFetch, apiPutJson } from '../api/client'
import type { ManagedServer, ServersListResponse, ServersProbeResponse } from '../api/types'

function emptyServer(): ManagedServer {
  return {
    id: '',
    name: '',
    api_base_url: 'http://',
    region: '',
    tls_skip_verify: false,
    sip_ingress_uri: '',
    interconnect_sip_uri: '',
    capacity_weight: 0,
  }
}

export default function ServersPage() {
  const [localId, setLocalId] = useState('')
  const [servers, setServers] = useState<ManagedServer[]>([])
  const [probed, setProbed] = useState<ServersProbeResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [probing, setProbing] = useState(false)
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')

  const load = useCallback(async () => {
    setErr('')
    const res = await apiFetch<ServersListResponse>('/v1/servers')
    setLocalId(res.local_instance_id ?? '')
    setServers(res.servers?.length ? res.servers : [])
    setProbed(null)
  }, [])

  useEffect(() => {
    setLoading(true)
    load()
      .catch((e) => setErr(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false))
  }, [load])

  async function save() {
    setSaving(true)
    setErr('')
    setStatus('')
    try {
      for (const s of servers) {
        if (!String(s.id).trim()) throw new Error('Each server needs an id')
        if (!String(s.name).trim()) throw new Error(`Server ${s.id}: name is required`)
        if (!String(s.api_base_url).trim()) throw new Error(`Server ${s.id}: API base URL is required`)
      }
      const ids = new Set<string>()
      for (const s of servers) {
        if (ids.has(s.id)) throw new Error(`Duplicate id: ${s.id}`)
        ids.add(s.id)
      }
      await apiPutJson<{ ok: boolean }>('/v1/servers', { servers })
      setStatus('Saved to config.')
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  async function runProbe() {
    setProbing(true)
    setErr('')
    try {
      const res = await apiFetch<ServersProbeResponse>('/v1/servers?probe=true')
      setProbed(res)
      setLocalId(res.local_instance_id ?? '')
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setProbing(false)
    }
  }

  function updateRow(i: number, next: ManagedServer) {
    setServers((rows) => {
      const copy = [...rows]
      copy[i] = next
      return copy
    })
  }

  function addRow() {
    setServers((rows) => [...rows, emptyServer()])
  }

  function removeRow(i: number) {
    setServers((rows) => rows.filter((_, j) => j !== i))
  }

  const rowsToShow = probed?.servers ?? null

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold text-slate-100">SIPBridge servers</h1>
        <p className="mt-1 text-sm text-slate-400">
          Register other SIPBridge nodes by their <span className="font-mono">HTTP API</span> base URL (same as you use
          in the UI proxy). This console stores inventory in <span className="font-mono">config.yaml</span> and can probe{' '}
          <span className="font-mono">/healthz</span> on each peer. SIP signaling is not clustered; each process keeps its
          own calls.
        </p>
        <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900/50 p-4 text-sm text-slate-400">
          <div className="font-medium text-slate-200">Ingress vs interconnect (all servers “seeing” each other)</div>
          <ul className="mt-2 list-inside list-disc space-y-1.5 text-xs leading-relaxed">
            <li>
              <span className="font-mono text-slate-300">api_base_url</span> — HTTP only: operations console and health
              probes. Required for each peer row. This does not carry SIP media or signaling between nodes.
            </li>
            <li>
              <span className="font-mono text-slate-300">sip_ingress_uri</span> — Per-node hint: where your{' '}
              <span className="text-slate-300">load balancer / VIP / DNS</span> should send new inbound SIP for that node
              (often different per server). Used for documentation and cluster views, not auto-wiring.
            </li>
            <li>
              <span className="font-mono text-slate-300">interconnect_sip_uri</span> — Optional{' '}
              <span className="font-mono">sip:</span> target toward this peer’s SIP path (commonly via your SBC).
              Listing every server here does <span className="text-slate-300">not</span> create a full mesh by itself: you
              still configure trunks / routing in the SBC (or add participants) so nodes that must exchange media actually
              have a SIP path. For N nodes talking pairwise, your network design (hub SBC or pairwise trunks) applies.
            </li>
          </ul>
        </div>
        {localId ? (
          <p className="mt-2 text-xs text-slate-500">
            This instance id (env <span className="font-mono">SIPBRIDGE_INSTANCE_ID</span>):{' '}
            <span className="font-mono text-slate-300">{localId}</span>
          </p>
        ) : null}
      </div>

      {loading ? (
        <div className="text-sm text-slate-400">Loading…</div>
      ) : (
        <>
          {err ? (
            <div className="rounded-md border border-red-900 bg-red-950/40 px-3 py-2 text-sm text-red-200">{err}</div>
          ) : null}
          {status ? (
            <div className="rounded-md border border-emerald-900 bg-emerald-950/30 px-3 py-2 text-sm text-emerald-200">
              {status}
            </div>
          ) : null}

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              className="rounded-md bg-slate-100 px-3 py-1.5 text-sm font-medium text-slate-900 hover:bg-white disabled:opacity-50"
              onClick={() => save()}
              disabled={saving}
            >
              {saving ? 'Saving…' : 'Save inventory'}
            </button>
            <button
              type="button"
              className="rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-sm text-slate-100 hover:bg-slate-800 disabled:opacity-50"
              onClick={() => runProbe()}
              disabled={probing || servers.length === 0}
            >
              {probing ? 'Probing…' : 'Probe all (healthz)'}
            </button>
            <button
              type="button"
              className="rounded-md border border-slate-700 px-3 py-1.5 text-sm text-slate-300 hover:bg-slate-900"
              onClick={() => {
                setProbed(null)
                load().catch(() => {})
              }}
            >
              Clear probe / reload
            </button>
          </div>

          <div className="overflow-x-auto rounded-lg border border-slate-800">
            <table className="min-w-full text-left text-sm">
              <thead className="border-b border-slate-800 bg-slate-900/60 text-xs uppercase text-slate-500">
                <tr>
                  <th className="px-3 py-2">Id</th>
                  <th className="px-3 py-2">Name</th>
                  <th className="px-3 py-2">API base URL</th>
                  <th
                    className="px-3 py-2"
                    title="Where external LB/carrier sends new SIP to this node (per-node hint)"
                  >
                    SIP ingress (LB)
                  </th>
                  <th
                    className="px-3 py-2"
                    title="SIP URI toward this peer (SBC/trunk); optional; does not auto-mesh all servers"
                  >
                    Interconnect
                  </th>
                  <th className="px-3 py-2">Wt</th>
                  <th className="px-3 py-2">Region</th>
                  <th className="px-3 py-2">TLS skip verify</th>
                  {rowsToShow ? <th className="px-3 py-2">Probe</th> : null}
                  <th className="px-3 py-2" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {servers.length === 0 ? (
                  <tr>
                    <td className="px-3 py-4 text-slate-500" colSpan={rowsToShow ? 10 : 9}>
                      No peer servers yet. Add a row, then save.
                    </td>
                  </tr>
                ) : (
                  servers.map((s, i) => {
                    const probeRow = rowsToShow?.find((r) => r.id === s.id)
                    return (
                      <tr key={i} className="bg-slate-950/40">
                        <td className="px-3 py-2 align-top">
                          <input
                            className="w-36 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                            value={s.id}
                            onChange={(e) => updateRow(i, { ...s, id: e.target.value })}
                            placeholder="east-1"
                          />
                        </td>
                        <td className="px-3 py-2 align-top">
                          <input
                            className="w-44 rounded border border-slate-800 bg-slate-950 px-2 py-1 text-slate-100"
                            value={s.name}
                            onChange={(e) => updateRow(i, { ...s, name: e.target.value })}
                          />
                        </td>
                        <td className="px-3 py-2 align-top">
                          <input
                            className="min-w-[14rem] rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                            value={s.api_base_url}
                            onChange={(e) => updateRow(i, { ...s, api_base_url: e.target.value })}
                            placeholder="http://10.0.0.5:8080"
                          />
                        </td>
                        <td className="px-3 py-2 align-top">
                          <input
                            className="min-w-[10rem] rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-[10px] text-slate-100"
                            value={s.sip_ingress_uri ?? ''}
                            onChange={(e) => updateRow(i, { ...s, sip_ingress_uri: e.target.value })}
                            placeholder="sip:pool@node-a"
                          />
                        </td>
                        <td className="px-3 py-2 align-top">
                          <input
                            className="min-w-[10rem] rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-[10px] text-slate-100"
                            value={s.interconnect_sip_uri ?? ''}
                            onChange={(e) => updateRow(i, { ...s, interconnect_sip_uri: e.target.value })}
                            placeholder="sip:trunk@peer"
                          />
                        </td>
                        <td className="px-3 py-2 align-top">
                          <input
                            type="number"
                            className="w-14 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100"
                            value={Number(s.capacity_weight ?? 0)}
                            min={0}
                            max={100}
                            onChange={(e) =>
                              updateRow(i, { ...s, capacity_weight: Number(e.target.value) || 0 })
                            }
                          />
                        </td>
                        <td className="px-3 py-2 align-top">
                          <input
                            className="w-28 rounded border border-slate-800 bg-slate-950 px-2 py-1 text-slate-100"
                            value={s.region ?? ''}
                            onChange={(e) => updateRow(i, { ...s, region: e.target.value })}
                          />
                        </td>
                        <td className="px-3 py-2 align-top">
                          <input
                            type="checkbox"
                            checked={Boolean(s.tls_skip_verify)}
                            onChange={(e) => updateRow(i, { ...s, tls_skip_verify: e.target.checked })}
                          />
                        </td>
                        {rowsToShow ? (
                          <td className="px-3 py-2 align-top text-xs">
                            {probeRow?.probe ? (
                              probeRow.probe.ok ? (
                                <span className="text-emerald-400">
                                  OK {probeRow.probe.latency_ms}ms
                                </span>
                              ) : (
                                <span className="text-red-400" title={probeRow.probe.error}>
                                  fail
                                </span>
                              )
                            ) : (
                              <span className="text-slate-600">—</span>
                            )}
                          </td>
                        ) : null}
                        <td className="px-3 py-2 align-top">
                          <button
                            type="button"
                            className="text-xs text-slate-500 hover:text-red-300"
                            onClick={() => removeRow(i)}
                          >
                            Remove
                          </button>
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>

          <button
            type="button"
            className="text-sm text-slate-400 underline hover:text-slate-200"
            onClick={() => addRow()}
          >
            + Add server
          </button>

          {probeRowDetail(rowsToShow)}
        </>
      )}
    </div>
  )
}

function probeRowDetail(rows: ServersProbeResponse['servers'] | null) {
  if (!rows?.length) return null
  const failed = rows.filter((r) => !r.probe?.ok)
  if (!failed.length) return null
  return (
    <div className="rounded-md border border-slate-800 bg-slate-900/50 p-3 text-xs text-slate-400">
      <div className="font-medium text-slate-300">Probe errors</div>
      <ul className="mt-2 list-inside list-disc space-y-1 font-mono">
        {failed.map((r) => (
          <li key={r.id}>
            <span className="text-slate-200">{r.id}</span>: {r.probe?.error || 'unknown'}
          </li>
        ))}
      </ul>
    </div>
  )
}
