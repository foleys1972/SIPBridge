import React, { useState, useEffect } from 'react';
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from "@/components/ui/sheet";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Loader2 } from 'lucide-react';

const defaultLine = {
  name: '', line_type: 'ARD', sbc_address: '', sbc_port: 5061,
  extension: '', counterparty: '', desk: '', codec: 'G.711',
  transport: 'TLS', auto_answer: false, priority: 0, notes: '',
};

export default function LineForm({ open, onClose, onSave, editLine, isSaving }) {
  const [form, setForm] = useState(defaultLine);

  useEffect(() => {
    if (editLine) {
      setForm({ ...defaultLine, ...editLine });
    } else {
      setForm(defaultLine);
    }
  }, [editLine, open]);

  const handleChange = (field, value) => {
    setForm(prev => ({ ...prev, [field]: value }));
  };

  const handleSubmit = () => {
    onSave(form);
  };

  return (
    <Sheet open={open} onOpenChange={onClose}>
      <SheetContent side="bottom" className="h-[90vh] rounded-t-2xl bg-card border-border">
        <SheetHeader className="pb-4">
          <SheetTitle className="text-foreground">{editLine ? 'Edit Line' : 'Add New Line'}</SheetTitle>
        </SheetHeader>

        <ScrollArea className="h-[calc(90vh-10rem)] pr-4">
          <div className="space-y-5 pb-4">
            {/* Line Name */}
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground uppercase tracking-wider">Line Name</Label>
              <Input value={form.name} onChange={e => handleChange('name', e.target.value)} placeholder="e.g. JPM FX Spot" className="bg-secondary border-border" />
            </div>

            {/* Line Type */}
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground uppercase tracking-wider">Line Type</Label>
              <Select value={form.line_type} onValueChange={v => handleChange('line_type', v)}>
                <SelectTrigger className="bg-secondary border-border"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="ARD">ARD — Auto Ring Down</SelectItem>
                  <SelectItem value="MRD">MRD — Manual Ring Down</SelectItem>
                  <SelectItem value="HOOT">HOOT — Broadcast</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Counterparty & Desk */}
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground uppercase tracking-wider">Counterparty</Label>
                <Input value={form.counterparty} onChange={e => handleChange('counterparty', e.target.value)} placeholder="e.g. JP Morgan" className="bg-secondary border-border" />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground uppercase tracking-wider">Desk</Label>
                <Input value={form.desk} onChange={e => handleChange('desk', e.target.value)} placeholder="e.g. FX Trading" className="bg-secondary border-border" />
              </div>
            </div>

            {/* SBC Connection */}
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground uppercase tracking-wider">SBC Address</Label>
              <div className="grid grid-cols-3 gap-2">
                <Input value={form.sbc_address} onChange={e => handleChange('sbc_address', e.target.value)} placeholder="sbc.bank.com" className="col-span-2 bg-secondary border-border" />
                <Input type="number" value={form.sbc_port || ''} onChange={e => handleChange('sbc_port', parseInt(e.target.value) || 0)} placeholder="5061" className="bg-secondary border-border font-mono" />
              </div>
            </div>

            {/* Extension */}
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground uppercase tracking-wider">Extension / Dial String</Label>
              <Input value={form.extension} onChange={e => handleChange('extension', e.target.value)} placeholder="e.g. 4001" className="bg-secondary border-border font-mono" />
            </div>

            {/* Codec & Transport */}
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground uppercase tracking-wider">Codec</Label>
                <Select value={form.codec} onValueChange={v => handleChange('codec', v)}>
                  <SelectTrigger className="bg-secondary border-border"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="G.711">G.711 (PCM)</SelectItem>
                    <SelectItem value="G.722">G.722 (HD)</SelectItem>
                    <SelectItem value="G.729">G.729</SelectItem>
                    <SelectItem value="Opus">Opus</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground uppercase tracking-wider">Transport</Label>
                <Select value={form.transport} onValueChange={v => handleChange('transport', v)}>
                  <SelectTrigger className="bg-secondary border-border"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="TLS">TLS (Secure)</SelectItem>
                    <SelectItem value="TCP">TCP</SelectItem>
                    <SelectItem value="UDP">UDP</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {/* Auto Answer */}
            <div className="flex items-center justify-between py-2 px-3 rounded-lg bg-secondary">
              <div>
                <Label className="text-sm text-foreground">Auto Answer</Label>
                <p className="text-xs text-muted-foreground">Automatically connect incoming calls</p>
              </div>
              <Switch checked={form.auto_answer} onCheckedChange={v => handleChange('auto_answer', v)} />
            </div>

            {/* Priority */}
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground uppercase tracking-wider">Display Priority</Label>
              <Input type="number" value={form.priority || ''} onChange={e => handleChange('priority', parseInt(e.target.value) || 0)} placeholder="0" className="bg-secondary border-border" />
            </div>

            {/* Notes */}
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground uppercase tracking-wider">Notes</Label>
              <Textarea value={form.notes} onChange={e => handleChange('notes', e.target.value)} placeholder="Additional notes..." className="bg-secondary border-border h-20" />
            </div>
          </div>
        </ScrollArea>

        <SheetFooter className="pt-4 gap-2">
          <Button variant="outline" onClick={() => onClose(false)} className="flex-1">Cancel</Button>
          <Button onClick={handleSubmit} disabled={!form.name || !form.counterparty || isSaving} className="flex-1 bg-primary text-primary-foreground">
            {isSaving ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
            {editLine ? 'Update Line' : 'Add Line'}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}