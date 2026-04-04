import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch, apiPostJson } from '../api/client'
import type { UserSummary, UsersListResponse } from '../api/types'

export default function UserListPage() {
  const [users, setUsers] = useState<UserSummary[]>([])
  const [err, setErr] = useState('')
  const [creating, setCreating] = useState(false)
  const [newId, setNewId] = useState('')
  const [newName, setNewName] = useState('')
  const [newPin, setNewPin] = useState('')

  async function load() {
    const res = await apiFetch<UsersListResponse>('/v1/users')
    setUsers(res.users ?? [])
  }

  useEffect(() => {
    load().catch((e) => setErr(e instanceof Error ? e.message : String(e)))
  }, [])

  async function addUser() {
    setErr('')
    const id = newId.trim()
    const pin = newPin.replace(/\D+/g, '')
    if (!id || !pin) {
      setErr('Employee ID and numeric PIN are required.')
      return
    }
    setCreating(true)
    try {
      await apiPostJson('/v1/users', {
        id,
        display_name: newName.trim() || id,
        region: '',
        allowed_bridge_ids: [],
        allowed_conference_group_ids: [],
        participant_id: pin,
      })
      setNewId('')
      setNewName('')
      setNewPin('')
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setCreating(false)
    }
  }

  return (
    <div>
      <div className="text-sm font-semibold text-slate-200">Dial-in users</div>
      <p className="mt-1 text-sm text-slate-400">
        Each person is identified by their <span className="text-slate-300">employee ID</span> (your bank’s HR / directory
        id). It must be entered when creating a user—nothing is auto-generated. Dial-in users authenticate with the bridge or
        conference access number, then PIN. Raw PINs are never shown after save.
      </p>

      <div className="mt-6 overflow-hidden rounded-lg border border-slate-800">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-xs text-slate-400">
            <tr>
              <th className="px-3 py-2">Employee ID</th>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">PIN set</th>
              <th className="px-3 py-2">Rec</th>
              <th className="px-3 py-2">Devices</th>
              <th className="px-3 py-2"></th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id} className="border-t border-slate-800">
                <td className="px-3 py-2 font-mono text-xs text-slate-200">{u.employee_id ?? u.id}</td>
                <td className="px-3 py-2 text-slate-200">{u.display_name ?? '—'}</td>
                <td className="px-3 py-2 text-slate-300">{u.pin_set ? 'yes' : 'no'}</td>
                <td className="px-3 py-2 text-right">
                  <Link
                    className="text-sky-400 underline hover:text-sky-300"
                    to={`/settings/users/${encodeURIComponent(u.id)}`}
                  >
                    Edit
                  </Link>
                </td>
              </tr>
            ))}
            {users.length === 0 ? (
              <tr>
                <td className="px-3 py-4 text-slate-500" colSpan={6}>
                  No users yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <div className="mt-8 rounded-xl border border-slate-800 bg-slate-950 p-4">
        <div className="text-sm font-semibold text-slate-200">Add user</div>
        <p className="mt-2 text-xs text-slate-500">
          Enter the official employee ID from HR (e.g. staff number). This becomes the permanent user key in config and appears
          on MI and bridge views when they dial in.
        </p>
        <div className="mt-3 flex flex-wrap items-end gap-3">
          <label className="flex flex-col gap-1 text-xs text-slate-400">
            Employee ID
            <input
              className="min-w-[10rem] rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-sm text-slate-100"
              value={newId}
              onChange={(e) => setNewId(e.target.value)}
              placeholder="e.g. 8844221"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-slate-400">
            Display name
            <input
              className="w-48 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-sm text-slate-100"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Ops desk"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-slate-400">
            Initial PIN (digits)
            <input
              className="w-36 rounded-md border border-slate-800 bg-slate-950 px-2 py-1 font-mono text-sm text-slate-100"
              value={newPin}
              onChange={(e) => setNewPin(e.target.value.replace(/\D+/g, ''))}
              placeholder="6 digits"
              inputMode="numeric"
            />
          </label>
          <button
            type="button"
            disabled={creating}
            className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500 disabled:opacity-50"
            onClick={() => addUser()}
          >
            Create
          </button>
        </div>
      </div>

      {err ? (
        <div className="mt-4 rounded-lg border border-rose-900/60 bg-rose-950 p-3 text-sm text-rose-200">{err}</div>
      ) : null}
    </div>
  )
}
