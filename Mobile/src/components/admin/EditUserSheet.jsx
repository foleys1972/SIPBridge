import React, { useState, useEffect } from 'react';
import { base44 } from '@/api/base44Client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from '@/components/ui/sheet';
import { toast } from 'sonner';

export default function EditUserSheet({ user, open, onClose, onSuccess }) {
  const [form, setForm] = useState({ trader_id: '', desk: '', role: 'user' });
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (user) {
      setForm({
        trader_id: user.trader_id || '',
        desk: user.desk || '',
        role: user.role || 'user',
      });
    }
  }, [user]);

  const handleSave = async () => {
    setLoading(true);
    await base44.entities.User.update(user.id, form);
    toast.success(`${user.full_name || user.email} updated`);
    setLoading(false);
    onSuccess?.();
    onClose();
  };

  return (
    <Sheet open={open} onOpenChange={onClose}>
      <SheetContent side="right" className="w-full sm:max-w-sm">
        <SheetHeader>
          <SheetTitle>Edit User</SheetTitle>
          <p className="text-xs text-muted-foreground">{user?.email}</p>
        </SheetHeader>
        <div className="space-y-4 py-6">
          <div className="space-y-1.5">
            <Label>Trader ID</Label>
            <Input
              placeholder="e.g. JD-01"
              value={form.trader_id}
              onChange={(e) => setForm((f) => ({ ...f, trader_id: e.target.value }))}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Desk</Label>
            <Input
              placeholder="e.g. FX Options"
              value={form.desk}
              onChange={(e) => setForm((f) => ({ ...f, desk: e.target.value }))}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Role</Label>
            <Select value={form.role} onValueChange={(v) => setForm((f) => ({ ...f, role: v }))}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="user">User (Trader)</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <SheetFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>Cancel</Button>
          <Button onClick={handleSave} disabled={loading}>
            {loading ? 'Saving…' : 'Save Changes'}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}