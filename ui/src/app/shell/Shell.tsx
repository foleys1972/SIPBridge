import React from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { Activity, Cable, LayoutDashboard, LineChart, Network, Server, Settings, Share2 } from 'lucide-react'
import { useAuth, type Role } from '../auth/AuthContext'

function navLinkClass({ isActive }: { isActive: boolean }) {
  return [
    'flex items-center gap-2 rounded-md px-3 py-2 text-sm transition',
    isActive ? 'bg-slate-800 text-slate-50' : 'text-slate-300 hover:bg-slate-900',
  ].join(' ')
}

function RoleSelect() {
  const { role, setRole } = useAuth()
  return (
    <div className="flex items-center gap-2">
      <div className="text-xs text-slate-400">Role</div>
      <select
        className="rounded-md border border-slate-800 bg-slate-950 px-2 py-1 text-sm text-slate-100"
        value={role}
        onChange={(e) => setRole(e.target.value as Role)}
      >
        <option value="readonly">readonly</option>
        <option value="operator">operator</option>
        <option value="admin">admin</option>
      </select>
    </div>
  )
}

export default function Shell({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const settingsArea = location.pathname.startsWith('/settings')

  return (
    <div className="min-h-screen">
      <div className="mx-auto grid max-w-7xl grid-cols-12 gap-6 px-6 py-6">
        <aside className="col-span-12 rounded-xl border border-slate-800 bg-slate-950 p-4 lg:col-span-3">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-semibold tracking-wide text-slate-100">SIPBridge</div>
              <div className="text-xs text-slate-400">Operations Console</div>
            </div>
            <RoleSelect />
          </div>

          <nav className="mt-4 flex flex-col gap-1">
            <NavLink to="/" className={navLinkClass}>
              <LayoutDashboard size={16} /> Overview
            </NavLink>
            <NavLink to="/bridges" className={navLinkClass}>
              <Cable size={16} /> Bridges & Lines
            </NavLink>
            <NavLink to="/mi" className={navLinkClass}>
              <Activity size={16} /> MI Dashboard
            </NavLink>
            <NavLink to="/usage" className={navLinkClass}>
              <LineChart size={16} /> Realtime Usage
            </NavLink>
            <NavLink to="/setup/sbc" className={navLinkClass}>
              <Network size={16} /> SBC &amp; SIP setup
            </NavLink>
            <NavLink to="/servers" className={navLinkClass}>
              <Server size={16} /> Servers
            </NavLink>
            <NavLink to="/cluster" className={navLinkClass}>
              <Share2 size={16} /> Cluster
            </NavLink>
            <NavLink
              to="/settings/config"
              className={({ isActive }) => navLinkClass({ isActive: isActive || settingsArea })}
            >
              <Settings size={16} /> Settings
            </NavLink>
          </nav>

          <div className="mt-6 rounded-lg border border-slate-800 bg-slate-950 p-3 text-xs text-slate-400">
            API base: <span className="font-mono">{import.meta.env.VITE_API_BASE ?? '/api'}</span>
          </div>
        </aside>

        <main className="col-span-12 lg:col-span-9">
          <div className="rounded-xl border border-slate-800 bg-slate-950 p-6">{children}</div>
        </main>
      </div>
    </div>
  )
}
