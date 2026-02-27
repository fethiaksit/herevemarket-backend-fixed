requireAuth();

const ORDERS_API_URL = "/admin/api/orders";
const AUTO_REFRESH_MS = 12000;
const MOBILE_MEDIA_QUERY = "(max-width: 767px)";
const STATUS_VALUES = ["pending", "approved", "cancelled", "delivered"];

const STATUS_LABELS = {
  pending: "Beklemede",
  approved: "Onaylandı",
  cancelled: "İptal Edildi",
  delivered: "Teslim Edildi",
};

const PAYMENT_LABELS = {
  cash: "Nakit",
  card: "Kart",
};

const state = {
  page: 1,
  limit: 20,
  totalPages: 1,
  filters: { status: "", paymentMethod: "", search: "" },
  loading: false,
  statusSaving: false,
  detailOpen: false,
  autoRefreshTimer: null,
  isMobile: window.matchMedia(MOBILE_MEDIA_QUERY).matches,
};

function debounce(fn, wait) {
  let timer = null;
  return (...args) => {
    window.clearTimeout(timer);
    timer = window.setTimeout(() => fn(...args), wait);
  };
}

function getStatusLabel(status) {
  return STATUS_LABELS[(status || "").toLowerCase()] || "-";
}

function getPaymentLabel(method) {
  return PAYMENT_LABELS[(method || "").toLowerCase()] || "-";
}

