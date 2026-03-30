import { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import ThemeToggle from '../components/ThemeToggle';

const baseUrl = import.meta.env.BASE_URL;

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      onClick={handleCopy}
      className="absolute top-2.5 right-2.5 p-1.5 rounded-md text-gray-400 hover:text-gray-200 hover:bg-white/10 transition-colors"
      title="Copy to clipboard"
    >
      {copied ? (
        <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      ) : (
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
      )}
    </button>
  );
}

function CodeBlock({ label, code }: { label: string; code: string }) {
  return (
    <div className="relative group">
      <div className="text-sm font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wider mb-2">
        {label}
      </div>
      <div className="relative bg-gray-900 dark:bg-gray-950 rounded-lg border border-gray-800 dark:border-gray-700/50">
        <pre className="px-4 py-3.5 pr-12 overflow-x-auto text-base font-mono text-gray-100 leading-relaxed">
          <code>{code}</code>
        </pre>
        <CopyButton text={code} />
      </div>
    </div>
  );
}

function useScrollReveal() {
  const ref = useRef<HTMLDivElement>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => { if (entry.isIntersecting) setIsVisible(true); },
      { threshold: 0.1, rootMargin: '0px 0px -40px 0px' }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return { ref, isVisible };
}

function Section({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  const { ref, isVisible } = useScrollReveal();
  return (
    <div
      ref={ref}
      className={`transition-all duration-700 ease-out ${isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-6'} ${className}`}
    >
      {children}
    </div>
  );
}

const features = [
  {
    title: 'File-based',
    description: 'Plain JSON and TOML that version control tracks like any other file. Clone a repo, get its board.',
    icon: (
      <svg className="w-5.5 h-5.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
      </svg>
    ),
  },
  {
    title: 'Local and fast',
    description: 'No network calls, no loading spinners. Your board is just files on disk.',
    icon: (
      <svg className="w-5.5 h-5.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
      </svg>
    ),
  },
  {
    title: 'Self-contained',
    description: 'No database, no login, no SaaS. Just a single binary that serves a web UI.',
    icon: (
      <svg className="w-5.5 h-5.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
      </svg>
    ),
  },
  {
    title: 'Full CLI',
    description: 'Scriptable for CI, automation, and AI agents. Add, edit, move, and query cards programmatically.',
    icon: (
      <svg className="w-5.5 h-5.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 7.5l3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0021 18V6a2.25 2.25 0 00-2.25-2.25H5.25A2.25 2.25 0 003 6v12a2.25 2.25 0 002.25 2.25z" />
      </svg>
    ),
  },
];

export default function LandingPage() {
  const heroLogoRef = useRef<HTMLImageElement>(null);
  const [showNavLogo, setShowNavLogo] = useState(false);

  useEffect(() => {
    const el = heroLogoRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => setShowNavLogo(!entry.isIntersecting),
      { threshold: 0 }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <div className="min-h-screen bg-white dark:bg-gray-950 text-gray-900 dark:text-gray-100">
      {/* Nav */}
      <nav className="sticky top-0 z-50 backdrop-blur-lg bg-white/80 dark:bg-gray-950/80 border-b border-gray-200/60 dark:border-gray-800/60">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 h-14 flex items-center gap-4">
          <Link
            to="/"
            className={`shrink-0 transition-opacity duration-300 ${showNavLogo ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
          >
            <img src={`${baseUrl}images/logo.png`} alt="Kan" className="h-7 w-auto" />
          </Link>
          <div className="flex-1" />
          <Link
            to="/docs"
            className="text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
          >
            Docs
          </Link>
          <a
            href="https://github.com/amterp/kan"
            target="_blank"
            rel="noopener noreferrer"
            className="text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
          >
            GitHub
          </a>
          <ThemeToggle />
        </div>
      </nav>

      {/* Hero */}
      <section className="relative overflow-hidden">
        {/* Subtle grid background */}
        <div className="absolute inset-0 opacity-[0.03] dark:opacity-[0.05]" style={{
          backgroundImage: 'linear-gradient(to right, currentColor 1px, transparent 1px), linear-gradient(to bottom, currentColor 1px, transparent 1px)',
          backgroundSize: '48px 48px',
        }} />

        <div className="relative max-w-6xl mx-auto px-4 sm:px-6 pt-14 sm:pt-20 pb-12 sm:pb-16">
          <Section>
            <div className="max-w-3xl">
              <img ref={heroLogoRef} src={`${baseUrl}images/logo.png`} alt="Kan" className="h-20 sm:h-28 w-auto mb-6" />
              <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold tracking-tight leading-[1.1] text-gray-950 dark:text-white">
                A kanban board that lives in your repository
              </h1>
              <p className="mt-6 text-lg sm:text-xl leading-relaxed text-gray-600 dark:text-gray-400 max-w-3xl">
                A file-based kanban board stored as plain files in your repo. Run{' '}
                <code className="text-base sm:text-lg font-mono bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded">kan serve</code>{' '}
                to open the web UI, or use the CLI for quick updates. Your board is version-controlled alongside your code.
              </p>
              <div className="mt-8 flex flex-wrap gap-3">
                <Link
                  to="/docs"
                  className="inline-flex items-center px-6 py-3 rounded-lg bg-blue-600 text-white text-base font-medium hover:bg-blue-700 transition-colors shadow-sm"
                >
                  Get Started
                  <svg className="ml-2 w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                </Link>
                <a
                  href="https://github.com/amterp/kan"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center px-6 py-3 rounded-lg text-base font-medium border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
                >
                  <svg className="mr-2 w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
                  </svg>
                  View on GitHub
                </a>
              </div>
            </div>
          </Section>
        </div>
      </section>

      {/* Screenshot */}
      <section className="max-w-6xl mx-auto px-4 sm:px-6 pb-14 sm:pb-18">
        <Section>
          <div className="rounded-xl border border-gray-200 dark:border-gray-800 shadow-xl dark:shadow-2xl overflow-hidden bg-gray-100 dark:bg-gray-900">
            {/* Browser chrome */}
            <div className="flex items-center gap-2 px-4 py-2.5 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-800">
              <div className="flex gap-1.5">
                <div className="w-2.5 h-2.5 rounded-full bg-gray-300 dark:bg-gray-700" />
                <div className="w-2.5 h-2.5 rounded-full bg-gray-300 dark:bg-gray-700" />
                <div className="w-2.5 h-2.5 rounded-full bg-gray-300 dark:bg-gray-700" />
              </div>
              <div className="flex-1 ml-2">
                <div className="max-w-xs mx-auto h-5 rounded-md bg-gray-200 dark:bg-gray-800 flex items-center justify-center">
                  <span className="text-[10px] font-mono text-gray-400 dark:text-gray-600">localhost:5260</span>
                </div>
              </div>
            </div>
            <img
              src={`${baseUrl}images/web-screenshot.png`}
              alt="Kan board showing columns with cards, drag-and-drop interface"
              className="w-full block"
            />
          </div>
        </Section>
      </section>

      {/* Features */}
      <section className="border-t border-gray-100 dark:border-gray-900 bg-gray-50/50 dark:bg-gray-900/30">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 py-10 sm:py-14">
          <Section>
            <h2 className="text-3xl sm:text-4xl font-bold tracking-tight text-gray-950 dark:text-white">
              Why Kan?
            </h2>
            <p className="mt-4 text-lg text-gray-600 dark:text-gray-400 max-w-xl">
              Every project gets its own board that lives in the repo. Open the project, run{' '}
              <code className="font-mono bg-gray-200/60 dark:bg-gray-800 px-1.5 py-0.5 rounded">kan serve</code>, and you're right where you left off.
            </p>
          </Section>

          <div className="mt-12 grid grid-cols-1 sm:grid-cols-2 gap-5">
            {features.map((feature, i) => (
              <Section key={feature.title}>
                <div
                  className="h-full rounded-xl border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 p-7 transition-colors hover:border-gray-300 dark:hover:border-gray-700"
                  style={{ transitionDelay: `${i * 75}ms` }}
                >
                  <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-blue-50 dark:bg-blue-950/50 text-blue-600 dark:text-blue-400">
                    {feature.icon}
                  </div>
                  <h3 className="mt-4 text-lg font-semibold text-gray-950 dark:text-white">
                    {feature.title}
                  </h3>
                  <p className="mt-2 text-base leading-relaxed text-gray-600 dark:text-gray-400">
                    {feature.description}
                  </p>
                </div>
              </Section>
            ))}
          </div>
        </div>
      </section>

      {/* Installation */}
      <section className="border-t border-gray-100 dark:border-gray-900">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 py-10 sm:py-14">
          <Section>
            <h2 className="text-3xl sm:text-4xl font-bold tracking-tight text-gray-950 dark:text-white">
              Installation
            </h2>
          </Section>

          <div className="mt-10 grid grid-cols-1 sm:grid-cols-2 gap-6">
            <Section>
              <CodeBlock label="Homebrew" code="brew tap amterp/tap && brew install kan" />
            </Section>
            <Section>
              <CodeBlock label="Go" code="go install github.com/amterp/kan/cmd/kan@latest" />
            </Section>
          </div>

          <div className="mt-10">
            <Section>
              <CodeBlock label="Quick Start" code="kan init && kan serve" />
            </Section>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-200 dark:border-gray-800">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 py-8 flex flex-col sm:flex-row items-center justify-between gap-4 text-sm text-gray-500 dark:text-gray-500">
          <span>Kan - MIT License</span>
          <div className="flex items-center gap-5">
            <Link
              to="/docs"
              className="hover:text-gray-900 dark:hover:text-gray-300 transition-colors"
            >
              Docs
            </Link>
            <a
              href="https://github.com/amterp/kan"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-gray-900 dark:hover:text-gray-300 transition-colors"
            >
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
