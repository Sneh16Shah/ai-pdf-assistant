import { useState, useEffect, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import { sendMessage, ChatMessage } from '../services/api';

interface ExplanationPanelProps {
    sessionId: string;
    selectedText: string | null;
    pageNumber: number | null;
    isOpen: boolean;
    onClose: () => void;
}

export default function ExplanationPanel({
    sessionId,
    selectedText,
    pageNumber,
    isOpen,
    onClose,
}: ExplanationPanelProps) {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);
    const [explanation, setExplanation] = useState<string | null>(null);
    const messagesEndRef = useRef<HTMLDivElement>(null);

    // Auto-explain selected text
    useEffect(() => {
        if (selectedText && isOpen) {
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

        const query = `Please explain the following text from page ${pageNumber || 'unknown'}:\n\n"${text}"`;

        try {
            const response = await sendMessage(sessionId, query);
            setExplanation(response.response);

            // Add to chat history
            setMessages((prev) => [
                ...prev,
                { role: 'user', content: `Explain: "${text.substring(0, 100)}${text.length > 100 ? '...' : ''}"` },
                { role: 'assistant', content: response.response },
            ]);
        } catch (error: any) {
            setExplanation(`Error: ${error.message || 'Failed to get explanation'}`);
        } finally {
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
        } catch (error: any) {
            setMessages((prev) => [
                ...prev,
                { role: 'assistant', content: `Error: ${error.message}` },
            ]);
        } finally {
            setLoading(false);
        }
    };

    if (!isOpen) return null;

    return (
        <div className="flex flex-col h-full bg-white border-l shadow-lg">
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 border-b bg-gray-50">
                <div className="flex items-center space-x-2">
                    <svg className="w-5 h-5 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                    </svg>
                    <h3 className="font-semibold text-gray-800">AI Assistant</h3>
                </div>
                <div className="flex items-center space-x-2">
                    <span className="text-xs text-gray-500">Ctrl+L to toggle</span>
                    <button
                        onClick={onClose}
                        className="p-1 text-gray-500 hover:text-gray-700 hover:bg-gray-200 rounded"
                    >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>
            </div>

            {/* Selected Text */}
            {selectedText && (
                <div className="px-4 py-3 bg-blue-50 border-b">
                    <p className="text-xs text-blue-600 font-medium mb-1">
                        Selected from page {pageNumber}:
                    </p>
                    <p className="text-sm text-gray-700 italic line-clamp-3">"{selectedText}"</p>
                </div>
            )}

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-4 space-y-4">
                {/* Explanation */}
                {loading && !explanation && (
                    <div className="flex items-center space-x-2 text-gray-500">
                        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600"></div>
                        <span>Analyzing...</span>
                    </div>
                )}

                {explanation && (
                    <div className="bg-gray-50 rounded-lg p-4">
                        <div className="prose prose-sm max-w-none">
                            <ReactMarkdown>{explanation}</ReactMarkdown>
                        </div>
                    </div>
                )}

                {/* Chat History */}
                {messages.slice(2).map((message, index) => (
                    <div
                        key={index}
                        className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
                    >
                        <div
                            className={`max-w-[90%] rounded-lg p-3 ${message.role === 'user'
                                    ? 'bg-blue-600 text-white'
                                    : 'bg-gray-100 text-gray-800'
                                }`}
                        >
                            {message.role === 'assistant' ? (
                                <div className="prose prose-sm max-w-none">
                                    <ReactMarkdown>{message.content}</ReactMarkdown>
                                </div>
                            ) : (
                                <p className="text-sm">{message.content}</p>
                            )}
                        </div>
                    </div>
                ))}

                {loading && messages.length > 0 && (
                    <div className="flex justify-start">
                        <div className="bg-gray-100 rounded-lg p-3">
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
            <form onSubmit={handleSendFollowUp} className="border-t p-3">
                <div className="flex space-x-2">
                    <input
                        type="text"
                        value={input}
                        onChange={(e) => setInput(e.target.value)}
                        placeholder="Ask a follow-up question..."
                        className="flex-1 px-3 py-2 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
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
