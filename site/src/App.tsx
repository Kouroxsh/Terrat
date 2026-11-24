import React from 'react';
import { Navbar } from './components/Navbar';
import { HeroTerminal } from './components/HeroTerminal';
import { Showcase } from './components/Showcase';
import { Features } from './components/Features';
import { Architecture } from './components/Architecture';
import { Benchmarks } from './components/Benchmarks';
import { ThemePreview } from './components/ThemePreview';
import { InstallSection } from './components/InstallSection';
import { Footer } from './components/Footer';
import { MobileCtaBar } from './components/MobileCtaBar';

export const App: React.FC = () => {
  return (
    <div className="min-h-screen flex flex-col bg-tokyo-bg text-tokyo-fg overflow-x-hidden pb-14 md:pb-0">
      <Navbar />
      <main className="flex-1">
        <HeroTerminal />
        <Showcase />
        <Features />
        <Architecture />
        <Benchmarks />
        <ThemePreview />
        <InstallSection />
      </main>
      <Footer />
      <MobileCtaBar />
    </div>
  );
};

export default App;
