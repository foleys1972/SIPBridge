import React from 'react';
import { useAuth } from '@/lib/AuthContext';
import { ShieldOff } from 'lucide-react';

export default function AdminGuard({ children }) {
  const { user, isLoadingAuth } = useAuth();

  if (isLoadingAuth) {
    return (
      <div className="flex-1 flex items-center justify-center min-h-[60vh]">
        <div className="w-6 h-6 border-4 border-slate-200 border-t-slate-800 rounded-full animate-spin" />
      </div>
    );
  }

  if (!user || user.role !== 'admin') {
    return (
      <div className="flex-1 flex flex-col items-center justify-center min-h-[60vh] gap-4 px-6 text-center">
        <div className="w-14 h-14 rounded-2xl bg-destructive/10 flex items-center justify-center">
          <ShieldOff className="w-7 h-7 text-destructive" />
        </div>
        <div>
          <h2 className="text-base font-semibold text-foreground mb-1">Access Denied</h2>
          <p className="text-sm text-muted-foreground">You need admin privileges to access this area.</p>
        </div>
      </div>
    );
  }

  return children;
}