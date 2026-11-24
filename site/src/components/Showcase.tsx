import React, { useState } from 'react';
import { TextCursor, AlertCircle, Search, Layers } from 'lucide-react';
import { SectionHeader } from './SectionHeader';

interface ShowcaseTab {
  id: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  desc: string;
  shortcut: string;
}

const tabs: ShowcaseTab[] = [
  {
    id: 'suggestions',
    label: 'Suggestions',
    icon: TextCursor,
    title: 'Completions as you type',
    desc: 'A completion hint appears as you type, drawn from Bash or Zsh history. It can also complete filesystem paths, including quoted or escaped paths.',
    shortcut: 'Tab / Right Arrow',
  },
  {
    id: 'diagnostics',
    label: 'Diagnostics',
    icon: AlertCircle,
    title: 'Typos, quotes, and command syntax',
    desc: 'Unknown commands, unclosed quotes and chained operators are flagged near the prompt, with a suggestion you can apply in one keystroke.',
    shortcut: 'Alt + Enter',
  },
  {
    id: 'search',
    label: 'Search',
    icon: Search,
    title: 'Search the scrollback',
    desc: 'Search the terminal buffer and highlight matches across the whole scrollback. Move between results without leaving the keyboard.',
    shortcut: 'Ctrl + Shift + F',
  },
  {
    id: 'tabs',
    label: 'Tabs',
    icon: Layers,
    title: 'Separate shell sessions',
    desc: 'Each tab runs a separate pseudo-terminal session with its own terminal state. Switch instantly, or jump straight to a tab by number.',
    shortcut: 'Ctrl + Shift + T',
  },
];

