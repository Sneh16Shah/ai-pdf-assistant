import { useState, useEffect } from 'react';
import { getUserSessions, deleteUserSession, UserSession } from '../services/api';

interface DashboardProps {
    onResumeSession?: (sessionId: string) => void;
}

export default function Dashboard({ onResumeSession }: DashboardProps) {
    const [sessions, setSessions] = useState<UserSession[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
    const [deleting, setDeleting] = useState(false);

    useEffect(() => {
        loadSessions();
    }, []);

    const loadSessions = async () => {
        try {
            setLoading(true);
            const data = await getUserSessions();
            setSessions(data || []);
        } catch {
            setError('Failed to load sessions');
        } finally {
            setLoading(false);
        }
    };

    const handleDelete = async () => {
        if (!deleteTarget) return;
        setDeleting(true);
        try {
            await deleteUserSession(deleteTarget);
            setSessions(prev => prev.filter(s => s.id !== deleteTarget));
        } catch {
            setError('Failed to delete session');
        } finally {
            setDeleting(false);
            setDeleteTarget(null);
        }
    };

    const formatDate = (dateStr: string) => {
        const date = new Date(dateStr);
        const now = new Date();
        const diff = now.getTime() - date.getTime();
        const days = Math.floor(diff / (1000 * 60 * 60 * 24));

        if (days === 0) return 'Today';
        if (days === 1) return 'Yesterday';
        if (days < 7) return `${days} days ago`;
        return date.toLocaleDateString();
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    return (
        <div className="max-w-4xl mx-auto p-6 md:p-8">
            {/* Header */}
            <div className="flex items-center justify-between mb-8">
                <div>
                    <h2 className="text-2xl md:text-3xl font-bold text-gray-900 dark:text-white">Your Sessions</h2>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Resume any previous PDF conversation</p>
                </div>
                <button
                    onClick={loadSessions}
                    className="backdrop-blur-md bg-white/50 dark:bg-white/5 border border-white/60 dark:border-white/10 text-blue-600 dark:text-blue-400 px-4 py-2 rounded-xl text-sm font-semibold shadow-sm hover:bg-white/70 dark:hover:bg-white/10 transition-all duration-200 hover:scale-105"
                >
                    Refresh
                </button>
            </div>

            {error && (
                <div className="mb-4 p-3 backdrop-blur-sm bg-red-50/80 dark:bg-red-900/20 border border-red-300/50 dark:border-red-700/30 text-red-700 dark:text-red-400 rounded-2xl text-sm">
                    {error}
                </div>
            )}

            {sessions.length === 0 ? (
                <div className="text-center py-20">
                    <div className="w-20 h-20 mx-auto mb-5 backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl flex items-center justify-center shadow-xl">
                        <svg className="w-9 h-9 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                        </svg>
                    </div>
                    <h3 className="text-lg font-semibold text-gray-700 dark:text-gray-300 mb-2">No sessions yet</h3>
                    <p className="text-gray-500 dark:text-gray-500 text-sm">Upload a PDF to start your first session</p>
                </div>
            ) : (
                <div className="space-y-3">
                    {sessions.map(session => (
                        <div
                            key={session.id}
                            className="group backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl p-5 shadow-xl shadow-black/5 hover:shadow-2xl hover:bg-white/65 dark:hover:bg-white/8 transition-all duration-300 hover:-translate-y-0.5"
                        >
                            <div className="flex items-start justify-between">
                                <div className="flex-1 min-w-0">
                                    <h3 className="font-semibold text-gray-900 dark:text-white truncate text-base">
                                        {session.title || 'Untitled Session'}
                                    </h3>
                                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
                                        {formatDate(session.last_activity)}
                                    </p>
                                    {session.documents && session.documents.length > 0 && (
                                        <div className="flex flex-wrap gap-2 mt-2.5">
                                            {session.documents.map(doc => (
                                                <span
                                                    key={doc.id}
                                                    className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100/80 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border border-blue-200/50 dark:border-blue-700/30"
                                                >
                                                    <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fillRule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clipRule="evenodd" />
                                                    </svg>
                                                    {doc.filename}
                                                </span>
                                            ))}
                                        </div>
                                    )}
                                </div>
                                <div className="flex items-center space-x-2 ml-4">
                                    {onResumeSession && (
                                        <button
                                            onClick={() => onResumeSession(session.id)}
                                            className="group/btn relative overflow-hidden px-4 py-2 text-sm font-semibold bg-gradient-to-r from-blue-600 to-violet-600 hover:from-blue-500 hover:to-violet-500 text-white rounded-xl shadow-lg shadow-blue-500/25 hover:shadow-blue-500/40 transition-all duration-200 hover:scale-105 border border-white/20"
                                        >
                                            <span className="relative z-10 flex items-center gap-1.5">
                                                Resume
                                                <svg className="w-3.5 h-3.5 group-hover/btn:translate-x-0.5 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 8l4 4m0 0l-4 4m4-4H3" />
                                                </svg>
                                            </span>
                                        </button>
                                    )}
                                    <button
                                        onClick={() => setDeleteTarget(session.id)}
                                        className="p-2 text-gray-400 hover:text-red-500 dark:hover:text-red-400 hover:bg-red-50/60 dark:hover:bg-red-900/20 rounded-xl transition-all duration-200"
                                        title="Delete session"
                                    >
                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                        </svg>
                                    </button>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Delete Confirmation Modal */}
            {deleteTarget && (
                <div className="fixed inset-0 z-50 flex items-center justify-center">
                    <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => !deleting && setDeleteTarget(null)} />
                    <div className="relative backdrop-blur-xl bg-white/80 dark:bg-gray-900/80 border border-white/60 dark:border-white/10 rounded-3xl shadow-2xl p-6 max-w-sm mx-4">
                        <div className="flex items-center space-x-3 mb-4">
                            <div className="p-2.5 bg-red-100/80 dark:bg-red-900/30 rounded-2xl">
                                <svg className="w-5 h-5 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                </svg>
                            </div>
                            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Delete Session</h3>
                        </div>
                        <p className="text-sm text-gray-600 dark:text-gray-400 mb-6">
                            Are you sure you want to delete this session? All chat history and uploaded documents will be permanently removed.
                        </p>
                        <div className="flex justify-end space-x-3">
                            <button
                                onClick={() => setDeleteTarget(null)}
                                disabled={deleting}
                                className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 backdrop-blur-sm bg-white/60 dark:bg-white/5 border border-white/60 dark:border-white/10 rounded-xl hover:bg-white/80 transition-all duration-200 disabled:opacity-50"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleDelete}
                                disabled={deleting}
                                className="px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 rounded-xl transition-colors disabled:opacity-50 flex items-center space-x-1.5 shadow-lg shadow-red-500/25"
                            >
                                {deleting ? (
                                    <>
                                        <div className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                                        <span>Deleting...</span>
                                    </>
                                ) : (
                                    <span>Delete</span>
                                )}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
