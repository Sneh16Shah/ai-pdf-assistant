import { useState, useEffect, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import { sendMessage, sendSmartMessage, sendEnterpriseMessage, compareAllPipelines, Citation, SmartTokenStats, getSessionMessages } from '../services/api';
import PipelineModeSelector, { PipelineMode } from './PipelineModeSelector';

// Mirrors the flag in PipelineModeSelector — when advanced pipelines are hidden,
// force Standard so we never call the gated endpoints.
const ADVANCED_PIPELINES_ENABLED =
    import.meta.env.VITE_ADVANCED_PIPELINES === 'true';

// Extend the API's ChatMessage with optional per-message citations
interface MessageWithCitations {
    role: 'user' | 'assistant';
    content: string;
    citations?: Citation[];
    // For compare mode results
    compareResults?: {
        pipeline: string;
        response: string;
        latency_ms: number;
        stats?: any;
        citations?: Citation[];
    }[];
}

interface ExplanationPanelProps {
    sessionId: string;
    selectedText: string | null;
    pageNumber: number | null;
    isOpen: boolean;
    onClose: () => void;
    onGoToPage?: (page: number) => void;
    onTextConsumed?: () => void;
}

export default function ExplanationPanel({
    sessionId,
    selectedText,
    pageNumber,
    isOpen,
    onClose,
    onGoToPage,
    onTextConsumed,
}: ExplanationPanelProps) {
    const [messages, setMessages] = useState<MessageWithCitations[]>([]);
    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);
    const [generatedImage, setGeneratedImage] = useState<{ base64: string; mimeType: string } | null>(null);
    const [pipelineMode, setPipelineMode] = useState<PipelineMode>('standard');

    // Guard: if advanced pipelines are disabled, ignore any attempt to switch
    // off Standard. Belt-and-suspenders alongside the hidden selector.
    const effectiveMode: PipelineMode = ADVANCED_PIPELINES_ENABLED ? pipelineMode : 'standard';
    const [lastTokenStats, setLastTokenStats] = useState<SmartTokenStats | null>(null);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const previousTextRef = useRef<string>('');
    const loadedSessionRef = useRef<string>('');

    // Load previous chat history from DB when session changes
    // Note: citations are not persisted in DB currently (they require a schema change).
    // They are preserved within the current session as they are stored per-message.
    useEffect(() => {
        if (sessionId && sessionId !== loadedSessionRef.current) {
            loadedSessionRef.current = sessionId;
            getSessionMessages(sessionId)
                .then((dbMessages) => {
                    if (dbMessages && dbMessages.length > 0) {
                        const loaded: MessageWithCitations[] = dbMessages.map((m: { role: string; content: string }) => ({
                            role: m.role as 'user' | 'assistant',
                            content: m.content,
                            // citations not available from DB — would need schema change
                        }));
                        setMessages(loaded);
                    }
                })
                .catch(() => {
                    // No saved history or not authenticated — that's fine
                });
        }
    }, [sessionId]);

    // Shared function to send a chat message (used by both Ask AI and manual Send)
    const sendChatMessage = async (message: string) => {
        if (!message.trim() || loading) return;

        const userMessage: MessageWithCitations = { role: 'user', content: message };
        setMessages((prev) => [...prev, userMessage]);
        setInput('');
        setLoading(true);

        try {
            if (effectiveMode === 'compare') {
                // Handle compare mode specially
                const compareData = await compareAllPipelines(sessionId, message, pageNumber || undefined);

                const assistantMessage: MessageWithCitations = {
                    role: 'assistant',
                    content: "Here is how the three pipelines answered:",
                    compareResults: compareData.results
                };

                setMessages((prev) => [...prev, assistantMessage]);
                setGeneratedImage(null); // Diagrams not supported in compare mode yet
                setLoading(false);
                return;
            }

            let responseData;
            if (effectiveMode === 'smart') {
                const smartResp = await sendSmartMessage(sessionId, message, pageNumber || undefined);
                responseData = smartResp;
                if (smartResp.token_stats) {
                    setLastTokenStats(smartResp.token_stats);
                }
            } else if (effectiveMode === 'enterprise') {
                responseData = await sendEnterpriseMessage(sessionId, message, pageNumber || undefined);
            } else {
                responseData = await sendMessage(sessionId, message, pageNumber || undefined);
            }

            // Store citations on the assistant message — not as separate state
            const assistantMessage: MessageWithCitations = {
                role: 'assistant',
                content: responseData.response,
                citations: responseData.citations && responseData.citations.length > 0
                    ? responseData.citations
                    : undefined,
            };
            setMessages((prev) => [...prev, assistantMessage]);

            if (responseData.image_base64 && responseData.image_mime_type) {
                setGeneratedImage({ base64: responseData.image_base64, mimeType: responseData.image_mime_type });
            } else {
                setGeneratedImage(null);
            }
            setLoading(false);
        } catch (error: unknown) {
            const errorMessage = error instanceof Error ? error.message : 'Unknown error';
            setMessages((prev) => [
                ...prev,
                { role: 'assistant', content: `Error: ${errorMessage}` },
            ]);
            setLoading(false);
        }
    };

    useEffect(() => {
        // When text is selected via "Ask AI", auto-send it as a normal chat message
        if (selectedText && isOpen && selectedText !== previousTextRef.current) {
            previousTextRef.current = selectedText;
            const truncated = selectedText.length > 300 ? selectedText.substring(0, 300) + '...' : selectedText;
            const prompt = `"${truncated}"\n\nExplain this.`;
            sendChatMessage(prompt);
            // Clear the selectedText in parent so re-mounting won't re-send
            onTextConsumed?.();
        }
    }, [selectedText, isOpen, sessionId, pageNumber, loading]); // Added dependencies for sendChatMessage

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages]);




    const handleSendFollowUp = async (e: React.FormEvent) => {
        e.preventDefault();
        sendChatMessage(input);
    };

    const handleCitationClick = (page: number) => {
        if (onGoToPage) {
            onGoToPage(page);
        }
    };

    if (!isOpen) return null;

    return (
        <div className="flex flex-col h-full backdrop-blur-md bg-white/70 dark:bg-gray-900/60 border-l border-white/40 dark:border-white/10 shadow-2xl">
            {/* Header */}
            <div className="flex items-center justify-between px-3 md:px-4 py-2 md:py-3 border-b border-white/40 dark:border-white/10 backdrop-blur-sm bg-white/40 dark:bg-white/5">
                <div className="flex items-center space-x-2">
                    <div className="w-6 h-6 bg-gradient-to-br from-blue-500 to-violet-600 rounded-lg flex items-center justify-center shadow-md">
                        <svg className="w-3.5 h-3.5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                        </svg>
                    </div>
                    <h3 className="text-sm md:text-base font-semibold text-gray-800 dark:text-white">AI Assistant</h3>
                </div>
                <div className="flex items-center space-x-2">
                    <span className="text-xs text-gray-400 dark:text-gray-500 hidden md:inline">Ctrl+L to toggle</span>
                    <button
                        onClick={onClose}
                        className="p-1.5 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-white/60 dark:hover:bg-white/10 rounded-lg transition-all duration-200"
                    >
                        <svg className="w-4 h-4 md:w-5 md:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>
            </div>

            {/* Pipeline Mode Selector */}
            <PipelineModeSelector
                sessionId={sessionId}
                mode={pipelineMode}
                onModeChange={setPipelineMode}
                lastTokenStats={lastTokenStats}
            />

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-4 space-y-4">
                {/* Empty State */}
                {!messages.length && !loading && (
                    <div className="flex flex-col items-center justify-center h-full text-center">
                        <div className="w-16 h-16 mb-4 backdrop-blur-xl bg-white/50 dark:bg-white/5 border border-white/70 dark:border-white/10 rounded-3xl flex items-center justify-center shadow-xl">
                            <svg className="w-8 h-8 text-blue-500 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                            </svg>
                        </div>
                        <h4 className="text-base font-semibold text-gray-700 dark:text-gray-300 mb-1.5">
                            Ask anything about your PDF
                        </h4>
                        <p className="text-sm text-gray-500 dark:text-gray-400 max-w-xs">
                            Select text in the PDF and click "Ask AI" or type a question below.
                        </p>
                    </div>
                )}



                {/* AI-Generated Image */}
                {generatedImage && (
                    <div className="backdrop-blur-sm bg-white/50 dark:bg-white/5 border border-white/60 dark:border-white/10 rounded-2xl p-4">
                        <div className="flex items-center gap-2 mb-2">
                            <svg className="w-4 h-4 text-purple-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                            </svg>
                            <span className="text-xs font-medium text-purple-600 dark:text-purple-400">AI-Generated Diagram</span>
                        </div>
                        <img
                            src={`data:${generatedImage.mimeType};base64,${generatedImage.base64}`}
                            alt="AI-generated diagram"
                            className="w-full rounded-md border border-gray-200 dark:border-gray-600"
                        />
                        <button
                            onClick={() => {
                                const link = document.createElement('a');
                                link.href = `data:${generatedImage.mimeType};base64,${generatedImage.base64}`;
                                link.download = 'ai-diagram.png';
                                link.click();
                            }}
                            className="mt-2 text-xs text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
                        >
                            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                            </svg>
                            Download image
                        </button>
                    </div>
                )}

                {messages.map((message, index) => (
                    <div key={index}>
                        <div
                            className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
                        >
                            <div
                                className={`max-w-[100%] rounded-2xl p-3 ${message.role === 'user'
                                    ? 'bg-gradient-to-br from-blue-600 to-violet-600 text-white max-w-[90%] shadow-lg shadow-blue-500/20 border border-white/20'
                                    : 'bg-transparent w-full p-0'
                                    }`}
                            >
                                {message.role === 'assistant' ? (
                                    message.compareResults ? (
                                        <div className="w-full space-y-3 mt-2">
                                            <div className="text-sm font-medium text-gray-500 mb-3">{message.content}</div>
                                            <div className="grid grid-cols-1 md:grid-cols-3 gap-3 items-start">
                                                {message.compareResults.map((result, rIdx) => (
                                                    <div key={rIdx} className="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 border border-gray-200 dark:border-gray-600 relative overflow-hidden">
                                                        <div className="flex items-center justify-between mb-2 pb-2 border-b border-gray-200 dark:border-gray-600">
                                                            <div className="flex items-center gap-1.5">
                                                                <span className={`text-xs font-bold uppercase tracking-wider ${result.pipeline === 'enterprise' ? 'text-purple-600 dark:text-purple-400' :
                                                                    result.pipeline === 'smart' ? 'text-emerald-600 dark:text-emerald-400' :
                                                                        'text-gray-600 dark:text-gray-400'
                                                                    }`}>
                                                                    {result.pipeline === 'enterprise' ? '🚀 Enterprise' : result.pipeline === 'smart' ? '🧠 Smart' : '⚡ Standard'}
                                                                </span>
                                                            </div>
                                                            <span className="text-[10px] bg-white dark:bg-gray-800 px-1.5 py-0.5 rounded text-gray-500 font-mono border border-gray-200 dark:border-gray-700 shadow-sm">
                                                                {result.latency_ms}ms
                                                            </span>
                                                        </div>
                                                        <div className="prose prose-sm dark:prose-invert max-w-none text-sm">
                                                            <ReactMarkdown>{result.response}</ReactMarkdown>
                                                        </div>

                                                        {result.pipeline === 'enterprise' &&
                                                            result.stats?.hybrid_stats?.grounding_score > 0 && (
                                                                <div className="mt-2 text-[10px] text-purple-600 dark:text-purple-400 flex flex-wrap gap-2 border-t border-purple-100 dark:border-purple-900/30 pt-1.5">
                                                                    <span>Grounding: {result.stats.hybrid_stats.grounding_score.toFixed(2)}</span>
                                                                    {result.stats.hybrid_stats.vector_candidates > 0 && <span>Chunks: {result.stats.hybrid_stats.vector_candidates}</span>}
                                                                    {result.stats.hybrid_stats.reranked_count > 0 && <span>Reranked: {result.stats.hybrid_stats.reranked_count}</span>}
                                                                </div>
                                                            )}
                                                        {result.pipeline === 'smart' &&
                                                            (result.stats?.savings_percent > 0 || result.stats?.chunks_used > 0) && (
                                                                <div className="mt-2 text-[10px] text-emerald-600 dark:text-emerald-400 flex gap-3 border-t border-emerald-100 dark:border-emerald-900/30 pt-1.5">
                                                                    {result.stats.savings_percent > 0 && <span>Tokens Saved: {result.stats.savings_percent.toFixed(0)}%</span>}
                                                                    {result.stats.chunks_used > 0 && <span>Chunks: {result.stats.chunks_used}</span>}
                                                                </div>
                                                            )}

                                                        {/* Inline citations for compare mode items */}
                                                        {result.citations && result.citations.length > 0 && (
                                                            <div className="flex flex-wrap items-center gap-1.5 mt-2 pt-2 border-t border-gray-200 dark:border-gray-600">
                                                                {result.citations.map((citation: Citation, cIdx: number) => (
                                                                    <button
                                                                        key={cIdx}
                                                                        onClick={() => handleCitationClick(citation.page)}
                                                                        className="inline-flex items-center px-1.5 py-0.5 text-[10px] bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 transition-colors"
                                                                        title={citation.text}
                                                                    >
                                                                        Pg {citation.page}
                                                                    </button>
                                                                ))}
                                                            </div>
                                                        )}
                                                    </div>
                                                ))}
                                            </div>
                                        </div>
                                    ) : (
                                        <div className="backdrop-blur-sm bg-white/60 dark:bg-white/5 border border-white/60 dark:border-white/10 text-gray-800 dark:text-gray-200 rounded-2xl p-3 prose prose-sm dark:prose-invert max-w-none shadow-sm">
                                            <ReactMarkdown>{message.content}</ReactMarkdown>
                                        </div>
                                    )
                                ) : (
                                    <p className="text-sm">{message.content}</p>
                                )}
                            </div>
                        </div>

                        {/* Per-message citations — rendered directly below each assistant reply (except for compareMode which renders inline) */}
                        {message.role === 'assistant' && !message.compareResults && message.citations && message.citations.length > 0 && (
                            <div className="flex flex-wrap items-center gap-2 px-1 mt-1">
                                <span className="text-xs text-gray-500 dark:text-gray-400">Sources:</span>
                                {message.citations.map((citation: Citation, citIndex: number) => (
                                    <button
                                        key={citIndex}
                                        onClick={() => handleCitationClick(citation.page)}
                                        className="inline-flex items-center px-2 py-1 text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded-full hover:bg-blue-200 dark:hover:bg-blue-800 transition-colors cursor-pointer"
                                        title={citation.text}
                                    >
                                        <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                                        </svg>
                                        Page {citation.page}
                                    </button>
                                ))}
                            </div>
                        )}
                    </div>
                ))}

                {loading && (
                    <div className="flex justify-start">
                        <div className="backdrop-blur-sm bg-white/50 dark:bg-white/5 border border-white/60 dark:border-white/10 rounded-2xl px-4 py-3 shadow-sm">
                            <div className="flex space-x-1.5 items-center">
                                <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce"></div>
                                <div className="w-2 h-2 bg-violet-500 rounded-full animate-bounce" style={{ animationDelay: '0.15s' }}></div>
                                <div className="w-2 h-2 bg-indigo-500 rounded-full animate-bounce" style={{ animationDelay: '0.3s' }}></div>
                            </div>
                        </div>
                    </div>
                )}

                <div ref={messagesEndRef} />
            </div>

            {/* Follow-up Input */}
            <form onSubmit={handleSendFollowUp} className="border-t border-white/40 dark:border-white/10 p-3 backdrop-blur-sm bg-white/30 dark:bg-white/5">
                <div className="flex space-x-2">
                    <input
                        type="text"
                        value={input}
                        onChange={(e) => setInput(e.target.value)}
                        placeholder="Ask a follow-up question..."
                        className="flex-1 px-3 py-2 text-sm bg-white/60 dark:bg-white/5 border border-white/60 dark:border-white/10 rounded-xl text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500/40 backdrop-blur-sm transition-all"
                        disabled={loading}
                    />
                    <button
                        type="submit"
                        disabled={loading || !input.trim()}
                        className="px-4 py-2 bg-gradient-to-r from-blue-600 to-violet-600 hover:from-blue-500 hover:to-violet-500 text-white text-sm font-semibold rounded-xl shadow-lg shadow-blue-500/25 disabled:opacity-50 transition-all duration-200 hover:scale-105 disabled:scale-100 border border-white/20"
                    >
                        Send
                    </button>
                </div>
            </form>
        </div>
    );
}