/** Interactive showcase: tabbed terminal mockups of real Terrat behaviour. */
export const Showcase: React.FC = () => {
  const [activeId, setActiveId] = useState(tabs[0].id);
  const active = tabs.find((t) => t.id === activeId) ?? tabs[0];
  const ActiveIcon = active.icon;

  return (
    <section id="showcase" className="py-20 sm:py-24 hairline-b bg-tokyo-bg">
      <div className="max-w-7xl mx-auto px-4 sm:px-6">
        <SectionHeader
          eyebrow="See it in action"
          title="Built for the daily terminal."
          lede="Four things you will reach for every day — each one working the way a terminal should."
        />

        {/* Tab list */}
        <div role="tablist" aria-label="Product showcase" className="flex flex-wrap gap-2 mb-8">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            const selected = tab.id === activeId;
            return (
              <button
                key={tab.id}
                role="tab"
                aria-selected={selected}
                aria-controls="showcase-panel"
                id={`showcase-tab-${tab.id}`}
                onClick={() => setActiveId(tab.id)}
                className={`anim inline-flex items-center gap-2 px-4 py-2.5 rounded-sm border font-mono text-sm ${
                  selected
                    ? 'bg-tokyo-surface border-tokyo-accent text-white'
                    : 'bg-tokyo-surface border-tokyo-border text-[#a9b1d6] hover:border-[#565f89] hover:text-white'
                }`}
              >
                <Icon className="w-4 h-4 text-tokyo-accent" aria-hidden="true" />
                {tab.label}
              </button>
            );
          })}
        </div>

        {/* Tab panel */}
        <div
          key={active.id}
          id="showcase-panel"
          role="tabpanel"
          aria-labelledby={`showcase-tab-${active.id}`}
          className="fade-in grid grid-cols-1 lg:grid-cols-12 gap-8 items-start"
        >
          <div className="lg:col-span-4">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-fit p-2 bg-tokyo-surface border border-tokyo-border rounded-sm">
                <ActiveIcon className="w-5 h-5 text-tokyo-accent" aria-hidden="true" />
              </div>
              <h3 className="text-xl font-bold text-white">{active.title}</h3>
            </div>
            <p className="text-sm text-[#a9b1d6] leading-relaxed">{active.desc}</p>
            <p className="mt-5 font-mono text-xs text-[#a9b1d6]">
              Shortcut: <span className="text-tokyo-fg">{active.shortcut}</span>
            </p>
          </div>

          <div className="lg:col-span-8">
            <div className="border border-tokyo-border bg-tokyo-surface rounded-sm overflow-hidden shadow-2xl">
              <div className="h-9 bg-tokyo-header border-b border-tokyo-border flex items-center gap-2 px-4 font-mono text-xs">
                <span className="w-2.5 h-2.5 rounded-full bg-tokyo-red" aria-hidden="true" />
                <span className="w-2.5 h-2.5 rounded-full bg-tokyo-yellow" aria-hidden="true" />
                <span className="w-2.5 h-2.5 rounded-full bg-tokyo-green" aria-hidden="true" />
                <span className="ml-2 text-[#a9b1d6]">terrat — {active.label.toLowerCase()}</span>
              </div>
              <div className="p-5 font-mono text-sm leading-relaxed min-h-40">
                {active.id === 'suggestions' && (
                  <>
                    <div>
                      <span className="text-tokyo-accent font-bold">vixland@box</span>
                      <span className="text-[#a9b1d6]">:</span>
                      <span className="text-tokyo-accent">~/terrat</span>
                      <span className="text-[#a9b1d6]">$ </span>
                      <span className="text-white">git stat</span>
                      <span className="text-tokyo-muted">us</span>
                      <span className="inline-block w-2 h-4 bg-tokyo-green align-middle ml-0.5" aria-hidden="true" />
                    </div>
                    <p className="text-[#a9b1d6] text-xs mt-4">Ghost text completes from shell history — press Tab to accept.</p>
                  </>
                )}
                {active.id === 'diagnostics' && (
                  <>
                    <div>
                      <span className="text-tokyo-accent font-bold">vixland@box</span>
                      <span className="text-[#a9b1d6]">:</span>
                      <span className="text-tokyo-accent">~/terrat</span>
                      <span className="text-[#a9b1d6]">$ </span>
                      <span className="text-white">gti status</span>
                    </div>
                    <p className="text-tokyo-yellow text-xs mt-3">gti → git — unknown command, did you mean “git”?</p>
                    <p className="text-[#a9b1d6] text-xs mt-2">Alt+Enter applies the fix.</p>
                  </>
                )}
                {active.id === 'search' && (
                  <>
                    <div className="flex items-center gap-2 text-xs border border-tokyo-border bg-tokyo-header px-3 py-2 rounded-sm">
                      <Search className="w-3.5 h-3.5 text-tokyo-accent" aria-hidden="true" />
                      <span className="text-white">test</span>
                      <span className="ml-auto text-[#a9b1d6]">3 / 12 matches</span>
                    </div>
                    <div className="mt-4 text-xs space-y-1.5">
                      <p className="text-[#a9b1d6]">=== RUN&nbsp;&nbsp;&nbsp;TestTabCreation</p>
                      <p>
                        <span className="text-white">--- PASS: </span>
                        <span className="bg-tokyo-accent/30 text-white px-0.5 rounded-sm">Test</span>
                        <span className="text-white">TabCreation (0.00s)</span>
                      </p>
                      <p className="text-[#a9b1d6]">=== RUN&nbsp;&nbsp;&nbsp;TestSearchBuffer</p>
                    </div>
                  </>
                )}
                {active.id === 'tabs' && (
                  <>
                    <div className="flex gap-1 font-mono text-xs">
                      <span className="px-3 py-1.5 bg-tokyo-header border border-tokyo-border border-b-0 rounded-t-sm text-white">1: bash</span>
                      <span className="px-3 py-1.5 bg-tokyo-surface border border-tokyo-border border-b-0 rounded-t-sm text-[#a9b1d6]">2: htop</span>
                      <span className="px-3 py-1.5 bg-tokyo-surface border border-tokyo-border border-b-0 rounded-t-sm text-[#a9b1d6]">3: vim</span>
                    </div>
                    <div className="border border-tokyo-border bg-tokyo-header p-4 text-xs">
                      <p>
                        <span className="text-tokyo-accent font-bold">vixland@box</span>
                        <span className="text-[#a9b1d6]">:</span>
                        <span className="text-tokyo-accent">~/terrat</span>
                        <span className="text-[#a9b1d6]">$ </span>
                        <span className="text-white">htop</span>
                      </p>
                      <p className="text-[#a9b1d6] mt-3">Tab 2 runs its own shell session — switch with Ctrl+Tab, jump with Alt+2.</p>
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};
