import { useState, useEffect, useCallback, useRef } from 'react';
import PDFUpload from './components/PDFUpload';
import PDFViewer from './components/PDFViewer';
import ExplanationPanel from './components/ExplanationPanel';
import SummaryView from './components/SummaryView';
import DocumentSidebar from './components/DocumentSidebar';
import LoginPage from './components/LoginPage';
import Dashboard from './components/Dashboard';
import { UploadResponse, SessionDocument, addPDFToSession, deleteDocument } from './services/api';
import { useTheme } from './contexts/ThemeContext';
import { useAuth } from './contexts/AuthContext';

type View = 'upload' | 'viewer' | 'summary' | 'dashboard';

export default function App() {
  const { theme, toggleTheme } = useTheme();
  const { user, isAuthenticated, isLoading, logout } = useAuth();
  const [view, setView] = useState<View>('upload');
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [documentInfo, setDocumentInfo] = useState<UploadResponse | null>(null);
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);

  // Multi-document state
  const [documents, setDocuments] = useState<SessionDocument[]>([]);
  const [activeDocumentId, setActiveDocumentId] = useState<string | null>(null);
  const [pdfUrls, setPdfUrls] = useState<Record<string, string>>({});
  const [isAddingPdf, setIsAddingPdf] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const addPdfInputRef = useRef<HTMLInputElement>(null);

  // Explanation panel state
  const [isPanelOpen, setIsPanelOpen] = useState(false);
  const [selectedText, setSelectedText] = useState<string | null>(null);
  const [selectedPage, setSelectedPage] = useState<number | null>(null);
  const [targetPage, setTargetPage] = useState<number | undefined>(undefined);

  const handleGoToPage = (page: number) => {
    setTargetPage(page);
    // Reset after a brief delay to allow re-clicking same page
    setTimeout(() => setTargetPage(undefined), 100);
  };

  const handleUploadSuccess = (data: UploadResponse, file: File) => {
    // Set session ID (only changes on first upload)
    if (!sessionId) {
      setSessionId(data.session_id);
    }

    const url = URL.createObjectURL(file);

    // Add to documents array (append, don't replace)
    const newDoc: SessionDocument = {
      id: data.document_id,
      filename: data.filename,
      pages: data.pages,
    };
    setDocuments(prev => [...prev, newDoc]);
    setPdfUrls(prev => ({ ...prev, [data.document_id]: url }));

    // Only set as active if it's the first document
    if (!activeDocumentId) {
      setActiveDocumentId(data.document_id);
      setDocumentInfo(data);
      setPdfUrl(url);
      setView('viewer');
      setIsPanelOpen(false);
      setSelectedText(null);
    }
  };

  // Handle adding more PDFs to session
  const handleAddMorePdfs = () => {
    addPdfInputRef.current?.click();
  };

  const handleAddPdfFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || !sessionId) return;

    const files = Array.from(e.target.files);
    setIsAddingPdf(true);

    for (const file of files) {
      try {
        const response = await addPDFToSession(sessionId, file);
        const url = URL.createObjectURL(file);
        const newDoc: SessionDocument = {
          id: response.document_id,
          filename: response.filename,
          pages: response.pages,
        };
        setDocuments(prev => [...prev, newDoc]);
        setPdfUrls(prev => ({ ...prev, [response.document_id]: url }));
      } catch (err) {
        console.error('Failed to add PDF:', err);
      }
    }

    setIsAddingPdf(false);
    e.target.value = ''; // Reset input
  };

  // Handle document selection
  const handleDocumentSelect = (documentId: string) => {
    const doc = documents.find(d => d.id === documentId);
    if (doc) {
      setActiveDocumentId(documentId);
      const url = pdfUrls[documentId];
      if (url) {
        setPdfUrl(url);
        setDocumentInfo(prev => prev ? { ...prev, filename: doc.filename, pages: doc.pages, document_id: doc.id } : null);
      }
    }
  };

  // Handle document deletion
  const handleDeleteDocument = async (documentId: string) => {
    if (!sessionId || documents.length <= 1) return;

    setIsDeleting(true);
    try {
      await deleteDocument(sessionId, documentId);

      // Clean up blob URL
      const url = pdfUrls[documentId];
      if (url) URL.revokeObjectURL(url);
      setPdfUrls(prev => {
        const updated = { ...prev };
        delete updated[documentId];
        return updated;
      });

      // Update documents list
      setDocuments(prev => prev.filter(d => d.id !== documentId));

      // Switch to another document if this was active
      if (activeDocumentId === documentId) {
        const remaining = documents.filter(d => d.id !== documentId);
        if (remaining.length > 0) {
          handleDocumentSelect(remaining[0].id);
        }
      }
    } catch (err) {
      console.error('Failed to delete document:', err);
    }
    setIsDeleting(false);
  };

  const handleTextSelect = (text: string, pageNumber: number) => {
    setSelectedText(text);
    setSelectedPage(pageNumber);
    setIsPanelOpen(true);
  };

  const togglePanel = useCallback(() => {
    setIsPanelOpen((prev) => !prev);
  }, []);

  // Ctrl+L keyboard shortcut
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key === 'l') {
        e.preventDefault();
        togglePanel();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [togglePanel]);

  // Cleanup blob URL on unmount
  useEffect(() => {
    return () => {
      if (pdfUrl) URL.revokeObjectURL(pdfUrl);
    };
  }, [pdfUrl]);

  // Show loading spinner while checking auth
  if (isLoading) {
    return (
      <div className="h-screen flex items-center justify-center bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  // Show login page if not authenticated
  if (!isAuthenticated) {
    return <LoginPage />;
  }

  return (
    <div className="h-screen overflow-hidden bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800 flex flex-col transition-colors">
      {/* Header */}
      <header className="bg-white dark:bg-gray-800 shadow-sm flex-shrink-0 border-b dark:border-gray-700">
        <div className="max-w-full mx-auto px-4 sm:px-6 lg:px-8 py-3">
          <div className="flex justify-between items-center">
            <h1 className="text-xl font-bold text-gray-900 dark:text-white">AskMyPDF</h1>
            <nav className="flex items-center space-x-2">
              <button
                onClick={() => setView('dashboard')}
                className={`px-3 py-1.5 text-sm rounded-lg transition-colors ${view === 'dashboard'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                  }`}
              >
                Dashboard
              </button>
              <button
                onClick={() => setView('upload')}
                className={`px-3 py-1.5 text-sm rounded-lg transition-colors ${view === 'upload'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                  }`}
              >
                Upload
              </button>
              {sessionId && (
                <>
                  <button
                    onClick={() => setView('viewer')}
                    className={`px-3 py-1.5 text-sm rounded-lg transition-colors ${view === 'viewer'
                      ? 'bg-blue-600 text-white'
                      : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                      }`}
                  >
                    Viewer
                  </button>
                  <button
                    onClick={() => setView('summary')}
                    className={`px-3 py-1.5 text-sm rounded-lg transition-colors ${view === 'summary'
                      ? 'bg-blue-600 text-white'
                      : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                      }`}
                  >
                    Summary
                  </button>
                  <div className="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-2"></div>
                  <button
                    onClick={togglePanel}
                    className={`flex items-center space-x-1 px-3 py-1.5 text-sm rounded-lg transition-colors ${isPanelOpen
                      ? 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300'
                      : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                      }`}
                    title="Toggle AI Panel (Ctrl+L)"
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                    </svg>
                    <span>AI</span>
                    <kbd className="hidden sm:inline-block ml-1 px-1 py-0.5 text-xs bg-gray-100 dark:bg-gray-700 rounded">⌃L</kbd>
                  </button>
                </>
              )}
              <div className="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-2"></div>
              {/* Theme Toggle */}
              <button
                onClick={toggleTheme}
                className="p-2 rounded-lg text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
              >
                {theme === 'light' ? (
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                  </svg>
                ) : (
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                  </svg>
                )}
              </button>
              {/* User Menu */}
              <div className="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-2"></div>
              <div className="flex items-center space-x-2">
                <span className="text-sm text-gray-600 dark:text-gray-300 hidden sm:inline">
                  {user?.name || user?.email}
                </span>
                <button
                  onClick={logout}
                  className="p-2 rounded-lg text-gray-600 dark:text-gray-300 hover:bg-red-100 dark:hover:bg-red-900/30 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                  title="Logout"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                  </svg>
                </button>
              </div>
            </nav>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 flex overflow-hidden">
        {view === 'dashboard' && (
          <div className="flex-1 overflow-auto">
            <Dashboard onResumeSession={(resumeSessionId) => {
              // Set the session ID so chat works
              setSessionId(resumeSessionId);
              // Open the AI panel for chatting
              setIsPanelOpen(true);
              // Switch to viewer - documents won't have PDF preview but chat will work
              setView('viewer');
            }} />
          </div>
        )}

        {view === 'upload' && (
          <div className="flex-1 flex items-center justify-center p-8">
            <div className="max-w-xl w-full">
              <div className="text-center mb-8">
                <h2 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
                  Upload Your PDF
                </h2>
                <p className="text-gray-600 dark:text-gray-400">
                  Upload a PDF document and ask questions about its content
                </p>
              </div>
              <PDFUpload
                onUploadSuccess={handleUploadSuccess}
                hasExistingPdf={!!pdfUrl}
              />
            </div>
          </div>
        )}

        {view === 'viewer' && sessionId && pdfUrl && (
          <div className="flex-1 flex overflow-hidden">
            {/* Hidden file input for adding PDFs */}
            <input
              type="file"
              ref={addPdfInputRef}
              onChange={handleAddPdfFile}
              accept=".pdf"
              multiple
              className="hidden"
            />

            {/* Document Sidebar */}
            {documents.length > 0 && (
              <DocumentSidebar
                documents={documents}
                activeDocumentId={activeDocumentId}
                onDocumentSelect={handleDocumentSelect}
                onDeleteDocument={handleDeleteDocument}
                onAddMore={handleAddMorePdfs}
                isDeleting={isDeleting}
              />
            )}

            {/* PDF Viewer */}
            <div className={`h-full overflow-hidden flex-1 transition-all duration-300 ${isPanelOpen ? 'w-3/5' : 'w-full'}`}>
              <div className="h-full flex flex-col">
                <div className="px-4 py-2 bg-white dark:bg-gray-800 border-b dark:border-gray-700 flex items-center justify-between">
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    {documentInfo ? (
                      <>{documentInfo.filename} • {documentInfo.pages} pages
                        <span className="text-gray-400 dark:text-gray-500 ml-2">• Select text to ask AI</span>
                      </>
                    ) : sessionId ? (
                      <>Resumed session • Use the AI panel to continue chatting</>
                    ) : (
                      <>No document loaded</>
                    )}
                  </p>
                  {isAddingPdf && (
                    <span className="text-xs text-blue-500 animate-pulse">Adding PDF...</span>
                  )}
                </div>
                <div className="flex-1 overflow-hidden">
                  {pdfUrl ? (
                    <PDFViewer pdfUrl={pdfUrl} onTextSelect={handleTextSelect} targetPage={targetPage} />
                  ) : (
                    <div className="h-full flex items-center justify-center">
                      <div className="text-center max-w-md">
                        <svg className="w-16 h-16 mx-auto text-gray-300 dark:text-gray-600 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                        </svg>
                        <h3 className="text-lg font-medium text-gray-600 dark:text-gray-400 mb-2">
                          Chat Session Resumed
                        </h3>
                        <p className="text-sm text-gray-500 dark:text-gray-500 mb-4">
                          Your previous conversation is loaded in the AI panel.
                          Upload the PDF again to view it alongside the chat.
                        </p>
                        <button
                          onClick={() => setView('upload')}
                          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 text-sm transition-colors"
                        >
                          Upload PDF
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Explanation Panel */}
            <div className={`h-full overflow-hidden transition-all duration-300 ${isPanelOpen ? 'w-2/5' : 'w-0'}`}>
              <ExplanationPanel
                sessionId={sessionId}
                selectedText={selectedText}
                pageNumber={selectedPage}
                isOpen={isPanelOpen}
                onClose={() => setIsPanelOpen(false)}
                onGoToPage={handleGoToPage}
              />
            </div>
          </div>
        )}

        {view === 'summary' && sessionId && (
          <div className="flex-1 p-8 overflow-auto">
            <div className="max-w-4xl mx-auto">
              <div className="mb-4">
                <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">
                  PDF Summary
                </h2>
                {documentInfo && (
                  <p className="text-gray-600 dark:text-gray-400">
                    {documentInfo.filename} • {documentInfo.pages} pages
                  </p>
                )}
              </div>
              <SummaryView sessionId={sessionId} />
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
