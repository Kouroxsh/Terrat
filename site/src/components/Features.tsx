import React from 'react';
import { Layers, ZoomIn, Search, Link2, Eye, MousePointer, Sparkles, AlertCircle } from 'lucide-react';

export const Features: React.FC = () => {
  const featureList = [
    {
      num: '01',
      title: 'Inline Ghost Text & Paths',
      subtitle: 'Fish-Like Cross-Platform Completion',
      desc: 'Real-time completion hints drawn dynamically in front of the cursor. Ingests Linux histories (~/.bash_history, ~/.zsh_history) and Windows PowerShell PSReadLine (ConsoleHost_history.txt). Intelligently auto-completes file paths with / and \\, escapes, and quotes.',
      shortcut: 'Tab / Right Arrow',
      icon: Sparkles,
      color: '#7dcfff',
    },
    {
      num: '02',
      title: 'Live Diagnostics & Linting',
      subtitle: 'Lexer-Based Error Detection & QuickFix',
      desc: 'Understands common typos (gti, dokcer, sl, git puch), unclosed quotes, chained operators (&&, ||, ;, |), and missing privileges. Shows guidance in the header and lets you auto-correct instantly with Alt+Enter.',
      shortcut: 'Alt + Enter (QuickFix) / Ctrl+Shift+D',
      icon: AlertCircle,
      color: '#e0af68',
    },
    {
      num: '03',
      title: 'Isolated Multi-Tabs',
      subtitle: 'Zero-Leak Concurrency & PTY Sessions',
      desc: 'Each tab allocates an independent pseudo-terminal session (/dev/ptmx on Linux, ConPTY on Windows) with dedicated I/O pumps bound to context.Context cancellation to eliminate goroutine leaks. Shell processes are fully isolated.',
      shortcut: 'Ctrl+Shift+T / Ctrl+Tab / Alt+1..9',
      icon: Layers,
      color: '#7aa2f7',
    },
    {
      num: '04',
      title: 'Live Font Scaling & Shaping',
      subtitle: 'Dynamic Rasterization & Unicode Fallback',
      desc: 'Adjust font size dynamically without restarting. TrueType glyphs are rendered into memory alpha masks in sub-millisecond time with automatic PTY reflow (SIGWINCH / ConPTY resize). Features comprehensive fallback with Persian/Arabic shaping.',
      shortcut: 'Ctrl+= / Ctrl+- / Ctrl+0 / Ctrl+Wheel',
      icon: ZoomIn,
      color: '#9ece6a',
    },
    {
      num: '05',
      title: 'In-Buffer Search',
      subtitle: 'Live Scrollback Matching',
      desc: 'Floating find bar searches through active screen rows and scrollback history. Highlights all matches with distinct active match indicators. Navigate forward and backward with Enter and Shift+Enter.',
      shortcut: 'Ctrl+Shift+F / Enter / Shift+Enter',
      icon: Search,
      color: '#e0af68',
    },
    {
      num: '06',
      title: 'URL Detection & Launch',
      subtitle: 'Async Browser Integration',
      desc: 'Scans visible cells for web URLs and file paths. Holding Ctrl highlights the detected URL and changes the mouse cursor to a pointer. Ctrl+Click opens the URL asynchronously in your default browser across Linux & Windows.',
      shortcut: 'Ctrl + Hover / Ctrl + Left Click',
      icon: Link2,
      color: '#7aa2f7',
    },
    {
      num: '07',
      title: 'Window Styling & Opacity',
      subtitle: 'Hardware-Accelerated Compositing',
      desc: 'Communicates directly with your window manager and compositor (Linux EWMH _NET_WM_WINDOW_OPACITY & Windows DWM). Tear-free double-buffering completely eliminates screen tearing and resizing artifacts.',
      shortcut: 'Configured in config.json',
      icon: Eye,
      color: '#bb9af7',
    },
    {
      num: '08',
      title: 'Hardened Clipboard & Safe Paste',
      subtitle: 'Anti-Pastejacking & Bracketed Paste',
      desc: 'Smart Ctrl+C/Ctrl+V behavior. DECSET 2004 bracketed paste mode prevents unintentional execution in REPLs. Anti-pastejacking sanitizer strips malicious escapes and terminal controls while preserving valid Unicode like Persian ZWNJ.',
      shortcut: 'Smart Ctrl+C / Ctrl+V / Bracketed Paste',
      icon: MousePointer,
      color: '#f7768e',
    },
  ];

  return (
    <section id="features" className="py-20 hairline-b bg-[#1a1b26]">
      <div className="max-w-7xl mx-auto px-6">
        {/* Section Header */}
        <div className="mb-14">
          <div className="font-mono text-xs text-[#7aa2f7] uppercase tracking-wider mb-2">
            CORE FEATURES
          </div>
          <h2 className="text-3xl sm:text-5xl font-bold tracking-tight text-white mb-3">
            Practical features for everyday terminal work.
          </h2>
          <p className="text-sm sm:text-base text-[#a9b1d6] max-w-2xl">
            Built-in capabilities designed for Linux &amp; Windows workstations without requiring external multiplexers or heavy desktop frameworks.
          </p>
        </div>

        {/* Feature Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {featureList.map((f, i) => {
            const Icon = f.icon;
            return (
              <div
                key={i}
                className="p-6 bg-[#16161e] border border-[#292e42] hover:border-[#3b4261] rounded-[2px] transition-colors flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-center justify-between mb-4">
                    <span className="font-mono text-xs text-[#565f89]">
                      {f.num}
                    </span>
                    <Icon className="w-5 h-5" style={{ color: f.color }} />
                  </div>

                  <h3 className="text-lg font-bold text-white mb-1">{f.title}</h3>
                  <div className="font-mono text-[11px] text-[#7aa2f7] mb-3">{f.subtitle}</div>
                  <p className="text-xs text-[#a9b1d6] leading-relaxed mb-6">
                    {f.desc}
                  </p>
                </div>

                <div className="pt-3 border-t border-[#292e42] font-mono text-[11px] text-[#565f89]">
                  Shortcut: <span className="text-[#c0caf5]">{f.shortcut}</span>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
};
