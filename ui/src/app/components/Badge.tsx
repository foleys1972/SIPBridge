import React from 'react'

export default function Badge({
  children,
  tone = 'slate',
}: {
  children: React.ReactNode
  tone?: 'green' | 'red' | 'amber' | 'slate' | 'blue'
}) {
  const cls =
    tone === 'green'
      ? 'border-emerald-900/60 bg-emerald-950 text-emerald-200'
      : tone === 'red'
        ? 'border-rose-900/60 bg-rose-950 text-rose-200'
        : tone === 'amber'
          ? 'border-amber-900/60 bg-amber-950 text-amber-200'
          : tone === 'blue'
            ? 'border-sky-900/60 bg-sky-950 text-sky-200'
            : 'border-slate-800 bg-slate-900 text-slate-200'

  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs ${cls}`}>
      {children}
    </span>
  )
}
