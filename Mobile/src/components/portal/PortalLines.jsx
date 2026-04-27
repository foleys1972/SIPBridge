import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { base44 } from '@/api/base44Client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Plus, Pencil, Trash2, Radio, Search } from 'lucide-react';
import { toast } from 'sonner';
import LineForm from '@/components/lines/LineForm';
import LineTypeBadge from '@/components/lines/LineTypeBadge';

const STATUS_COLORS = {
  active: 'bg-green-500/10 text-green-400 border-green-500/20',
  ringing: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  dnd: 'bg-red-500/10 text-red-400 border-red-500/20',
  idle: 'bg-secondary text-muted-foreground border-border',
  error: 'bg-red-500/10 text-red-400 border-red-500/20',
  disconnected: 'bg-secondary text-muted-foreground border-border',
};

export default function PortalLines() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [formOpen, setFormOpen] = useState(false);
  const [editLine, setEditLine] = useState(null);
  const [deleteId, setDeleteId] = useState(null);

  const { data: lines = [], isLoading } = useQuery({
    queryKey: ['portal-lines'],
    queryFn: () => base44.entities.Line.list('-priority', 200),
  });

  const createMutation = useMutation({
    mutationFn: (data) => base44.entities.Line.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['portal-lines'] });
      queryClient.invalidateQueries({ queryKey: ['lines'] });
      setFormOpen(false);
      setEditLine(null);
      toast.success('Line added');
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }) => base44.entities.Line.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['portal-lines'] });
      queryClient.invalidateQueries({ queryKey: ['lines'] });
      setFormOpen(false);
      setEditLine(null);
      toast.success('Line updated');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id) => base44.entities.Line.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['portal-lines'] });
      queryClient.invalidateQueries({ queryKey: ['lines'] });
      setDeleteId(null);
      toast.success('Line removed');
    },
  });

  const handleSave = (form) => {
    if (editLine) {
      updateMutation.mutate({ id: editLine.id, data: form });
    } else {
      createMutation.mutate({ ...form, status: 'idle' });
    }
  };

  const filtered = lines.filter(l => {
    const q = search.toLowerCase();
    return !q || l.name.toLowerCase().includes(q) || l.counterparty.toLowerCase().includes(q) || (l.desk || '').toLowerCase().includes(q) || (l.sbc_address || '').toLowerCase().includes(q);
  });

  return (
    <div className="p-6 max-w-5xl mx-auto">
      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 mb-6">
        <div>
          <h2 className="text-base font-semibold text-foreground">Lines</h2>
          <p className="text-xs text-muted-foreground">{lines.length} configured line{lines.length !== 1 ? 's' : ''}</p>
        </div>
        <div className="flex items-center gap-2 w-full sm:w-auto">
          <div className="relative flex-1 sm:w-64">
            <Search className="absolute left-2.5 top-2.5 w-3.5 h-3.5 text-muted-foreground" />
            <Input
              placeholder="Search lines…"
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="pl-8 h-9 bg-secondary border-border text-sm"
            />
          </div>
          <Button size="sm" onClick={() => { setEditLine(null); setFormOpen(true); }} className="gap-1.5 flex-shrink-0">
            <Plus className="w-3.5 h-3.5" />
            Add Line
          </Button>
        </div>
      </div>

      {/* Table */}
      {isLoading ? (
        <div className="flex justify-center py-20">
          <div className="w-6 h-6 border-4 border-primary/20 border-t-primary rounded-full animate-spin" />
        </div>
      ) : lines.length === 0 ? (
        <div className="text-center py-20 space-y-3">
          <Radio className="w-12 h-12 text-muted-foreground/20 mx-auto" />
          <p className="text-sm text-muted-foreground">No lines configured</p>
          <Button variant="outline" size="sm" className="gap-1.5" onClick={() => { setEditLine(null); setFormOpen(true); }}>
            <Plus className="w-4 h-4" /> Add first line
          </Button>
        </div>
      ) : (
        <div className="rounded-xl border border-border overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-secondary/50">
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider">Line</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider hidden sm:table-cell">Counterparty</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider hidden md:table-cell">SBC</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider hidden lg:table-cell">Codec / Transport</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
                <th className="px-4 py-3 w-20" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {filtered.map(line => (
                <tr key={line.id} className="bg-card hover:bg-secondary/30 transition-colors">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-foreground truncate">{line.name}</span>
                          <LineTypeBadge type={line.line_type} />
                        </div>
                        {line.desk && <div className="text-xs text-muted-foreground">{line.desk}</div>}
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 hidden sm:table-cell">
                    <span className="text-xs text-muted-foreground">{line.counterparty}</span>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <span className="text-xs font-mono text-muted-foreground">{line.sbc_address}{line.sbc_port ? `:${line.sbc_port}` : ''}</span>
                    {line.extension && <div className="text-[10px] font-mono text-muted-foreground/60">EXT {line.extension}</div>}
                  </td>
                  <td className="px-4 py-3 hidden lg:table-cell">
                    <div className="text-xs text-muted-foreground">{line.codec} / {line.transport}</div>
                  </td>
                  <td className="px-4 py-3">
                    <Badge variant="outline" className={`text-[10px] capitalize ${STATUS_COLORS[line.status] || STATUS_COLORS.idle}`}>
                      {line.status || 'idle'}
                    </Badge>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1 justify-end">
                      <Button size="icon" variant="ghost" className="w-7 h-7 text-muted-foreground hover:text-foreground" onClick={() => { setEditLine(line); setFormOpen(true); }}>
                        <Pencil className="w-3.5 h-3.5" />
                      </Button>
                      <Button size="icon" variant="ghost" className="w-7 h-7 text-muted-foreground hover:text-destructive" onClick={() => setDeleteId(line.id)}>
                        <Trash2 className="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr><td colSpan={6} className="text-center py-12 text-sm text-muted-foreground">No lines match your search</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <LineForm
        open={formOpen}
        onClose={setFormOpen}
        onSave={handleSave}
        editLine={editLine}
        isSaving={createMutation.isPending || updateMutation.isPending}
      />

      <AlertDialog open={!!deleteId} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent className="bg-card border-border">
          <AlertDialogHeader>
            <AlertDialogTitle>Remove Line</AlertDialogTitle>
            <AlertDialogDescription>Permanently remove this line configuration? This cannot be undone.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => deleteMutation.mutate(deleteId)} className="bg-destructive text-destructive-foreground">Remove</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}