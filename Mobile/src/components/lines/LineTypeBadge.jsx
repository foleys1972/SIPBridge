import React from 'react';

const typeConfig = {
  ARD: { bg: 'bg-blue-500/15', text: 'text-blue-400', border: 'border-blue-500/30', label: 'ARD' },
  MRD: { bg: 'bg-amber-500/15', text: 'text-amber-400', border: 'border-amber-500/30', label: 'MRD' },
  HOOT: { bg: 'bg-green-500/15', text: 'text-green-400', border: 'border-green-500/30', label: 'HOOT' },
};

export default function LineTypeBadge({ type }) {
  const config = typeConfig[type] || typeConfig.ARD;

  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-mono font-semibold tracking-wider border ${config.bg} ${config.text} ${config.border}`}>
      {config.label}
    </span>
  );
}