/* ─────────────────────────────────────────────────────────────────────────
   VelocityPay — API Client
   All HTTP calls to the backend go through this module.
   ───────────────────────────────────────────────────────────────────────── */

const API_BASE = window.location.hostname === 'localhost'
  ? 'http://localhost:8080/api/v1'
  : '/api/v1';

const Api = (() => {

  // ── Token management ──────────────────────────────────────────────────────
  const getToken  = () => localStorage.getItem('vp_token');
  const setToken  = (t) => localStorage.setItem('vp_token', t);
  const clearToken = () => localStorage.removeItem('vp_token');

  const getUser   = () => {
    try { return JSON.parse(localStorage.getItem('vp_user') || 'null'); }
    catch { return null; }
  };
  const setUser  = (u) => localStorage.setItem('vp_user', JSON.stringify(u));
  const clearUser = () => localStorage.removeItem('vp_user');

  // ── Core fetch wrapper ────────────────────────────────────────────────────
  async function request(method, path, body = null) {
    const headers = { 'Content-Type': 'application/json' };
    const token = getToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const opts = { method, headers };
    if (body) opts.body = JSON.stringify(body);

    const res = await fetch(`${API_BASE}${path}`, opts);
    const json = await res.json();

    if (res.status === 401) {
      // Token expired or invalid — clear session and redirect to login
      clearToken();
      clearUser();
      const isAuthPage = window.location.pathname.includes('login') ||
                         window.location.pathname.includes('register');
      if (!isAuthPage) {
        window.location.href = '/web/pages/login.html?session=expired';
      }
      throw new Error('Session expired. Please sign in again.');
    }

    if (!res.ok) {
      const msg = json?.error?.message || 'Something went wrong';
      throw new Error(msg);
    }
    return json.data ?? json;
  }

  // ── Auth ──────────────────────────────────────────────────────────────────
  async function register(payload) {
    const data = await request('POST', '/auth/register', payload);
    setToken(data.access_token);
    setUser(data.user);
    return data;
  }

  async function login(email, password) {
    const data = await request('POST', '/auth/login', { email, password });
    setToken(data.access_token);
    setUser(data.user);
    return data;
  }

  function logout() {
    clearToken();
    clearUser();
    window.location.href = '/web/pages/login.html';
  }

  // ── Users ─────────────────────────────────────────────────────────────────
  const getProfile     = () => request('GET', '/users/profile');
  const updateProfile  = (p) => request('PUT', '/users/profile', p);
  const changePassword = (p) => request('PUT', '/users/change-password', p);

  // ── Wallet ────────────────────────────────────────────────────────────────
  const createWallet = (currency) => request('POST', '/wallet/create', { currency });
  const addMoney     = (amount, notes) => request('POST', '/wallet/add-money', { amount, notes });
  const getBalance   = () => request('GET', '/wallet/balance');
  const getWalletDetails = () => request('GET', '/wallet/details');

  // ── Transactions ──────────────────────────────────────────────────────────
  const transfer = (payload) => request('POST', '/transactions/transfer', payload);
  const getHistory = (params = {}) => {
    const qs = new URLSearchParams(params).toString();
    return request('GET', `/transactions/history${qs ? '?' + qs : ''}`);
  };
  const getTransaction = (id) => request('GET', `/transactions/${id}`);

  // ── Refunds ───────────────────────────────────────────────────────────────
  const requestRefund  = (payload) => request('POST', '/refunds', payload);
  const getMyRefunds   = () => request('GET', '/refunds');
  const getRefund      = (id) => request('GET', `/refunds/${id}`);

  // ── Notifications ─────────────────────────────────────────────────────────
  const getNotifications = () => request('GET', '/notifications');
  const markRead         = (id) => request('PUT', `/notifications/${id}/read`);
  const markAllRead      = () => request('PUT', '/notifications/read-all');

  // ── Analytics ─────────────────────────────────────────────────────────────
  const getDashboard    = () => request('GET', '/analytics/dashboard');
  const getUserStats    = () => request('GET', '/analytics/stats');
  const getMonthly      = (months = 6) => request('GET', `/analytics/monthly?months=${months}`);
  const getDaily        = (days = 30)  => request('GET', `/analytics/daily?days=${days}`);
  const getPlatformStats = () => request('GET', '/analytics/platform');
  const getTopSenders   = (limit = 10) => request('GET', `/analytics/top-senders?limit=${limit}`);

  // ── Audit ─────────────────────────────────────────────────────────────────
  const getMyAuditLogs = (page = 1) => request('GET', `/audit/me?page=${page}&page_size=20`);

  // ── Fraud ─────────────────────────────────────────────────────────────────
  const getMyFraudAlerts = () => request('GET', '/fraud/alerts/me');

  // ── Auth guard ────────────────────────────────────────────────────────────
  function requireAuth() {
    if (!getToken()) {
      window.location.href = '/web/pages/login.html';
      return false;
    }
    return true;
  }

  return {
    getToken, setToken, clearToken,
    getUser, setUser, clearUser,
    requireAuth, logout,
    register, login,
    getProfile, updateProfile, changePassword,
    createWallet, addMoney, getBalance, getWalletDetails,
    transfer, getHistory, getTransaction,
    requestRefund, getMyRefunds, getRefund,
    getNotifications, markRead, markAllRead,
    getDashboard, getUserStats, getMonthly, getDaily,
    getPlatformStats, getTopSenders,
    getMyAuditLogs,
    getMyFraudAlerts,
  };
})();
