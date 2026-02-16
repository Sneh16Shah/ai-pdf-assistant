// AskMyPDF Chrome Extension — Background Service Worker
// Manages auth, API communication, PDF upload, Side Panel, and context menus.

const DEFAULT_API_URL = 'http://localhost:8081/api/v1';

// ─── Side Panel Setup ───────────────────────────────────────────────
// Click the toolbar icon → open the side panel
chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true })
    .catch((err) => console.error('setPanelBehavior error:', err));

// Disable side panel globally by default — only enable on PDF tabs
chrome.sidePanel.setOptions({ enabled: false })
    .catch(() => { });

// Track which tabs have PDFs so we can enable/disable the side panel
const pdfTabs = new Set();
// Track which tabs currently have the side panel open (for toggle)
const openPanelTabs = new Set();

// ─── Tab-specific Side Panel ────────────────────────────────────────
// When tabs update, check if they're PDFs and enable/disable side panel
chrome.tabs.onUpdated.addListener(async (tabId, changeInfo, tab) => {
    if (changeInfo.status !== 'complete' || !tab.url) return;

    const isPDF = tab.url.endsWith('.pdf') ||
        tab.url.includes('.pdf?') ||
        tab.url.startsWith('file://') && tab.url.endsWith('.pdf') ||
        (tab.title && tab.title.endsWith('.pdf'));

    if (isPDF || pdfTabs.has(tabId)) {
        // Will be confirmed by content script's PDF_DETECTED message
    }
});

// When tab closes, clean up
chrome.tabs.onRemoved.addListener((tabId) => {
    pdfTabs.delete(tabId);
    openPanelTabs.delete(tabId);
});

// When switching tabs, the panel visibility is handled by Chrome automatically
// based on per-tab setOptions

// ─── Context Menu (right-click "Ask AI about this") ─────────────────
chrome.runtime.onInstalled.addListener(() => {
    chrome.contextMenus.create({
        id: 'askmypdf-ask-selection',
        title: 'Ask AI: "%s"',
        contexts: ['selection'],
    });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
    if (info.menuItemId === 'askmypdf-ask-selection' && info.selectionText) {
        chrome.sidePanel.open({ tabId: tab.id }).then(() => {
            setTimeout(() => {
                chrome.runtime.sendMessage({
                    type: 'SELECTED_TEXT_FROM_BG',
                    text: info.selectionText,
                }).catch(() => { });
            }, 500);
        }).catch((err) => console.error('sidePanel.open error:', err));
    }
});

// ─── Storage helpers ────────────────────────────────────────────────
async function getStorage(keys) {
    return chrome.storage.local.get(keys);
}

async function setStorage(obj) {
    return chrome.storage.local.set(obj);
}

// ─── API helpers ────────────────────────────────────────────────────
async function getApiUrl() {
    const { apiUrl } = await getStorage('apiUrl');
    return apiUrl || DEFAULT_API_URL;
}

async function getAuthToken() {
    const { authToken } = await getStorage('authToken');
    return authToken || null;
}

async function apiFetch(path, options = {}) {
    const apiUrl = await getApiUrl();
    const token = await getAuthToken();

    const headers = { ...options.headers };
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }
    if (!headers['Content-Type'] && !(options.body instanceof FormData)) {
        headers['Content-Type'] = 'application/json';
    }

    const response = await fetch(`${apiUrl}${path}`, { ...options, headers });
    if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `HTTP ${response.status}`);
    }
    return response.json();
}

// ─── Message handler ────────────────────────────────────────────────
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    // TOGGLE_SIDE_PANEL must be handled synchronously to preserve user gesture
    if (message.type === 'OPEN_SIDE_PANEL') {
        const tabId = sender.tab?.id;
        if (!tabId) {
            sendResponse({ error: 'No tab context' });
            return true;
        }

        if (openPanelTabs.has(tabId)) {
            // Panel is open → close it by disabling, then re-enable for next toggle
            openPanelTabs.delete(tabId);
            chrome.sidePanel.setOptions({ tabId, enabled: false })
                .then(() => chrome.sidePanel.setOptions({ tabId, path: 'src/chat-interface.html', enabled: true }))
                .then(() => sendResponse({ success: true, action: 'closed' }))
                .catch((err) => sendResponse({ error: err.message }));
        } else {
            // Panel is closed → open it
            openPanelTabs.add(tabId);
            chrome.sidePanel.open({ tabId })
                .then(() => sendResponse({ success: true, action: 'opened' }))
                .catch((err) => {
                    openPanelTabs.delete(tabId);
                    sendResponse({ error: err.message });
                });
        }
        return true;
    }

    handleMessage(message, sender).then(sendResponse).catch(err => {
        sendResponse({ error: err.message || String(err) });
    });
    return true; // keep channel open for async response
});

