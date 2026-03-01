import { useState, useEffect } from 'react';
import { SmartTokenStats, getSmartStats, SmartSessionStats } from '../services/api';

interface SmartModeToggleProps {
    sessionId: string;
    isSmartMode: boolean;
    onToggle: (enabled: boolean) => void;
    lastTokenStats?: SmartTokenStats | null;
}

export default function SmartModeToggle({
    sessionId,
    isSmartMode,
    onToggle,
    lastTokenStats,
}: SmartModeToggleProps) {
    const [sessionStats, setSessionStats] = useState<SmartSessionStats | null>(null);

    // Refresh session stats when we get new token stats
    useEffect(() => {
        if (isSmartMode && sessionId && lastTokenStats) {
            getSmartStats(sessionId)
                .then(setSessionStats)
                .catch(() => { /* ignore errors */ });
        }
    }, [lastTokenStats, sessionId, isSmartMode]);

    return (
        <div className="flex flex-col gap-1 px-4 py-2 border-b dark:border-gray-700 bg-gray-50/50 dark:bg-gray-900/50">
            {/* Toggle Row */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <button
                        onClick={() => onToggle(!isSmartMode)}
                        className={`relative inline-flex h-5 w-10 items-center rounded-full transition-colors ${isSmartMode
                                ? 'bg-emerald-500'
                                : 'bg-gray-300 dark:bg-gray-600'
                            }`}
                        id="smart-mode-toggle"
                    >
                        <span
                            className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform shadow-sm ${isSmartMode ? 'translate-x-5' : 'translate-x-1'
                                }`}
                        />
                    </button>
                    <span className="text-xs font-medium text-gray-700 dark:text-gray-300">
                        {isSmartMode ? '🧠 Smart Mode' : '⚡ Standard'}
                    </span>
                </div>
                {isSmartMode && (
                    <span className="text-[10px] text-emerald-600 dark:text-emerald-400 font-medium">
                        BM25 + TextRank
                    </span>
                )}
            </div>

            {/* Token Stats Bar (shown after first smart response) */}
            {isSmartMode && lastTokenStats && lastTokenStats.savings_percent > 0 && (
                <div className="flex items-center gap-2 text-[10px]">
                    <div className="flex-1 bg-gray-200 dark:bg-gray-700 rounded-full h-1.5 overflow-hidden">
                        <div
                            className="h-full bg-gradient-to-r from-emerald-500 to-green-400 rounded-full transition-all duration-500"
                            style={{ width: `${Math.min(lastTokenStats.savings_percent, 100)}%` }}
                        />
                    </div>
                    <span className="text-emerald-600 dark:text-emerald-400 font-mono whitespace-nowrap">
                        {lastTokenStats.raw_tokens.toLocaleString()}→{lastTokenStats.final_tokens.toLocaleString()} tokens
                        ({lastTokenStats.savings_percent.toFixed(0)}% saved)
                    </span>
                </div>
            )}

            {/* Cumulative session stats */}
            {isSmartMode && sessionStats && sessionStats.total_queries > 1 && (
                <div className="text-[10px] text-gray-500 dark:text-gray-400">
                    Session total: {sessionStats.total_saved_tokens.toLocaleString()} tokens saved
                    across {sessionStats.total_queries} queries
                    ({sessionStats.avg_savings_percent.toFixed(0)}% avg)
                </div>
            )}
        </div>
    );
}
