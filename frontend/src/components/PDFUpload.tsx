import { useState } from 'react';
import { uploadPDF, addPDFToSession, UploadResponse } from '../services/api';

interface PDFUploadProps {
  onUploadSuccess: (data: UploadResponse, file: File) => void;
  hasExistingPdf?: boolean;
  sessionId?: string | null; // For batch upload to existing session
}

export default function PDFUpload({ onUploadSuccess, hasExistingPdf = false, sessionId }: PDFUploadProps) {
  const [files, setFiles] = useState<File[]>([]);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dragActive, setDragActive] = useState(false);
  const [showConfirmDialog, setShowConfirmDialog] = useState(false);
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [uploadProgress, setUploadProgress] = useState(0);

  const handleFileSelect = (selectedFiles: FileList | File[]) => {
    const validFiles: File[] = [];
    const fileArray = Array.from(selectedFiles);

    for (const file of fileArray) {
      if (file.type !== 'application/pdf') {
        setError(`${file.name}: Please select PDF files only`);
        continue;
      }
      if (file.size > 50 * 1024 * 1024) {
        setError(`${file.name}: File size must be less than 50MB`);
        continue;
      }
      validFiles.push(file);
    }

    if (validFiles.length === 0) return;

    if (hasExistingPdf) {
      setPendingFiles(validFiles);
      setShowConfirmDialog(true);
      return;
    }

    setFiles(validFiles);
    setError(null);
  };

  const confirmReplace = () => {
    if (pendingFiles.length > 0) {
      setFiles(pendingFiles);
      setPendingFiles([]);
      setError(null);
    }
    setShowConfirmDialog(false);
  };

  const cancelReplace = () => {
    setPendingFiles([]);
    setShowConfirmDialog(false);
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      handleFileSelect(e.target.files);
    }
  };

  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);

    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      handleFileSelect(e.dataTransfer.files);
    }
  };

  const handleUpload = async () => {
    if (files.length === 0) {
      setError('Please select at least one file');
      return;
    }

    setUploading(true);
    setError(null);
    setUploadProgress(0);

    try {
      // Upload first file to create session
      const firstFile = files[0];
      const firstResponse = await uploadPDF(firstFile);
      onUploadSuccess(firstResponse, firstFile);
      setUploadProgress(1);

      // Upload remaining files to session
      if (files.length > 1) {
        for (let i = 1; i < files.length; i++) {
          const file = files[i];
          try {
            const response = await addPDFToSession(firstResponse.session_id, file);
            onUploadSuccess(response, file);
            setUploadProgress(i + 1);
          } catch (err) {
            console.error(`Failed to upload ${file.name}:`, err);
          }
        }
      }

      setFiles([]);
    } catch (err: any) {
      setError(err instanceof Error ? err.message : 'Upload failed');
    } finally {
      setUploading(false);
      setUploadProgress(0);
    }
  };

  return (
    <div className="w-full max-w-2xl mx-auto p-6">
      {/* Confirmation Dialog */}
      {showConfirmDialog && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md mx-4 shadow-xl">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
              Replace Current PDF?
            </h3>
            <p className="text-gray-600 dark:text-gray-400 mb-4">
              You already have a PDF loaded. Uploading a new one will replace it and clear your chat history.
            </p>
            <div className="flex justify-end space-x-3">
              <button
                onClick={cancelReplace}
                className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
              >
                Cancel
              </button>
              <button
                onClick={confirmReplace}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
              >
                Replace PDF
              </button>
            </div>
          </div>
        </div>
      )}

      <div
        className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors ${dragActive
          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
          : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500 bg-white dark:bg-gray-800'
          }`}
        onDragEnter={handleDrag}
        onDragLeave={handleDrag}
        onDragOver={handleDrag}
        onDrop={handleDrop}
      >
        <input
          type="file"
          accept=".pdf"
          multiple
          onChange={handleFileChange}
          className="hidden"
          id="pdf-upload"
          disabled={uploading}
        />
        <label
          htmlFor="pdf-upload"
          className="cursor-pointer flex flex-col items-center"
        >
          <svg
            className="w-12 h-12 text-gray-400 dark:text-gray-500 mb-4"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
            />
          </svg>
          <p className="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">
            {files.length > 0
              ? `${files.length} file${files.length > 1 ? 's' : ''} selected`
              : 'Drag & drop your PDFs here'}
          </p>
          <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
            or click to browse (Max 50MB each) - Select multiple with Ctrl/Shift
          </p>
          {files.length > 0 && (
            <div className="text-xs text-gray-500 dark:text-gray-400 max-h-20 overflow-y-auto">
              {files.map((f, i) => (
                <div key={i}>{f.name}</div>
              ))}
            </div>
          )}
          {hasExistingPdf && files.length === 0 && (
            <p className="text-xs text-amber-600 dark:text-amber-400 mt-2">
              ⚠️ Uploading will replace your current session
            </p>
          )}
        </label>

        {files.length > 0 && (
          <div className="mt-4">
            <button
              onClick={handleUpload}
              disabled={uploading}
              className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {uploading ? `Uploading... (${uploadProgress}/${files.length})` : `Upload ${files.length} PDF${files.length > 1 ? 's' : ''}`}
            </button>
          </div>
        )}

        {error && (
          <div className="mt-4 p-3 bg-red-100 dark:bg-red-900/30 border border-red-400 dark:border-red-600 text-red-700 dark:text-red-400 rounded">
            {error}
          </div>
        )}
      </div>
    </div>
  );
}