async function handleMessage(msg, sender) {
    switch (msg.type) {
        // ── Auth ──
        case 'LOGIN': {
            const { email, password } = msg;
            const data = await apiFetch('/auth/login', {
                method: 'POST',
                body: JSON.stringify({ email, password }),
            });
            await setStorage({ authToken: data.token, user: data.user });
            return { success: true, user: data.user };
        }

        case 'LOGOUT': {
            await chrome.storage.local.remove(['authToken', 'user']);
            return { success: true };
        }

        case 'GET_AUTH': {
            const { authToken, user } = await getStorage(['authToken', 'user']);
            if (!authToken) return { authenticated: false };
            try {
                const userData = await apiFetch('/auth/me');
                await setStorage({ user: userData });
                return { authenticated: true, user: userData, token: authToken };
            } catch {
                await chrome.storage.local.remove(['authToken', 'user']);
                return { authenticated: false };
            }
        }

        // ── Settings ──
        case 'SET_API_URL': {
            await setStorage({ apiUrl: msg.apiUrl });
            return { success: true };
        }

        case 'GET_API_URL': {
            return { apiUrl: await getApiUrl() };
        }

        // ── PDF Upload ──
        case 'UPLOAD_PDF': {
            const { pdfDataUrl, filename } = msg;
            const res = await fetch(pdfDataUrl);
            const blob = await res.blob();

            const formData = new FormData();
            formData.append('pdf', blob, filename || 'document.pdf');

            const apiUrl = await getApiUrl();
            const token = await getAuthToken();
            const headers = {};
            if (token) headers['Authorization'] = `Bearer ${token}`;

            const uploadRes = await fetch(`${apiUrl}/pdf/upload`, {
                method: 'POST',
                headers,
                body: formData,
            });

            if (!uploadRes.ok) {
                const text = await uploadRes.text();
                throw new Error(text || `Upload failed: HTTP ${uploadRes.status}`);
            }
            return uploadRes.json();
        }

        // ── Chat ──
        case 'SEND_MESSAGE': {
            const { sessionId, message, pageNumber } = msg;
            const body = { session_id: sessionId, message };
            if (pageNumber) body.page_number = pageNumber;
            return apiFetch('/chat/message', {
                method: 'POST',
                body: JSON.stringify(body),
            });
        }

        case 'GET_HISTORY': {
            const data = await apiFetch(`/chat/history/${msg.sessionId}`);
            return { messages: data.messages || [] };
        }

        // ── PDF Info (from content script → side panel) ──
        case 'PDF_DETECTED': {
            // Enable the side panel for this specific tab
            const tabId = sender?.tab?.id;
            if (tabId) {
                pdfTabs.add(tabId);
                await chrome.sidePanel.setOptions({
                    tabId,
                    path: 'src/chat-interface.html',
                    enabled: true,
                });
            }
            await setStorage({
                currentPDF: {
                    tabId,
                    url: msg.url,
                    filename: msg.filename,
                },
            });
            return { success: true };
        }

        case 'GET_CURRENT_PDF': {
            const { currentPDF } = await getStorage('currentPDF');
            return { pdf: currentPDF || null };
        }

        // ── Active Tab ──
        case 'GET_ACTIVE_TAB': {
            const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
            if (tab) {
                return { tabId: tab.id, url: tab.url };
            }
            return { tabId: null };
        }

        case 'PDF_UPLOADED': {
            return { success: true };
        }

        case 'SELECTED_TEXT_FROM_BG':
            return { received: true };

        default:
            throw new Error(`Unknown message type: ${msg.type}`);
    }
}
