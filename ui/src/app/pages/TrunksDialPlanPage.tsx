import { useCallback, useEffect, useMemo, useState } from 'react'
import yaml from 'js-yaml'
import { apiFetch, apiPutText } from '../api/client'
import type { ConfigStatus, DialPlanRule, RootConfig, SIPTrunkSpec } from '../api/types'

function cloneConfig(c: RootConfig): RootConfig {
  if (typeof structuredClone === 'function') return structuredClone(c) as RootConfig
  return JSON.parse(JSON.stringify(c)) as RootConfig
}

export default function TrunksDialPlanPage() {
  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [draft, setDraft] = useState<RootConfig | null>(null)
  const [cfgStatus, setCfgStatus] = useState<ConfigStatus | null>(null)
  const [status, setStatus] = useState('')
  const [err, setErr] = useState('')

  const canApply = useMemo(() => Boolean(draft?.apiVersion && draft?.kind && draft?.metadata?.name), [draft])

  const load = useCallback(async () => {
    const [c, st] = await Promise.all([
      apiFetch<RootConfig>('/v1/config'),
      apiFetch<ConfigStatus>('/v1/config/status').catch(() => null),
    ])
    setCfg(c)
    setDraft(cloneConfig(c))
    if (st) setCfgStatus(st)
  }, [])

  useEffect(() => {
    load().catch((e) => setErr(e instanceof Error ? e.message : String(e)))
  }, [load])

  function updateTrunk(idx: number, next: SIPTrunkSpec) {
    setDraft((d) => {
      if (!d) return d
      const sipTrunks = [...(d.spec.sipTrunks ?? [])]
      sipTrunks[idx] = next
      return { ...d, spec: { ...d.spec, sipTrunks } }
    })
  }

  function addTrunk() {
    setDraft((d) => {
      if (!d) return d
      const sipTrunks = [...(d.spec.sipTrunks ?? [])]
      sipTrunks.push({
        id: `trunk-${String(sipTrunks.length + 1).padStart(2, '0')}`,
        name: 'New trunk',
        proxy_addr: '',
        proxy_port: 5060,
        transport: 'udp',
        tls_insecure_skip_verify: false,
      })
      return { ...d, spec: { ...d.spec, sipTrunks } }
    })
  }

  function removeTrunk(idx: number) {
    setDraft((d) => {
      if (!d) return d
      const sipTrunks = [...(d.spec.sipTrunks ?? [])]
      sipTrunks.splice(idx, 1)
      return { ...d, spec: { ...d.spec, sipTrunks } }
    })
  }

  function updateRule(idx: number, next: DialPlanRule) {
    setDraft((d) => {
      if (!d) return d
      const dialPlan = [...(d.spec.dialPlan ?? [])]
      dialPlan[idx] = next
      return { ...d, spec: { ...d.spec, dialPlan } }
    })
  }

  function addRule() {
    setDraft((d) => {
      if (!d) return d
      const dialPlan = [...(d.spec.dialPlan ?? [])]
      dialPlan.push({
        id: `rule-${String(dialPlan.length + 1).padStart(2, '0')}`,
        enabled: true,
        user_prefix: '',
        domain: '',
        uri_regex: '',
        target_trunk_id: '',
      })
      return { ...d, spec: { ...d.spec, dialPlan } }
    })
  }

  function removeRule(idx: number) {
    setDraft((d) => {
      if (!d) return d
      const dialPlan = [...(d.spec.dialPlan ?? [])]
      dialPlan.splice(idx, 1)
      return { ...d, spec: { ...d.spec, dialPlan } }
    })
  }

  async function save() {
    setStatus('Saving...')
    setErr('')
    try {
      if (cfgStatus?.config_read_only) throw new Error('Config is read-only (CONFIG_HTTP_URL).')
      if (!draft) throw new Error('No config loaded')
      const body = yaml.dump(draft, { noRefs: true, lineWidth: 120 })
      await apiPutText('/v1/config', body)
      setStatus('Saved.')
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div>
      <div className="text-sm font-semibold text-slate-200">SIP Trunks &amp; Dial Plan</div>
      <p className="mt-1 text-sm text-slate-400">
        Configure Oracle/AudioCodes trunk profiles (including TLS certs) and dial-plan rules that route outbound SIP URIs to trunks.
      </p>

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="flex items-center justify-between">
          <div className="text-sm font-semibold">SIP trunks</div>
          <button className="rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-200 hover:bg-slate-800" onClick={() => addTrunk()}>
            Add trunk
          </button>
        </div>
        <div className="mt-4 grid grid-cols-1 gap-3">
          {(draft?.spec.sipTrunks ?? []).map((t, i) => (
            <div key={`${t.id}-${i}`} className="rounded-lg border border-slate-800 bg-slate-950 p-3">
              <div className="grid grid-cols-1 gap-2 md:grid-cols-12">
                <input className="md:col-span-2 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={t.id} onChange={(e) => updateTrunk(i, { ...t, id: e.target.value })} placeholder="oracle-emea" />
                <input className="md:col-span-2 rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100" value={t.name ?? ''} onChange={(e) => updateTrunk(i, { ...t, name: e.target.value })} placeholder="Oracle EMEA" />
                <input className="md:col-span-2 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={t.proxy_addr} onChange={(e) => updateTrunk(i, { ...t, proxy_addr: e.target.value })} placeholder="10.0.0.10" />
                <input className="md:col-span-1 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={String(t.proxy_port ?? 5060)} onChange={(e) => updateTrunk(i, { ...t, proxy_port: Number(e.target.value) || 0 })} />
                <select className="md:col-span-1 rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100" value={(t.transport ?? 'udp') as string} onChange={(e) => updateTrunk(i, { ...t, transport: e.target.value })}>
                  <option value="udp">udp</option>
                  <option value="tls">tls</option>
                </select>
                <input className="md:col-span-2 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={t.tls_server_name ?? ''} onChange={(e) => updateTrunk(i, { ...t, tls_server_name: e.target.value })} placeholder="sbc.company.net" />
                <label className="md:col-span-1 flex items-center gap-2 text-xs text-slate-300">
                  <input type="checkbox" checked={Boolean(t.tls_insecure_skip_verify)} onChange={(e) => updateTrunk(i, { ...t, tls_insecure_skip_verify: e.target.checked })} />
                  Skip verify
                </label>
              </div>
              <div className="mt-2 grid grid-cols-1 gap-2 md:grid-cols-3">
                <input className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={t.tls_root_ca_file ?? ''} onChange={(e) => updateTrunk(i, { ...t, tls_root_ca_file: e.target.value })} placeholder="CA file path" />
                <input className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={t.tls_client_cert_file ?? ''} onChange={(e) => updateTrunk(i, { ...t, tls_client_cert_file: e.target.value })} placeholder="Client cert path" />
                <input className="rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={t.tls_client_key_file ?? ''} onChange={(e) => updateTrunk(i, { ...t, tls_client_key_file: e.target.value })} placeholder="Client key path" />
              </div>
              <button className="mt-2 rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900" onClick={() => removeTrunk(i)}>
                Remove trunk
              </button>
            </div>
          ))}
          {!(draft?.spec.sipTrunks?.length ?? 0) ? <div className="rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-500">No trunks configured.</div> : null}
        </div>
      </section>

      <section className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="flex items-center justify-between">
          <div className="text-sm font-semibold">Dial plan rules (first match wins)</div>
          <button className="rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-200 hover:bg-slate-800" onClick={() => addRule()}>
            Add rule
          </button>
        </div>
        <div className="mt-4 overflow-hidden rounded-lg border border-slate-800">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-xs text-slate-400">
              <tr>
                <th className="px-2 py-2">On</th>
                <th className="px-2 py-2">Rule id</th>
                <th className="px-2 py-2">User prefix</th>
                <th className="px-2 py-2">Domain</th>
                <th className="px-2 py-2">URI regex</th>
                <th className="px-2 py-2">Target trunk</th>
                <th className="px-2 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {(draft?.spec.dialPlan ?? []).map((r, i) => (
                <tr key={`${r.id}-${i}`} className="border-t border-slate-800">
                  <td className="px-2 py-2"><input type="checkbox" checked={r.enabled !== false} onChange={(e) => updateRule(i, { ...r, enabled: e.target.checked })} /></td>
                  <td className="px-2 py-2"><input className="w-28 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={r.id} onChange={(e) => updateRule(i, { ...r, id: e.target.value })} /></td>
                  <td className="px-2 py-2"><input className="w-24 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={r.user_prefix ?? ''} onChange={(e) => updateRule(i, { ...r, user_prefix: e.target.value })} /></td>
                  <td className="px-2 py-2"><input className="w-36 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={r.domain ?? ''} onChange={(e) => updateRule(i, { ...r, domain: e.target.value })} /></td>
                  <td className="px-2 py-2"><input className="w-44 rounded border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-xs text-slate-100" value={r.uri_regex ?? ''} onChange={(e) => updateRule(i, { ...r, uri_regex: e.target.value })} /></td>
                  <td className="px-2 py-2">
                    <select className="w-36 rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-100" value={r.target_trunk_id} onChange={(e) => updateRule(i, { ...r, target_trunk_id: e.target.value })}>
                      <option value="">— select —</option>
                      {(draft?.spec.sipTrunks ?? []).map((t) => <option key={t.id} value={t.id}>{t.id}</option>)}
                    </select>
                  </td>
                  <td className="px-2 py-2 text-right">
                    <button className="rounded border border-slate-800 bg-slate-950 px-2 py-1 text-xs text-slate-200 hover:bg-slate-900" onClick={() => removeRule(i)}>Remove</button>
                  </td>
                </tr>
              ))}
              {!(draft?.spec.dialPlan?.length ?? 0) ? (
                <tr><td className="px-3 py-3 text-slate-500" colSpan={7}>No dial plan rules.</td></tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <div className="mt-6 flex items-center gap-3">
        <button className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm text-slate-200 hover:bg-slate-900" onClick={() => load().catch((e) => setErr(e instanceof Error ? e.message : String(e)))}>
          Refresh
        </button>
        <button className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50" disabled={!canApply || Boolean(cfgStatus?.config_read_only)} onClick={() => save()}>
          Save
        </button>
        {status ? <span className="text-sm text-slate-400">{status}</span> : null}
      </div>
      {err ? <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div> : null}
      {cfgStatus?.config_read_only ? <div className="mt-4 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">Read-only config (CONFIG_HTTP_URL). Save is disabled.</div> : null}
      <div className="mt-4 text-xs text-slate-500">Loaded config: <span className="font-mono">{cfg?.metadata?.name ?? '—'}</span></div>
    </div>
  )
}
