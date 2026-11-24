import React, { useState } from 'react';
import { Copy, Check, Terminal, Download, Keyboard, Settings, Bug } from 'lucide-react';
import { SectionHeader } from './SectionHeader';

export const InstallSection: React.FC = () => {
  const [copiedTab, setCopiedTab] = useState<string | null>(null);

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedTab(id);
    setTimeout(() => setCopiedTab(null), 2000);
  };

  const keybindings = [
    { key: 'Tab / Right Arrow', desc: 'Accept completion suggestion' },
    { key: 'Alt + Enter', desc: 'Apply command suggestion' },
    { key: 'Ctrl + Shift + D', desc: 'Toggle command diagnostics' },
    { key: 'Ctrl + Shift + T', desc: 'Open a new tab' },
    { key: 'Ctrl + Shift + W', desc: 'Close active tab' },
    { key: 'Ctrl + Tab', desc: 'Switch to next tab' },
    { key: 'Alt + 1..9', desc: 'Jump to tab 1..9' },
    { key: 'Ctrl + Shift + F', desc: 'Toggle in-buffer search' },
    { key: 'Ctrl + = / - / 0', desc: 'Zoom font size in / out / reset' },
    { key: 'Ctrl + Click', desc: 'Open URL in default browser' },
    { key: 'Ctrl + ,', desc: 'Open preferences & themes modal' },
    { key: 'Ctrl + Shift + C', desc: 'Copy selected buffer text' },
    { key: 'Ctrl + Shift + V', desc: 'Paste text from clipboard' },
    { key: 'Ctrl + Shift + A', desc: 'Select entire terminal buffer' },
    { key: 'Shift + PageUp / Down', desc: 'Scroll buffer half-page' },
    { key: 'Mouse Middle Click', desc: 'Paste primary selection' },
  ];

  const packages = [
    { label: '.deb — Debian / Ubuntu', href: 'https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4' },
    { label: '.rpm — Fedora / RHEL / openSUSE', href: 'https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4' },
    { label: '.pkg.tar.zst — Arch Linux', href: 'https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4' },
    { label: '.tar.gz — Portable Linux', href: 'https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4' },
  ];

  return (
    <section id="install" className="py-20 sm:py-24 hairline-b bg-tokyo-bg">
      <div className="max-w-7xl mx-auto px-4 sm:px-6">
        <SectionHeader
          eyebrow="Installation & configuration"
          title="Install Terrat in minutes."
          lede="On Linux, Terrat needs an X11 server or XWayland. It builds without CGO."
        />

        {/* Installation Boxes */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-12 items-start">
          {/* Method 1: Go Install */}
          <div className="anim p-6 bg-tokyo-surface border border-tokyo-border rounded-sm hover:border-[#3b4261] flex flex-col justify-between">
            <div>
              <div className="flex items-center justify-between mb-3">
                <span className="font-mono text-xs text-tokyo-accent uppercase tracking-wider font-bold">
                  Go Install
                </span>
                <Terminal className="w-4 h-4 text-tokyo-accent" aria-hidden="true" />
              </div>
              <p className="text-sm text-[#a9b1d6] mb-4">
                Installs binary directly to your <code className="text-tokyo-fg">$GOPATH/bin</code>:
              </p>
              <div className="p-3 bg-tokyo-header border border-tokyo-border rounded-sm font-mono text-xs flex items-center justify-between gap-3">
                <div className="overflow-x-auto min-w-0 flex-1">
                  <code className="text-tokyo-fg select-all whitespace-nowrap">go install github.com/Kouroxsh/Terrat/cmd/terrat@latest</code>
                </div>
                <button
                  onClick={() => copyToClipboard('go install github.com/Kouroxsh/Terrat/cmd/terrat@latest', 'go')}
                  aria-label="Copy go install command"
                  className="anim text-[#a9b1d6] hover:text-white shrink-0"
                >
                  {copiedTab === 'go' ? <Check className="w-4 h-4 text-tokyo-green" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />}
                </button>
              </div>
            </div>
            <div className="mt-4 text-xs font-mono text-[#a9b1d6]">
              Ensure <code className="text-tokyo-fg">~/go/bin</code> is in your <code className="text-tokyo-fg">$PATH</code>.
            </div>
          </div>

          {/* Method 2: Build Linux */}
          <div className="anim p-6 bg-tokyo-surface border border-tokyo-border rounded-sm hover:border-[#3b4261] flex flex-col justify-between">
            <div>
              <div className="flex items-center justify-between mb-3">
                <span className="font-mono text-xs text-tokyo-green uppercase tracking-wider font-bold">
                  Linux: Make Install
                </span>
                <Download className="w-4 h-4 text-tokyo-green" aria-hidden="true" />
              </div>
              <p className="text-sm text-[#a9b1d6] mb-4">
                Builds the program and installs a desktop launcher and icon:
              </p>
              <div className="p-3 bg-tokyo-header border border-tokyo-border rounded-sm font-mono text-xs flex items-center justify-between gap-3">
                <div className="overflow-x-auto min-w-0 flex-1">
                  <pre className="text-tokyo-fg select-all leading-relaxed whitespace-pre font-mono text-xs">{`git clone https://github.com/Kouroxsh/Terrat.git
cd Terrat && make install`}</pre>
                </div>
                <button
                  onClick={() =>
                    copyToClipboard(
                      'git clone https://github.com/Kouroxsh/Terrat.git && cd Terrat && make install',
                      'make'
                    )
                  }
                  aria-label="Copy make install commands"
                  className="anim text-[#a9b1d6] hover:text-white shrink-0 self-start"
                >
                  {copiedTab === 'make' ? <Check className="w-4 h-4 text-tokyo-green" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />}
                </button>
              </div>
            </div>
            <div className="mt-4 text-xs font-mono text-[#a9b1d6]">
              Installs to <code className="text-tokyo-fg">~/go/bin/terrat</code> and registers in the app menu.
            </div>
          </div>

          {/* Method 3: Download Packages */}
          <div className="anim p-6 bg-tokyo-surface border border-tokyo-border rounded-sm hover:border-[#3b4261] flex flex-col justify-between">
            <div>
              <div className="flex items-center justify-between mb-3">
                <span className="font-mono text-xs text-tokyo-yellow uppercase tracking-wider font-bold">
                  Download Packages
                </span>
                <Download className="w-4 h-4 text-tokyo-yellow" aria-hidden="true" />
              </div>
              <p className="text-sm text-[#a9b1d6] mb-4">
                v3.0.4 packages built with nfpm — no build needed:
              </p>
              <div className="flex flex-col gap-2 font-mono text-xs">
                {packages.map((pkg) => (
                  <a
                    key={pkg.label}
                    href={pkg.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="anim p-3 bg-tokyo-header border border-tokyo-border hover:border-tokyo-accent text-tokyo-fg flex items-center justify-between gap-3"
                  >
                    <span>{pkg.label}</span>
                    <span className="text-[#a9b1d6] shrink-0 whitespace-nowrap">amd64 · arm64</span>
                  </a>
                ))}
              </div>
            </div>
            <div className="mt-4 text-xs font-mono text-[#a9b1d6]">
              v3.0.4 stable release — see the release page for packages.
            </div>
          </div>
        </div>

        {/* Bug report callout */}
        <div className="border border-tokyo-yellow/30 bg-tokyo-yellow/5 rounded-sm p-5 mb-8 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 font-mono text-xs">
          <div className="flex items-start gap-3">
            <Bug className="w-4 h-4 text-tokyo-yellow shrink-0 mt-0.5" aria-hidden="true" />
            <div>
              <div className="text-tokyo-yellow font-bold uppercase tracking-wider mb-1">
                Found a bug?
              </div>
              <p className="text-[#a9b1d6] leading-relaxed">
                If you encounter a crash, rendering issue, or other unexpected behavior, please report it on GitHub.
              </p>
            </div>
          </div>
          <a
            href="https://github.com/Kouroxsh/Terrat/issues"
            target="_blank"
            rel="noopener noreferrer"
            className="anim px-3.5 py-2 bg-tokyo-yellow/15 hover:bg-tokyo-yellow/25 text-tokyo-yellow hover:text-white border border-tokyo-yellow/40 rounded-sm shrink-0 font-semibold"
          >
            Open GitHub Issue &rarr;
          </a>
        </div>

        {/* Configuration snippet */}
        <div className="border border-tokyo-border bg-tokyo-surface rounded-sm p-6 mb-8">
          <div className="flex items-center gap-2 font-mono text-xs text-tokyo-accent uppercase tracking-wider mb-4">
            <Settings className="w-4 h-4" aria-hidden="true" />
            <span>Configuration (~/.config/terrat/config.json)</span>
          </div>
          <div className="p-4 bg-tokyo-header border border-tokyo-border rounded-sm font-mono text-xs text-[#a9b1d6] overflow-x-auto">
            <pre>{`{
  "theme": "auto",
  "font_size": 13.0,
  "opacity": 0.95,
  "ghost_text": true,
  "diagnostics": true,
  "sanitize_paste": true,
  "bracketed_paste": true
}`}</pre>
          </div>
          <div className="mt-3 text-xs text-[#a9b1d6] font-mono">
            Automatically generated on first launch if not present. All settings can be updated at runtime via the Preferences modal.
          </div>
        </div>

        {/* Keybindings Table */}
        <div className="border border-tokyo-border bg-tokyo-surface rounded-sm p-6">
          <div className="font-mono text-xs text-tokyo-accent uppercase tracking-wider mb-4 flex items-center gap-2">
            <Keyboard className="w-4 h-4" aria-hidden="true" />
            <span>Default Keyboard Shortcuts</span>
          </div>

          <div className="flex flex-wrap justify-center gap-3 font-mono text-xs">
            {keybindings.map((item) => (
              <div
                key={item.key}
                className="w-full md:w-[calc(50%-0.375rem)] lg:w-[calc(33.333%-0.5rem)] p-2.5 bg-tokyo-header border border-tokyo-border rounded-sm flex items-center justify-between gap-2"
              >
                <span className="text-tokyo-yellow font-medium">{item.key}</span>
                <span className="text-[#a9b1d6] text-right">{item.desc}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
};
