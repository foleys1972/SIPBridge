import React from 'react';

const statusConfig = {
  idle: { color: 'bg-slate-500', text: 'Idle', textColor: 'text-slate-400' },
  active: { color: 'bg-green-500', text: 'Active', textColor: 'text-green-400', pulse: true },
  ringing: { color: 'bg-amber-500', text: 'Ringing', textColor: 'text-amber-400', pulse: true },
  dnd: { color: 'bg-red-500', text: 'DND', textColor: 'text-red-400' },
  error: { color: 'bg-red-500', text: 'Error', textColor: 'text-red-400' },
  disconnected: { color: 'bg-slate-600', text: 'Offline', textColor: 'text-slate-500' },
};

export default function StatusIndicator({ status, size = 'sm' }) {
  const config = statusConfig[status] || statusConfig.idle;
  const dotSize = size === 'sm' ? 'w-2 h-2' : 'w-3 h-3';

  return (
    <div className="flex items-center gap-1.5">
      <div className={`${dotSize} rounded-full ${config.color} ${config.pulse ? 'animate-pulse' : ''}`} />
      <span className={`text-[10px] font-mono uppercase tracking-wider ${config.textColor}`}>
        {config.text}
      </span>
    </div>
  );
}