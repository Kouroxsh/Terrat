import React, { useState } from 'react';
import { Copy, Check, ArrowDown } from 'lucide-react';

const facts = [
  { value: '117', label: 'fixes shipped in v3.0.1' },
  { value: 'No CGO', label: 'pure Go build' },
  { value: '5', label: 'built-in color themes' },
  { value: '10', label: 'packages per release · amd64 + arm64' },
];

export const HeroTerminal: React.FC = () => {
  const [copied, setCopied] = useState(false);

  const handleCopyInstall = () => {
    navigator.clipboard.writeText('go install github.com/Kouroxsh/Terrat/cmd/terrat@latest');
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <section className="pt-16 sm:pt-24 pb-16 sm:pb-20 hairline-b bg-grid-pattern overflow-hidden">
      <div className="max-w-7xl mx-auto px-4 sm:px-6">
        {/* Tag badge */}
        <div className="flex justify-center mb-6">
          <span className="inline-flex items-center gap-2 font-mono text-xs px-3 py-1.5 rounded-full border border-tokyo-border bg-tokyo-surface text-[#a9b1d6]">
            <span className="w-2 h-2 rounded-full bg-tokyo-green" aria-hidden="true" />
            v3.0.4 · Latest stable release
          </span>
        </div>

        {/* Headline */}
        <h1 className="text-display font-semibold tracking-tight text-white text-center max-w-4xl mx-auto">
          A Linux terminal in <span className="text-tokyo-accent">pure Go</span>.
        </h1>
        <p className="text-lede text-[#a9b1d6] text-center max-w-2xl mx-auto mt-6">
          Terrat is a desktop terminal for Linux with tabs, in-buffer search, command
          diagnostics and typo suggestions. It builds without CGO and runs on X11 or XWayland.
        </p>

        {/* Dual CTAs */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-3 mt-8">
          <a
            href="https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4"
            className="anim w-full sm:w-auto inline-flex items-center justify-center gap-2 px-6 py-3 rounded-sm bg-tokyo-accent hover:bg-[#89b4fa] text-tokyo-header font-mono text-sm font-semibold hover:-translate-y-0.5"
          >
            <ArrowDown className="w-4 h-4" aria-hidden="true" />
            Download v3.0.4
          </a>
          <a
            href="https://github.com/Kouroxsh/Terrat"
            className="anim w-full sm:w-auto inline-flex items-center justify-center gap-2 px-6 py-3 rounded-sm border border-tokyo-border text-tokyo-fg font-mono text-sm hover:bg-tokyo-subtle hover:-translate-y-0.5"
          >
            View source
          </a>
        </div>

        {/* Install command */}
        <div className="max-w-2xl mx-auto mt-6">
          <div className="flex items-center gap-3 p-3 sm:p-4 bg-tokyo-surface border border-tokyo-border rounded-sm font-mono text-xs sm:text-sm">
            <span className="text-tokyo-green shrink-0" aria-hidden="true">$</span>
            <div className="overflow-x-auto min-w-0 flex-1">
              <code className="text-tokyo-fg select-all whitespace-nowrap">go install github.com/Kouroxsh/Terrat/cmd/terrat@latest</code>
            </div>
            <button
              onClick={handleCopyInstall}
              aria-label="Copy go install command"
              className="anim shrink-0 flex items-center gap-1.5 px-2.5 py-1.5 rounded-sm border border-tokyo-border text-[#a9b1d6] hover:text-white hover:bg-tokyo-subtle"
            >
              {copied ? <Check className="w-4 h-4 text-tokyo-green" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />}
              <span className="hidden sm:inline">{copied ? 'Copied' : 'Copy'}</span>
            </button>
          </div>
          <p className="text-center font-mono text-xs text-[#a9b1d6] mt-3">
            v3.0.4 packages: .deb · .rpm · .pkg.tar.zst · .tar.gz — see Releases
          </p>
        </div>

        {/* Product mockup frame */}
        <div className="max-w-5xl mx-auto mt-12 sm:mt-16">
          <div className="border border-tokyo-border bg-tokyo-bg shadow-2xl rounded-sm overflow-hidden">
            <div className="h-10 bg-tokyo-header border-b border-tokyo-border flex items-center justify-between px-4 font-mono text-xs">
              <div className="flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full bg-tokyo-red" aria-hidden="true" />
                <span className="w-2.5 h-2.5 rounded-full bg-tokyo-yellow" aria-hidden="true" />
                <span className="w-2.5 h-2.5 rounded-full bg-tokyo-green" aria-hidden="true" />
                <span className="ml-2 text-tokyo-fg">
                  <span className="text-tokyo-accent">1:</span> bash
                </span>
              </div>
              <span className="text-[#a9b1d6]">Screenshot</span>
            </div>
            <img
              src="https://raw.githubusercontent.com/Kouroxsh/Terrat/main/assets/screenshot.png"
              alt="Terrat screenshot — real terminal window showing fastfetch system information"
              className="w-full block"
            />
          </div>
          <p className="text-center font-mono text-xs text-[#a9b1d6] mt-3">
            Real screenshot from the repository — assets/screenshot.png
          </p>
        </div>

        {/* Honest facts strip */}
        <div className="max-w-5xl mx-auto mt-12 sm:mt-16">
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-px bg-tokyo-border border border-tokyo-border rounded-sm overflow-hidden">
            {facts.map((fact) => (
              <div key={fact.label} className="bg-tokyo-surface px-5 py-6 text-center">
                <p className="font-mono text-2xl sm:text-3xl font-semibold text-white">{fact.value}</p>
                <p className="font-mono text-xs text-[#a9b1d6] mt-2 leading-relaxed">{fact.label}</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
};
