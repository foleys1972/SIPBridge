import { useCallback, useEffect, useState } from 'react'
import yaml from 'js-yaml'
import { apiFetch, apiPostJson, apiPutText } from '../api/client'
import type { CaptureSpec, ConfigStatus, RootConfig } from '../api/types'

function cloneConfig(c: RootConfig): RootConfig {
  if (typeof structuredClone === 'function') return structuredClone(c) as RootConfig
  return JSON.parse(JSON.stringify(c)) as RootConfig
}

function normalizeCapture(c?: CaptureSpec): CaptureSpec {
  return {
    enabled: Boolean(c?.enabled),
    directory: c?.directory ?? '',
    capture_bridges: c?.capture_bridges ?? true,
    capture_conferences: c?.capture_conferences ?? true,
  }
}

export default function CaptureSettingsPage() {
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')
  const [cfgStatus, setCfgStatus] = useState<ConfigStatus | null>(null)
  const [cfg, setCfg] = useState<RootConfig | null>(null)
  const [capture, setCapture] = useState<CaptureSpec>(normalizeCapture())
  const [testStatus, setTestStatus] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setErr('')
    const [c, st] = await Promise.all([
      apiFetch<RootConfig>('/v1/config'),
      apiFetch<ConfigStatus>('/v1/config/status').catch(() => null),
    ])
    setCfg(c)
    setCapture(normalizeCapture(c.spec.capture))
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
    setStatus('Saving...')
    setErr('')
    try {
      if (cfgStatus?.config_read_only) throw new Error('Config is read-only (CONFIG_HTTP_URL).')
      if (!cfg) throw new Error('Config not loaded')
      const draft = cloneConfig(cfg)
      draft.spec.capture = {
        enabled: Boolean(capture.enabled),
        directory: (capture.directory ?? '').trim(),
        capture_bridges: Boolean(capture.capture_bridges),
        capture_conferences: Boolean(capture.capture_conferences),
      }
      const bodyText = yaml.dump(draft, { noRefs: true, lineWidth: 120 })
      await apiPutText('/v1/config', bodyText)
      setStatus('Saved.')
      await load()
    } catch (e) {
      setStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  async function testWrite() {
    setErr('')
    setTestStatus('Testing...')
    try {
      const res = await apiPostJson<{ ok: boolean; path?: string; error?: string }>('/v1/settings/capture/test-write', {
        directory: capture.directory ?? '',
      })
      if (!res.ok) throw new Error(res.error ?? 'Write test failed')
      setTestStatus(`OK: write access confirmed (${res.path ?? ''})`)
    } catch (e) {
      setTestStatus('')
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  if (loading) return <div className="text-sm text-slate-400">Loading...</div>

  return (
    <div>
      <div className="text-sm font-semibold text-slate-200">Local Capture Archive</div>
      <p className="mt-1 text-sm text-slate-400">
        Configure where local WAV + metadata JSON captures are stored. SIPBridge automatically creates a folder per day under the
        archive root (for example <span className="font-mono">D:/SIPBridgeArchive/2026-04-27</span>).
      </p>

      {cfgStatus?.config_read_only ? (
        <div className="mt-4 rounded-lg border border-amber-800 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">
          Read-only config (CONFIG_HTTP_URL). Changes cannot be applied from this UI.
        </div>
      ) : null}

      <div className="mt-6 space-y-4 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <label className="flex items-center gap-2 text-sm text-slate-300">
          <input
            type="checkbox"
            checked={Boolean(capture.enabled)}
            onChange={(e) => setCapture((c) => ({ ...c, enabled: e.target.checked }))}
            disabled={Boolean(cfgStatus?.config_read_only)}
          />
          Enable local capture (WAV + metadata JSON)
        </label>

        <label className="flex flex-col gap-1 text-xs text-slate-400">
          Archive root folder (drive/path)
          <input
            className="rounded-md border border-slate-800 bg-slate-950 px-2 py-2 font-mono text-sm text-slate-100"
            value={capture.directory ?? ''}
            onChange={(e) => setCapture((c) => ({ ...c, directory: e.target.value }))}
            placeholder="D:/SIPBridgeArchive"
            disabled={Boolean(cfgStatus?.config_read_only)}
          />
        </label>

        <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
          <label className="flex items-center gap-2 text-sm text-slate-300">
            <input
              type="checkbox"
              checked={Boolean(capture.capture_bridges)}
              onChange={(e) => setCapture((c) => ({ ...c, capture_bridges: e.target.checked }))}
              disabled={Boolean(cfgStatus?.config_read_only)}
            />
            Capture bridge calls
          </label>
          <label className="flex items-center gap-2 text-sm text-slate-300">
            <input
              type="checkbox"
              checked={Boolean(capture.capture_conferences)}
              onChange={(e) => setCapture((c) => ({ ...c, capture_conferences: e.target.checked }))}
              disabled={Boolean(cfgStatus?.config_read_only)}
            />
            Capture conference/hoot calls
          </label>
        </div>
      </div>

      <div className="mt-6 flex flex-wrap items-center gap-3">
        <button
          type="button"
          className="rounded-md border border-emerald-800 bg-emerald-950/40 px-4 py-2 text-sm text-emerald-100 hover:bg-emerald-900/40 disabled:opacity-50"
          disabled={!capture.directory}
          onClick={() => testWrite()}
        >
          Test write access
        </button>
        <button
          type="button"
          className="rounded-md bg-sky-600 px-4 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50"
          disabled={Boolean(cfgStatus?.config_read_only)}
          onClick={() => save()}
        >
          Save capture settings
        </button>
        {status ? <span className="text-sm text-slate-400">{status}</span> : null}
        {testStatus ? <span className="text-sm text-emerald-300">{testStatus}</span> : null}
      </div>

      {err ? <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div> : null}
    </div>
  )
}
