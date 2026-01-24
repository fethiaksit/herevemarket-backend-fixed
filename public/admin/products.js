requireAuth();

let selectedProduct = null;

// ---------- Helpers ----------
function el(id) { return document.getElementById(id); }

function setStatus(msg) {
  const s = el("productStatus");
  if (s) s.textContent = msg || "";
}

function fdLog(fd) {
  // debug için aç:
  // for (const [k,v] of fd.entries()) console.log(k, v);
}

// Kategori option doldurucu
function fillSelect(select, categories, placeholderHtml) {
  if (!select) return;
  select.innerHTML = placeholderHtml;

  if (!Array.isArray(categories)) return;

  categories.forEach((cat) => {
    const id = getId(cat) || cat.id || cat._id; // admin.js helper varsa getId kullan
    if (!id) return;

    const opt = document.createElement("option");
    opt.value = id;                 // 🔥 ID basıyoruz
    opt.textContent = cat.name || "-";
    select.appendChild(opt);
  });
}

async function fetchAdminCategories() {
  const res = await fetch("/admin/api/categories", { headers: authHeaders() });
  if (handleUnauthorized(res)) return [];

  const payload = await safeJson(res);
  return (payload && payload.data) ? payload.data : (payload || []);
}

async function loadCategoriesEverywhere() {
  const categories = await fetchAdminCategories();

  fillSelect(
    el("categoryFilter"),
    categories,
    `<option value="">Tüm Kategoriler</option>`
  );

  fillSelect(
    el("addProductCategorySelect"),
    categories,
    `<option value="" disabled selected>Kategori Seç</option>`
  );

  fillSelect(
    el("editProductCategorySelect"),
    categories,
    `<option value="">Kategori Seç</option>`
  );
}

// ---------- Products (basic) ----------
async function fetchProducts(params = {}) {
  const url = new URL("/admin/api/products", window.location.origin);

  // pagination varsayılanları (istersen değiştir)
  if (params.page) url.searchParams.set("page", params.page);
  if (params.limit) url.searchParams.set("limit", params.limit);

  // Yeni doğru filtre: categoryId
  if (params.categoryId) url.searchParams.set("categoryId", params.categoryId);

  const res = await fetch(url.toString(), { headers: authHeaders() });
  if (handleUnauthorized(res)) return null;

  return await safeJson(res);
}

function renderProductsTable(payload) {
  const tbody = document.querySelector("#productList tbody");
  if (!tbody) return;

  tbody.innerHTML = "";

  const data = payload && payload.data ? payload.data : (payload || []);
  if (!Array.isArray(data) || data.length === 0) {
    setStatus("Ürün yok");
    return;
  }

  setStatus("");

  data.forEach((p) => {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${p.name || "-"}</td>
      <td>${p.brand || "-"}</td>
      <td>${p.barcode || "-"}</td>
      <td>${(p.stock ?? "-")}</td>
      <td>${p.isCampaign ? "Evet" : "Hayır"}</td>
      <td><button type="button" class="small">Düzenle</button></td>
    `;

    tr.querySelector("button").onclick = () => selectProduct(p);
    tbody.appendChild(tr);
  });
}

function selectProduct(product) {
  selectedProduct = product;

  el("editProduct").style.display = "grid";
  el("prodName").innerText = product.name || "-";
  el("prodId").innerText = product.id || product._id ? `(id: ${(product.id || product._id)})` : "";

  const form = el("editProduct");
  form.elements.name.value = product.name || "";
  form.elements.price.value = product.price ?? "";
  form.elements.brand.value = product.brand || "";
  form.elements.barcode.value = product.barcode || "";
  form.elements.stock.value = product.stock ?? "";
  form.elements.description.value = product.description || "";
  form.elements.isCampaign.checked = !!product.isCampaign;
  form.elements.isActive.checked = product.isActive !== false;

  // kategori seçimini doldur (categoryIds varsa onu seç)
  const select = el("editProductCategorySelect");
  if (select) {
    // önce hepsini kaldır
    [...select.options].forEach(o => o.selected = false);

    // API’den gelebilecek formatlar: categoryIds (new), category (legacy names)
    const ids = Array.isArray(product.categoryIds) ? product.categoryIds : [];

    if (ids.length > 0) {
      // categoryIds ObjectID string olarak gelmeli
      ids.forEach((id) => {
        const opt = [...select.options].find(o => o.value === id);
        if (opt) opt.selected = true;
      });
    }
  }
}

async function refreshProducts() {
  const categoryId = el("categoryFilter")?.value || "";
  const payload = await fetchProducts({ page: 1, limit: 20, categoryId: categoryId || "" });
  if (!payload) return;
  renderProductsTable(payload);
}

// ---------- Create ----------
el("addProduct")?.addEventListener("submit", async (event) => {
  event.preventDefault();

  const form = event.target;
  const fd = new FormData(form);

  // ✅ multiple seçimi yok ama name="category_id" var
  // checkbox değerleri backend parseBoolValue ile okunuyor
  // unchecked ise hiç gitmeyebilir, sorun değil.
  if (!fd.get("isActive")) {
    // isActive checkbox unchecked ise backend default true yapıyorsa sorun değil,
    // ama tutarlı olsun diye gönderelim:
    // fd.set("isActive", "false"); // istersen aç
  }

  fdLog(fd);

  const res = await fetch("/admin/api/products", {
    method: "POST",
    headers: authHeaders(), // token + JSON content-type yok; multipart auto
    body: fd
  });

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    alert(payload && payload.error ? payload.error : "Ürün eklenemedi");
    return;
  }

  form.reset();
  await refreshProducts();
});

// ---------- Update ----------
el("editProduct")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!selectedProduct) return;

  const id = selectedProduct.id || selectedProduct._id;
  if (!id) {
    alert("Ürün id yok");
    return;
  }

  const form = event.target;
  const fd = new FormData(form);

  // ✅ multiple category: selected options -> category_id tekrar tekrar append edilmeli
  const catSelect = el("editProductCategorySelect");
  if (catSelect) {
    fd.delete("category_id");
    [...catSelect.selectedOptions].forEach(o => {
      if (o.value) fd.append("category_id", o.value);
    });
  }

  fdLog(fd);

  const res = await fetch("/admin/api/products/" + id, {
    method: "PUT",
    headers: authHeaders(),
    body: fd
  });

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    alert(payload && payload.error ? payload.error : "Ürün güncellenemedi");
    return;
  }

  await refreshProducts();
});

// ---------- Delete (soft) ----------
el("deleteProduct")?.addEventListener("click", async () => {
  if (!selectedProduct) return;

  const id = selectedProduct.id || selectedProduct._id;
  if (!id) {
    alert("Ürün id yok");
    return;
  }

  const res = await fetch("/admin/api/products/" + id, {
    method: "DELETE",
    headers: authHeaders()
  });

  if (handleUnauthorized(res)) return;

  if (!res.ok) {
    const payload = await safeJson(res);
    alert(payload && payload.error ? payload.error : "Ürün silinemedi");
    return;
  }

  selectedProduct = null;
  el("editProduct").style.display = "none";
  await refreshProducts();
});

// ---------- Filter change ----------
el("categoryFilter")?.addEventListener("change", async () => {
  await refreshProducts();
});

// ---------- Init ----------
(async function init() {
  await loadCategoriesEverywhere();
  await refreshProducts();
})();