function formatDateTime(value) {
  const date = value ? new Date(value) : null;
  if (!date || Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString("tr-TR");
}

function formatCurrency(value) {
  if (typeof value !== "number") return "-";
  return value.toLocaleString("tr-TR", { style: "currency", currency: "TRY", maximumFractionDigits: 2 });
}

function statusClass(status) {
  const key = (status || "").toLowerCase();
  return `hm-status hm-status-${STATUS_VALUES.includes(key) ? key : "pending"}`;
}

function currentQuery() {
  const params = new URLSearchParams();
  params.set("page", String(state.page));
  params.set("limit", String(state.limit));
  if (state.filters.status) params.set("status", state.filters.status);
  if (state.filters.paymentMethod) params.set("paymentMethod", state.filters.paymentMethod);
  if (state.filters.search) params.set("search", state.filters.search);
  return params.toString();
}

function updateLastRefreshed() {
  setText("ordersLastUpdated", `Son güncelleme: ${new Date().toLocaleTimeString("tr-TR")}`);
}

function itemQuantityTotal(items) {
  if (!Array.isArray(items)) return 0;
  return items.reduce((sum, item) => sum + (Number(item && item.quantity) || 0), 0);
}

function renderEmpty(message) {
  const tbody = document.getElementById("ordersTableBody");
  const cardList = document.getElementById("ordersCardList");
  if (tbody) {
    tbody.innerHTML = "";
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 8;
    cell.className = "muted";
    cell.textContent = message;
    row.appendChild(cell);
    tbody.appendChild(row);
  }
  if (cardList) cardList.innerHTML = `<p class="muted">${message}</p>`;
}

function orderActions(orderId, currentStatus) {
  const wrap = document.createElement("div");
  wrap.className = "hm-action-group";

  const detailButton = document.createElement("button");
  detailButton.type = "button";
  detailButton.className = "small ghost";
  detailButton.textContent = "Detay";
  detailButton.addEventListener("click", () => openDetailById(orderId));

  const statusSelect = document.createElement("select");
  statusSelect.className = "table-input";
  STATUS_VALUES.forEach((status) => {
    const option = document.createElement("option");
    option.value = status;
    option.textContent = getStatusLabel(status);
    option.selected = status === (currentStatus || "").toLowerCase();
    statusSelect.appendChild(option);
  });

  const saveStatusButton = document.createElement("button");
  saveStatusButton.type = "button";
  saveStatusButton.className = "small";
  saveStatusButton.textContent = "Durum Değiştir";
  saveStatusButton.addEventListener("click", () => updateOrderStatus(orderId, statusSelect.value));

  const deleteButton = document.createElement("button");
  deleteButton.type = "button";
  deleteButton.className = "small danger";
  deleteButton.textContent = "Sil";
  deleteButton.addEventListener("click", () => deleteOrder(orderId));

  wrap.append(detailButton, statusSelect, saveStatusButton, deleteButton);
  return wrap;
}

function renderTable(orders) {
  const tbody = document.getElementById("ordersTableBody");
  if (!tbody) return;
  tbody.innerHTML = "";

  orders.forEach((order) => {
    const row = document.createElement("tr");
    const orderId = getId(order);
    row.dataset.orderId = orderId || "";

    const cells = [
      formatDateTime(order.createdAt),
      order.orderCode || "-",
      order.userPhone || "-",
      formatCurrency(order.totalPrice),
      getPaymentLabel(order.paymentMethod),
    ];

    cells.forEach((value) => {
      const cell = document.createElement("td");
      cell.textContent = value;
      row.appendChild(cell);
    });

    const statusCell = document.createElement("td");
    const statusBadge = document.createElement("span");
    statusBadge.className = statusClass(order.status);
    statusBadge.textContent = getStatusLabel(order.status);
    statusCell.appendChild(statusBadge);
    row.appendChild(statusCell);

    const qtyCell = document.createElement("td");
    qtyCell.textContent = String(itemQuantityTotal(order.items));
    row.appendChild(qtyCell);

    const actionCell = document.createElement("td");
    actionCell.appendChild(orderActions(orderId, order.status));
    row.appendChild(actionCell);

    tbody.appendChild(row);
  });
}

function renderCards(orders) {
  const cardList = document.getElementById("ordersCardList");
  if (!cardList) return;
  cardList.innerHTML = "";

  orders.forEach((order) => {
    const orderId = getId(order);
    const card = document.createElement("article");
    card.className = "hm-order-card";
    card.dataset.orderId = orderId || "";

    const header = document.createElement("h4");
    header.innerHTML = `<span>${order.orderCode || "-"}</span><span class="${statusClass(order.status)}">${getStatusLabel(order.status)}</span>`;

    const details = document.createElement("div");
    details.innerHTML = `
      <p><strong>Tarih:</strong> ${formatDateTime(order.createdAt)}</p>
      <p><strong>Telefon:</strong> ${order.userPhone || "-"}</p>
      <p><strong>Toplam:</strong> ${formatCurrency(order.totalPrice)}</p>
      <p><strong>Ödeme:</strong> ${getPaymentLabel(order.paymentMethod)}</p>
      <p><strong>Ürün Adedi:</strong> ${itemQuantityTotal(order.items)}</p>
    `;

    card.append(header, details, orderActions(orderId, order.status));
    cardList.appendChild(card);
  });
}

function renderOrders(orders) {
  if (!Array.isArray(orders) || orders.length === 0) {
    renderEmpty("Henüz sipariş yok.");
    return;
  }

  renderTable(orders);
  renderCards(orders);
}

function openDetail(order) {
  const dialog = document.getElementById("orderDetailDialog");
  const content = document.getElementById("orderDetailContent");
  if (!dialog || !content) return;

  const items = Array.isArray(order.items) ? order.items : [];
  const itemRows = items.map((item) => {
    const subtotal = (Number(item.price) || 0) * (Number(item.quantity) || 0);
    return `<li>${item.name || "-"} x${item.quantity || 0} – ${formatCurrency(subtotal)}</li>`;
  }).join("");

  content.innerHTML = `
    <p><strong>Sipariş Kodu:</strong> ${order.orderCode || "-"}</p>
    <p><strong>Tarih:</strong> ${formatDateTime(order.createdAt)}</p>
    <p><strong>Telefon:</strong> ${order.userPhone || "-"}</p>
    <p><strong>Ödeme:</strong> ${getPaymentLabel(order.paymentMethod)}</p>
    <p><strong>Durum:</strong> ${getStatusLabel(order.status)}</p>
    <h4>Ürünler:</h4>
    <ul>${itemRows || "<li>Ürün bulunmuyor.</li>"}</ul>
    <p><strong>Toplam:</strong> ${formatCurrency(order.totalPrice)}</p>
  `;

  state.detailOpen = true;
  dialog.showModal();
}

async function openDetailById(orderId) {
  if (!orderId) return;
  const res = await fetch(`${ORDERS_API_URL}/${orderId}`, { headers: authHeaders() });
  if (handleUnauthorized(res)) return;
  const payload = await safeJson(res);
  if (!res.ok) {
    setText("ordersStatus", "Siparişler alınamadı. Lütfen tekrar deneyin.");
    return;
  }
  openDetail(payload);
}

function closeDetail() {
  const dialog = document.getElementById("orderDetailDialog");
  if (dialog?.open) dialog.close();
  state.detailOpen = false;
}

async function fetchOrders(manual = false) {
  if (state.loading) return;
  if (!manual && (state.detailOpen || state.statusSaving)) return;

  state.loading = true;
  if (manual) setText("ordersStatus", "Siparişler yükleniyor...");

  const res = await fetch(`${ORDERS_API_URL}?${currentQuery()}`, { headers: authHeaders() });
  state.loading = false;

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    setText("ordersStatus", "Siparişler alınamadı. Lütfen tekrar deneyin.");
    renderEmpty("Siparişler alınamadı. Lütfen tekrar deneyin.");
    return;
  }

  const data = Array.isArray(payload?.data) ? payload.data : [];
  const pagination = payload?.pagination || {};
  state.totalPages = Number(pagination.totalPages) || 1;

  setText("ordersPaginationInfo", `Toplam kayıt: ${pagination.total || 0} • Toplam sayfa: ${state.totalPages}`);
  setText("currentPageText", `Sayfa ${state.page}`);
  setText("ordersStatus", "");

  const prevButton = document.getElementById("prevPageButton");
  const nextButton = document.getElementById("nextPageButton");
  if (prevButton) prevButton.disabled = state.page <= 1;
  if (nextButton) nextButton.disabled = state.page >= state.totalPages;

  renderOrders(data);
  updateLastRefreshed();
}

