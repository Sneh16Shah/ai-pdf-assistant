import { useState, useEffect, useCallback } from 'react';
import PDFUpload from './components/PDFUpload';
import PDFViewer from './components/PDFViewer';
import ExplanationPanel from './components/ExplanationPanel';
import SummaryView from './components/SummaryView';
import { UploadResponse } from './services/api';

type View = 'upload' | 'viewer' | 'summary';

export default function App() {
  const [view, setView] = useState<View>('upload');
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [documentInfo, setDocumentInfo] = useState<UploadResponse | null>(null);
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);

  // Explanation panel state
  const [isPanelOpen, setIsPanelOpen] = useState(false);
  const [selectedText, setSelectedText] = useState<string | null>(null);
  const [selectedPage, setSelectedPage] = useState<number | null>(null);

  const handleUploadSuccess = (data: UploadResponse, file: File) => {
    // If PDF already loaded, this is handled by PDFUpload with confirmation
    setSessionId(data.session_id);
    setDocumentInfo(data);

    // Create URL for the uploaded PDF
    const url = URL.createObjectURL(file);
    setPdfUrl(url);

    setView('viewer');
    setIsPanelOpen(false);
    setSelectedText(null);
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

  // Cleanup blob URL on unmount or PDF change
  useEffect(() => {
    return () => {
      if (pdfUrl) {
        URL.revokeObjectURL(pdfUrl);
      }
    };
  }, [pdfUrl]);

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 flex flex-col">
      {/* Header */}
      <header className="bg-white shadow-sm flex-shrink-0">
        <div className="max-w-full mx-auto px-4 sm:px-6 lg:px-8 py-3">
          <div className="flex justify-between items-center">
            <h1 className="text-xl font-bold text-gray-900">AskMyPDF</h1>
            <nav className="flex items-center space-x-2">
              <button
                onClick={() => setView('upload')}
                className={`px-3 py-1.5 text-sm rounded-lg ${view === 'upload'
                    ? 'bg-blue-600 text-white'
                    : 'text-gray-600 hover:bg-gray-100'
                  }`}
              >
                Upload
              </button>
              {sessionId && (
                <>
                  <button
                    onClick={() => setView('viewer')}
                    className={`px-3 py-1.5 text-sm rounded-lg ${view === 'viewer'
                        ? 'bg-blue-600 text-white'
                        : 'text-gray-600 hover:bg-gray-100'
                      }`}
                  >
                    Viewer
                  </button>
                  <button
                    onClick={() => setView('summary')}
                    className={`px-3 py-1.5 text-sm rounded-lg ${view === 'summary'
                        ? 'bg-blue-600 text-white'
                        : 'text-gray-600 hover:bg-gray-100'
                      }`}
                  >
                    Summary
                  </button>
                  <div className="w-px h-6 bg-gray-300 mx-2"></div>
                  <button
                    onClick={togglePanel}
                    className={`flex items-center space-x-1 px-3 py-1.5 text-sm rounded-lg ${isPanelOpen
                        ? 'bg-blue-100 text-blue-700'
                        : 'text-gray-600 hover:bg-gray-100'
                      }`}
                    title="Toggle AI Panel (Ctrl+L)"
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                    </svg>
                    <span>AI</span>
                    <kbd className="hidden sm:inline-block ml-1 px-1 py-0.5 text-xs bg-gray-100 rounded">⌃L</kbd>
                  </button>
                </>
              )}
            </nav>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 flex overflow-hidden">
        {view === 'upload' && (
          <div className="flex-1 flex items-center justify-center p-8">
            <div className="max-w-xl w-full">
              <div className="text-center mb-8">
                <h2 className="text-3xl font-bold text-gray-900 mb-2">
                  Upload Your PDF
                </h2>
                <p className="text-gray-600">
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
            {/* PDF Viewer */}
            <div className={`flex-1 transition-all duration-300 ${isPanelOpen ? 'w-3/5' : 'w-full'}`}>
              <div className="h-full flex flex-col">
                <div className="px-4 py-2 bg-white border-b">
                  <p className="text-sm text-gray-600">
                    {documentInfo?.filename} • {documentInfo?.pages} pages
                    <span className="text-gray-400 ml-2">• Select text to ask AI</span>
                  </p>
                </div>
                <div className="flex-1 overflow-hidden">
                  <PDFViewer pdfUrl={pdfUrl} onTextSelect={handleTextSelect} />
                </div>
              </div>
            </div>

            {/* Explanation Panel */}
            <div className={`transition-all duration-300 ${isPanelOpen ? 'w-2/5' : 'w-0'}`}>
              <ExplanationPanel
                sessionId={sessionId}
                selectedText={selectedText}
                pageNumber={selectedPage}
                isOpen={isPanelOpen}
                onClose={() => setIsPanelOpen(false)}
              />
            </div>
          </div>
        )}

        {view === 'summary' && sessionId && (
          <div className="flex-1 p-8 overflow-auto">
            <div className="max-w-4xl mx-auto">
              <div className="mb-4">
                <h2 className="text-2xl font-bold text-gray-900 mb-2">
                  PDF Summary
                </h2>
                {documentInfo && (
                  <p className="text-gray-600">
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
