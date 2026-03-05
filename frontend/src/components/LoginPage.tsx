import { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { useTheme } from '../contexts/ThemeContext';

interface LoginPageProps {
    onGoHome?: () => void;
}

export default function LoginPage({ onGoHome }: LoginPageProps) {
    const [isLogin, setIsLogin] = useState(true);
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [name, setName] = useState('');
    const [error, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);

    const { login, register } = useAuth();
    const { theme, toggleTheme } = useTheme();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);
        setLoading(true);

        try {
            if (isLogin) {
                await login(email, password);
            } else {
                if (password.length < 6) {
                    setError('Password must be at least 6 characters');
                    setLoading(false);
                    return;
                }
                await register(email, password, name);
            }
        } catch (err: any) {
            setError(err.response?.data?.error || 'An error occurred');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen bg-gradient-to-br from-blue-50 via-indigo-50 to-violet-50 dark:from-gray-950 dark:via-indigo-950 dark:to-slate-900 relative overflow-hidden">

            {/* Background orbs — same as landing page */}
            <div className="pointer-events-none absolute inset-0 overflow-hidden">
                <div className="absolute -top-40 -left-40 w-96 h-96 bg-blue-400/20 dark:bg-blue-600/15 rounded-full blur-3xl" />
                <div className="absolute top-1/3 -right-40 w-80 h-80 bg-violet-400/20 dark:bg-violet-600/15 rounded-full blur-3xl" />
                <div className="absolute bottom-0 left-1/3 w-72 h-72 bg-indigo-400/20 dark:bg-indigo-600/10 rounded-full blur-3xl" />
            </div>

            {/* ── Nav — same style as landing page ── */}
            <nav className="relative z-10 flex items-center justify-between px-6 md:px-12 py-5">
                {/* Logo */}
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

                    {/* Home button */}
                    {onGoHome && (
                        <button
                            onClick={onGoHome}
                            className="backdrop-blur-md bg-white/40 dark:bg-white/5 border border-white/60 dark:border-white/10 text-gray-700 dark:text-gray-200 px-5 py-2.5 rounded-xl font-semibold text-sm shadow-md hover:bg-white/60 dark:hover:bg-white/10 transition-all duration-200 hover:scale-105 flex items-center gap-2"
                        >
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
                            </svg>
                            Home
                        </button>
                    )}
                </div>
            </nav>

            {/* ── Main content ── */}
            <div className="relative z-10 flex items-center justify-center px-4 pt-8 pb-16 min-h-[calc(100vh-88px)]">
                <div className="w-full max-w-md">

                    {/* Headline */}
                    <div className="text-center mb-8">
                        <div className="inline-block backdrop-blur-sm bg-white/30 dark:bg-white/5 border border-white/50 dark:border-white/10 rounded-2xl px-4 py-1.5 mb-4 shadow-sm">
                            <span className="text-xs font-semibold text-indigo-700 dark:text-indigo-300 uppercase tracking-widest">
                                {isLogin ? 'Welcome Back' : 'Get Started Free'}
                            </span>
                        </div>
                        <h1 className="text-4xl font-extrabold text-gray-900 dark:text-white mb-2">
                            {isLogin ? 'Sign in to ' : 'Join '}
                            <span className="bg-gradient-to-r from-blue-600 to-violet-600 dark:from-blue-400 dark:to-violet-400 bg-clip-text text-transparent">
                                AskMyPDF
                            </span>
                        </h1>
                        <p className="text-gray-500 dark:text-gray-400 text-sm">
                            {isLogin
                                ? 'Chat with your PDF documents using AI'
                                : 'Create your account and start chatting with PDFs'}
                        </p>
                    </div>

                    {/* Glass Card */}
                    <div className="backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl shadow-2xl shadow-black/10 dark:shadow-black/30 p-8">

                        {/* Tab switcher */}
                        <div className="flex mb-7 backdrop-blur-md bg-white/40 dark:bg-white/5 border border-white/60 dark:border-white/10 rounded-2xl p-1 shadow-inner">
                            <button
                                onClick={() => setIsLogin(true)}
                                className={`flex-1 py-2.5 px-4 rounded-xl text-sm font-semibold transition-all duration-200 ${isLogin
                                        ? 'bg-gradient-to-r from-blue-600 to-violet-600 text-white shadow-lg shadow-blue-500/25'
                                        : 'text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
                                    }`}
                            >
                                Login
                            </button>
                            <button
                                onClick={() => setIsLogin(false)}
                                className={`flex-1 py-2.5 px-4 rounded-xl text-sm font-semibold transition-all duration-200 ${!isLogin
                                        ? 'bg-gradient-to-r from-blue-600 to-violet-600 text-white shadow-lg shadow-blue-500/25'
                                        : 'text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
                                    }`}
                            >
                                Sign Up
                            </button>
                        </div>

                        {/* Form */}
                        <form onSubmit={handleSubmit} className="space-y-4">
                            {!isLogin && (
                                <div>
                                    <label htmlFor="name" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                                        Name <span className="text-gray-400 font-normal">(optional)</span>
                                    </label>
                                    <input
                                        type="text"
                                        id="name"
                                        value={name}
                                        onChange={(e) => setName(e.target.value)}
                                        className="w-full px-4 py-3 rounded-xl bg-white/60 dark:bg-white/5 border border-white/70 dark:border-white/10 backdrop-blur-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-400/50 transition-all duration-200 shadow-sm"
                                        placeholder="Your name"
                                    />
                                </div>
                            )}

                            <div>
                                <label htmlFor="email" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                                    Email
                                </label>
                                <input
                                    type="email"
                                    id="email"
                                    value={email}
                                    onChange={(e) => setEmail(e.target.value)}
                                    required
                                    className="w-full px-4 py-3 rounded-xl bg-white/60 dark:bg-white/5 border border-white/70 dark:border-white/10 backdrop-blur-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-400/50 transition-all duration-200 shadow-sm"
                                    placeholder="you@example.com"
                                />
                            </div>

                            <div>
                                <label htmlFor="password" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                                    Password
                                </label>
                                <input
                                    type="password"
                                    id="password"
                                    value={password}
                                    onChange={(e) => setPassword(e.target.value)}
                                    required
                                    minLength={6}
                                    className="w-full px-4 py-3 rounded-xl bg-white/60 dark:bg-white/5 border border-white/70 dark:border-white/10 backdrop-blur-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-400/50 transition-all duration-200 shadow-sm"
                                    placeholder={isLogin ? '••••••••' : 'At least 6 characters'}
                                />
                            </div>

                            {error && (
                                <div className="p-3 backdrop-blur-sm bg-red-50/80 dark:bg-red-900/20 border border-red-300/50 dark:border-red-700/30 text-red-700 dark:text-red-400 rounded-xl text-sm">
                                    {error}
                                </div>
                            )}

                            <button
                                type="submit"
                                disabled={loading}
                                className="group relative w-full py-3.5 px-4 overflow-hidden bg-gradient-to-r from-blue-600 to-violet-600 hover:from-blue-500 hover:to-violet-500 disabled:from-blue-400 disabled:to-violet-400 text-white font-semibold rounded-xl shadow-xl shadow-blue-500/30 hover:shadow-blue-500/50 transition-all duration-300 hover:scale-[1.02] disabled:scale-100 border border-white/20"
                            >
                                <span className="relative z-10 flex items-center justify-center gap-2">
                                    {loading ? (
                                        <>
                                            <svg className="animate-spin h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                                            </svg>
                                            {isLogin ? 'Signing in...' : 'Creating account...'}
                                        </>
                                    ) : (
                                        <>
                                            {isLogin ? 'Sign In' : 'Create Account'}
                                            <svg className="w-4 h-4 group-hover:translate-x-1 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 8l4 4m0 0l-4 4m4-4H3" />
                                            </svg>
                                        </>
                                    )}
                                </span>
                                <div className="absolute inset-0 bg-white/10 opacity-0 group-hover:opacity-100 transition-opacity" />
                            </button>
                        </form>

                        {/* Footer link */}
                        <div className="mt-6 pt-5 border-t border-white/40 dark:border-white/10">
                            <p className="text-sm text-gray-500 dark:text-gray-400 text-center">
                                {isLogin ? "Don't have an account? " : 'Already have an account? '}
                                <button
                                    onClick={() => { setIsLogin(!isLogin); setError(null); }}
                                    className="font-semibold text-blue-600 dark:text-blue-400 hover:text-violet-600 dark:hover:text-violet-400 transition-colors"
                                >
                                    {isLogin ? 'Sign up free' : 'Sign in'}
                                </button>
                            </p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
