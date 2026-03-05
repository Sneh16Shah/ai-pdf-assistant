import { useState, useEffect, useRef } from 'react';
import { getGlobalMetrics, GlobalMetrics } from '../services/api';
import { useTheme } from '../contexts/ThemeContext';

interface LandingPageProps {
    onGetStarted: () => void;
}

// Animated counter hook
function useCounter(target: number, duration = 2000, start = false) {
    const [value, setValue] = useState(0);
    useEffect(() => {
        if (!start || target === 0) return;
        const startTime = performance.now();
        const tick = (now: number) => {
            const elapsed = now - startTime;
            const progress = Math.min(elapsed / duration, 1);
            // Ease-out cubic
            const eased = 1 - Math.pow(1 - progress, 3);
            setValue(Math.round(eased * target));
            if (progress < 1) requestAnimationFrame(tick);
        };
        requestAnimationFrame(tick);
    }, [target, duration, start]);
    return value;
}

function formatNumber(n: number): string {
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
    if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
    return n.toString();
}

export default function LandingPage({ onGetStarted }: LandingPageProps) {
    const { theme, toggleTheme } = useTheme();
    const [metrics, setMetrics] = useState<GlobalMetrics | null>(null);
    const [countersStarted, setCountersStarted] = useState(false);
    const statsRef = useRef<HTMLDivElement>(null);

    const users = useCounter(metrics?.total_users ?? 0, 2000, countersStarted);
    const docs = useCounter(metrics?.total_documents ?? 0, 2200, countersStarted);
    const responses = useCounter(metrics?.total_responses ?? 0, 2400, countersStarted);

    useEffect(() => {
        getGlobalMetrics().then(setMetrics).catch(console.warn);
    }, []);

    // Start counters when stats section enters viewport
    useEffect(() => {
        const observer = new IntersectionObserver(
            ([entry]) => { if (entry.isIntersecting) setCountersStarted(true); },
            { threshold: 0.3 }
        );
        if (statsRef.current) observer.observe(statsRef.current);
        return () => observer.disconnect();
    }, []);

    // Also start once metrics arrive if already in view
    useEffect(() => {
        if (metrics) setCountersStarted(true);
    }, [metrics]);

    return (
        <div className="min-h-screen bg-gradient-to-br from-blue-50 via-indigo-50 to-violet-50 dark:from-gray-950 dark:via-indigo-950 dark:to-slate-900 relative overflow-hidden">

            {/* Background orbs */}
            <div className="pointer-events-none absolute inset-0 overflow-hidden">
                <div className="absolute -top-40 -left-40 w-96 h-96 bg-blue-400/20 dark:bg-blue-600/15 rounded-full blur-3xl" />
                <div className="absolute top-1/3 -right-40 w-80 h-80 bg-violet-400/20 dark:bg-violet-600/15 rounded-full blur-3xl" />
                <div className="absolute bottom-0 left-1/3 w-72 h-72 bg-indigo-400/20 dark:bg-indigo-600/10 rounded-full blur-3xl" />
            </div>

            {/* ── Nav ─────────────────────────────────────────────────────────────────── */}
            <nav className="relative z-10 flex items-center justify-between px-6 md:px-12 py-5">
                <div className="backdrop-blur-md bg-white/40 dark:bg-white/5 border border-white/60 dark:border-white/10 rounded-2xl px-5 py-3 flex items-center gap-2.5 shadow-lg shadow-black/5">
                    <div className="w-8 h-8 bg-gradient-to-br from-blue-500 to-violet-600 rounded-lg flex items-center justify-center shadow-md">
                        <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                        </svg>
                    </div>
                    <span className="font-bold text-gray-900 dark:text-white text-lg tracking-tight">AskMyPDF</span>
                </div>
                <div className="flex items-center gap-2">
                    {/* Theme toggle */}
                    <button
                        onClick={toggleTheme}
                        title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
                        className="backdrop-blur-md bg-white/40 dark:bg-white/5 border border-white/60 dark:border-white/10 text-gray-600 dark:text-gray-300 p-2.5 rounded-xl shadow-md hover:bg-white/60 dark:hover:bg-white/10 transition-all duration-200 hover:scale-105"
                    >
                        {theme === 'light' ? (
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                            </svg>
                        ) : (
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                            </svg>
                        )}
                    </button>
                    {/* Sign In */}
                    <button
                        onClick={onGetStarted}
                        className="backdrop-blur-md bg-blue-600/90 hover:bg-blue-700/90 dark:bg-blue-500/80 dark:hover:bg-blue-500 border border-blue-400/30 text-white px-5 py-2.5 rounded-xl font-semibold text-sm shadow-lg shadow-blue-500/25 transition-all duration-200 hover:shadow-blue-500/40 hover:scale-105"
                    >
                        Sign In
                    </button>
                </div>
            </nav>

            {/* ── Hero ────────────────────────────────────────────────────────────────── */}
            <section className="relative z-10 flex flex-col items-center text-center px-6 pt-16 pb-20 md:pt-24 md:pb-28">
                <div className="backdrop-blur-sm bg-white/30 dark:bg-white/5 border border-white/50 dark:border-white/10 rounded-2xl px-4 py-1.5 mb-6 shadow-sm">
                    <span className="text-xs font-semibold text-indigo-700 dark:text-indigo-300 uppercase tracking-widest">AI-Powered PDF Assistant</span>
                </div>
                <h1 className="text-5xl md:text-7xl font-extrabold text-gray-900 dark:text-white leading-tight mb-6 max-w-4xl">
                    Chat with your{' '}
                    <span className="bg-gradient-to-r from-blue-600 to-violet-600 dark:from-blue-400 dark:to-violet-400 bg-clip-text text-transparent">
                        PDFs
                    </span>{' '}
                    using AI
                </h1>
                <p className="text-lg md:text-xl text-gray-600 dark:text-gray-400 max-w-2xl mb-10 leading-relaxed">
                    Upload any PDF, ask questions in natural language, and get grounded answers with page citations — powered by multi-pipeline RAG.
                </p>
                <div className="flex flex-col sm:flex-row gap-4">
                    <button
                        onClick={onGetStarted}
                        className="group relative overflow-hidden backdrop-blur-md bg-gradient-to-r from-blue-600 to-violet-600 hover:from-blue-500 hover:to-violet-500 text-white px-8 py-4 rounded-2xl font-bold text-lg shadow-2xl shadow-blue-500/30 hover:shadow-blue-500/50 transition-all duration-300 hover:scale-105 border border-white/20"
                    >
                        <span className="relative z-10 flex items-center gap-2">
                            Get Started Free
                            <svg className="w-5 h-5 group-hover:translate-x-1 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 8l4 4m0 0l-4 4m4-4H3" />
                            </svg>
                        </span>
                        <div className="absolute inset-0 bg-white/10 opacity-0 group-hover:opacity-100 transition-opacity" />
                    </button>
                    <button
                        onClick={onGetStarted}
                        className="backdrop-blur-md bg-white/40 dark:bg-white/5 hover:bg-white/60 dark:hover:bg-white/10 border border-white/60 dark:border-white/15 text-gray-800 dark:text-white px-8 py-4 rounded-2xl font-semibold text-lg shadow-lg transition-all duration-200"
                    >
                        View Demo
                    </button>
                </div>
            </section>

            {/* ── Stats ───────────────────────────────────────────────────────────────── */}
            <section ref={statsRef} className="relative z-10 px-6 md:px-12 pb-20">
                <div className="max-w-4xl mx-auto">
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-5">
                        {/* Users */}
                        <div className="group backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl p-7 shadow-xl shadow-black/5 hover:shadow-2xl hover:bg-white/65 dark:hover:bg-white/8 transition-all duration-300 hover:-translate-y-1">
                            <div className="w-12 h-12 bg-gradient-to-br from-blue-400 to-blue-600 rounded-2xl flex items-center justify-center mb-4 shadow-lg shadow-blue-500/30 group-hover:scale-110 transition-transform">
                                <svg className="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
                                </svg>
                            </div>
                            <div className="text-4xl font-black text-gray-900 dark:text-white mb-1 tabular-nums">
                                {metrics ? formatNumber(users) : '—'}
                            </div>
                            <div className="text-sm font-medium text-gray-500 dark:text-gray-400">Total Users</div>
                        </div>

                        {/* Documents */}
                        <div className="group backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl p-7 shadow-xl shadow-black/5 hover:shadow-2xl hover:bg-white/65 dark:hover:bg-white/8 transition-all duration-300 hover:-translate-y-1">
                            <div className="w-12 h-12 bg-gradient-to-br from-violet-400 to-violet-600 rounded-2xl flex items-center justify-center mb-4 shadow-lg shadow-violet-500/30 group-hover:scale-110 transition-transform">
                                <svg className="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                                </svg>
                            </div>
                            <div className="text-4xl font-black text-gray-900 dark:text-white mb-1 tabular-nums">
                                {metrics ? formatNumber(docs) : '—'}
                            </div>
                            <div className="text-sm font-medium text-gray-500 dark:text-gray-400">Documents Processed</div>
                        </div>

                        {/* Responses */}
                        <div className="group backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl p-7 shadow-xl shadow-black/5 hover:shadow-2xl hover:bg-white/65 dark:hover:bg-white/8 transition-all duration-300 hover:-translate-y-1">
                            <div className="w-12 h-12 bg-gradient-to-br from-emerald-400 to-emerald-600 rounded-2xl flex items-center justify-center mb-4 shadow-lg shadow-emerald-500/30 group-hover:scale-110 transition-transform">
                                <svg className="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                                </svg>
                            </div>
                            <div className="text-4xl font-black text-gray-900 dark:text-white mb-1 tabular-nums">
                                {metrics ? formatNumber(responses) : '—'}
                            </div>
                            <div className="text-sm font-medium text-gray-500 dark:text-gray-400">AI Responses Generated</div>
                        </div>
                    </div>
                </div>
            </section>

            {/* ── Pipeline Feature Cards ───────────────────────────────────────────────── */}
            <section className="relative z-10 px-6 md:px-12 pb-24">
                <div className="max-w-5xl mx-auto">
                    <div className="text-center mb-10">
                        <h2 className="text-3xl md:text-4xl font-bold text-gray-900 dark:text-white mb-3">Three AI Pipelines</h2>
                        <p className="text-gray-500 dark:text-gray-400">Compare how different retrieval strategies answer your questions</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-5">

                        {/* Standard */}
                        <div className="backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl p-6 shadow-xl hover:shadow-2xl hover:-translate-y-1 transition-all duration-300 group">
                            <div className="w-10 h-10 bg-gradient-to-br from-amber-400 to-orange-500 rounded-xl flex items-center justify-center mb-4 shadow-md shadow-amber-400/30 group-hover:scale-110 transition-transform">
                                <span className="text-lg">⚡</span>
                            </div>
                            <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-2">Standard</h3>
                            <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">Fast keyword search with BM25 ranking. Perfect for most everyday queries on your PDF content.</p>
                            <div className="mt-4 flex flex-wrap gap-1.5">
                                {['BM25 Search', 'Fast', 'Free'].map(t => (
                                    <span key={t} className="text-[10px] font-semibold px-2 py-0.5 bg-amber-100/80 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300 rounded-full border border-amber-200/50 dark:border-amber-700/30">{t}</span>
                                ))}
                            </div>
                        </div>

                        {/* Smart */}
                        <div className="backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl p-6 shadow-xl hover:shadow-2xl hover:-translate-y-1 transition-all duration-300 group">
                            <div className="w-10 h-10 bg-gradient-to-br from-emerald-400 to-teal-500 rounded-xl flex items-center justify-center mb-4 shadow-md shadow-emerald-400/30 group-hover:scale-110 transition-transform">
                                <span className="text-lg">🧠</span>
                            </div>
                            <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-2">Smart RAG</h3>
                            <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">TextRank summarisation, chunk deduplication, and boilerplate stripping save tokens while improving quality.</p>
                            <div className="mt-4 flex flex-wrap gap-1.5">
                                {['TextRank', 'Token Saving', 'Dedup'].map(t => (
                                    <span key={t} className="text-[10px] font-semibold px-2 py-0.5 bg-emerald-100/80 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300 rounded-full border border-emerald-200/50 dark:border-emerald-700/30">{t}</span>
                                ))}
                            </div>
                        </div>

                        {/* Enterprise */}
                        <div className="backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl p-6 shadow-xl hover:shadow-2xl hover:-translate-y-1 transition-all duration-300 group">
                            <div className="w-10 h-10 bg-gradient-to-br from-violet-400 to-purple-600 rounded-xl flex items-center justify-center mb-4 shadow-md shadow-violet-400/30 group-hover:scale-110 transition-transform">
                                <span className="text-lg">🚀</span>
                            </div>
                            <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-2">Enterprise</h3>
                            <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">Hybrid vector + BM25 search, cross-encoder reranking, and grounded citations. Production-grade accuracy.</p>
                            <div className="mt-4 flex flex-wrap gap-1.5">
                                {['Hybrid Search', 'Reranking', 'Citations'].map(t => (
                                    <span key={t} className="text-[10px] font-semibold px-2 py-0.5 bg-violet-100/80 dark:bg-violet-900/30 text-violet-700 dark:text-violet-300 rounded-full border border-violet-200/50 dark:border-violet-700/30">{t}</span>
                                ))}
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            {/* ── Footer ──────────────────────────────────────────────────────────────── */}
            <footer className="relative z-10 px-6 md:px-12 py-8 border-t border-white/30 dark:border-white/5">
                <div className="backdrop-blur-md bg-white/30 dark:bg-white/3 rounded-2xl px-6 py-4 flex flex-col sm:flex-row items-center justify-between gap-3 max-w-5xl mx-auto">
                    <div className="flex items-center gap-2">
                        <div className="w-6 h-6 bg-gradient-to-br from-blue-500 to-violet-600 rounded-md flex items-center justify-center">
                            <svg className="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                            </svg>
                        </div>
                        <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">AskMyPDF</span>
                    </div>
                    <p className="text-xs text-gray-500 dark:text-gray-500">© 2026 AskMyPDF · AI-powered PDF conversations</p>
                    <button onClick={onGetStarted} className="text-xs font-semibold text-blue-600 dark:text-blue-400 hover:underline">
                        Get Started →
                    </button>
                </div>
            </footer>
        </div>
    );
}
