requireAuth();

let selectedProduct = null;
let cachedCategories = [];
const isDev = ["localhost", "127.0.0.1"].includes(window.location.hostname);

// ---------- Helpers ----------
function el(id) { return document.getElementById(id); }

function setStatus(msg) {
  const s = el("productStatus");
  if (s) s.textContent = msg || "";
}

function setCategoryStatus(msg) {
  const s = el("categoryStatus");
  if (s) s.textContent = msg || "";
}

function fdLog(fd) {
  // debug için aç:
  // for (const [k,v] of fd.entries()) console.log(k, v);
}

function warnMissingSelect(select, label) {
  if (!isDev) return;
  console.warn(`[products] ${label} select bulunamadı veya <select> değil.`, select);
}

// Kategori option doldurucu
function fillSelect(select, categories, placeholderHtml) {
  if (!select || select.tagName !== "SELECT") {
    warnMissingSelect(select, "Kategori");
    return;
  }
  select.innerHTML = placeholderHtml;

  if (!Array.isArray(categories)) return;

  categories.forEach((cat) => {
    const id = getId(cat) || cat.id || cat._id; // admin.js helper varsa getId kullan
    if (!id) return;

    const opt = document.createElement("option");
    opt.value = id;                 
    opt.textContent = cat.name || "-";
    select.appendChild(opt);
  });
}

function extractCategories(payload) {
  const candidates = [
    payload?.data?.data,
    payload?.data?.categories,
    payload?.data,
    payload?.categories,
    payload
  ];

  const found = candidates.find((item) => Array.isArray(item));
  return found || [];
}

function getCategoryIdByName(name, categories) {
  if (!name || !Array.isArray(categories)) return "";
  const target = name.trim().toLowerCase();
  const match = categories.find((cat) => {
    const catName = (cat?.name || "").trim().toLowerCase();
    return catName === target;
  });

  return match ? (getId(match) || match.id || match._id || "") : "";
}

function normalizeCategoryIds(product, categories) {
  const ids = new Set();

  const addId = (value) => {
    if (!value) return;
    ids.add(String(value));
  };

  const addFromArray = (arr) => {
    arr.forEach((item) => {
      if (!item) return;
      if (typeof item === "string" || typeof item === "number") {
        addId(item);
        return;
      }
      if (typeof item === "object") {
        const id = getId(item) || item.id || item._id;
        if (id) {
          addId(id);
          return;
        }
        const nameId = getCategoryIdByName(item.name, categories);
        if (nameId) addId(nameId);
      }
    });
  };

  if (Array.isArray(product.categoryIds)) addFromArray(product.categoryIds);

  if (Array.isArray(product.category_id)) {
    addFromArray(product.category_id);
  } else if (product.category_id) {
    addId(product.category_id);
  }

  if (Array.isArray(product.categories)) addFromArray(product.categories);

  if (product.category && typeof product.category === "string") {
    const nameMatch = getCategoryIdByName(product.category, categories);
    if (nameMatch) addId(nameMatch);
  } else if (product.category && typeof product.category === "object") {
    const id = getId(product.category) || product.category.id || product.category._id;
    if (id) {
      addId(id);
    } else {
      const nameMatch = getCategoryIdByName(product.category.name, categories);
      if (nameMatch) addId(nameMatch);
    }
  }

  return [...ids];
}

function appendSelectedCategories(fd, select, label) {
  if (!select || select.tagName !== "SELECT") {
    warnMissingSelect(select, label);
    return;
  }

  fd.delete("category_id");
  [...select.selectedOptions].forEach((o) => {
    if (o.value) fd.append("category_id", o.value);
  });
}

async function fetchAdminCategories() {
  const res = await fetch("/admin/categories", { headers: authHeaders() });
  if (handleUnauthorized(res)) return [];

  const payload = await safeJson(res);
  return extractCategories(payload);
}

async function loadCategoriesEverywhere() {
  const categories = await fetchAdminCategories();
  cachedCategories = Array.isArray(categories) ? categories : [];

  if (cachedCategories.length === 0) {
    setCategoryStatus("Kategori bulunamadı.");
  } else {
    setCategoryStatus("");
  }

  fillSelect(
    el("categoryFilter"),
    cachedCategories,
    `<option value="">Tüm Kategoriler</option>`
  );

  fillSelect(
    el("addProductCategorySelect"),
    cachedCategories,
    `<option value="" disabled>Kategori Seç</option>`
  );

  fillSelect(
    el("editProductCategorySelect"),
    cachedCategories,
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
    const ids = normalizeCategoryIds(product, cachedCategories);

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

  // ✅ multiple seçimi name="category_id" ile gönder
  appendSelectedCategories(fd, el("addProductCategorySelect"), "Yeni ürün");

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
    headers: authHeadersMultipart(),
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
  appendSelectedCategories(fd, el("editProductCategorySelect"), "Düzenle");

  fdLog(fd);

  const res = await fetch("/admin/api/products/" + id, {
    method: "PUT",
    headers: authHeadersMultipart(),
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
