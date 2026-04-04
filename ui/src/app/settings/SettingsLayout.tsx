import { NavLink, Outlet } from 'react-router-dom'

function subNavClass({ isActive }: { isActive: boolean }) {
  return [
    'rounded-md px-3 py-2 text-sm transition',
    isActive ? 'bg-slate-800 text-slate-50' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200',
  ].join(' ')
}

const links = [
  { to: '/settings/users', label: 'Users' },
  { to: '/settings/config', label: 'Configuration' },
  { to: '/settings/conference', label: 'Conference groups' },
  { to: '/settings/recording', label: 'Recording' },
  { to: '/settings/database', label: 'Database' },
]

export default function SettingsLayout() {
  return (
    <div>
      <div className="text-xl font-semibold">Settings</div>
      <div className="mt-1 text-sm text-slate-400">
        Manage dial-in users, YAML configuration, conference groups, and database / config storage options.
      </div>

      <nav className="mt-6 flex flex-wrap gap-2 border-b border-slate-800 pb-3">
        {links.map((l) => (
          <NavLink key={l.to} to={l.to} className={subNavClass}>
            {l.label}
          </NavLink>
        ))}
      </nav>

      <div className="mt-6">
        <Outlet />
      </div>
    </div>
  )
}
