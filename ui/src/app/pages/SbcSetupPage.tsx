import { useCallback, useEffect, useMemo, useState } from 'react'
import { ChevronLeft, ChevronRight, Network, Radio } from 'lucide-react'
import { apiDelete, apiFetch, apiPutJson } from '../api/client'
import type { SIPConfig, SIPSettingsResponse, SIPStackSpec } from '../api/types'

const steps = [
  { id: 0, title: 'Listener', desc: 'Where SIPBridge receives SIP (UDP).' },
  { id: 1, title: 'Path to SBC', desc: 'How signaling reaches your Oracle / AudioCodes SBC.' },
  { id: 2, title: 'TLS & mTLS', desc: 'Trust anchor and optional client certificates.' },
  { id: 3, title: 'Advanced', desc: 'NAT, session timers, lab options.' },
  { id: 4, title: 'Review & save', desc: 'Persist to config.yaml and restart.' },
]

function effectiveToForm(e: SIPConfig): Record<string, string | number | boolean> {
  return {
    bind_addr: e.bind_addr,
    udp_port: e.udp_port,
    outbound_proxy_addr: e.outbound_proxy_addr,
    outbound_proxy_port: e.outbound_proxy_port,
    outbound_transport: e.outbound_transport,
    advertise_addr: e.advertise_addr,
    tls_root_ca_file: e.tls_root_ca_file,
    tls_client_cert_file: e.tls_client_cert_file,
    tls_client_key_file: e.tls_client_key_file,
    tls_insecure_skip_verify: e.tls_insecure_skip_verify,
    tls_server_name: e.tls_server_name,
    session_timer_enabled: e.session_timer_enabled,
  }
}

function formToSpec(f: Record<string, string | number | boolean>): SIPStackSpec {
  const out: SIPStackSpec = {}
  const s = (k: string) => String(f[k] ?? '').trim()
  const n = (k: string) => {
    const v = Number(f[k])
    return Number.isFinite(v) ? v : 0
  }
  out.bind_addr = s('bind_addr') || undefined
  const port = n('udp_port')
  if (port > 0) out.udp_port = port
  out.outbound_proxy_addr = s('outbound_proxy_addr') || undefined
  const opp = n('outbound_proxy_port')
  if (opp > 0) out.outbound_proxy_port = opp
  const tx = s('outbound_transport').toLowerCase()
  if (tx === 'udp' || tx === 'tls') out.outbound_transport = tx
  out.advertise_addr = s('advertise_addr') || undefined
  out.tls_root_ca_file = s('tls_root_ca_file') || undefined
  out.tls_client_cert_file = s('tls_client_cert_file') || undefined
  out.tls_client_key_file = s('tls_client_key_file') || undefined
  out.tls_insecure_skip_verify = Boolean(f.tls_insecure_skip_verify)
  out.tls_server_name = s('tls_server_name') || undefined
  out.session_timer_enabled = Boolean(f.session_timer_enabled)
  return out
}

