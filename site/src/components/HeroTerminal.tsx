import React from 'react';

export const HeroTerminal: React.FC = () => (
  <section className="pt-16 pb-20 hairline-b bg-grid-pattern">
    <div className="max-w-7xl mx-auto px-6">
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 items-end mb-14">
        <div className="lg:col-span-8">
          <p className="font-mono text-xs text-[#7aa2f7] uppercase tracking-wider mb-5">Linux · Go · v3.0.2</p>
          <h1 className="text-4xl sm:text-6xl lg:text-7xl font-semibold tracking-tight text-white leading-[1.05] mb-6">
            A Linux terminal with tabs and command suggestions.
          </h1>
          <p className="text-base sm:text-lg text-[#a9b1d6] max-w-2xl leading-relaxed">
            Terrat is a desktop terminal for Linux. It has configurable themes, search, shell history suggestions, and command diagnostics.
          </p>
          <div className="flex gap-2 mt-6 font-mono text-[11px]">
            <span className="px-2 py-1 border border-[#292e42] text-[#c0caf5]">v3.0.2</span>
            <span className="px-2 py-1 border border-[#292e42] text-[#565f89]">X11 / XWayland · No CGO</span>
          </div>
        </div>
        <div className="lg:col-span-4 flex flex-col gap-3 font-mono text-xs">
          <div className="p-4 bg-[#16161e] border border-[#292e42]">
            <div className="text-[#565f89] mb-2 uppercase tracking-wider">Install with Go</div>
            <code className="text-[#c0caf5] break-all">go install github.com/Kouroxsh/Terrat/cmd/terrat@latest</code>
          </div>
          <div className="flex gap-3">
            <a href="https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.2" className="flex-1 text-center py-2.5 bg-[#7aa2f7] hover:bg-[#89b4fa] text-[#13141c] font-semibold transition-colors">
              Download v3.0.2
            </a>
            <a href="https://github.com/Kouroxsh/Terrat" className="flex-1 text-center py-2.5 border border-[#292e42] text-[#c0caf5] hover:bg-[#24283b] transition-colors">
              View source
            </a>
          </div>
          <p className="text-[11px] text-[#565f89] leading-relaxed">
            v3.0.2 packages: .deb · .rpm · .pkg.tar.zst · .tar.gz — see Releases
          </p>
        </div>
      </div>

      <div className="max-w-5xl mx-auto border border-[#292e42] bg-[#1a1b26] shadow-2xl">
        <div className="h-10 bg-[#13141c] border-b border-[#292e42] flex items-center justify-between px-4 font-mono text-xs">
          <div className="flex items-center gap-3 text-[#c0caf5]">
            <span className="text-[#7aa2f7]">1:</span><span>bash</span>
          </div>
          <span className="text-[#565f89]">Screenshot</span>
        </div>
        <img
          src="https://raw.githubusercontent.com/Kouroxsh/Terrat/main/assets/screenshot.png"
          alt="Terrat screenshot — real terminal window"
          className="w-full block"
        />
      </div>
      <p className="text-center font-mono text-[11px] text-[#565f89] mt-3">
        Real screenshot from the repository — assets/screenshot.png
      </p>
    </div>
  </section>
);
