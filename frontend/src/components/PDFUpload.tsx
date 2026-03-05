import { useState } from 'react';
import { uploadPDF, addPDFToSession, UploadResponse } from '../services/api';

interface PDFUploadProps {
  onUploadSuccess: (data: UploadResponse, file: File) => void;
  hasExistingPdf?: boolean;
  sessionId?: string | null;
}

export default function PDFUpload({ onUploadSuccess, hasExistingPdf = false }: PDFUploadProps) {
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
      const firstFile = files[0];
      const firstResponse = await uploadPDF(firstFile);
      onUploadSuccess(firstResponse, firstFile);
      setUploadProgress(1);

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
    <div className="w-full max-w-2xl mx-auto p-2">
      {/* Confirmation Dialog */}
      {showConfirmDialog && (
        <div className="fixed inset-0 bg-black/40 backdrop-blur-sm flex items-center justify-center z-50">
          <div className="backdrop-blur-xl bg-white/80 dark:bg-gray-900/80 border border-white/60 dark:border-white/10 rounded-3xl p-6 max-w-md mx-4 shadow-2xl">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
              Replace Current PDF?
            </h3>
            <p className="text-gray-600 dark:text-gray-400 text-sm mb-5">
              You already have a PDF loaded. Uploading a new one will replace it and clear your chat history.
            </p>
            <div className="flex justify-end space-x-3">
              <button
                onClick={cancelReplace}
                className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 backdrop-blur-sm bg-white/60 dark:bg-white/5 border border-white/60 dark:border-white/10 rounded-xl hover:bg-white/80 transition-all duration-200"
              >
                Cancel
              </button>
              <button
                onClick={confirmReplace}
                className="px-4 py-2 text-sm font-semibold text-white bg-gradient-to-r from-blue-600 to-violet-600 hover:from-blue-500 hover:to-violet-500 rounded-xl shadow-lg shadow-blue-500/25 transition-all duration-200"
              >
                Replace PDF
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Dropzone */}
      <div
        className={`relative border-2 border-dashed rounded-3xl p-10 text-center transition-all duration-300 ${dragActive
          ? 'border-blue-400/70 bg-blue-50/60 dark:bg-blue-900/20 ring-2 ring-blue-500/30 shadow-xl shadow-blue-500/10'
          : files.length > 0
            ? 'border-violet-400/50 backdrop-blur-xl bg-white/60 dark:bg-white/5 border-solid shadow-xl'
            : 'border-white/50 dark:border-white/15 backdrop-blur-xl bg-white/50 dark:bg-white/5 hover:bg-white/65 dark:hover:bg-white/8 hover:border-blue-400/40 shadow-xl'
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
        <label htmlFor="pdf-upload" className="cursor-pointer flex flex-col items-center">
          {/* Icon */}
          <div className={`w-16 h-16 mb-5 rounded-2xl flex items-center justify-center shadow-lg transition-all duration-300 ${files.length > 0
            ? 'bg-gradient-to-br from-violet-500 to-blue-600 shadow-violet-500/30'
            : dragActive
              ? 'bg-gradient-to-br from-blue-500 to-violet-600 shadow-blue-500/30 scale-110'
              : 'bg-gradient-to-br from-blue-400/80 to-violet-500/80 shadow-blue-400/20'
            }`}>
            {files.length > 0 ? (
              <svg className="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            ) : (
              <svg className="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
              </svg>
            )}
          </div>

          <p className="text-lg font-semibold text-gray-800 dark:text-gray-100 mb-1.5">
            {files.length > 0
              ? `${files.length} file${files.length > 1 ? 's' : ''} selected`
              : dragActive
                ? 'Drop your PDFs here'
                : 'Drag & drop your PDFs here'}
          </p>
          <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
            {files.length > 0 ? 'Ready to upload' : 'or click to browse (Max 50MB each) · Select multiple with Ctrl/Shift'}
          </p>

          {files.length > 0 && (
            <div className="w-full max-h-24 overflow-y-auto space-y-1 mb-2">
              {files.map((f, i) => (
                <div key={i} className="text-xs text-gray-600 dark:text-gray-400 backdrop-blur-sm bg-white/50 dark:bg-white/5 border border-white/50 dark:border-white/10 rounded-lg px-3 py-1.5 text-left">
                  📄 {f.name}
                </div>
              ))}
            </div>
          )}

          {hasExistingPdf && files.length === 0 && (
            <p className="text-xs text-amber-600 dark:text-amber-400 mt-1 backdrop-blur-sm bg-amber-50/60 dark:bg-amber-900/20 border border-amber-200/40 dark:border-amber-700/30 px-3 py-1 rounded-full">
              ⚠️ Uploading will replace your current session
            </p>
          )}
        </label>

        {/* Upload button */}
        {files.length > 0 && (
          <div className="mt-5">
            <button
              onClick={handleUpload}
              disabled={uploading}
              className="group relative overflow-hidden px-8 py-3 bg-gradient-to-r from-blue-600 to-violet-600 hover:from-blue-500 hover:to-violet-500 disabled:from-blue-400 disabled:to-violet-400 text-white font-semibold rounded-2xl shadow-xl shadow-blue-500/30 hover:shadow-blue-500/50 transition-all duration-300 hover:scale-105 disabled:scale-100 border border-white/20"
            >
              <span className="relative z-10 flex items-center gap-2">
                {uploading ? (
                  <>
                    <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                    Uploading... ({uploadProgress}/{files.length})
                  </>
                ) : (
                  <>
                    Upload {files.length} PDF{files.length > 1 ? 's' : ''}
                    <svg className="w-4 h-4 group-hover:translate-x-1 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 8l4 4m0 0l-4 4m4-4H3" />
                    </svg>
                  </>
                )}
              </span>
              <div className="absolute inset-0 bg-white/10 opacity-0 group-hover:opacity-100 transition-opacity" />
            </button>
          </div>
        )}

        {error && (
          <div className="mt-4 p-3 backdrop-blur-sm bg-red-50/80 dark:bg-red-900/20 border border-red-300/50 dark:border-red-700/30 text-red-700 dark:text-red-400 rounded-xl text-sm">
            {error}
          </div>
        )}
      </div>
    </div>
  );
}
