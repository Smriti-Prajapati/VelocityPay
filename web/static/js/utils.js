/* ─────────────────────────────────────────────────────────────────────────
   VelocityPay — UI Utilities  (premium edition)
   ───────────────────────────────────────────────────────────────────────── */

// ── Toast ──────────────────────────────────────────────────────────────────
(function() {
  const container = document.createElement('div');
  container.className = 'toast-container';
  document.body.appendChild(container);
  window._toastContainer = container;
})();

function showToast(message, type = 'success', duration = 3500) {
  const icons = {
    success: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>`,
    error:   `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`,
    info:    `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`,
  };
  const t = document.createElement('div');
  t.className = `toast ${type}`;
  t.innerHTML = `${icons[type] || icons.info}<span>${message}</span>`;
  window._toastContainer.appendChild(t);
  setTimeout(() => {
    t.style.transition = 'all .3s';
    t.style.opacity = '0';
    t.style.transform = 'translateX(12px)';
    setTimeout(() => t.remove(), 320);
  }, duration);
}

// ── Money ──────────────────────────────────────────────────────────────────
function formatMoney(amount, currency = 'INR') {
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency, minimumFractionDigits: 2 }).format(amount ?? 0);
}
function formatMoneyShort(amount) {
  const n = amount ?? 0;
  if (n >= 10000000) return '₹' + (n / 10000000).toFixed(1) + 'Cr';
  if (n >= 100000)   return '₹' + (n / 100000).toFixed(1) + 'L';
  if (n >= 1000)     return '₹' + (n / 1000).toFixed(1) + 'K';
  return '₹' + n.toFixed(2);
}

// ── Dates ──────────────────────────────────────────────────────────────────
function formatDate(iso) {
  if (!iso) return '—';
  return new Date(iso).toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' });
}
function formatDateTime(iso) {
  if (!iso) return '—';
  return new Date(iso).toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}
function timeAgo(iso) {
  if (!iso) return '';
  const diff = Date.now() - new Date(iso).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1)  return 'just now';
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

// ── Avatar initials ────────────────────────────────────────────────────────
function getInitials(name = '') {
  return name.trim().split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2) || 'VP';
}

// ── Badge ──────────────────────────────────────────────────────────────────
function statusBadge(status) {
  const map = { completed:'badge-completed', pending:'badge-pending', failed:'badge-failed', reversed:'badge-reversed' };
  return `<span class="badge ${map[status]||'badge-pending'}">${status}</span>`;
}

// ── Button loader ──────────────────────────────────────────────────────────
function showLoader(btn) {
  if (!btn) return;
  btn._orig = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = `<span class="spinner"></span>`;
}
function hideLoader(btn) {
  if (!btn) return;
  btn.disabled = false;
  btn.innerHTML = btn._orig || 'Submit';
}

// ── Scroll reveal ──────────────────────────────────────────────────────────
function initReveal() {
  const els = document.querySelectorAll('.reveal');
  const obs = new IntersectionObserver((entries) => {
    entries.forEach(e => { if (e.isIntersecting) { e.target.classList.add('visible'); obs.unobserve(e.target); } });
  }, { threshold: 0.12 });
  els.forEach(el => obs.observe(el));
}

// ── Number counter animation ───────────────────────────────────────────────
function animateCounter(el, target, duration = 1800, prefix = '', suffix = '') {
  const start = Date.now();
  const tick = () => {
    const elapsed = Date.now() - start;
    const progress = Math.min(elapsed / duration, 1);
    const eased = 1 - Math.pow(1 - progress, 3);
    el.textContent = prefix + Math.floor(eased * target).toLocaleString('en-IN') + suffix;
    if (progress < 1) requestAnimationFrame(tick);
  };
  requestAnimationFrame(tick);
}

// ── Sidebar mobile ─────────────────────────────────────────────────────────
function initSidebar() {
  const toggle  = document.getElementById('hamburger');
  const sidebar = document.getElementById('sidebar');
  const overlay = document.getElementById('sidebarOverlay');
  if (!toggle || !sidebar) return;
  toggle.addEventListener('click', () => {
    sidebar.classList.toggle('open');
    overlay && overlay.classList.toggle('show');
  });
  overlay && overlay.addEventListener('click', () => {
    sidebar.classList.remove('open');
    overlay.classList.remove('show');
  });
}

// ── Populate topbar & sidebar user ─────────────────────────────────────────
function populateUser() {
  const user = Api.getUser();
  if (!user) return;
  const init = getInitials(user.name);
  document.querySelectorAll('[data-user-avatar]').forEach(el => el.textContent = init);
  document.querySelectorAll('[data-user-name]').forEach(el => el.textContent = user.name);
  document.querySelectorAll('[data-user-email]').forEach(el => el.textContent = user.email);
}

// ── Pagination ─────────────────────────────────────────────────────────────
function renderPagination(containerId, page, totalPages, cb) {
  const el = document.getElementById(containerId);
  if (!el || totalPages <= 1) { if (el) el.innerHTML = ''; return; }
  let html = `<div style="display:flex;gap:6px;align-items:center;justify-content:center;margin-top:20px">`;
  const btn = (p, label, disabled, active) =>
    `<button onclick="${active||disabled?'':`(${cb})(${p})`}" style="min-width:34px;height:34px;border-radius:8px;border:1.5px solid ${active?'var(--primary)':'var(--border)'};background:${active?'var(--primary)':'transparent'};color:${active?'#fff':'var(--sub)'};font-size:13px;font-weight:600;cursor:${disabled?'default':'pointer'};opacity:${disabled?.4:1}">${label}</button>`;
  html += btn(page-1,'←', page===1, false);
  for (let i=1; i<=totalPages; i++) html += btn(i, i, false, i===page);
  html += btn(page+1,'→', page===totalPages, false);
  html += '</div>';
  el.innerHTML = html;
}

document.addEventListener('DOMContentLoaded', () => { initReveal(); initSidebar(); populateUser(); });
window.addEventListener('scroll', initReveal);
