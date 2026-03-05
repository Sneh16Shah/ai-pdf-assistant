import { useState, useEffect } from 'react';
import { SmartTokenStats, getSmartStats, SmartSessionStats } from '../services/api';

export type PipelineMode = 'standard' | 'smart' | 'enterprise' | 'compare';

interface PipelineModeSelectorProps {
    sessionId: string;
    mode: PipelineMode;
    onModeChange: (mode: PipelineMode) => void;
    lastTokenStats?: SmartTokenStats | null;
}

const MODES: { id: PipelineMode; label: string; icon: string; description: string; color: string }[] = [
    {
        id: 'standard',
        label: 'Standard',
        icon: '⚡',
        description: 'Basic keyword search',
        color: 'gray',
    },
    {
        id: 'smart',
        label: 'Smart',
        icon: '🧠',
        description: 'BM25 + TextRank',
        color: 'emerald',
    },
    {
        id: 'enterprise',
        label: 'Enterprise',
        icon: '🚀',
        description: 'Hybrid + Rerank + Citations',
        color: 'purple',
    },
    {
        id: 'compare',
        label: 'Compare All',
        icon: '⚖️',
        description: 'Run all 3 side-by-side',
        color: 'amber',
    },
];

const COLOR_CLASSES: Record<string, { active: string; text: string; ring: string }> = {
    gray: { active: 'bg-gray-600 text-white', text: 'text-gray-600 dark:text-gray-400', ring: 'ring-gray-400' },
    emerald: { active: 'bg-emerald-600 text-white', text: 'text-emerald-600 dark:text-emerald-400', ring: 'ring-emerald-400' },
    purple: { active: 'bg-purple-600 text-white', text: 'text-purple-600 dark:text-purple-400', ring: 'ring-purple-400' },
    amber: { active: 'bg-amber-500 text-white', text: 'text-amber-600 dark:text-amber-400', ring: 'ring-amber-400' },
};

export default function PipelineModeSelector({
    sessionId,
    mode,
    onModeChange,
    lastTokenStats,
}: PipelineModeSelectorProps) {
    const [sessionStats, setSessionStats] = useState<SmartSessionStats | null>(null);

    useEffect(() => {
        if (mode === 'smart' && sessionId && lastTokenStats) {
            getSmartStats(sessionId)
                .then(setSessionStats)
                .catch(() => { /* ignore */ });
        }
    }, [lastTokenStats, sessionId, mode]);

    return (
        <div className="flex flex-col gap-1.5 px-3 py-2 border-b dark:border-gray-700 bg-gray-50/50 dark:bg-gray-900/50">
            {/* Mode Tabs */}
            <div className="grid grid-cols-4 gap-1 bg-gray-200 dark:bg-gray-700 rounded-lg p-0.5">
                {MODES.map((m) => {
                    const isActive = mode === m.id;
                    const colors = COLOR_CLASSES[m.color];
                    return (
                        <button
                            key={m.id}
                            onClick={() => onModeChange(m.id)}
                            title={m.description}
                            className={`flex flex-col items-center justify-center py-1 px-0.5 rounded-md text-[10px] font-medium transition-all duration-150 ${isActive
                                    ? `${colors.active} shadow-sm`
                                    : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
                                }`}
                            id={`pipeline-mode-${m.id}`}
                        >
                            <span className="text-sm leading-none mb-0.5">{m.icon}</span>
                            <span className="leading-none">{m.label}</span>
                        </button>
                    );
                })}
            </div>

            {/* Active mode description */}
            <div className="flex items-center justify-between">
                <span className="text-[10px] text-gray-400 dark:text-gray-500 italic">
                    {MODES.find(m => m.id === mode)?.description}
                </span>
                {/* Token savings bar for smart mode */}
                {mode === 'smart' && lastTokenStats && lastTokenStats.savings_percent > 0 && (
                    <span className="text-[10px] text-emerald-600 dark:text-emerald-400 font-mono">
                        {lastTokenStats.savings_percent.toFixed(0)}% tokens saved
                    </span>
                )}
                {mode === 'smart' && sessionStats && sessionStats.total_queries > 1 && (
                    <span className="text-[10px] text-gray-400 dark:text-gray-500">
                        {sessionStats.total_saved_tokens.toLocaleString()} saved total
                    </span>
                )}
            </div>
        </div>
    );
}
