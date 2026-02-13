import { useState, useEffect } from 'react';
import { getUserSessions, deleteUserSession, UserSession } from '../services/api';

interface DashboardProps {
    onResumeSession?: (sessionId: string) => void;
}

export default function Dashboard({ onResumeSession }: DashboardProps) {
    const [sessions, setSessions] = useState<UserSession[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

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

    const handleDelete = async (sessionId: string) => {
        if (!confirm('Delete this session and all its data?')) return;
        try {
            await deleteUserSession(sessionId);
            setSessions(prev => prev.filter(s => s.id !== sessionId));
        } catch {
            setError('Failed to delete session');
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
        <div className="max-w-4xl mx-auto p-6">
            <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Your Sessions</h2>
                <button
                    onClick={loadSessions}
                    className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                >
                    Refresh
                </button>
            </div>

            {error && (
                <div className="mb-4 p-3 bg-red-100 dark:bg-red-900/30 border border-red-400 dark:border-red-600 text-red-700 dark:text-red-400 rounded-lg text-sm">
                    {error}
                </div>
            )}

            {sessions.length === 0 ? (
                <div className="text-center py-16">
                    <svg className="w-16 h-16 mx-auto text-gray-400 dark:text-gray-600 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                    </svg>
                    <h3 className="text-lg font-medium text-gray-600 dark:text-gray-400 mb-2">
                        No sessions yet
                    </h3>
                    <p className="text-gray-500 dark:text-gray-500">
                        Upload a PDF to start your first session
                    </p>
                </div>
            ) : (
                <div className="space-y-3">
                    {sessions.map(session => (
                        <div
                            key={session.id}
                            className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-shadow"
                        >
                            <div className="flex items-start justify-between">
                                <div className="flex-1 min-w-0">
                                    <h3 className="font-medium text-gray-900 dark:text-white truncate">
                                        {session.title || 'Untitled Session'}
                                    </h3>
                                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                                        {formatDate(session.last_activity)}
                                    </p>
                                    {session.documents && session.documents.length > 0 && (
                                        <div className="flex flex-wrap gap-2 mt-2">
                                            {session.documents.map(doc => (
                                                <span
                                                    key={doc.id}
                                                    className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400"
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
                                            className="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                                        >
                                            Resume
                                        </button>
                                    )}
                                    <button
                                        onClick={() => handleDelete(session.id)}
                                        className="p-1.5 text-gray-400 hover:text-red-500 dark:hover:text-red-400 transition-colors"
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
        </div>
    );
}
