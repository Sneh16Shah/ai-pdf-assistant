// AskMyPDF Extension — Popup Script
// Handles login/logout UI and settings

const $ = (sel) => document.querySelector(sel);

const el = {
    loading: $('#loading'),
    loginState: $('#login-state'),
    authState: $('#auth-state'),
    loginForm: $('#login-form'),
    loginBtn: $('#login-btn'),
    loginError: $('#login-error'),
    email: $('#email'),
    password: $('#password'),
    userName: $('#user-name'),
    userEmail: $('#user-email'),
    userAvatar: $('#user-avatar'),
    logoutBtn: $('#logout-btn'),
    settingsToggle: $('#settings-toggle'),
    settingsPanel: $('#settings-panel'),
    apiUrl: $('#api-url'),
    saveUrl: $('#save-url'),
};

// ─── Init ────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', init);

async function init() {
    // Load saved API URL
    const { apiUrl } = await chrome.runtime.sendMessage({ type: 'GET_API_URL' });
    if (apiUrl) el.apiUrl.value = apiUrl;

    // Check auth
    const auth = await chrome.runtime.sendMessage({ type: 'GET_AUTH' });
    if (auth.authenticated) {
        showAuthState(auth.user);
    } else {
        showLoginState();
    }

    // ── Event listeners ──
    el.loginForm.addEventListener('submit', handleLogin);
    el.logoutBtn.addEventListener('click', handleLogout);
    el.settingsToggle.addEventListener('click', () => {
        el.settingsPanel.classList.toggle('hidden');
    });
    el.saveUrl.addEventListener('click', handleSaveUrl);
}

// ─── Login ───────────────────────────────────────────────────────
async function handleLogin(e) {
    e.preventDefault();
    el.loginError.classList.add('hidden');
    el.loginBtn.disabled = true;
    el.loginBtn.textContent = 'Signing in...';

    try {
        const result = await chrome.runtime.sendMessage({
            type: 'LOGIN',
            email: el.email.value,
            password: el.password.value,
        });

        if (result.error) throw new Error(result.error);
        showAuthState(result.user);
    } catch (err) {
        el.loginError.textContent = err.message || 'Login failed';
        el.loginError.classList.remove('hidden');
    } finally {
        el.loginBtn.disabled = false;
        el.loginBtn.textContent = 'Sign In';
    }
}

// ─── Logout ──────────────────────────────────────────────────────
async function handleLogout() {
    await chrome.runtime.sendMessage({ type: 'LOGOUT' });
    showLoginState();
}

// ─── Save API URL ────────────────────────────────────────────────
async function handleSaveUrl() {
    const url = el.apiUrl.value.trim();
    if (!url) return;
    await chrome.runtime.sendMessage({ type: 'SET_API_URL', apiUrl: url });
    el.saveUrl.textContent = '✓ Saved';
    setTimeout(() => { el.saveUrl.textContent = 'Save'; }, 1500);
}

// ─── UI State Switches ──────────────────────────────────────────
function showLoginState() {
    el.loading.classList.add('hidden');
    el.authState.classList.add('hidden');
    el.loginState.classList.remove('hidden');
}

function showAuthState(user) {
    el.loading.classList.add('hidden');
    el.loginState.classList.add('hidden');
    el.authState.classList.remove('hidden');

    const name = user?.name || user?.email?.split('@')[0] || 'User';
    el.userName.textContent = name;
    el.userEmail.textContent = user?.email || '';
    el.userAvatar.textContent = name.charAt(0).toUpperCase();
}
