import React from 'react';
import { Network, Cpu, Layers, Terminal } from 'lucide-react';

export const Architecture: React.FC = () => {
  const pillars = [
    {
      icon: Network,
      title: 'Direct X11 Wire & Native ConPTY',
      desc: 'Instead of dynamically linking against libX11.so, libxcb.so, or GTK/Qt frameworks, Terrat implements the X11 wire protocol directly in pure Go via XGB on Linux, and natively connects to the Windows PseudoConsole API (ConPTY) on Windows. It compiles to a self-contained static binary that runs across Linux distributions and Windows 10/11 without CGO.',
    },
    {
      icon: Cpu,
      title: 'In-Memory Glyph Cache & Double Buffering',
      desc: 'Text rendering does not require heavy GPU context initialization. TrueType glyphs are rasterized into byte alpha masks and cached in RAM. Visible cells are blitted into off-screen double buffers (X11 Pixmap / Win32 GDI back-buffer) and transferred atomically, completely eliminating screen tearing and resizing artifacts.',
    },
    {
      icon: Terminal,
      title: 'Isolated PTYs & Leak-Free Lifecycles',
      desc: 'Each tab allocates an independent pseudo-terminal (/dev/ptmx on Linux, ConPTY on Windows) with dedicated I/O pumps bound to Go context.Context cancellation, guaranteeing zero goroutine or memory leaks. Changing window size or zooming fonts transmits SIGWINCH or ConPTY resize events to active child processes instantly.',
    },
    {
      icon: Layers,
      title: 'Predictable Memory Bounds',
      desc: 'The scrollback buffer is structured as a circular ring buffer with pre-allocated cell matrix chunks. Once the terminal initializes, steady-state typing and heavy stdout throughput produce practically zero heap allocations, keeping resident memory at a rock-solid ~11MB without GC pauses.',
    },
  ];

  return (
    <section id="architecture" className="py-20 hairline-b bg-[#13141c]">
      <div className="max-w-7xl mx-auto px-6">
        <div className="max-w-3xl mb-14">
          <div className="font-mono text-xs text-[#7aa2f7] uppercase tracking-wider mb-2">
            ARCHITECTURE &amp; DESIGN
          </div>
          <h2 className="text-3xl sm:text-5xl font-bold tracking-tight text-white mb-4">
            How Terrat works under the hood.
          </h2>
          <p className="text-sm sm:text-base text-[#a9b1d6] leading-relaxed">
            Most modern terminal emulators either bundle a web browser (Electron) or require full GPU shader pipelines with heavy dynamic libraries. Terrat is built for developers who want a fast, simple terminal that opens instantly, uses minimal memory, and has no runtime dependencies.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {pillars.map((item, idx) => {
            const Icon = item.icon;
            return (
              <div
                key={idx}
                className="p-6 bg-[#16161e] border border-[#292e42] rounded-[2px] hover:border-[#3b4261] transition-colors"
              >
                <div className="mb-4">
                  <div className="w-fit p-2 bg-[#1a1b26] border border-[#292e42] rounded-[2px]">
                    <Icon className="w-5 h-5 text-[#7aa2f7]" />
                  </div>
                </div>
                <h3 className="text-lg font-bold text-white mb-2">{item.title}</h3>
                <p className="text-xs text-[#a9b1d6] leading-relaxed">
                  {item.desc}
                </p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
};
