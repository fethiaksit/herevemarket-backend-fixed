requireAuth();

const ORDERS_API_URL = "/admin/api/orders";
const AUTO_REFRESH_MS = 12000;
const STATUS_VALUES = ["pending", "approved", "cancelled", "delivered"];

const state = {
  page: 1,
  limit: 20,
  totalPages: 1,
  filters: {
    status: "",
    paymentMethod: "",
    startDate: "",
    endDate: "",
    search: "",
  },
  loading: false,
  statusSaving: false,
  detailOpen: false,
  autoRefreshTimer: null,
};

function formatDateTime(value) {
  const date = value ? new Date(value) : null;
  if (!date || Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString("tr-TR");
}

function formatCurrency(value) {
  if (typeof value !== "number") return "-";
  return value.toLocaleString("tr-TR", { style: "currency", currency: "TRY" });
}

function statusBadgeClass(status) {
  const normalized = (status || "").toLowerCase();
  if (normalized === "approved" || normalized === "delivered") return "badge completed";
  if (normalized === "cancelled") return "badge canceled";
  return "badge pending";
}

function toIsoOrEmpty(value) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function currentQuery() {
  const params = new URLSearchParams();
  params.set("page", String(state.page));
  params.set("limit", String(state.limit));
  if (state.filters.status) params.set("status", state.filters.status);
  if (state.filters.paymentMethod) params.set("paymentMethod", state.filters.paymentMethod);
  if (state.filters.search) params.set("search", state.filters.search);
  if (state.filters.startDate) params.set("startDate", state.filters.startDate);
  if (state.filters.endDate) params.set("endDate", state.filters.endDate);
  return params.toString();
}

function updateLastRefreshed() {
  const now = new Date();
  setText("ordersLastUpdated", `Son güncelleme: ${now.toLocaleTimeString("tr-TR")}`);
}

function itemQuantityTotal(items) {
  if (!Array.isArray(items)) return 0;
  return items.reduce((sum, item) => sum + (Number(item && item.quantity) || 0), 0);
}

function clearTable() {
  const tbody = document.getElementById("ordersTableBody");
  if (tbody) tbody.innerHTML = "";
}

function addEmptyRow(message) {
  const tbody = document.getElementById("ordersTableBody");
  if (!tbody) return;
  const row = document.createElement("tr");
  const cell = document.createElement("td");
  cell.colSpan = 9;
  cell.className = "muted";
  cell.textContent = message;
  row.appendChild(cell);
  tbody.appendChild(row);
}

function openDetail(order) {
  const dialog = document.getElementById("orderDetailDialog");
  const content = document.getElementById("orderDetailContent");
  if (!dialog || !content) return;

  const items = Array.isArray(order.items) ? order.items : [];
  const rows = items.map((item) => {
    const subtotal = (Number(item.price) || 0) * (Number(item.quantity) || 0);
    return `
      <tr>
        <td>${item.name || "-"}</td>
        <td class="numeric">${item.quantity ?? "-"}</td>
        <td class="numeric">${formatCurrency(item.price)}</td>
        <td class="numeric">${formatCurrency(subtotal)}</td>
      </tr>
    `;
  }).join("");

  content.innerHTML = `
    <div class="grid-two">
      <div>
        <h4>Müşteri Bilgisi</h4>
        <p><strong>Başlık:</strong> ${order.customer && order.customer.title ? order.customer.title : "-"}</p>
        <p><strong>Adres:</strong> ${order.customer && order.customer.detail ? order.customer.detail : "-"}</p>
        <p><strong>Not:</strong> ${order.customer && order.customer.note ? order.customer.note : "-"}</p>
      </div>
      <div>
        <h4>Sipariş Özeti</h4>
        <p><strong>Durum:</strong> ${order.status || "-"}</p>
        <p><strong>Ödeme:</strong> ${order.paymentMethod || "-"}</p>
        <p><strong>Tarih:</strong> ${formatDateTime(order.createdAt)}</p>
        <p><strong>Toplam:</strong> ${formatCurrency(order.totalPrice)}</p>
      </div>
    </div>
    <h4>Ürünler</h4>
    <table class="order-items">
      <thead>
        <tr><th>Ürün</th><th class="numeric">Adet</th><th class="numeric">Birim fiyat</th><th class="numeric">Satır toplam</th></tr>
      </thead>
      <tbody>${rows || '<tr><td colspan="4" class="muted">Ürün yok</td></tr>'}</tbody>
    </table>
  `;

  state.detailOpen = true;
  dialog.showModal();
}

function closeDetail() {
  const dialog = document.getElementById("orderDetailDialog");
  if (dialog && dialog.open) dialog.close();
  state.detailOpen = false;
}

function renderOrders(orders) {
  clearTable();
  const tbody = document.getElementById("ordersTableBody");
  if (!tbody) return;

  if (!Array.isArray(orders) || orders.length === 0) {
    addEmptyRow("Sipariş bulunamadı");
    return;
  }

  orders.forEach((order) => {
    const row = document.createElement("tr");
    row.className = "order-row";

    const orderId = getId(order) || "-";
    const userId = order && order.userId ? String(order.userId) : "-";
    const cols = [
      formatDateTime(order.createdAt),
      orderId,
      userId,
      order.userPhone || "—",
      formatCurrency(order.totalPrice),
      order.paymentMethod || "-",
    ];

    cols.forEach((value, idx) => {
      const cell = document.createElement("td");
      if (idx === 4) cell.className = "numeric";
      cell.textContent = value;
      row.appendChild(cell);
    });

    const statusCell = document.createElement("td");
    const statusBadge = document.createElement("span");
    statusBadge.className = statusBadgeClass(order.status);
    statusBadge.textContent = order.status || "-";
    statusCell.appendChild(statusBadge);
    row.appendChild(statusCell);

    const qtyCell = document.createElement("td");
    qtyCell.className = "numeric";
    qtyCell.textContent = String(itemQuantityTotal(order.items));
    row.appendChild(qtyCell);

    const actionCell = document.createElement("td");
    actionCell.className = "actions";

    const detailButton = document.createElement("button");
    detailButton.type = "button";
    detailButton.className = "small ghost";
    detailButton.textContent = "Detay";
    detailButton.addEventListener("click", (event) => {
      event.stopPropagation();
      openDetail(order);
    });

    const statusWrap = document.createElement("span");
    statusWrap.className = "status-editor";

    const statusSelect = document.createElement("select");
    statusSelect.className = "table-input";
    STATUS_VALUES.forEach((status) => {
      const option = document.createElement("option");
      option.value = status;
      option.textContent = status;
      option.selected = status === (order.status || "").toLowerCase();
      statusSelect.appendChild(option);
    });

    const saveStatusButton = document.createElement("button");
    saveStatusButton.type = "button";
    saveStatusButton.className = "small";
    saveStatusButton.textContent = "Kaydet";
    saveStatusButton.addEventListener("click", async (event) => {
      event.stopPropagation();
      await updateOrderStatus(orderId, statusSelect.value);
    });

    statusWrap.appendChild(statusSelect);
    statusWrap.appendChild(saveStatusButton);

    const deleteButton = document.createElement("button");
    deleteButton.type = "button";
    deleteButton.className = "small danger";
    deleteButton.textContent = "Sil";
    deleteButton.addEventListener("click", async (event) => {
      event.stopPropagation();
      await deleteOrder(orderId);
    });

    actionCell.appendChild(detailButton);
    actionCell.appendChild(statusWrap);
    actionCell.appendChild(deleteButton);
    row.appendChild(actionCell);

    tbody.appendChild(row);
  });
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
    const err = payload && payload.error ? payload.error : "siparişler getirilemedi";
    setText("ordersStatus", `Hata: ${err}`);
    addEmptyRow("Siparişler yüklenemedi");
    return;
  }

  const data = payload && Array.isArray(payload.data) ? payload.data : [];
  const pagination = payload && payload.pagination ? payload.pagination : {};

  state.totalPages = Number(pagination.totalPages) || 1;
  setText("ordersPaginationInfo", `Toplam kayıt: ${pagination.total || 0} • Toplam sayfa: ${state.totalPages}`);
  setText("currentPageText", `Sayfa ${state.page}`);

  const prevBtn = document.getElementById("prevPageButton");
  const nextBtn = document.getElementById("nextPageButton");
  if (prevBtn) prevBtn.disabled = state.page <= 1;
  if (nextBtn) nextBtn.disabled = state.page >= state.totalPages;

  renderOrders(data);
  setText("ordersStatus", "");
  updateLastRefreshed();
}

