import React, { useEffect, useRef } from 'react';
import { motion } from 'framer-motion';
import StatusIndicator from './StatusIndicator';
import LineTypeBadge from './LineTypeBadge';
import CallControls from './CallControls';
import { Clock, Building2, UserCheck, AlertTriangle } from 'lucide-react';
import { useTrader } from '@/lib/TraderContext';

export default function LineCard({ line, onUpdateStatus }) {
  const { name, line_type, counterparty, desk, status, extension, codec } = line;
  const { displayName, isIdentified, trader } = useTrader();

  // Snapshot the trader identity at the moment the call goes active
  const stampedTrader = useRef(null);
  useEffect(() => {
    if (status === 'active' && !stampedTrader.current) {
      stampedTrader.current = { displayName, desk: trader?.desk || '' };
    }
    if (status !== 'active') {
      stampedTrader.current = null;
    }
  }, [status]);

  const borderColor = {
    active: 'border-green-500/30',
    ringing: 'border-amber-500/30',
    dnd: 'border-red-500/30',
    error: 'border-red-500/30',
    idle: 'border-border',
    disconnected: 'border-border',
  }[status] || 'border-border';

  const glowClass = status === 'active' ? 'pulse-active' : status === 'ringing' ? 'pulse-ringing' : '';

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -10 }}
      className={`rounded-xl border ${borderColor} bg-card p-3.5 transition-all ${glowClass}`}
    >
      {/* Top Row */}
      <div className="flex items-start justify-between mb-2.5">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <h3 className="text-sm font-semibold text-foreground truncate">{name}</h3>
            <LineTypeBadge type={line_type} />
          </div>
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1 text-muted-foreground">
              <Building2 className="w-3 h-3" />
              <span className="text-xs">{counterparty}</span>
            </div>
            {desk && (
              <span className="text-xs text-muted-foreground">• {desk}</span>
            )}
          </div>
        </div>
        <StatusIndicator status={status} />
      </div>

      {/* Info Row */}
      <div className="flex items-center gap-3 mb-3 text-[10px] font-mono text-muted-foreground">
        {extension && <span>EXT: {extension}</span>}
        {codec && <span>{codec}</span>}
        {status === 'active' && (
          <span className="flex items-center gap-1 text-green-400">
            <Clock className="w-3 h-3" />
            Connected
          </span>
        )}
      </div>

      {/* Trader stamp — visible only when active */}
      {status === 'active' && (
        <div className={`flex items-center gap-2 rounded-lg px-3 py-2 mb-3 border ${
          stampedTrader.current
            ? 'bg-primary/10 border-primary/20'
            : 'bg-amber-500/10 border-amber-500/20'
        }`}>
          {stampedTrader.current ? (
            <>
              <UserCheck className="w-3.5 h-3.5 text-primary flex-shrink-0" />
              <div className="flex flex-col">
                <span className="text-[11px] font-semibold text-primary leading-none">{stampedTrader.current.displayName}</span>
                {stampedTrader.current.desk && (
                  <span className="text-[10px] text-primary/60 font-mono leading-none mt-0.5">{stampedTrader.current.desk}</span>
                )}
              </div>
              <span className="ml-auto text-[9px] font-mono text-primary/50 uppercase tracking-widest">Stamped</span>
            </>
          ) : (
            <>
              <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
              <span className="text-[11px] text-amber-300 font-semibold">No Trader ID — call unattributed</span>
            </>
          )}
        </div>
      )}

      {/* Controls */}
      <div className="flex items-center justify-between">
        <div className="flex-1" />
        <CallControls line={line} onUpdateStatus={onUpdateStatus} />
      </div>
    </motion.div>
  );
}