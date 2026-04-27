import React, { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { base44 } from '@/api/base44Client';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from '@/components/ui/sheet';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { UserPlus, Pencil, Trash2, ShieldCheck, User, IdCard, Search, Wifi, WifiOff } from 'lucide-react';
import { toast } from 'sonner';
import { useAuth } from '@/lib/AuthContext';
import InviteUserModal from '@/components/admin/InviteUserModal';

function EditSheet({ user, open, onClose, onSuccess }) {
  const [form, setForm] = useState({ trader_id: '', desk: '', role: 'user' });
  const [loading, setLoading] = useState(false);

  React.useEffect(() => {
    if (user) setForm({ trader_id: user.trader_id || '', desk: user.desk || '', role: user.role || 'user' });
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
          <p className="text-xs text-muted-foreground">{user?.full_name} — {user?.email}</p>
        </SheetHeader>
        <div className="space-y-4 py-6">
          <div className="space-y-1.5">
            <Label>Trader ID</Label>
            <Input placeholder="e.g. JD-01" value={form.trader_id} onChange={e => setForm(f => ({ ...f, trader_id: e.target.value }))} />
          </div>
          <div className="space-y-1.5">
            <Label>Desk</Label>
            <Input placeholder="e.g. FX Options" value={form.desk} onChange={e => setForm(f => ({ ...f, desk: e.target.value }))} />
          </div>
          <div className="space-y-1.5">
            <Label>Role</Label>
            <Select value={form.role} onValueChange={v => setForm(f => ({ ...f, role: v }))}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="user">User (Trader)</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <SheetFooter>
          <Button variant="outline" onClick={onClose} disabled={loading}>Cancel</Button>
          <Button onClick={handleSave} disabled={loading}>{loading ? 'Saving…' : 'Save Changes'}</Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

export default function PortalUsers() {
  const { user: currentUser } = useAuth();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [inviteOpen, setInviteOpen] = useState(false);
  const [editUser, setEditUser] = useState(null);
  const [deleteUser, setDeleteUser] = useState(null);

  const { data: users = [], isLoading } = useQuery({
    queryKey: ['portal-users'],
    queryFn: () => base44.entities.User.list(),
  });

  const deleteMutation = useMutation({
    mutationFn: (id) => base44.entities.User.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['portal-users'] });
      toast.success('User removed');
      setDeleteUser(null);
    },
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['portal-users'] });

  const filtered = users.filter(u => {
    const q = search.toLowerCase();
    return !q || (u.full_name || '').toLowerCase().includes(q) || u.email.toLowerCase().includes(q) || (u.desk || '').toLowerCase().includes(q) || (u.trader_id || '').toLowerCase().includes(q);
  });

  return (
    <div className="p-6 max-w-4xl mx-auto">
      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 mb-6">
        <div>
          <h2 className="text-base font-semibold text-foreground">Users</h2>
          <p className="text-xs text-muted-foreground">{users.length} registered user{users.length !== 1 ? 's' : ''}</p>
        </div>
        <div className="flex items-center gap-2 w-full sm:w-auto">
          <div className="relative flex-1 sm:w-60">
            <Search className="absolute left-2.5 top-2.5 w-3.5 h-3.5 text-muted-foreground" />
            <Input
              placeholder="Search users…"
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="pl-8 h-9 bg-secondary border-border text-sm"
            />
          </div>
          <Button size="sm" onClick={() => setInviteOpen(true)} className="gap-1.5 flex-shrink-0">
            <UserPlus className="w-3.5 h-3.5" />
            Invite
          </Button>
        </div>
      </div>

      {/* Table */}
      {isLoading ? (
        <div className="flex justify-center py-20">
          <div className="w-6 h-6 border-4 border-primary/20 border-t-primary rounded-full animate-spin" />
        </div>
      ) : (
        <div className="rounded-xl border border-border overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-secondary/50">
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider">User</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider hidden sm:table-cell">Trader ID</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider hidden md:table-cell">Desk</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider hidden lg:table-cell">IP Address</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider">Role</th>
                <th className="px-4 py-3 w-20" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {filtered.map(u => (
                <tr key={u.id} className="bg-card hover:bg-secondary/30 transition-colors">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2.5">
                      <div className="w-8 h-8 rounded-full bg-secondary flex items-center justify-center flex-shrink-0 text-xs font-semibold text-secondary-foreground">
                        {(u.full_name || u.email || '?')[0].toUpperCase()}
                      </div>
                      <div className="min-w-0">
                        <div className="font-medium text-foreground truncate">{u.full_name || '—'}</div>
                        <div className="text-xs text-muted-foreground truncate">{u.email}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 hidden sm:table-cell">
                    {u.trader_id ? (
                      <span className="flex items-center gap-1 text-xs font-mono text-primary/80">
                        <IdCard className="w-3 h-3" />{u.trader_id}
                      </span>
                    ) : <span className="text-xs text-muted-foreground/40">—</span>}
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <span className="text-xs text-muted-foreground">{u.desk || '—'}</span>
                  </td>
                  <td className="px-4 py-3 hidden lg:table-cell">
                    {u.ip_address ? (
                      <span className="flex items-center gap-1 text-xs font-mono text-green-400">
                        <Wifi className="w-3 h-3" />{u.ip_address}
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-xs text-muted-foreground/40">
                        <WifiOff className="w-3 h-3" />Not set
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <Badge variant="outline" className={u.role === 'admin' ? 'border-primary/40 text-primary text-[10px] gap-1' : 'border-border text-muted-foreground text-[10px] gap-1'}>
                      {u.role === 'admin' ? <ShieldCheck className="w-2.5 h-2.5" /> : <User className="w-2.5 h-2.5" />}
                      {u.role}
                    </Badge>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1 justify-end">
                      <Button size="icon" variant="ghost" className="w-7 h-7 text-muted-foreground hover:text-foreground" onClick={() => setEditUser(u)}>
                        <Pencil className="w-3.5 h-3.5" />
                      </Button>
                      <Button size="icon" variant="ghost" className="w-7 h-7 text-muted-foreground hover:text-destructive" disabled={u.id === currentUser?.id} onClick={() => setDeleteUser(u)}>
                        <Trash2 className="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr><td colSpan={6} className="text-center py-12 text-sm text-muted-foreground">No users found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <InviteUserModal open={inviteOpen} onClose={() => setInviteOpen(false)} onSuccess={refresh} />
      <EditSheet user={editUser} open={!!editUser} onClose={() => setEditUser(null)} onSuccess={refresh} />

      <AlertDialog open={!!deleteUser} onOpenChange={() => setDeleteUser(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove User</AlertDialogTitle>
            <AlertDialogDescription>Remove <strong>{deleteUser?.full_name || deleteUser?.email}</strong> from the system? This cannot be undone.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction className="bg-destructive hover:bg-destructive/90" onClick={() => deleteMutation.mutate(deleteUser.id)}>Remove</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}