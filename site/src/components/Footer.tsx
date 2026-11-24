import React from 'react';
import { GithubIcon } from './GithubIcon';

const navLinks = [
  { href: '#showcase', label: 'Showcase' },
  { href: '#features', label: 'Features' },
  { href: '#architecture', label: 'Architecture' },
  { href: '#platforms', label: 'Benchmarks' },
  { href: '#themes', label: 'Themes' },
  { href: '#install', label: 'Install' },
];

export const Footer: React.FC = () => {
  return (
    <footer className="bg-tokyo-header text-[#a9b1d6] font-mono text-xs hairline-t">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 py-12">
        <div className="grid grid-cols-1 md:grid-cols-12 gap-8">
          <div className="md:col-span-5">
            <div className="flex items-center gap-3 mb-3">
              <img src="./icon.png" alt="Terrat logo" className="w-6 h-6 rounded-full border border-tokyo-border" />
              <span className="text-tokyo-fg font-semibold text-sm">Terrat</span>
              <span className="text-xs px-2 py-0.5 border border-tokyo-border text-tokyo-yellow">v3.0.4</span>
            </div>
            <p className="leading-relaxed max-w-sm">
              Go terminal emulator for Linux. Tabs, themes, in-buffer search and command diagnostics — no CGO.
            </p>
          </div>
          <div className="md:col-span-4">
            <div className="text-tokyo-accent uppercase tracking-wider mb-3">Explore</div>
            <div className="grid grid-cols-2 gap-x-4 gap-y-2">
              {navLinks.map((link) => (
                <a key={link.href} href={link.href} className="anim hover:text-white">
                  {link.label}
                </a>
              ))}
            </div>
          </div>
          <div className="md:col-span-3">
            <div className="text-tokyo-accent uppercase tracking-wider mb-3">Releases</div>
            <a
              href="https://github.com/Kouroxsh/Terrat/releases/tag/v3.0.4"
              target="_blank"
              rel="noopener noreferrer"
              className="anim block hover:text-white mb-2"
            >
              v3.0.4 — Latest stable
            </a>
            <a
              href="https://github.com/Kouroxsh/Terrat"
              target="_blank"
              rel="noopener noreferrer"
              className="anim flex items-center gap-1.5 text-tokyo-accent hover:text-white"
            >
              <GithubIcon className="w-3.5 h-3.5" />
              <span>github.com/Kouroxsh/Terrat</span>
            </a>
          </div>
        </div>

        <div className="mt-10 pt-6 border-t border-tokyo-border flex flex-col sm:flex-row items-center justify-between gap-3">
          <span>MIT License</span>
          <span>Terrat — a Linux terminal in pure Go</span>
        </div>
      </div>
    </footer>
  );
};
