import React, { useState } from 'react';
import { Copy, Check, Menu, X } from 'lucide-react';
import { GithubIcon } from './GithubIcon';

const links = [
  { href: '#showcase', label: 'Showcase' },
  { href: '#features', label: 'Features' },
  { href: '#architecture', label: 'Architecture' },
  { href: '#platforms', label: 'Benchmarks' },
  { href: '#themes', label: 'Themes' },
  { href: '#install', label: 'Install' },
];

export const Navbar: React.FC = () => {
  const [copied, setCopied] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  const handleCopyInstall = () => {
    navigator.clipboard.writeText('go install github.com/Kouroxsh/Terrat/cmd/terrat@latest');
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <nav className="sticky top-0 z-50 bg-tokyo-header/90 backdrop-blur-md hairline-b">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 h-16 flex items-center justify-between gap-3">
        {/* Logo & Brand */}
        <a href="#" className="flex items-center gap-3 shrink-0" aria-label="Terrat — back to top">
          <img src="./icon.png" alt="Terrat logo" className="w-8 h-8 rounded-full border border-tokyo-border" />
          <span className="font-mono font-bold tracking-wider text-white text-lg">TERRAT</span>
          <span className="font-mono text-xs px-2 py-0.5 border border-tokyo-border text-tokyo-yellow">v3.0.4</span>
        </a>

        {/* Navigation Links */}
        <div className="hidden lg:flex items-center gap-7 font-mono text-xs uppercase tracking-wider text-[#a9b1d6]">
          {links.map((link) => (
            <a key={link.href} href={link.href} className="anim hover:text-white">
              {link.label}
            </a>
          ))}
        </div>

        {/* Right CTA */}
        <div className="flex items-center gap-2 sm:gap-3">
          <button
            onClick={handleCopyInstall}
            aria-label="Copy go install command"
            className="anim hidden sm:flex items-center gap-2 font-mono text-xs px-3 py-1.5 rounded-sm bg-tokyo-bg hover:bg-tokyo-subtle border border-tokyo-border text-tokyo-fg"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-tokyo-green" aria-hidden="true" /> : <Copy className="w-3.5 h-3.5 text-tokyo-accent" aria-hidden="true" />}
            <span>{copied ? 'Copied' : 'go install terrat'}</span>
          </button>

          <a
            href="https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4"
            className="anim hidden md:flex items-center gap-2 font-mono text-xs px-3 py-1.5 rounded-sm bg-tokyo-accent hover:bg-[#89b4fa] text-tokyo-header font-semibold"
          >
            Download v3.0.4
          </a>

          <a
            href="https://github.com/Kouroxsh/Terrat"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Terrat on GitHub"
            className="anim flex items-center gap-2 font-mono text-xs px-3 py-1.5 rounded-sm border border-tokyo-border text-tokyo-fg hover:bg-tokyo-subtle"
          >
            <GithubIcon className="w-4 h-4" />
            <span className="hidden sm:inline">GitHub</span>
          </a>

          <button
            type="button"
            onClick={() => setMenuOpen((open) => !open)}
            aria-expanded={menuOpen}
            aria-controls="mobile-nav"
            aria-label={menuOpen ? 'Close menu' : 'Open menu'}
            className="anim lg:hidden flex items-center justify-center w-9 h-9 rounded-sm border border-tokyo-border text-tokyo-fg hover:bg-tokyo-subtle"
          >
            {menuOpen ? <X className="w-4 h-4" aria-hidden="true" /> : <Menu className="w-4 h-4" aria-hidden="true" />}
          </button>
        </div>
      </div>

      {/* Mobile navigation panel */}
      {menuOpen && (
        <div id="mobile-nav" className="lg:hidden border-t border-tokyo-border bg-tokyo-header fade-in">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-3 flex flex-col font-mono text-sm">
            {links.map((link) => (
              <a
                key={link.href}
                href={link.href}
                onClick={() => setMenuOpen(false)}
                className="anim py-2.5 text-[#a9b1d6] hover:text-white border-b border-tokyo-border last:border-b-0"
              >
                {link.label}
              </a>
            ))}
            <a
              href="https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4"
              className="anim mt-3 mb-1 text-center py-2.5 rounded-sm bg-tokyo-accent text-tokyo-header font-semibold"
            >
              Download v3.0.4
            </a>
          </div>
        </div>
      )}
    </nav>
  );
};