async function updateOrderStatus(orderId, status) {
  if (!orderId) return;
  state.statusSaving = true;
  setText("ordersStatus", "Durum değiştiriliyor...");

  const res = await fetch(`${ORDERS_API_URL}/${orderId}/status`, {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify({ status }),
  });

  state.statusSaving = false;
  if (handleUnauthorized(res)) return;

  if (!res.ok) {
    setText("ordersStatus", "Siparişler alınamadı. Lütfen tekrar deneyin.");
    return;
  }

  setText("ordersStatus", "Durum güncellendi.");
  await fetchOrders(true);
}

async function deleteOrder(orderId) {
  if (!orderId) return;
  if (!window.confirm("Sipariş silinsin mi?")) return;

  const res = await fetch(`${ORDERS_API_URL}/${orderId}`, {
    method: "DELETE",
    headers: authHeaders(),
  });

  if (handleUnauthorized(res)) return;
  if (!res.ok) {
    setText("ordersStatus", "Siparişler alınamadı. Lütfen tekrar deneyin.");
    return;
  }

  await fetchOrders(true);
}

function readFiltersFromUI() {
  state.filters.status = document.getElementById("statusFilter")?.value || "";
  state.filters.paymentMethod = document.getElementById("paymentFilter")?.value || "";
  state.filters.search = (document.getElementById("searchFilter")?.value || "").trim();
  state.limit = Number(document.getElementById("limitFilter")?.value || "20") || 20;
}

function clearFilters() {
  document.getElementById("statusFilter").value = "";
  document.getElementById("paymentFilter").value = "";
  document.getElementById("searchFilter").value = "";
  document.getElementById("limitFilter").value = "20";
  readFiltersFromUI();
  state.page = 1;
  fetchOrders(true);
}

function startAutoRefresh() {
  stopAutoRefresh();
  state.autoRefreshTimer = window.setInterval(() => fetchOrders(false), AUTO_REFRESH_MS);
}

function stopAutoRefresh() {
  if (!state.autoRefreshTimer) return;
  window.clearInterval(state.autoRefreshTimer);
  state.autoRefreshTimer = null;
}

const onResize = debounce(() => {
  const nextMode = window.matchMedia(MOBILE_MEDIA_QUERY).matches;
  if (nextMode !== state.isMobile) {
    state.isMobile = nextMode;
    fetchOrders(true);
  }
}, 150);

function bindEvents() {
  document.getElementById("applyFiltersButton")?.addEventListener("click", () => {
    readFiltersFromUI();
    state.page = 1;
    fetchOrders(true);
  });

  document.getElementById("clearFiltersButton")?.addEventListener("click", clearFilters);
  document.getElementById("refreshOrdersButton")?.addEventListener("click", () => fetchOrders(true));

  document.getElementById("prevPageButton")?.addEventListener("click", () => {
    if (state.page <= 1) return;
    state.page -= 1;
    fetchOrders(true);
  });

  document.getElementById("nextPageButton")?.addEventListener("click", () => {
    if (state.page >= state.totalPages) return;
    state.page += 1;
    fetchOrders(true);
  });

  document.getElementById("closeDetailButton")?.addEventListener("click", closeDetail);
  document.getElementById("orderDetailDialog")?.addEventListener("close", () => {
    state.detailOpen = false;
  });

  window.addEventListener("resize", onResize);
  window.addEventListener("beforeunload", () => {
    stopAutoRefresh();
    window.removeEventListener("resize", onResize);
  });
}

document.addEventListener("DOMContentLoaded", () => {
  bindEvents();
  readFiltersFromUI();
  fetchOrders(true);
  startAutoRefresh();
});
