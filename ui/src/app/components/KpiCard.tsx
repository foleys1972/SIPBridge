import React from 'react'

export default function KpiCard({
  title,
  value,
  hint,
}: {
  title: string
  value: React.ReactNode
  hint?: React.ReactNode
}) {
  return (
    <div className="rounded-xl border border-slate-800 bg-slate-950 p-4">
      <div className="text-xs font-medium uppercase tracking-wide text-slate-400">
        {title}
      </div>
      <div className="mt-2 text-2xl font-semibold text-slate-50">{value}</div>
      {hint ? <div className="mt-1 text-xs text-slate-400">{hint}</div> : null}
    </div>
  )
}
