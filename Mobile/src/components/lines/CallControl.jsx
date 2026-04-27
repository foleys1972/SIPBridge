import React, { useRef } from 'react';
import { Button } from "@/components/ui/button";
import { PhoneOff, BellRing, BellOff, Phone, VolumeX } from 'lucide-react';
import { toast } from "sonner";
import { useTrader } from '@/lib/TraderContext';
import { base44 } from '@/api/base44Client';

export default function CallControls({ line, onUpdateStatus }) {
  const { status, line_type } = line;
  const { displayName, isIdentified, trader } = useTrader();
  const callStartRef = useRef(null);

  const handleAnswer = () => {
    if (!isIdentified) {
      toast.warning('Set your Trader ID before connecting — required for call recording compliance.');
      return;
    }
    callStartRef.current = new Date().toISOString();
    onUpdateStatus(line.id, 'active');
    toast.success(`${line.name} — Connected`, {
      description: `Attributed to: ${displayName}`,
    });
  };

  const handleEndCall = async () => {
    const endedAt = new Date().toISOString();
    const startedAt = callStartRef.current;
    const durationSeconds = startedAt
      ? Math.round((new Date(endedAt) - new Date(startedAt)) / 1000)
      : null;

    onUpdateStatus(line.id, 'idle');
    toast.success(`${line.name} — Call ended`);

    // Record call history
    base44.entities.CallHistory.create({
      line_id: line.id,
      line_name: line.name,
      counterparty: line.counterparty || '',
      line_type: line.line_type,
      trader_id: trader?.trader_id || '',
      trader_name: trader?.full_name || '',
      desk: trader?.desk || '',
      started_at: startedAt,
      ended_at: endedAt,
      duration_seconds: durationSeconds,
      outcome: 'connected',
    });

    callStartRef.current = null;
  };

  const handleRing = () => {
    onUpdateStatus(line.id, 'ringing');
    toast.success(`${line.name} — Ringing far end`);
  };

  const handleDND = () => {
    const newStatus = status === 'dnd' ? 'idle' : 'dnd';
    onUpdateStatus(line.id, newStatus);
    toast.success(`${line.name} — ${newStatus === 'dnd' ? 'Do Not Disturb enabled' : 'DND disabled'}`);
  };

  const handleMute = () => {
    toast.success(`${line.name} — Muted`);
  };

  return (
    <div className="flex items-center gap-1.5">
      {/* Answer / Connect */}
      {(status === 'idle' || status === 'ringing') && (
        <Button
          size="sm"
          onClick={handleAnswer}
          className="h-8 w-8 p-0 rounded-full bg-green-600 hover:bg-green-500 text-white shadow-lg shadow-green-500/20"
        >
          <Phone className="w-3.5 h-3.5" />
        </Button>
      )}

      {/* End Call */}
      {status === 'active' && (
        <>
          <Button
            size="sm"
            onClick={handleMute}
            variant="outline"
            className="h-8 w-8 p-0 rounded-full border-border"
          >
            <VolumeX className="w-3.5 h-3.5" />
          </Button>
          <Button
            size="sm"
            onClick={handleEndCall}
            className="h-8 w-8 p-0 rounded-full bg-red-600 hover:bg-red-500 text-white shadow-lg shadow-red-500/20"
          >
            <PhoneOff className="w-3.5 h-3.5" />
          </Button>
        </>
      )}

      {/* Ring Far End — MRD only */}
      {line_type === 'MRD' && status !== 'active' && status !== 'dnd' && (
        <Button
          size="sm"
          onClick={handleRing}
          variant="outline"
          className="h-8 w-8 p-0 rounded-full border-amber-500/30 text-amber-400 hover:bg-amber-500/10"
        >
          <BellRing className="w-3.5 h-3.5" />
        </Button>
      )}

      {/* DND Toggle */}
      {status !== 'active' && (
        <Button
          size="sm"
          onClick={handleDND}
          variant="outline"
          className={`h-8 w-8 p-0 rounded-full ${
            status === 'dnd'
              ? 'bg-red-500/20 border-red-500/30 text-red-400'
              : 'border-border text-muted-foreground hover:text-foreground'
          }`}
        >
          <BellOff className="w-3.5 h-3.5" />
        </Button>
      )}
    </div>
  );
}