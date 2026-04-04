export default function Placeholder({ title }: { title: string }) {
  return (
    <div>
      <div className="text-xl font-semibold">{title}</div>
      <div className="mt-2 text-sm text-slate-400">Coming soon.</div>
    </div>
  )
}
