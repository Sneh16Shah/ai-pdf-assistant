// AskMyPDF Extension — Chat Interface Script (Side Panel)
// Runs inside Chrome's Side Panel. Handles auth, PDF upload, and chat.
// Communicates with background + content scripts via chrome.runtime messaging.

(function () {
    'use strict';

    // ─── State ──────────────────────────────────────────────────────
    let sessionId = null;
    let authToken = null;
    let loading = false;
    let currentTabId = null;
    const messages = [];

    // ─── DOM refs ───────────────────────────────────────────────────
    const authScreen = document.getElementById('auth-screen');
    const noPdfScreen = document.getElementById('no-pdf-screen');
    const messagesEl = document.getElementById('messages');
    const emptyState = document.getElementById('empty-state');
    const statusMsg = document.getElementById('status-msg');
    const statusText = document.getElementById('status-text');
    const chatForm = document.getElementById('chat-form');
    const chatInput = document.getElementById('chat-input');
    const sendBtn = document.getElementById('send-btn');
    const docInfo = document.getElementById('doc-info');

    // Auth refs
    const loginForm = document.getElementById('login-form');
    const authEmail = document.getElementById('auth-email');
    const authPassword = document.getElementById('auth-password');
    const authError = document.getElementById('auth-error');
    const authSubmit = document.getElementById('auth-submit');
    const settingsToggle = document.getElementById('settings-toggle');
    const settingsPanel = document.getElementById('settings-panel');
    const apiUrlInput = document.getElementById('api-url');
    const saveUrlBtn = document.getElementById('save-url');

    // ─── Init ───────────────────────────────────────────────────────
    chatForm.addEventListener('submit', handleSubmit);
    loginForm.addEventListener('submit', handleLogin);
    settingsToggle.addEventListener('click', () => {
        settingsPanel.classList.toggle('hidden');
    });
    saveUrlBtn.addEventListener('click', handleSaveUrl);

    // Start the flow
    initAuth();

    // ─── Auth ───────────────────────────────────────────────────────
    async function initAuth() {
        try {
            // Load saved API URL
            const urlResult = await chrome.runtime.sendMessage({ type: 'GET_API_URL' });
            if (urlResult.apiUrl) apiUrlInput.value = urlResult.apiUrl;

            // Check auth
            const auth = await chrome.runtime.sendMessage({ type: 'GET_AUTH' });
            if (auth.authenticated) {
                authToken = auth.token;
                showChatScreen();
                checkForPDF();
            } else {
                showAuthScreen();
            }
        } catch (err) {
            console.error('Auth check failed:', err);
            showAuthScreen();
        }
    }

    async function handleLogin(e) {
        e.preventDefault();
        authError.classList.add('hidden');
        authSubmit.disabled = true;
        authSubmit.textContent = 'Signing in...';

        try {
            const result = await chrome.runtime.sendMessage({
                type: 'LOGIN',
                email: authEmail.value,
                password: authPassword.value,
            });

            if (result.error) throw new Error(result.error);

            authToken = result.token || true;
            showChatScreen();
            checkForPDF();
        } catch (err) {
            authError.textContent = err.message || 'Login failed';
            authError.classList.remove('hidden');
        } finally {
            authSubmit.disabled = false;
            authSubmit.textContent = 'Sign In';
        }
    }

    async function handleSaveUrl() {
        const url = apiUrlInput.value.trim();
        if (!url) return;
        await chrome.runtime.sendMessage({ type: 'SET_API_URL', apiUrl: url });
        saveUrlBtn.textContent = '✓ Saved';
        setTimeout(() => { saveUrlBtn.textContent = 'Save'; }, 1500);
    }

    // ─── Screen management ──────────────────────────────────────────
    function showAuthScreen() {
        authScreen.classList.remove('hidden');
        noPdfScreen.classList.add('hidden');
        messagesEl.classList.add('hidden');
        chatForm.classList.add('hidden');
    }

    function showNoPDFScreen() {
        authScreen.classList.add('hidden');
        noPdfScreen.classList.remove('hidden');
        messagesEl.classList.add('hidden');
        chatForm.classList.add('hidden');
    }

    function showChatScreen() {
        authScreen.classList.add('hidden');
        noPdfScreen.classList.add('hidden');
        messagesEl.classList.remove('hidden');
        chatForm.classList.remove('hidden');
    }

    // ─── PDF Detection & Upload ─────────────────────────────────────
    async function checkForPDF() {
        statusText.textContent = 'Looking for PDF...';
        statusMsg.classList.remove('hidden', 'error');

        try {
            // Ask background for the active tab
            const tabInfo = await chrome.runtime.sendMessage({ type: 'GET_ACTIVE_TAB' });
            if (!tabInfo || !tabInfo.tabId) {
                showNoPDFScreen();
                return;
            }

            currentTabId = tabInfo.tabId;

            // Ask the content script in the active tab if it has a PDF
            try {
                const response = await chrome.tabs.sendMessage(currentTabId, { type: 'REQUEST_UPLOAD' });

                if (response && response.session_id) {
                    // Already uploaded
                    sessionId = response.session_id;
                    statusMsg.classList.add('hidden');
                    chatInput.disabled = false;
                    sendBtn.disabled = false;
                    if (response.filename) {
                        docInfo.textContent = response.filename;
                        docInfo.classList.remove('hidden');
                    }
                    loadHistory();
                } else if (response && response.error && response.error !== 'Upload already in progress') {
                    // Need to upload — content script will handle it
                    statusText.textContent = 'Uploading PDF...';
                }
            } catch {
                // Content script not available — not a PDF page
                showNoPDFScreen();
            }
        } catch (err) {
            console.error('PDF check failed:', err);
            showNoPDFScreen();
        }
    }

    // ─── Listen for messages from content script / background ────────
    chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
        if (msg.type === 'SELECTED_TEXT_FROM_BG') {
            if (msg.text && sessionId && !loading) {
                const truncated = msg.text.length > 300
                    ? msg.text.substring(0, 300) + '...'
                    : msg.text;
                const prompt = `"${truncated}"\n\nExplain this.`;
                sendChatMessage(prompt);
            }
            sendResponse({ received: true });
        }

        if (msg.type === 'PDF_UPLOADED') {
            sessionId = msg.sessionId;
            statusMsg.classList.add('hidden');
            chatInput.disabled = false;
            sendBtn.disabled = false;
            if (msg.filename) {
                const label = msg.pages
                    ? `${msg.filename} • ${msg.pages} pages`
                    : msg.filename;
                docInfo.textContent = label;
                docInfo.classList.remove('hidden');
            }
            loadHistory();
            sendResponse({ received: true });
        }
    });

    // ─── Load Chat History ──────────────────────────────────────────
    async function loadHistory() {
        if (!sessionId) return;
        try {
            const result = await chrome.runtime.sendMessage({ type: 'GET_HISTORY', sessionId });
            if (result.messages && result.messages.length > 0) {
                emptyState.classList.add('hidden');
                result.messages.forEach((m) => {
                    messages.push({ role: m.role, content: m.content });
                });
                renderAllMessages();
            }
        } catch {
            // No history, that's fine
        }
    }

    // ─── Send Message ───────────────────────────────────────────────
    async function handleSubmit(e) {
        e.preventDefault();
        const text = chatInput.value.trim();
        if (!text || loading || !sessionId) return;
        chatInput.value = '';
        sendChatMessage(text);
    }

    async function sendChatMessage(text) {
        if (!text || loading || !sessionId) return;

        addMessage('user', text);
        setLoading(true);

        try {
            const result = await chrome.runtime.sendMessage({
                type: 'SEND_MESSAGE',
                sessionId,
                message: text,
            });

            if (result.error) throw new Error(result.error);

            addMessage('assistant', result.response);

            if (result.citations && result.citations.length > 0) {
                renderCitations(result.citations);
            }

            if (result.image_base64 && result.image_mime_type) {
                renderAIImage(result.image_base64, result.image_mime_type);
            }
        } catch (err) {
            addMessage('assistant', `Error: ${err.message || 'Failed to get response'}`);
        } finally {
            setLoading(false);
        }
    }

    // ─── Helpers ────────────────────────────────────────────────────
    function addMessage(role, content) {
        messages.push({ role, content });
        emptyState.classList.add('hidden');
        appendMessageEl(role, content);
        scrollToBottom();
    }

    function appendMessageEl(role, content) {
        const div = document.createElement('div');
        div.className = `message ${role}`;

        const bubble = document.createElement('div');
        bubble.className = 'message-bubble';

        if (role === 'assistant') {
            bubble.innerHTML = renderMarkdown(content);
        } else {
            bubble.textContent = content;
        }

        div.appendChild(bubble);
        messagesEl.appendChild(div);
    }

    function renderAllMessages() {
        const renders = messagesEl.querySelectorAll('.message, .loading-dots, .citations, .ai-image-block');
        renders.forEach((el) => el.remove());
        messages.forEach((m) => appendMessageEl(m.role, m.content));
        scrollToBottom();
    }

    function renderCitations(citations) {
        const container = document.createElement('div');
        container.className = 'citations';

        const label = document.createElement('span');
        label.className = 'citations-label';
        label.textContent = 'Sources:';
        container.appendChild(label);

        citations.forEach((c) => {
            const pill = document.createElement('button');
            pill.className = 'citation-pill';
            pill.innerHTML = `
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
        </svg>
        Page ${c.page}
      `;
            pill.title = c.text || '';
            container.appendChild(pill);
        });

        messagesEl.appendChild(container);
        scrollToBottom();
    }

    function renderAIImage(base64, mimeType) {
        const block = document.createElement('div');
        block.className = 'ai-image-block';
        block.innerHTML = `
      <div class="ai-image-label">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"/>
        </svg>
        AI-Generated Diagram
      </div>
      <img src="data:${mimeType};base64,${base64}" alt="AI-generated diagram">
    `;
        messagesEl.appendChild(block);
        scrollToBottom();
    }

    function setLoading(state) {
        loading = state;
        chatInput.disabled = state || !sessionId;
        sendBtn.disabled = state || !chatInput.value.trim();

        const existing = messagesEl.querySelector('.loading-dots');
        if (state && !existing) {
            const dots = document.createElement('div');
            dots.className = 'loading-dots';
            dots.innerHTML = '<div class="dot"></div><div class="dot"></div><div class="dot"></div>';
            messagesEl.appendChild(dots);
            scrollToBottom();
        } else if (!state && existing) {
            existing.remove();
        }
    }

    function scrollToBottom() {
        requestAnimationFrame(() => {
            messagesEl.scrollTop = messagesEl.scrollHeight;
        });
    }

    function renderMarkdown(text) {
        if (typeof marked !== 'undefined' && marked.parse) {
            try {
                return marked.parse(text);
            } catch {
                return escapeHtml(text);
            }
        }
        return escapeHtml(text);
    }

    function escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML.replace(/\n/g, '<br>');
    }

    // Enable/disable send button based on input
    chatInput.addEventListener('input', () => {
        sendBtn.disabled = loading || !chatInput.value.trim() || !sessionId;
    });
})();
