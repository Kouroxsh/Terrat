import React from 'react';
import { Monitor, Terminal, Layers } from 'lucide-react';

const measurements = [
  { label: 'Plain terminal output', detail: '128 lines', value: '95.3 µs', allocation: '0 B allocated' },
  { label: 'ANSI-colored output', detail: '128 lines', value: '119.4 µs', allocation: '2 KB allocated' },
  { label: 'Frame render', detail: '1024 × 640 canvas', value: '973 µs', allocation: '32 B allocated' },
  { label: 'Frame render with search', detail: '1024 × 640 canvas', value: '2.06 ms', allocation: '32 B allocated' },
  { label: 'Font engine startup', detail: 'Create and close', value: '1.96 ms', allocation: '677 KB allocated' },
];

const platforms = [
  {
    label: 'Linux',
    title: 'X11 and XWayland',
    detail: 'Uses XGB to communicate with the X server. Wayland desktops need XWayland enabled.',
    icon: Monitor,
  },
  {
    label: 'Windows',
    title: 'Windows 10 and 11',
    detail: 'Uses native Win32 windowing and ConPTY for shell sessions (Windows 10 build 1809 or newer).',
    icon: Terminal,
  },
  {
    label: 'Build',
    title: 'Go, without CGO',
    detail: 'The application and platform code are Go packages. Release builds produce a standalone executable.',
    icon: Layers,
  },
];

export const Benchmarks: React.FC = () => (
  <section id="platforms" className="py-20 hairline-b bg-[#13141c]">
    <div className="max-w-7xl mx-auto px-6">
      <div className="max-w-3xl mb-10">
        <div className="font-mono text-xs text-[#7aa2f7] uppercase tracking-wider mb-2">BENCHMARKS</div>
        <h2 className="text-3xl sm:text-5xl font-bold tracking-tight text-white mb-4">Measured on one Linux system.</h2>
        <p className="text-sm sm:text-base text-[#a9b1d6] leading-relaxed">
          These Go microbenchmarks show the cost of selected terminal and rendering operations. They are useful for tracking changes, but do not represent whole-app startup, memory use, or Windows performance.
        </p>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 border border-[#292e42] divide-y sm:divide-y-0 sm:divide-x divide-[#292e42] bg-[#16161e]">
        {measurements.map(({ label, detail, value, allocation }) => (
          <article key={label} className="p-6 sm:p-7">
            <p className="text-sm font-semibold text-white">{label}</p>
            <p className="mt-1 text-xs text-[#565f89]">{detail}</p>
            <p className="mt-6 font-mono text-2xl text-[#7aa2f7]">{value}</p>
            <p className="mt-1 text-xs text-[#a9b1d6]">{allocation} per operation</p>
          </article>
        ))}
      </div>
      <p className="mt-4 text-xs text-[#565f89] leading-relaxed">
        Median of five runs · Intel Core i5-8500 @ 3.00 GHz · Linux/amd64 · Go 1.22.6. Rendering measures the CPU canvas only; it excludes native window presentation and shell startup. Reproduce with <code className="text-[#a9b1d6]">go test ./internal/terminal ./internal/render ./internal/autosuggest -run '^$' -bench . -benchmem -benchtime=500ms -count=5</code>.
      </p>

      <div className="max-w-2xl mt-16 mb-8">
        <div className="font-mono text-xs text-[#7aa2f7] uppercase tracking-wider mb-2">PLATFORM SUPPORT</div>
        <h3 className="text-2xl sm:text-3xl font-bold tracking-tight text-white mb-3">Runs on Linux and Windows.</h3>
        <p className="text-sm text-[#a9b1d6] leading-relaxed">
          Linux builds need an X11 server or XWayland. Windows builds use Win32 and ConPTY.
        </p>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 border border-[#292e42] divide-y md:divide-y-0 md:divide-x divide-[#292e42] bg-[#16161e]">
        {platforms.map(({ label, title, detail, icon: Icon }) => (
          <article key={label} className="p-6 sm:p-7">
            <div className="flex items-center justify-between mb-8">
              <span className="font-mono text-xs text-[#7aa2f7] uppercase tracking-wider">{label}</span>
              <Icon aria-hidden="true" className="w-5 h-5 text-[#565f89]" />
            </div>
            <h3 className="text-lg font-semibold text-white mb-2">{title}</h3>
            <p className="text-sm text-[#a9b1d6] leading-relaxed">{detail}</p>
          </article>
        ))}
      </div>
    </div>
  </section>
);
