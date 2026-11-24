import React from 'react';
import { Network, Cpu, Layers, Terminal } from 'lucide-react';
import { SectionHeader } from './SectionHeader';

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
    <section id="architecture" className="py-20 sm:py-24 hairline-b bg-tokyo-header">
      <div className="max-w-7xl mx-auto px-4 sm:px-6">
        <SectionHeader
          eyebrow="Implementation"
          title="How it works"
          lede="Terrat uses X11 or XWayland to draw its window. It builds without CGO; each tab has its own shell session and terminal state."
        />

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          {pillars.map((item) => {
            const Icon = item.icon;
            return (
              <article
                key={item.title}
                className="anim p-6 bg-tokyo-surface border border-tokyo-border rounded-sm hover:border-[#3b4261] hover:-translate-y-0.5"
              >
                <div className="mb-5">
                  <div className="w-fit p-2.5 bg-tokyo-bg border border-tokyo-border rounded-sm">
                    <Icon className="w-5 h-5 text-tokyo-accent" aria-hidden="true" />
                  </div>
                </div>
                <h3 className="text-lg font-bold text-white mb-2">{item.title}</h3>
                <p className="text-sm text-[#a9b1d6] leading-relaxed">{item.desc}</p>
              </article>
            );
          })}
        </div>
      </div>
    </section>
  );
};
