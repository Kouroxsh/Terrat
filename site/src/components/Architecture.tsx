import React from 'react';
import { Network, Cpu, Layers, Terminal } from 'lucide-react';

export const Architecture: React.FC = () => {
  const pillars = [
    {
      icon: Network,
      title: 'Direct X11 Connection',
      desc: 'Terrat connects to X11 through XGB and works in Wayland sessions that provide XWayland. The project builds without CGO.',
    },
    {
      icon: Cpu,
      title: 'Cached Glyphs & Buffered Drawing',
      desc: 'Glyphs are cached in memory and drawn into an X11 pixmap before the frame is shown.',
    },
    {
      icon: Terminal,
      title: 'Separate Shell Sessions',
      desc: 'Each tab has a separate pseudo-terminal and terminal state. Resizing the window or changing the font updates the PTY dimensions for the running shell.',
    },
    {
      icon: Layers,
      title: 'Screen and Scrollback',
      desc: 'The terminal keeps the visible screen and scrollback in its terminal state. Memory use varies with the number of open tabs and their output.',
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
            Terrat uses X11 or XWayland directly. It builds without CGO and keeps rendering, terminal state, and shell sessions in one desktop application.
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
