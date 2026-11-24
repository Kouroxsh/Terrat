import React from 'react';
import { Monitor, Layers } from 'lucide-react';
import { SectionHeader } from './SectionHeader';

const measurements = [
  { label: 'Plain terminal output', detail: '128 lines', value: '150.5 µs', allocation: '0 B allocated' },
  { label: 'ANSI-colored output', detail: '128 lines', value: '183.2 µs', allocation: '2 KB allocated' },
  { label: 'Frame render', detail: '1024 × 640 canvas', value: '989.9 µs', allocation: '32 B allocated' },
  { label: 'Frame render with search', detail: '1024 × 640 canvas', value: '2.21 ms', allocation: '32 B allocated' },
  { label: 'Font engine startup', detail: 'Create and close', value: '1.53 ms', allocation: '703 KB allocated' },
];

const platforms = [
  {
    label: 'Linux',
    title: 'X11 and XWayland',
    detail: 'Uses XGB to communicate with the X server. Wayland desktops need XWayland enabled.',
    icon: Monitor,
  },
  {
    label: 'Build',
    title: 'Go, without CGO',
    detail: 'The application and platform code are Go packages. Release builds produce a standalone executable.',
    icon: Layers,
  },
];

export const Benchmarks: React.FC = () => (
  <section id="platforms" className="py-20 sm:py-24 hairline-b bg-tokyo-header">
    <div className="max-w-7xl mx-auto px-4 sm:px-6">
      <SectionHeader
        eyebrow="Benchmarks"
        title="Measured on one Linux system."
        lede="These Go microbenchmarks show the cost of selected terminal and rendering operations. They are useful for tracking changes, but do not represent whole-app startup or memory use."
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-px bg-tokyo-border border border-tokyo-border rounded-sm overflow-hidden">
        {measurements.map(({ label, detail, value, allocation }) => (
          <article key={label} className="p-6 sm:p-7 bg-tokyo-surface">
            <p className="text-sm font-semibold text-white">{label}</p>
            <p className="mt-1 text-xs text-[#a9b1d6]">{detail}</p>
            <p className="mt-6 font-mono text-2xl text-tokyo-accent">{value}</p>
            <p className="mt-1 text-xs text-[#a9b1d6]">{allocation} per operation</p>
          </article>
        ))}
      </div>
      <p className="mt-4 text-xs text-[#a9b1d6] leading-relaxed max-w-4xl">
        Median of five runs · Nov 2026 · AMD EPYC 9D25 (cloud VM) · Linux/amd64 · Go 1.23.0. Rendering measures the CPU canvas only; it excludes native window presentation and shell startup. Reproduce with <code className="text-tokyo-fg">go test ./internal/terminal ./internal/render ./internal/autosuggest -run '^$' -bench . -benchmem -benchtime=500ms -count=5</code>.
      </p>

      <div className="mt-16 sm:mt-20 mb-8 max-w-3xl">
        <div className="font-mono text-xs text-tokyo-accent uppercase tracking-wider mb-3">Platform support</div>
        <h3 className="text-h2 font-bold tracking-tight text-white">Runs on Linux.</h3>
        <p className="text-lede text-[#a9b1d6] mt-3">Linux builds need an X11 server or XWayland.</p>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-px bg-tokyo-border border border-tokyo-border rounded-sm overflow-hidden">
        {platforms.map(({ label, title, detail, icon: Icon }) => (
          <article key={label} className="anim p-6 sm:p-7 bg-tokyo-surface hover:bg-tokyo-bg">
            <div className="flex items-center justify-between mb-8">
              <span className="font-mono text-xs text-tokyo-accent uppercase tracking-wider">{label}</span>
              <Icon aria-hidden="true" className="w-5 h-5 text-[#a9b1d6]" />
            </div>
            <h3 className="text-lg font-semibold text-white mb-2">{title}</h3>
            <p className="text-sm text-[#a9b1d6] leading-relaxed">{detail}</p>
          </article>
        ))}
      </div>
    </div>
  </section>
);
