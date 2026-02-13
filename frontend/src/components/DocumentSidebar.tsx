import { SessionDocument } from '../services/api';

interface DocumentSidebarProps {
    documents: SessionDocument[];
    activeDocumentId: string | null;
    onDocumentSelect: (documentId: string) => void;
    onDeleteDocument: (documentId: string) => void;
    onAddMore: () => void;
    isDeleting?: boolean;
}

export default function DocumentSidebar({
    documents,
    activeDocumentId,
    onDocumentSelect,
    onDeleteDocument,
    onAddMore,
    isDeleting = false,
}: DocumentSidebarProps) {
    return (
        <div className="w-48 bg-white dark:bg-gray-800 border-r dark:border-gray-700 flex flex-col h-full">
            {/* Header */}
            <div className="p-3 border-b dark:border-gray-700">
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                    Documents ({documents.length})
                </h3>
            </div>

            {/* Document List */}
            <div className="flex-1 overflow-y-auto p-2 space-y-1">
                {documents.map((doc) => (
                    <div
                        key={doc.id}
                        className={`group relative p-2 rounded-lg cursor-pointer transition-colors ${activeDocumentId === doc.id
                                ? 'bg-blue-100 dark:bg-blue-900/50 border border-blue-300 dark:border-blue-700'
                                : 'hover:bg-gray-100 dark:hover:bg-gray-700'
                            }`}
                        onClick={() => onDocumentSelect(doc.id)}
                    >
                        {/* Document Icon */}
                        <div className="flex items-start gap-2">
                            <svg
                                className={`w-5 h-5 flex-shrink-0 mt-0.5 ${activeDocumentId === doc.id
                                        ? 'text-blue-600 dark:text-blue-400'
                                        : 'text-gray-400 dark:text-gray-500'
                                    }`}
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                            >
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth={2}
                                    d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                                />
                            </svg>
                            <div className="flex-1 min-w-0">
                                <p
                                    className={`text-xs font-medium truncate ${activeDocumentId === doc.id
                                            ? 'text-blue-700 dark:text-blue-300'
                                            : 'text-gray-700 dark:text-gray-300'
                                        }`}
                                    title={doc.filename}
                                >
                                    {doc.filename}
                                </p>
                                <p className="text-xs text-gray-500 dark:text-gray-400">
                                    {doc.pages} pages
                                </p>
                            </div>
                        </div>

                        {/* Delete Button */}
                        {documents.length > 1 && (
                            <button
                                onClick={(e) => {
                                    e.stopPropagation();
                                    onDeleteDocument(doc.id);
                                }}
                                disabled={isDeleting}
                                className="absolute top-1 right-1 p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-red-100 dark:hover:bg-red-900/50 text-gray-400 hover:text-red-500 dark:hover:text-red-400 transition-all"
                                title="Remove document"
                            >
                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        )}
                    </div>
                ))}
            </div>

            {/* Add More Button */}
            <div className="p-2 border-t dark:border-gray-700">
                <button
                    onClick={onAddMore}
                    className="w-full flex items-center justify-center gap-2 px-3 py-2 text-sm bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-lg transition-colors"
                >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                    </svg>
                    Add PDF
                </button>
            </div>
        </div>
    );
}
