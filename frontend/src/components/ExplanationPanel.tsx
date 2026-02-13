import { useState, useEffect, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import { sendMessage, ChatMessage, Citation, getSessionMessages } from '../services/api';

interface ExplanationPanelProps {
    sessionId: string;
    selectedText: string | null;
    pageNumber: number | null;
    isOpen: boolean;
    onClose: () => void;
    onGoToPage?: (page: number) => void;
}

export default function ExplanationPanel({
    sessionId,
    selectedText,
    pageNumber,
    isOpen,
    onClose,
    onGoToPage,
}: ExplanationPanelProps) {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);
    const [explanation, setExplanation] = useState<string | null>(null);
    const [citations, setCitations] = useState<Citation[]>([]);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const previousTextRef = useRef<string>('');
    const loadedSessionRef = useRef<string>('');

    // Load previous chat history from DB when session changes
    useEffect(() => {
        if (sessionId && sessionId !== loadedSessionRef.current) {
            loadedSessionRef.current = sessionId;
            getSessionMessages(sessionId)
                .then((dbMessages) => {
                    if (dbMessages && dbMessages.length > 0) {
                        const loaded: ChatMessage[] = dbMessages.map((m: { role: string; content: string }) => ({
                            role: m.role as 'user' | 'assistant',
                            content: m.content,
                        }));
                        setMessages(loaded);
                    }
                })
                .catch(() => {
                    // No saved history or not authenticated — that's fine
                });
        }
    }, [sessionId]);

    useEffect(() => {
        // Only call explainText if text actually changed and panel is open
        if (selectedText && isOpen && selectedText !== previousTextRef.current) {
            previousTextRef.current = selectedText;
            explainText(selectedText);
        }
    }, [selectedText, isOpen]);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages, explanation]);

    const explainText = async (text: string) => {
        setLoading(true);
        setExplanation(null);
        setCitations([]);

        const query = `Please explain the following text from page ${pageNumber || 'unknown'}:\n\n"${text}"`;

        try {
            const response = await sendMessage(sessionId, query);
            setExplanation(response.response);
            if (response.citations) {
                setCitations(response.citations);
            }
            setMessages((prev) => [
                ...prev,
                { role: 'user', content: `Explain: "${text.substring(0, 100)}${text.length > 100 ? '...' : ''}"` },
                { role: 'assistant', content: response.response },
            ]);
            setLoading(false);
        } catch (error: unknown) {
            let errorMessage = 'Failed to get explanation';
            if (error instanceof Error) {
                if (error.message.includes('404')) {
                    errorMessage = 'Session expired. Please re-upload your PDF to start a new session.';
                } else {
                    errorMessage = error.message;
                }
            }
            setExplanation(`Error: ${errorMessage}`);
            setLoading(false);
        }
    };

    const handleSendFollowUp = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!input.trim() || loading) return;

        const userMessage: ChatMessage = { role: 'user', content: input };
        setMessages((prev) => [...prev, userMessage]);
        setInput('');
        setLoading(true);

        try {
            const response = await sendMessage(sessionId, input);
            setMessages((prev) => [...prev, { role: 'assistant', content: response.response }]);
            if (response.citations) {
                setCitations(response.citations);
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

    const handleCitationClick = (page: number) => {
        if (onGoToPage) {
            onGoToPage(page);
        }
    };

    if (!isOpen) return null;

    return (
        <div className="flex flex-col h-full bg-white dark:bg-gray-800 border-l dark:border-gray-700 shadow-lg">
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 border-b dark:border-gray-700 bg-gray-50 dark:bg-gray-900">
                <div className="flex items-center space-x-2">
                    <svg className="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                    </svg>
                    <h3 className="font-semibold text-gray-800 dark:text-white">AI Assistant</h3>
                </div>
                <div className="flex items-center space-x-2">
                    <span className="text-xs text-gray-500 dark:text-gray-400">Ctrl+L to toggle</span>
                    <button
                        onClick={onClose}
                        className="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-700 rounded"
                    >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>
            </div>

            {/* Selected Text */}
            {selectedText && (
                <div className="px-4 py-3 bg-blue-50 dark:bg-blue-900/30 border-b dark:border-gray-700">
                    <p className="text-xs text-blue-600 dark:text-blue-400 font-medium mb-1">
                        Selected from page {pageNumber}:
                    </p>
                    <p className="text-sm text-gray-700 dark:text-gray-300 italic line-clamp-3">"{selectedText}"</p>
                </div>
            )}

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-4 space-y-4">
                {/* Empty State */}
                {!explanation && messages.length === 0 && !loading && (
                    <div className="flex flex-col items-center justify-center h-full text-center text-gray-400 dark:text-gray-500">
                        <svg className="w-16 h-16 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                        </svg>
                        <h4 className="text-lg font-medium text-gray-600 dark:text-gray-400 mb-2">
                            Ask anything about your PDF
                        </h4>
                        <p className="text-sm max-w-xs">
                            Select text in the PDF and click "Ask AI" or type a question below to get started.
                        </p>
                    </div>
                )}

                {explanation && (
                    <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                        <div className="prose prose-sm dark:prose-invert max-w-none">
                            <ReactMarkdown>{explanation}</ReactMarkdown>
                        </div>
                    </div>
                )}

                {/* Citations */}
                {citations.length > 0 && (
                    <div className="flex flex-wrap items-center gap-2 px-1">
                        <span className="text-xs text-gray-500 dark:text-gray-400">Sources:</span>
                        {citations.map((citation, index) => (
                            <button
                                key={index}
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
                {/* Chat messages - skip first 2 only when explanation exists (text selection flow) */}
                {(explanation ? messages.slice(2) : messages).map((message, index) => (
                    <div
                        key={index}
                        className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
                    >
                        <div
                            className={`max-w-[90%] rounded-lg p-3 ${message.role === 'user'
                                ? 'bg-blue-600 text-white'
                                : 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200'
                                }`}
                        >
                            {message.role === 'assistant' ? (
                                <div className="prose prose-sm dark:prose-invert max-w-none">
                                    <ReactMarkdown>{message.content}</ReactMarkdown>
                                </div>
                            ) : (
                                <p className="text-sm">{message.content}</p>
                            )}
                        </div>
                    </div>
                ))}

                {loading && (
                    <div className="flex justify-start">
                        <div className="bg-gray-100 dark:bg-gray-700 rounded-lg p-3">
                            <div className="flex space-x-1">
                                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce"></div>
                                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0.1s' }}></div>
                                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0.2s' }}></div>
                            </div>
                        </div>
                    </div>
                )}

                <div ref={messagesEndRef} />
            </div>

            {/* Follow-up Input */}
            <form onSubmit={handleSendFollowUp} className="border-t dark:border-gray-700 p-3">
                <div className="flex space-x-2">
                    <input
                        type="text"
                        value={input}
                        onChange={(e) => setInput(e.target.value)}
                        placeholder="Ask a follow-up question..."
                        className="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                        disabled={loading}
                    />
                    <button
                        type="submit"
                        disabled={loading || !input.trim()}
                        className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50"
                    >
                        Send
                    </button>
                </div>
            </form>
        </div>
    );
}