async function updateOrderStatus(orderId, status) {
  if (!orderId || orderId === "-") return;
  state.statusSaving = true;
  setText("ordersStatus", "Durum güncelleniyor...");

  const res = await fetch(`${ORDERS_API_URL}/${orderId}/status`, {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify({ status }),
  });

  state.statusSaving = false;
  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    const err = payload && payload.error ? payload.error : "durum güncellenemedi";
    setText("ordersStatus", `Hata: ${err}`);
    return;
  }

  setText("ordersStatus", "Durum güncellendi");
  await fetchOrders(true);
}

async function deleteOrder(orderId) {
  if (!orderId || orderId === "-") return;
  if (!window.confirm("Sipariş silinsin mi?")) return;

  setText("ordersStatus", "Sipariş siliniyor...");
  const res = await fetch(`${ORDERS_API_URL}/${orderId}`, {
    method: "DELETE",
    headers: authHeaders(),
  });

  if (handleUnauthorized(res)) return;
  if (!res.ok) {
    const payload = await safeJson(res);
    const err = payload && payload.error ? payload.error : "silinemedi";
    setText("ordersStatus", `Hata: ${err}`);
    return;
  }

  await fetchOrders(true);
}

function readFiltersFromUI() {
  state.filters.status = document.getElementById("statusFilter")?.value || "";
  state.filters.paymentMethod = document.getElementById("paymentFilter")?.value || "";
  state.filters.startDate = toIsoOrEmpty(document.getElementById("startDateFilter")?.value || "");
  state.filters.endDate = toIsoOrEmpty(document.getElementById("endDateFilter")?.value || "");
  state.filters.search = (document.getElementById("searchFilter")?.value || "").trim();
  state.limit = Number(document.getElementById("limitFilter")?.value || "20") || 20;
}

