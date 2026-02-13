import { useState, useCallback, useRef, useEffect } from 'react';
import { Document, Page, pdfjs } from 'react-pdf';
import 'react-pdf/dist/esm/Page/AnnotationLayer.css';
import 'react-pdf/dist/esm/Page/TextLayer.css';

// Set up the worker
pdfjs.GlobalWorkerOptions.workerSrc = `//unpkg.com/pdfjs-dist@${pdfjs.version}/build/pdf.worker.min.js`;

interface PDFViewerProps {
  pdfUrl: string;
  onTextSelect?: (text: string, pageNumber: number) => void;
  targetPage?: number;
}

interface SelectionPosition {
  x: number;
  y: number;
}

export default function PDFViewer({ pdfUrl, onTextSelect, targetPage }: PDFViewerProps) {
  const [numPages, setNumPages] = useState<number>(0);
  const [scale, setScale] = useState<number>(1.0);
  const [selectedText, setSelectedText] = useState<string>('');
  const [selectionPosition, setSelectionPosition] = useState<SelectionPosition | null>(null);
  const [currentPage, setCurrentPage] = useState<number>(1);
  const containerRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const pageRefs = useRef<(HTMLDivElement | null)[]>([]);

  const onDocumentLoadSuccess = ({ numPages }: { numPages: number }) => {
    setNumPages(numPages);
    pageRefs.current = new Array(numPages).fill(null);
  };

  // Handle external page navigation (from citations)
  useEffect(() => {
    if (targetPage && targetPage >= 1 && targetPage <= numPages) {
      scrollToPage(targetPage);
    }
  }, [targetPage, numPages]);

  const scrollToPage = (pageNum: number) => {
    const pageElement = pageRefs.current[pageNum - 1];
    if (pageElement && scrollContainerRef.current) {
      pageElement.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  // Track current page based on scroll position
  const handleScroll = useCallback(() => {
    if (!scrollContainerRef.current || numPages === 0) return;

    const container = scrollContainerRef.current;
    const scrollTop = container.scrollTop;
    const containerHeight = container.clientHeight;

    // Find which page is most visible
    for (let i = 0; i < pageRefs.current.length; i++) {
      const pageEl = pageRefs.current[i];
      if (pageEl) {
        const rect = pageEl.getBoundingClientRect();
        const containerRect = container.getBoundingClientRect();
        const pageTop = rect.top - containerRect.top;
        const pageBottom = rect.bottom - containerRect.top;

        // If page is visible in the top half of the container
        if (pageTop < containerHeight / 2 && pageBottom > 0) {
          setCurrentPage(i + 1);
          break;
        }
      }
    }
  }, [numPages]);

  useEffect(() => {
    const container = scrollContainerRef.current;
    if (container) {
      container.addEventListener('scroll', handleScroll);
      return () => container.removeEventListener('scroll', handleScroll);
    }
  }, [handleScroll]);

  const handleTextSelection = useCallback(() => {
    const selection = window.getSelection();
    const text = selection?.toString().trim();

    if (text && text.length > 0) {
      setSelectedText(text);

      // Determine which page the selection is from
      const range = selection?.getRangeAt(0);
      const rect = range?.getBoundingClientRect();

      if (rect && containerRef.current) {
        const containerRect = containerRef.current.getBoundingClientRect();
        setSelectionPosition({
          x: rect.left - containerRect.left + rect.width / 2,
          y: rect.bottom - containerRect.top + 10,
        });
      }
    } else {
      setSelectedText('');
      setSelectionPosition(null);
    }
  }, []);

  const handleAskAI = () => {
    if (selectedText && onTextSelect) {
      onTextSelect(selectedText, currentPage);
      setSelectedText('');
      setSelectionPosition(null);
      window.getSelection()?.removeAllRanges();
    }
  };

  useEffect(() => {
    document.addEventListener('mouseup', handleTextSelection);
    return () => {
      document.removeEventListener('mouseup', handleTextSelection);
    };
  }, [handleTextSelection]);

  return (
    <div ref={containerRef} className="relative flex flex-col h-full bg-gray-100 dark:bg-gray-900">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-2 bg-white dark:bg-gray-800 border-b dark:border-gray-700 shadow-sm">
        <div className="flex items-center space-x-2">
          <span className="text-sm text-gray-600 dark:text-gray-400">
            Page {currentPage} of {numPages}
          </span>
        </div>

        <div className="flex items-center space-x-2">
          <button
            onClick={() => setScale((s) => Math.max(s - 0.1, 0.5))}
            className="px-2 py-1 text-sm bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-200 dark:hover:bg-gray-600"
          >
            −
          </button>
          <span className="text-sm text-gray-600 dark:text-gray-400 w-16 text-center">
            {Math.round(scale * 100)}%
          </span>
          <button
            onClick={() => setScale((s) => Math.min(s + 0.1, 2.0))}
            className="px-2 py-1 text-sm bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-200 dark:hover:bg-gray-600"
          >
            +
          </button>
        </div>
      </div>

      {/* PDF Content - Scrollable container with all pages */}
      <div
        ref={scrollContainerRef}
        className="flex-1 overflow-auto p-4"
      >
        <div className="flex flex-col items-center space-y-4">
          <Document
            file={pdfUrl}
            onLoadSuccess={onDocumentLoadSuccess}
            loading={
              <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
              </div>
            }
            error={
              <div className="text-red-500 dark:text-red-400 p-4">
                Failed to load PDF. Please try again.
              </div>
            }
          >
            {Array.from(new Array(numPages), (_, index) => (
              <div
                key={`page_${index + 1}`}
                ref={(el) => { pageRefs.current[index] = el; }}
                className="mb-4"
              >
                <Page
                  pageNumber={index + 1}
                  scale={scale}
                  renderTextLayer={true}
                  renderAnnotationLayer={true}
                  className="shadow-lg bg-white"
                />
              </div>
            ))}
          </Document>
        </div>
      </div>

      {/* Ask AI Popup */}
      {selectedText && selectionPosition && (
        <div
          className="absolute z-50 transform -translate-x-1/2"
          style={{
            left: selectionPosition.x,
            top: selectionPosition.y,
          }}
        >
          <button
            onClick={handleAskAI}
            className="flex items-center space-x-1 px-3 py-2 bg-blue-600 text-white text-sm rounded-lg shadow-lg hover:bg-blue-700 transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>Ask AI</span>
          </button>
        </div>
      )}
    </div>
  );
}
