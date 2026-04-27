import React from 'react';
import { AlertTriangle, Wifi, X } from 'lucide-react';
import { Button } from '@/components/ui/button';

export default function IpChangeBanner({ currentIp, savedIp, registering, onRegister, onDismiss }) {
  return (
    <div className="bg-amber-500/10 border-b border-amber-500/20 px-4 py-2 flex items-center gap-2">
      <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
      <p className="text-xs text-amber-300 flex-1">
        <span className="font-semibold">IP address changed.</span>
        {savedIp
          ? ` Previous: ${savedIp} → Current: ${currentIp}.`
          : ` Current IP ${currentIp} not registered.`}{' '}
        Update to ensure SBC call routing works.
      </p>
      <Button
        size="sm"
        variant="outline"
        disabled={registering}
        onClick={onRegister}
        className="gap-1 text-xs h-7 border-amber-500/30 text-amber-300 hover:bg-amber-500/10 flex-shrink-0"
      >
        <Wifi className="w-3 h-3" />
        {registering ? 'Updating…' : 'Update IP'}
      </Button>
      <button onClick={onDismiss} className="text-amber-400/60 hover:text-amber-400 flex-shrink-0">
        <X className="w-3.5 h-3.5" />
      </button>
    </div>
  );
}