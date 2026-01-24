const TOKEN_KEY = "admin_token";

/* ---------------- TOKEN ---------------- */

function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
}

function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

/* ---------------- REDIRECT ---------------- */

function redirectToLogin() {
  window.location.replace("/admin/login");
}

function redirectToCategories() {
  window.location.replace("/admin/categories");
}

/* ---------------- AUTH GUARDS ---------------- */

// SADECE login.html'de çağrılacak
function redirectIfAuthenticated() {
  const token = getToken();
  if (!token) return;

  if (window.location.pathname === "/admin/login") {
    redirectToCategories();
  }
}

// SADECE admin sayfalarında çağrılacak
function requireAuth() {
  const token = getToken();
  if (!token) {
    redirectToLogin();
  }
}

/* ---------------- HELPERS ---------------- */

function authHeaders() {
  return {
    "Content-Type": "application/json",
    "Authorization": "Bearer " + getToken(),
  };
}

function authHeadersMultipart() {
  return {
    "Authorization": "Bearer " + getToken(),
  };
}

async function safeJson(res) {
  try {
    return await res.json();
  } catch {
    return null;
  }
}

function getId(obj) {
  return obj && (obj._id || obj.id) ? (obj._id || obj.id) : null;
}

function handleUnauthorized(res) {
  if (res && res.status === 401) {
    clearToken();
    redirectToLogin();
    return true;
  }
  return false;
}

/* ---------------- UI ---------------- */

function setText(id, text) {
  const el = document.getElementById(id);
  if (el) el.innerText = text || "";
}

function logout() {
  clearToken();
  redirectToLogin();
}

function bindLogout() {
  const btn = document.getElementById("logoutButton");
  if (btn) btn.addEventListener("click", logout);
}

document.addEventListener("DOMContentLoaded", bindLogout);
