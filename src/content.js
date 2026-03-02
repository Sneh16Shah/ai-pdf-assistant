// AskMyPDF Extension — Content Script
// Lightweight: detects PDFs, provides FAB, and extracts selected text from PDF viewer.
// The chat panel is now handled by Chrome's Side Panel API.

(function () {
    'use strict';

    if (window.__askmypdf_injected) return;
    window.__askmypdf_injected = true;

    // ─── PDF Detection ──────────────────────────────────────────────
    function isPDFPage() {
        if (document.contentType === 'application/pdf') return true;
        if (location.href.match(/\.pdf(\?.*)?$/i)) return true;
        if (document.querySelector('embed[type="application/pdf"]')) return true;
        if (document.querySelector('object[type="application/pdf"]')) return true;
        return false;
    }

    if (!isPDFPage()) return;

    // Notify the background script that we're on a PDF page
    const urlPath = new URL(location.href).pathname;
    const filename = urlPath.split('/').pop() || 'document.pdf';

    chrome.runtime.sendMessage({
        type: 'PDF_DETECTED',
        url: location.href,
        filename: filename,
        tabId: null, // background will know the sender tab
    });

    // ─── State ──────────────────────────────────────────────────────
    let sessionId = null;
    let isUploading = false;
    let uploadAttempts = 0;
    let uploadFailed = false;
    const MAX_UPLOAD_ATTEMPTS = 2;

    // ─── Create FAB (Ask AI button) ─────────────────────────────────
    const fab = document.createElement('div');
    fab.id = 'askmypdf-fab';
    fab.innerHTML = `
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/>
    </svg>
    <span>Ask AI</span>
  `;
    fab.addEventListener('click', () => {
        // MUST call OPEN_SIDE_PANEL synchronously first — user gesture is lost after await
        chrome.runtime.sendMessage({ type: 'OPEN_SIDE_PANEL' });

        // NOTE: Do NOT upload here — the side panel's checkForPDF() will trigger
        // REQUEST_UPLOAD after the user is authenticated. This ensures the auth
        // token is always sent, so the backend persists the session to the DB.

        // Try to get selected text and send it (async, after panel opens)
        getSelectedTextFromPDF().then((selectedText) => {
            if (selectedText && sessionId) {
                chrome.runtime.sendMessage({
                    type: 'SELECTED_TEXT_FROM_BG',
                    text: selectedText,
                });
            }
        });
    });
    document.documentElement.appendChild(fab);

    // ─── Inject PDF Helper Script (page context) ────────────────────
    // This script can communicate with the PDF embed's internal API
    const helperScript = document.createElement('script');
    helperScript.src = chrome.runtime.getURL('src/pdf-helper.js');
    document.documentElement.appendChild(helperScript);

    // ─── Get Selected Text from PDF ─────────────────────────────────
    function getSelectedTextFromPDF() {
        return new Promise((resolve) => {
            // First try standard DOM selection (works for HTML pages with PDF.js)
            const sel = window.getSelection();
            const text = sel?.toString().trim();
            if (text && text.length > 2) {
                resolve(text);
                return;
            }

            // Try the embed's internal API via postMessage
            const embed = document.querySelector('embed[type="application/pdf"]');
            if (!embed) {
                resolve(null);
                return;
            }

            // Listen for the reply
            const timeout = setTimeout(() => {
                window.removeEventListener('message', handler);
                resolve(null);
            }, 500);

            function handler(e) {
                if (e.data && e.data.type === 'askmypdf-selectedTextReply') {
                    clearTimeout(timeout);
                    window.removeEventListener('message', handler);
                    const selectedText = e.data.selectedText?.trim();
                    resolve(selectedText && selectedText.length > 0 ? selectedText : null);
                }
            }

            window.addEventListener('message', handler);

            // Ask the helper script to query the embed
            window.postMessage({
                type: 'askmypdf-getSelectedText',
            }, '*');
        });
    }

    // ─── Listen for messages from background/side panel ──────────────
    chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
        if (msg.type === 'GET_PDF_URL') {
            let pdfUrl = location.href;
            const embed = document.querySelector('embed[type="application/pdf"]');
            if (embed && embed.src) pdfUrl = embed.src;
            const obj = document.querySelector('object[type="application/pdf"]');
            if (obj && obj.data) pdfUrl = obj.data;
            sendResponse({ url: pdfUrl, filename });
        }

        if (msg.type === 'REQUEST_UPLOAD') {
            if (uploadFailed) {
                sendResponse({ error: 'Upload failed after multiple attempts. Please reload the page to try again.' });
            } else if (!sessionId && !isUploading) {
                uploadCurrentPDF().then((result) => {
                    sendResponse(result);
                }).catch((err) => {
                    sendResponse({ error: err.message });
                });
                return true; // async response
            } else if (sessionId) {
                sendResponse({ session_id: sessionId, filename });
            } else {
                sendResponse({ error: 'Upload already in progress' });
            }
        }

        if (msg.type === 'GET_SELECTED_TEXT') {
            getSelectedTextFromPDF().then((text) => {
                sendResponse({ text: text || '' });
            });
            return true; // async
        }
    });

    // ─── Upload PDF ─────────────────────────────────────────────────
    async function uploadCurrentPDF() {
        if (uploadAttempts >= MAX_UPLOAD_ATTEMPTS) {
            uploadFailed = true;
            chrome.runtime.sendMessage({
                type: 'UPLOAD_FAILED',
                error: 'Upload failed after multiple attempts. Please reload the page.',
            }).catch(() => { });
            throw new Error('Max upload attempts reached');
        }

        uploadAttempts++;
        isUploading = true;
        try {
            let pdfUrl = location.href;
            const embed = document.querySelector('embed[type="application/pdf"]');
            if (embed && embed.src) pdfUrl = embed.src;
            const obj = document.querySelector('object[type="application/pdf"]');
            if (obj && obj.data) pdfUrl = obj.data;

            // Send URL to background — it fetches directly with host_permissions
            const result = await chrome.runtime.sendMessage({
                type: 'UPLOAD_PDF',
                pdfUrl: pdfUrl,
                filename: filename,
            });

            if (result.error) throw new Error(result.error);

            sessionId = result.session_id;

            // Notify background about the session
            await chrome.runtime.sendMessage({
                type: 'PDF_UPLOADED',
                sessionId: sessionId,
                filename: result.filename || filename,
                pages: result.pages,
            });

            return result;
        } catch (err) {
            console.error(`PDF upload attempt ${uploadAttempts}/${MAX_UPLOAD_ATTEMPTS} failed:`, err);
            if (uploadAttempts >= MAX_UPLOAD_ATTEMPTS) {
                uploadFailed = true;
                chrome.runtime.sendMessage({
                    type: 'UPLOAD_FAILED',
                    error: `Upload failed: ${err.message}`,
                }).catch(() => { });
            }
            throw err;
        } finally {
            isUploading = false;
        }
    }
})();
