import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch } from '../api/client'
import type { MIAttendanceResponse } from '../api/types'
import { employeeIdFromAttendance } from '../lib/employeeId'
import Badge from '../components/Badge'

export default function MIDashboardPage() {
  const [rows, setRows] = useState<MIAttendanceResponse['attendance']>([])
  const [err, setErr] = useState('')

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const res = await apiFetch<MIAttendanceResponse>('/v1/mi/attendance')
        if (!cancelled) setRows(res.attendance ?? [])
      } catch (e) {
        if (!cancelled) setErr(e instanceof Error ? e.message : String(e))
      }
    }
    load()
    const t = window.setInterval(load, 5000)
    return () => {
      cancelled = true
      window.clearInterval(t)
    }
  }, [])

  return (
    <div>
      <div className="flex items-start justify-between gap-6">
        <div>
          <div className="text-xl font-semibold">MI Dashboard</div>
          <div className="mt-1 text-sm text-slate-400">
            Management information: active bridge attendance includes dial-in identity when callers authenticate with PIN.
          </div>
        </div>
        <Badge tone="blue">live</Badge>
      </div>

      <div className="mt-6 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="flex items-center justify-between gap-3">
          <div className="text-sm font-semibold">Bridge attendance</div>
          <Link className="text-xs text-sky-400 hover:text-sky-300" to="/bridges">
            Bridges
          </Link>
        </div>
        {err ? (
          <div className="mt-3 text-sm text-rose-300">{err}</div>
        ) : (
          <div className="mt-3 overflow-x-auto">
            <table className="w-full min-w-[640px] text-left text-sm">
              <thead className="border-b border-slate-800 text-xs text-slate-400">
                <tr>
                  <th className="py-2 pr-3">Bridge</th>
                  <th className="py-2 pr-3">Employee ID</th>
                  <th className="py-2 pr-3">Display name</th>
                  <th className="py-2 pr-3">PIN</th>
                  <th className="py-2 pr-3">Remote</th>
                  <th className="py-2">Since</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={`${r.bridge_id}-${r.call_id}`} className="border-t border-slate-800">
                    <td className="py-2 pr-3 font-mono text-xs text-slate-200">{r.bridge_id}</td>
                    <td className="py-2 pr-3 font-mono text-xs text-slate-300">
                      {employeeIdFromAttendance(r) || '—'}
                    </td>
                    <td className="py-2 pr-3 text-slate-300">{r.display_name || '—'}</td>
                    <td className="py-2 pr-3 font-mono text-xs tracking-widest text-slate-400">{r.pin_masked || '—'}</td>
                    <td className="py-2 pr-3 font-mono text-xs text-slate-500">{r.remote_addr}</td>
                    <td className="py-2 text-xs text-slate-500">
                      {r.created_at ? new Date(r.created_at).toLocaleTimeString() : '—'}
                    </td>
                  </tr>
                ))}
                {rows.length === 0 ? (
                  <tr>
                    <td className="py-4 text-slate-500" colSpan={6}>
                      No active bridge calls.
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="mt-6 grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-950 p-4">
          <div className="text-sm font-semibold">KPIs</div>
          <div className="mt-2 text-sm text-slate-400">
            Aggregated counters (attempts, answer ratios) can be added next to complement live attendance.
          </div>
        </div>
        <div className="rounded-xl border border-slate-800 bg-slate-950 p-4">
          <div className="text-sm font-semibold">Quality</div>
          <div className="mt-2 text-sm text-slate-400">
            Placeholder for RTP stats, jitter, and packet loss tied to bridge legs.
          </div>
        </div>
      </div>
    </div>
  )
}
