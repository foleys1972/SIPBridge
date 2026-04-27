import React from 'react';
import { Outlet, Link, useLocation } from 'react-router-dom';
import { Radio, Settings, Shield, LayoutGrid, ListPlus, AlertTriangle, Users, History, BarChart2, ExternalLink } from 'lucide-react';
import TraderIdentity from './TraderIdentity';
import IpChangeBanner from './IpChangeBanner';
import { TraderProvider, useTrader } from '@/lib/TraderContext';
import { useAuth } from '@/lib/AuthContext';
import { useIpCheck } from '@/hooks/useIpCheck';

function LayoutInner() {
  const location = useLocation();
  const { isIdentified, loading } = useTrader();
  const { user } = useAuth();
  const isAdmin = user?.role === 'admin';
  const { currentIp, ipChanged, registering, registerIp, dismiss } = useIpCheck(user);

  const navItems = [
    { path: '/', icon: LayoutGrid, label: 'Lines' },
    { path: '/history', icon: History, label: 'History' },
    { path: '/settings', icon: Settings, label: 'Settings' },
    ...(isAdmin ? [
      { path: '/analytics', icon: BarChart2, label: 'Analytics' },
      { path: '/admin', icon: Users, label: 'Admin' },
    ] : []),
  ];

  return (
    <div className="min-h-screen bg-background flex flex-col">
      {/* Header */}
      <header className="sticky top-0 z-50 bg-card/80 backdrop-blur-xl border-b border-border">
        <div className="flex items-center justify-between px-4 py-3">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-primary/20 flex items-center justify-center">
              <Radio className="w-4 h-4 text-primary" />
            </div>
            <div>
              <h1 className="text-sm font-semibold tracking-tight text-foreground">VoiceWire</h1>
              <p className="text-[10px] text-muted-foreground font-mono tracking-wider uppercase">Private Wire Console</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <TraderIdentity />
            <Link
              to="/manage"
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-primary/10 border border-primary/20 text-primary hover:bg-primary/20 transition-colors"
            >
              <ListPlus className="w-3.5 h-3.5" />
              <span className="text-[11px] font-semibold">Manage Lines</span>
            </Link>
            {isAdmin && (
              <Link
                to="/portal"
                className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-secondary border border-border hover:border-primary/30 transition-colors"
              >
                <ExternalLink className="w-3.5 h-3.5 text-muted-foreground" />
                <span className="text-[11px] font-semibold text-muted-foreground">Portal</span>
              </Link>
            )}
            <Link
              to="/certificates"
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-secondary border border-border hover:border-primary/30 transition-colors"
            >
              <Shield className="w-3.5 h-3.5 text-muted-foreground" />
              <span className="text-[11px] font-semibold text-muted-foreground">Certs</span>
            </Link>
            <div className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-green-500/10 border border-green-500/20">
              <div className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />
              <span className="text-[10px] font-mono text-green-400">SBC</span>
            </div>
          </div>
        </div>
      </header>

      {/* IP change banner */}
      {ipChanged && (
        <IpChangeBanner
          currentIp={currentIp}
          savedIp={user?.ip_address}
          registering={registering}
          onRegister={registerIp}
          onDismiss={dismiss}
        />
      )}

      {/* Trader identity warning banner */}
      {!loading && !isIdentified && (
        <div className="bg-amber-500/10 border-b border-amber-500/20 px-4 py-2 flex items-center gap-2">
          <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
          <p className="text-xs text-amber-300">
            <span className="font-semibold">No Trader ID set.</span> Calls cannot be attributed for recording compliance. Tap your name in the header to set your identity.
          </p>
        </div>
      )}

      {/* Main Content */}
      <main className="flex-1 overflow-auto pb-24">
        <Outlet />
      </main>

      {/* Bottom Navigation */}
      <nav className="fixed bottom-0 left-0 right-0 z-50 bg-card/90 backdrop-blur-xl border-t border-border">
        <div className="flex items-center justify-around px-2 py-1 pb-[env(safe-area-inset-bottom,8px)]">
          {navItems.map(({ path, icon: Icon, label }) => {
            const isActive = location.pathname === path;
            return (
              <Link
                key={path}
                to={path}
                className={`flex flex-col items-center gap-0.5 px-4 py-2 rounded-xl transition-all ${
                  isActive
                    ? 'text-primary'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                <Icon className={`w-5 h-5 ${isActive ? 'stroke-[2.5]' : ''}`} />
                <span className="text-[10px] font-medium">{label}</span>
              </Link>
            );
          })}
        </div>
      </nav>
    </div>
  );
}

export default function AppLayout() {
  return (
    <TraderProvider>
      <LayoutInner />
    </TraderProvider>
  );
}