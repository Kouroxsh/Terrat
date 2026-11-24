import React, { useState } from 'react';
import { X } from 'lucide-react';

/** Sticky conversion bar — mobile only, dismissible, persistent CTA. */
export const MobileCtaBar: React.FC = () => {
  const [dismissed, setDismissed] = useState(false);

  if (dismissed) return null;

  return (
    <div className="md:hidden fixed bottom-0 inset-x-0 z-50 bg-tokyo-header/95 backdrop-blur-md border-t border-tokyo-border">
      <div className="flex items-center gap-3 px-4 py-3">
        <img src="./icon.png" alt="Terrat logo" className="w-7 h-7 rounded-full border border-tokyo-border shrink-0" />
        <div className="min-w-0 flex-1">
          <p className="font-mono text-xs text-white font-semibold truncate">Terrat v3.0.4</p>
          <p className="font-mono text-xs text-[#a9b1d6] truncate">A Linux terminal in pure Go</p>
        </div>
        <a
          href="https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4"
          className="anim shrink-0 px-4 py-2 rounded-sm bg-tokyo-accent text-tokyo-header font-mono text-xs font-semibold"
        >
          Download
        </a>
        <button
          type="button"
          onClick={() => setDismissed(true)}
          aria-label="Dismiss download bar"
          className="anim shrink-0 flex items-center justify-center w-8 h-8 rounded-sm text-[#a9b1d6] hover:text-white hover:bg-tokyo-subtle"
        >
          <X className="w-4 h-4" aria-hidden="true" />
        </button>
      </div>
    </div>
  );
};
