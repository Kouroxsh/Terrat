import React from 'react';

export const HeroTerminal: React.FC = () => (
  <section className="pt-16 pb-20 hairline-b bg-grid-pattern">
    <div className="max-w-7xl mx-auto px-6">
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 items-end mb-14">
        <div className="lg:col-span-8">
          <p className="font-mono text-xs text-[#7aa2f7] uppercase tracking-wider mb-5">Linux · Go</p>
          <h1 className="text-4xl sm:text-6xl lg:text-7xl font-semibold tracking-tight text-white leading-[1.05] mb-6">
            A terminal built in Go.
          </h1>
          <p className="text-base sm:text-lg text-[#a9b1d6] max-w-2xl leading-relaxed">
            TerraTerminal is a desktop terminal for Linux, with tabs, configurable themes, in-buffer search, and shell history suggestions.
          </p>
        </div>
        <div className="lg:col-span-4 flex flex-col gap-3 font-mono text-xs">
          <div className="p-4 bg-[#16161e] border border-[#292e42]">
            <div className="text-[#565f89] mb-2 uppercase tracking-wider">Install with Go</div>
            <code className="text-[#c0caf5] break-all">go install github.com/Kouroxsh/Terrat/cmd/terrat@latest</code>
          </div>
          <div className="flex gap-3">
            <a href="https://github.com/Kouroxsh/Terrat/releases" className="flex-1 text-center py-2.5 bg-[#7aa2f7] hover:bg-[#89b4fa] text-[#13141c] font-semibold transition-colors">
              Download release
            </a>
            <a href="https://github.com/Kouroxsh/Terrat" className="flex-1 text-center py-2.5 border border-[#292e42] text-[#c0caf5] hover:bg-[#24283b] transition-colors">
              View source
            </a>
          </div>
        </div>
      </div>

      <div className="max-w-5xl mx-auto border border-[#292e42] bg-[#1a1b26] shadow-2xl">
        <div className="h-10 bg-[#13141c] border-b border-[#292e42] flex items-center justify-between px-4 font-mono text-xs">
          <div className="flex items-center gap-3 text-[#c0caf5]">
            <span className="text-[#7aa2f7]">1:</span><span>bash</span>
          </div>
          <span className="text-[#565f89]">Preview</span>
        </div>
        <div className="min-h-64 p-5 sm:p-7 font-mono text-sm leading-7 overflow-x-auto">
          <p><span className="text-[#9ece6a]">user@machine</span><span className="text-[#565f89]">:~$ </span><span className="text-[#c0caf5]">terrat --help</span></p>
          <p className="text-[#a9b1d6] mt-2">Usage: terrat [flags] [-e command [args...]]</p>
          <p className="text-[#565f89] mt-3">Options</p>
          <div className="grid grid-cols-[8rem_1fr] gap-x-4 text-xs sm:text-sm text-[#a9b1d6]">
            <span className="text-[#7aa2f7]">-theme</span><span>Choose a color theme</span>
            <span className="text-[#7aa2f7]">-font-size</span><span>Set the font size</span>
            <span className="text-[#7aa2f7]">-shell</span><span>Choose the default shell</span>
            <span className="text-[#7aa2f7]">-e</span><span>Run a command in a new terminal session</span>
          </div>
          <p className="mt-5"><span className="text-[#9ece6a]">user@machine</span><span className="text-[#565f89]">:~$ </span><span className="inline-block w-2 h-4 align-middle bg-[#7aa2f7]" /></p>
        </div>
      </div>
    </div>
  </section>
);
