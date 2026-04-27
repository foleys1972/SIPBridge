import React, { useState } from 'react';
import { useTrader } from '@/lib/TraderContext';
import { User, ChevronDown, Check, AlertTriangle } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';

export default function TraderIdentity() {
  const { trader, updateTrader, displayName, isIdentified } = useTrader();
  const [traderId, setTraderId] = useState('');
  const [desk, setDesk] = useState('');
  const [open, setOpen] = useState(false);

  const handleOpen = (val) => {
    if (val) {
      setTraderId(trader?.trader_id || '');
      setDesk(trader?.desk || '');
    }
    setOpen(val);
  };

  const handleSave = async () => {
    await updateTrader({ trader_id: traderId, desk });
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={handleOpen}>
      <PopoverTrigger asChild>
        <button
          className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border transition-colors max-w-[150px] ${
            isIdentified
              ? 'bg-secondary border-border hover:border-primary/30'
              : 'bg-amber-500/10 border-amber-500/30 hover:bg-amber-500/20'
          }`}
        >
          {isIdentified
            ? <User className="w-3.5 h-3.5 text-primary flex-shrink-0" />
            : <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
          }
          <div className="flex flex-col items-start overflow-hidden">
            <span className={`text-[11px] font-semibold truncate w-full ${isIdentified ? 'text-foreground' : 'text-amber-400'}`}>
              {isIdentified ? displayName : 'Set Trader ID'}
            </span>
            {trader?.desk && (
              <span className="text-[9px] text-muted-foreground truncate w-full">{trader.desk}</span>
            )}
          </div>
          <ChevronDown className="w-3 h-3 text-muted-foreground flex-shrink-0" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-68 p-4" align="end">
        <div className="space-y-3">
          <div>
            <p className="text-xs font-semibold text-foreground mb-0.5">Trader Identity</p>
            <p className="text-[11px] text-muted-foreground">Stamped on all calls &amp; SBC metadata</p>
          </div>
          <div className="space-y-2">
            <div>
              <Label className="text-[11px] text-muted-foreground">Trader ID / Name</Label>
              <Input
                value={traderId}
                onChange={(e) => setTraderId(e.target.value)}
                placeholder="e.g. jsmith or John Smith"
                className="h-8 text-xs mt-1"
              />
            </div>
            <div>
              <Label className="text-[11px] text-muted-foreground">Desk</Label>
              <Input
                value={desk}
                onChange={(e) => setDesk(e.target.value)}
                placeholder="e.g. FX Sales"
                className="h-8 text-xs mt-1"
              />
            </div>
          </div>
          {trader?.email && (
            <p className="text-[10px] text-muted-foreground font-mono border-t border-border pt-2">{trader.email}</p>
          )}
          <Button size="sm" className="w-full h-8 text-xs" onClick={handleSave} disabled={!traderId.trim()}>
            <Check className="w-3.5 h-3.5 mr-1" /> Save Identity
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}