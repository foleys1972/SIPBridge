import React from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { Activity, Cable, LayoutDashboard, LineChart, Network, Server, Settings, Share2 } from 'lucide-react'
import { useAuth } from '../auth/AuthContext'

function navLinkClass({ isActive }: { isActive: boolean }) {
  return [
    'flex items-center gap-3 rounded-2xl px-4 py-3 text-sm font-medium transition duration-150',
    isActive ? 'bg-slate-800 text-slate-50 shadow-sm' : 'text-slate-300 hover:bg-slate-900 hover:text-slate-50',
  ].join(' ')
}

function UserBadge() {
  const { role, user, logout, authEnabled } = useAuth()
  if (!authEnabled) {
    return (
      <div className="flex flex-wrap items-center gap-2 rounded-full border border-slate-800 bg-slate-900 px-3 py-2 text-xs text-slate-300">
        <span className="font-semibold text-slate-200">RBAC</span>
        <span className="font-mono">disabled</span>
      </div>
    )
  }
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-xl border border-slate-800 bg-slate-900 px-3 py-2 text-xs text-slate-300">
      <span className="font-semibold text-slate-200">{user?.username ?? '—'}</span>
      <span className="rounded bg-slate-800 px-2 py-0.5 font-mono">{role}</span>
      <button className="rounded border border-slate-700 px-2 py-0.5 hover:bg-slate-800" onClick={() => logout()}>Sign out</button>
    </div>
  )
}

export default function Shell({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const settingsArea = location.pathname.startsWith('/settings')

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="mx-auto grid max-w-7xl grid-cols-12 gap-6 px-6 py-6">
        <aside className="col-span-12 rounded-[28px] border border-slate-800 bg-slate-900/95 p-5 shadow-xl shadow-slate-950/40 lg:col-span-3">
          <div className="flex flex-col gap-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-300">SIPBridge</div>
                <div className="mt-1 text-sm font-semibold text-slate-100">Operations Console</div>
              </div>
              <UserBadge />
            </div>
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
          <div className="rounded-[28px] border border-slate-800 bg-slate-950/95 p-6 shadow-xl shadow-slate-950/20">
            {children}
          </div>
        </main>
      </div>
    </div>
  )
}
