requireAuth();

const ORDERS_API_URL = "/orders";
const DELETE_ORDER_API_URL = "/admin/api/orders";

function formatDateTime(value) {
  const date = value ? new Date(value) : null;
  if (!date || Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString("tr-TR");
}

function formatCurrency(value) {
  if (typeof value !== "number") return "-";
  return value.toLocaleString("tr-TR", {
    style: "currency",
    currency: "TRY",
    minimumFractionDigits: 2,
  });
}

function statusBadgeClass(status) {
  const normalized = (status || "").toLowerCase();
  if (normalized === "completed") return "badge completed";
  if (normalized === "canceled" || normalized === "cancelled") return "badge canceled";
  return "badge pending";
}

function tableBody() {
  return document.getElementById("ordersTableBody");
}

function clearTable() {
  const tbody = tableBody();
  if (tbody) tbody.innerHTML = "";
}

function addEmptyRow(message) {
  const tbody = tableBody();
  if (!tbody) return;
  const row = document.createElement("tr");
  const cell = document.createElement("td");
  cell.colSpan = 8;
  cell.className = "muted";
  cell.textContent = message;
  row.appendChild(cell);
  tbody.appendChild(row);
}

function normalizeOrderId(order, index) {
  const id = getId(order);
  return id ? String(id) : `row-${index}`;
}

function itemSubtotal(item) {
  if (!item || typeof item.price !== "number" || typeof item.quantity !== "number") return 0;
  return item.price * item.quantity;
}

function buildDetailRow(order, columnCount) {
  const detailRow = document.createElement("tr");
  detailRow.className = "order-detail-row";

  const detailCell = document.createElement("td");
  detailCell.colSpan = columnCount;

  const detailWrapper = document.createElement("div");
  detailWrapper.className = "order-detail-content";

  const grid = document.createElement("div");
  grid.className = "grid-two";

  const addressCard = document.createElement("div");
  const addressTitle = document.createElement("h4");
  addressTitle.textContent = "📍 Adres";
  addressCard.appendChild(addressTitle);

  const addressBlock = document.createElement("div");
  addressBlock.className = "address-block";

  const addressTitleLabel = document.createElement("span");
  addressTitleLabel.className = "address-label";
  addressTitleLabel.textContent = order && order.customer && order.customer.title ? order.customer.title : "-";
  addressBlock.appendChild(addressTitleLabel);

  const addressDetail = document.createElement("span");
  addressDetail.textContent = order && order.customer && order.customer.detail ? order.customer.detail : "-";
  addressBlock.appendChild(addressDetail);

  if (order && order.customer && order.customer.note) {
    const addressNote = document.createElement("span");
    addressNote.className = "muted";
    addressNote.textContent = `Not: ${order.customer.note}`;
    addressBlock.appendChild(addressNote);
  }

  addressCard.appendChild(addressBlock);

  const summaryCard = document.createElement("div");
  const summaryTitle = document.createElement("h4");
  summaryTitle.textContent = "Sipariş Özeti";
  summaryCard.appendChild(summaryTitle);

  const summaryList = document.createElement("div");
  summaryList.className = "order-summary";

  const payment = document.createElement("span");
  payment.textContent = `💳 Ödeme yöntemi: ${order && order.paymentMethod ? order.paymentMethod : "-"}`;
  summaryList.appendChild(payment);

  const status = document.createElement("span");
  status.textContent = `📦 Durum: ${order && order.status ? order.status : "-"}`;
  summaryList.appendChild(status);

  const createdAt = document.createElement("span");
  createdAt.textContent = `🕒 Tarih: ${formatDateTime(order && order.createdAt)}`;
  summaryList.appendChild(createdAt);

  const total = document.createElement("strong");
  total.textContent = `Toplam: ${formatCurrency(order && order.totalPrice)}`;
  summaryList.appendChild(total);

  summaryCard.appendChild(summaryList);

  grid.appendChild(addressCard);
  grid.appendChild(summaryCard);

  const itemsTitle = document.createElement("h4");
  itemsTitle.textContent = "🛒 Ürünler";

  const itemsTable = document.createElement("table");
  itemsTable.className = "order-items";

  const itemsHead = document.createElement("thead");
  itemsHead.innerHTML = `
    <tr>
      <th>Ürün</th>
      <th class="numeric">Adet</th>
      <th class="numeric">Birim fiyat</th>
      <th class="numeric">Ara toplam</th>
    </tr>
  `;
  itemsTable.appendChild(itemsHead);

  const itemsBody = document.createElement("tbody");
  const items = Array.isArray(order && order.items) ? order.items : [];
  if (items.length === 0) {
    const emptyRow = document.createElement("tr");
    const emptyCell = document.createElement("td");
    emptyCell.colSpan = 4;
    emptyCell.className = "muted";
    emptyCell.textContent = "Ürün bulunamadı";
    emptyRow.appendChild(emptyCell);
    itemsBody.appendChild(emptyRow);
  } else {
    items.forEach((item) => {
      const itemRow = document.createElement("tr");

      const nameCell = document.createElement("td");
      nameCell.textContent = item && item.name ? item.name : "-";
      itemRow.appendChild(nameCell);

      const qtyCell = document.createElement("td");
      qtyCell.className = "numeric";
      qtyCell.textContent = typeof item.quantity === "number" ? item.quantity : "-";
      itemRow.appendChild(qtyCell);

      const priceCell = document.createElement("td");
      priceCell.className = "numeric";
      priceCell.textContent = formatCurrency(item && item.price);
      itemRow.appendChild(priceCell);

      const subtotalCell = document.createElement("td");
      subtotalCell.className = "numeric";
      subtotalCell.textContent = formatCurrency(itemSubtotal(item));
      itemRow.appendChild(subtotalCell);

      itemsBody.appendChild(itemRow);
    });
  }

  itemsTable.appendChild(itemsBody);

  detailWrapper.appendChild(grid);
  detailWrapper.appendChild(itemsTitle);
  detailWrapper.appendChild(itemsTable);

  detailCell.appendChild(detailWrapper);
  detailRow.appendChild(detailCell);

  return detailRow;
}

function renderOrders(orders) {
  clearTable();
  if (!Array.isArray(orders) || orders.length === 0) {
    addEmptyRow("Sipariş yok");
    return;
  }

  const tbody = tableBody();
  if (!tbody) return;

  let activeDetailRow = null;
  let activeOrderKey = null;
  let activeRow = null;

  const closeActiveDetail = () => {
    if (activeDetailRow) {
      activeDetailRow.remove();
      activeDetailRow = null;
    }
    if (activeRow) {
      activeRow.classList.remove("is-expanded");
      activeRow = null;
    }
    activeOrderKey = null;
  };

  orders.forEach((order, index) => {
    const row = document.createElement("tr");
    row.className = "order-row";
    const orderId = getId(order) || "-";
    const orderKey = normalizeOrderId(order, index);
    const customerTitle = order && order.customer && order.customer.title ? order.customer.title : "-";
    const itemCount = Array.isArray(order && order.items) ? order.items.length : 0;

    const cells = [
      orderId,
      formatDateTime(order && order.createdAt),
      customerTitle,
      order && order.paymentMethod ? order.paymentMethod : "-",
      itemCount,
      formatCurrency(order && order.totalPrice),
    ];

    cells.forEach((value, index) => {
      const cell = document.createElement("td");
      if (index >= 4 && index <= 5) {
        cell.className = "numeric";
      }
      cell.textContent = value;
      row.appendChild(cell);
    });

    const statusCell = document.createElement("td");
    const badge = document.createElement("span");
    badge.className = statusBadgeClass(order && order.status);
    badge.textContent = order && order.status ? order.status : "Bilinmiyor";
    statusCell.appendChild(badge);
    row.appendChild(statusCell);

    const actionCell = document.createElement("td");
    actionCell.className = "actions";
    const deleteButton = document.createElement("button");
    deleteButton.type = "button";
    deleteButton.className = "small danger";
    deleteButton.textContent = "Sil";
    deleteButton.addEventListener("click", (event) => {
      event.stopPropagation();
      if (orderId === "-") return;
      deleteOrder(orderId);
    });
    actionCell.appendChild(deleteButton);
    row.appendChild(actionCell);

    row.addEventListener("click", () => {
      if (activeOrderKey === orderKey) {
        closeActiveDetail();
        return;
      }

      closeActiveDetail();
      const detailRow = buildDetailRow(order, 8);
      row.after(detailRow);
      activeDetailRow = detailRow;
      activeOrderKey = orderKey;
      activeRow = row;
      row.classList.add("is-expanded");
    });

    tbody.appendChild(row);
  });
}

async function loadOrders() {
  setText("ordersStatus", "Siparişler yükleniyor...");
  const res = await fetch(ORDERS_API_URL, { headers: authHeaders() });
  if (handleUnauthorized(res)) return;
  const payload = await safeJson(res);
  if (!res.ok) {
    setText("ordersStatus", "Hata: siparişler getirilemedi");
    addEmptyRow("Siparişler yüklenemedi");
    return;
  }

  const data = payload && payload.data ? payload.data : payload || [];
  renderOrders(data);
  setText("ordersStatus", "");
}

async function deleteOrder(orderId) {
  if (!window.confirm("Sipariş silinsin mi?")) {
    return;
  }

  setText("ordersStatus", "Sipariş siliniyor...");
  const res = await fetch(`${DELETE_ORDER_API_URL}/${orderId}`, {
    method: "DELETE",
    headers: authHeaders(),
  });

  if (handleUnauthorized(res)) return;
  if (!res.ok) {
    setText("ordersStatus", "Hata: sipariş silinemedi");
    return;
  }

  setText("ordersStatus", "");
  await loadOrders();
}

document.addEventListener("DOMContentLoaded", loadOrders);