function clearFilters() {
  document.getElementById("statusFilter").value = "";
  document.getElementById("paymentFilter").value = "";
  document.getElementById("startDateFilter").value = "";
  document.getElementById("endDateFilter").value = "";
  document.getElementById("searchFilter").value = "";
  document.getElementById("limitFilter").value = "20";
  readFiltersFromUI();
  state.page = 1;
  fetchOrders(true);
}

function startAutoRefresh() {
  stopAutoRefresh();
  state.autoRefreshTimer = window.setInterval(() => {
    fetchOrders(false);
  }, AUTO_REFRESH_MS);
}

function stopAutoRefresh() {
  if (state.autoRefreshTimer) {
    window.clearInterval(state.autoRefreshTimer);
    state.autoRefreshTimer = null;
  }
}

function bindEvents() {
  document.getElementById("applyFiltersButton")?.addEventListener("click", () => {
    readFiltersFromUI();
    state.page = 1;
    fetchOrders(true);
  });

  document.getElementById("clearFiltersButton")?.addEventListener("click", clearFilters);

  document.getElementById("refreshOrdersButton")?.addEventListener("click", () => {
    readFiltersFromUI();
    fetchOrders(true);
  });

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

  window.addEventListener("beforeunload", stopAutoRefresh);
}

document.addEventListener("DOMContentLoaded", () => {
  bindEvents();
  readFiltersFromUI();
  fetchOrders(true);
  startAutoRefresh();
});
