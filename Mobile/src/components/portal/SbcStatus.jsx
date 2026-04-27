import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { base44 } from '@/api/base44Client';
import { Wifi, WifiOff, RefreshCw, Globe } from 'lucide-react';
import { Button } from '@/components/ui/button';

function IpStatusRow({ user }) {
  const hasIp = !!user.ip_address;

  return (
    <tr className="bg-card hover:bg-secondary/30 transition-colors border-b border-border">
      <td className="px-4 py-3">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-full bg-secondary flex items-center justify-center flex-shrink-0 text-xs font-semibold text-secondary-foreground">
            {(user.full_name || user.email || '?')[0].toUpperCase()}
          </div>
          <div className="min-w-0">
            <div className="font-medium text-foreground truncate">{user.full_name || '—'}</div>
            <div className="text-xs text-muted-foreground truncate">{user.email}</div>
          </div>
        </div>
      </td>
      <td className="px-4 py-3 hidden sm:table-cell">
        <span className="text-xs text-muted-foreground">{user.trader_id || '—'}</span>
      </td>
      <td className="px-4 py-3 hidden md:table-cell">
        <span className="text-xs text-muted-foreground">{user.desk || '—'}</span>
      </td>
      <td className="px-4 py-3">
        {hasIp ? (
          <div className="flex items-center gap-1.5">
            <Wifi className="w-3.5 h-3.5 text-green-400 flex-shrink-0" />
            <span className="text-xs font-mono text-green-400">{user.ip_address}</span>
          </div>
        ) : (
          <div className="flex items-center gap-1.5">
            <WifiOff className="w-3.5 h-3.5 text-muted-foreground/50 flex-shrink-0" />
            <span className="text-xs text-muted-foreground/50">Not registered</span>
          </div>
        )}
      </td>
      <td className="px-4 py-3">
        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium ${
          hasIp
            ? 'bg-green-500/10 text-green-400 border border-green-500/20'
            : 'bg-muted text-muted-foreground border border-border'
        }`}>
          {hasIp ? 'Routable' : 'Unregistered'}
        </span>
      </td>
    </tr>
  );
}

export default function SbcStatus() {
  const { data: users = [], isLoading, refetch, isFetching } = useQuery({
    queryKey: ['sbc-status-users'],
    queryFn: () => base44.entities.User.list(),
  });

  const routable = users.filter(u => !!u.ip_address);
  const unregistered = users.filter(u => !u.ip_address);

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      {/* Summary */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold text-foreground">SBC Registration Status</h2>
          <p className="text-xs text-muted-foreground">IP addresses registered for call routing</p>
        </div>
        <Button size="sm" variant="outline" onClick={() => refetch()} className="gap-1.5 text-xs">
          <RefreshCw className={`w-3.5 h-3.5 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3">
        <StatCard label="Total Users" value={users.length} color="text-foreground" />
        <StatCard label="Routable" value={routable.length} color="text-green-400" />
        <StatCard label="Unregistered" value={unregistered.length} color="text-amber-400" />
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
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider">Registered IP</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
              </tr>
            </thead>
            <tbody>
              {users.map(u => <IpStatusRow key={u.id} user={u} />)}
              {users.length === 0 && (
                <tr><td colSpan={5} className="text-center py-12 text-sm text-muted-foreground">No users found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function StatCard({ label, value, color }) {
  return (
    <div className="rounded-xl bg-card border border-border px-4 py-3 flex flex-col gap-1">
      <span className={`text-2xl font-bold ${color}`}>{value}</span>
      <span className="text-[11px] text-muted-foreground uppercase tracking-wider">{label}</span>
    </div>
  );
}