export default function SbcSetupPage() {
  const [step, setStep] = useState(0)
  const [form, setForm] = useState<Record<string, string | number | boolean>>({})
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')
  const [hasSaved, setHasSaved] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setErr('')
    const data = await apiFetch<SIPSettingsResponse>('/v1/settings/sip')
    const base = data.saved
      ? { ...effectiveToForm(data.effective), ...flattenSaved(data.saved) }
      : effectiveToForm(data.effective)
    setForm(base)
    setHasSaved(Boolean(data.saved))
    setLoading(false)
  }, [])

  useEffect(() => {
    load().catch((e) => {
      setErr(e instanceof Error ? e.message : String(e))
      setLoading(false)
    })
  }, [load])

  function flattenSaved(s: SIPStackSpec): Record<string, string | number | boolean> {
    const o: Record<string, string | number | boolean> = {}
    if (s.bind_addr != null) o.bind_addr = s.bind_addr
    if (s.udp_port != null) o.udp_port = s.udp_port
    if (s.outbound_proxy_addr != null) o.outbound_proxy_addr = s.outbound_proxy_addr
    if (s.outbound_proxy_port != null) o.outbound_proxy_port = s.outbound_proxy_port
    if (s.outbound_transport != null) o.outbound_transport = s.outbound_transport
    if (s.advertise_addr != null) o.advertise_addr = s.advertise_addr
    if (s.tls_root_ca_file != null) o.tls_root_ca_file = s.tls_root_ca_file
    if (s.tls_client_cert_file != null) o.tls_client_cert_file = s.tls_client_cert_file
    if (s.tls_client_key_file != null) o.tls_client_key_file = s.tls_client_key_file
    if (s.tls_insecure_skip_verify != null) o.tls_insecure_skip_verify = s.tls_insecure_skip_verify
    if (s.tls_server_name != null) o.tls_server_name = s.tls_server_name
    if (s.session_timer_enabled != null) o.session_timer_enabled = s.session_timer_enabled
    return o
  }

  async function save() {
    setStatus('Saving…')
    setErr('')
    try {
      const spec = formToSpec(form)
      await apiPutJson<{ ok: boolean; restart_required?: boolean; message?: string }>('/v1/settings/sip', spec)
      setStatus('Saved. Restart the sipbridge process to apply.')
      setHasSaved(true)
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  async function clearSaved() {
    if (!confirm('Remove spec.sipStack from config.yaml and rely on environment variables only?')) return
    setStatus('Removing…')
    setErr('')
    try {
      await apiDelete<{ ok: boolean }>('/v1/settings/sip')
      setStatus('Removed. Restart sipbridge to apply.')
      setHasSaved(false)
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  const setField = (k: string, v: string | number | boolean) => {
    setForm((f) => ({ ...f, [k]: v }))
  }

  const tlsRequired = useMemo(() => String(form.outbound_transport || '').toLowerCase() === 'tls', [form.outbound_transport])

  const canNext = useMemo(() => {
    if (step === 0) return Boolean(String(form.bind_addr || '').trim()) && Number(form.udp_port) > 0
    if (step === 1) {
      if (!tlsRequired) return true
      return Boolean(String(form.outbound_proxy_addr || '').trim()) && Number(form.outbound_proxy_port) > 0
    }
    return true
  }, [step, form, tlsRequired])

  if (loading) {
    return <div className="text-sm text-slate-400">Loading SIP settings…</div>
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold text-slate-100">SBC & SIP setup</h1>
        <p className="mt-1 text-sm text-slate-400">
          Guided steps to point SIPBridge at an Oracle or AudioCodes SBC (TLS, mTLS, optional session timers). Values are
          stored in <code className="text-slate-300">config.yaml</code> under <code className="text-slate-300">spec.sipStack</code>{' '}
          and override environment variables at startup.
        </p>
      </div>

      <div className="flex flex-wrap gap-2">
        {steps.map((st, i) => (
          <button
            key={st.id}
            type="button"
            onClick={() => setStep(i)}
            className={[
              'rounded-lg border px-3 py-2 text-left text-xs transition',
              i === step ? 'border-sky-600 bg-slate-900 text-slate-50' : 'border-slate-800 text-slate-400 hover:border-slate-600',
            ].join(' ')}
          >
            <div className="font-medium text-slate-200">{i + 1}. {st.title}</div>
            <div className="text-slate-500">{st.desc}</div>
          </button>
        ))}
      </div>

      {err && (
        <div className="rounded-lg border border-red-900 bg-red-950/40 px-3 py-2 text-sm text-red-200">{err}</div>
      )}
      {status && (
        <div className="rounded-lg border border-sky-900 bg-sky-950/30 px-3 py-2 text-sm text-sky-200">{status}</div>
      )}

      <div className="rounded-xl border border-slate-800 bg-slate-950/60 p-5">
        {step === 0 && (
          <div className="space-y-4">
            <div className="flex items-center gap-2 text-slate-200">
              <Radio size={18} /> 1. Listener (UDP)
            </div>
            <label className="block text-xs text-slate-400">
              Bind address
              <input
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={String(form.bind_addr ?? '')}
                onChange={(e) => setField('bind_addr', e.target.value)}
                placeholder="0.0.0.0"
              />
            </label>
            <label className="block text-xs text-slate-400">
              UDP port
              <input
                type="number"
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={Number(form.udp_port) || 5060}
                onChange={(e) => setField('udp_port', parseInt(e.target.value, 10) || 0)}
              />
            </label>
          </div>
        )}

        {step === 1 && (
          <div className="space-y-4">
            <div className="flex items-center gap-2 text-slate-200">
              <Network size={18} /> 2. Path to SBC
            </div>
            <p className="text-xs text-slate-500">
              Choose <strong className="text-slate-300">tls</strong> when the SBC expects SIP over TLS (SIPS) on the next hop.
              Set the SBC <strong className="text-slate-300">IP</strong> and port (often 5061) for the outbound proxy.
            </p>
            <label className="block text-xs text-slate-400">
              Outbound transport
              <select
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 text-sm text-slate-100"
                value={String(form.outbound_transport ?? 'udp')}
                onChange={(e) => setField('outbound_transport', e.target.value)}
              >
                <option value="udp">udp (direct SIP)</option>
                <option value="tls">tls (SIPS to SBC)</option>
              </select>
            </label>
            <label className="block text-xs text-slate-400">
              SBC / outbound proxy address (IPv4)
              <input
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={String(form.outbound_proxy_addr ?? '')}
                onChange={(e) => setField('outbound_proxy_addr', e.target.value)}
                placeholder="10.0.0.5"
                disabled={!tlsRequired}
              />
            </label>
            <label className="block text-xs text-slate-400">
              SBC / outbound proxy port
              <input
                type="number"
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={Number(form.outbound_proxy_port) || (tlsRequired ? 5061 : 0)}
                onChange={(e) => setField('outbound_proxy_port', parseInt(e.target.value, 10) || 0)}
                disabled={!tlsRequired}
              />
            </label>
          </div>
        )}

        {step === 2 && (
          <div className="space-y-4">
            <div className="text-slate-200">3. TLS &amp; mTLS</div>
            <p className="text-xs text-slate-500">
              Paths are on the <strong className="text-slate-300">sipbridge host</strong>. Place PEM files on the server, then
              enter the paths. Use <strong className="text-slate-300">SNI</strong> when the certificate name does not match the SIP proxy IP.
            </p>
            <label className="block text-xs text-slate-400">
              TLS root CA file (PEM)
              <input
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={String(form.tls_root_ca_file ?? '')}
                onChange={(e) => setField('tls_root_ca_file', e.target.value)}
                placeholder="C:\certs\sbc-ca.pem"
              />
            </label>
            <label className="block text-xs text-slate-400">
              Client certificate (PEM)
              <input
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={String(form.tls_client_cert_file ?? '')}
                onChange={(e) => setField('tls_client_cert_file', e.target.value)}
              />
            </label>
            <label className="block text-xs text-slate-400">
              Client private key (PEM)
              <input
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={String(form.tls_client_key_file ?? '')}
                onChange={(e) => setField('tls_client_key_file', e.target.value)}
              />
            </label>
            <label className="block text-xs text-slate-400">
              TLS server name (SNI)
              <input
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={String(form.tls_server_name ?? '')}
                onChange={(e) => setField('tls_server_name', e.target.value)}
                placeholder="sbc.example.com"
              />
            </label>
            <label className="flex items-center gap-2 text-xs text-slate-300">
              <input
                type="checkbox"
                checked={Boolean(form.tls_insecure_skip_verify)}
                onChange={(e) => setField('tls_insecure_skip_verify', e.target.checked)}
              />
              Insecure TLS verify (lab only)
            </label>
          </div>
        )}

        {step === 3 && (
          <div className="space-y-4">
            <div className="text-slate-200">4. Advanced</div>
            <label className="block text-xs text-slate-400">
              Advertise address (optional)
              <input
                className="mt-1 w-full rounded border border-slate-800 bg-slate-950 px-2 py-1.5 font-mono text-sm text-slate-100"
                value={String(form.advertise_addr ?? '')}
                onChange={(e) => setField('advertise_addr', e.target.value)}
                placeholder="Public IP or hostname for Contact/SDP"
              />
            </label>
            <label className="flex items-center gap-2 text-xs text-slate-300">
              <input
                type="checkbox"
                checked={Boolean(form.session_timer_enabled)}
                onChange={(e) => setField('session_timer_enabled', e.target.checked)}
              />
              Session timer on INVITE (Min-SE / Session-Expires)
            </label>
          </div>
        )}

        {step === 4 && (
          <div className="space-y-4 text-sm text-slate-300">
            <div className="text-slate-200">5. Review</div>
            <ul className="list-inside list-disc space-y-1 text-xs text-slate-400">
              <li>Listener: {String(form.bind_addr)}:{String(form.udp_port)}</li>
              <li>Outbound: {String(form.outbound_transport)} → {String(form.outbound_proxy_addr || '—')}:{String(form.outbound_proxy_port || '—')}</li>
              <li>TLS: CA {form.tls_root_ca_file ? 'set' : '—'}, mTLS {form.tls_client_cert_file ? 'set' : '—'}, SNI {String(form.tls_server_name || '—')}</li>
              <li>Advertise: {String(form.advertise_addr || '—')}</li>
              <li>Session timer: {form.session_timer_enabled ? 'on' : 'off'}</li>
            </ul>
            <p className="text-xs text-slate-500">
              Saving writes <code className="text-slate-400">spec.sipStack</code> into {hasSaved ? 'your' : 'the'} config file.
              Restart the sipbridge service so the SIP stack reloads.
            </p>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className="rounded-lg bg-sky-700 px-4 py-2 text-sm font-medium text-white hover:bg-sky-600"
                onClick={() => save()}
              >
                Save to config
              </button>
              <button
                type="button"
                className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-300 hover:bg-slate-900"
                onClick={() => clearSaved()}
              >
                Remove saved overrides
              </button>
            </div>
          </div>
        )}
      </div>

      <div className="flex items-center justify-between">
        <button
          type="button"
          disabled={step === 0}
          className="flex items-center gap-1 rounded-lg border border-slate-800 px-3 py-2 text-sm text-slate-300 disabled:opacity-40"
          onClick={() => setStep((s) => Math.max(0, s - 1))}
        >
          <ChevronLeft size={16} /> Back
        </button>
        <button
          type="button"
          disabled={step >= steps.length - 1 || !canNext}
          className="flex items-center gap-1 rounded-lg border border-slate-800 px-3 py-2 text-sm text-slate-300 disabled:opacity-40"
          onClick={() => setStep((s) => Math.min(steps.length - 1, s + 1))}
        >
          Next <ChevronRight size={16} />
        </button>
      </div>
    </div>
  )
}
