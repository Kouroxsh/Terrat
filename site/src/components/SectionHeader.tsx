import React from 'react';

interface SectionHeaderProps {
  eyebrow: string;
  title: string;
  lede?: string;
  id?: string;
}

/** Shared section header: eyebrow, fluid h2, optional lede. */
export const SectionHeader: React.FC<SectionHeaderProps> = ({ eyebrow, title, lede, id }) => (
  <div className="max-w-3xl mb-12 sm:mb-14">
    <div className="font-mono text-xs text-tokyo-accent uppercase tracking-wider mb-3">
      {eyebrow}
    </div>
    <h2 id={id} className="text-h2 font-bold tracking-tight text-white">
      {title}
    </h2>
    {lede && <p className="text-lede text-[#a9b1d6] mt-4 max-w-2xl">{lede}</p>}
  </div>
);
