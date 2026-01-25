requireAuth();

let selectedProduct = null;
let currentProducts = [];
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

function getCategoryId(category) {
  if (!category) return null;
  return category._id || category.id || null;
}

function parseCategoriesPayload(payload) {
  // /categories -> array bekliyoruz ama tolerant dursun
  const data = Array.isArray(payload)
    ? payload
    : (payload && payload.data) ? payload.data : [];

  if (!Array.isArray(data)) {
    console.error("Kategori payload beklenmeyen formatta:", payload);
    return [];
  }
  return data;
}

function normalizeCategoryValues(values) {
  if (Array.isArray(values)) return values.filter(Boolean);
  if (typeof values === "string" && values) return [values];
  return [];
}

function getSelectedCategories(selectEl) {
  if (!selectEl) return [];
  return Array.from(selectEl.selectedOptions || [])
    .map(opt => opt.value)
    .filter(Boolean);
}

function assertSelectExists(selectId, nameAttrExpected) {
  const selectEl = el(selectId);
  if (!selectEl) {
    console.error(`[ADMIN PRODUCTS] Select bulunamadı: #${selectId}`);
    return null;
  }
  if (selectEl.tagName !== "SELECT") {
    console.error(`[ADMIN PRODUCTS] #${selectId} SELECT değil:`, selectEl);
    return null;
  }
  if (nameAttrExpected && selectEl.getAttribute("name") !== nameAttrExpected) {
    console.warn(
      `[ADMIN PRODUCTS] #${selectId} name="${selectEl.getAttribute("name")}" beklenen "${nameAttrExpected}" değildi. ` +
      `Form submit seçimlerinde querySelector('select[name="${nameAttrExpected}"]') kullandığın için bu önemli.`
    );
  }
  return selectEl;
}

function fillCategorySelect(selectEl, categories, selectedValues) {
  if (!selectEl || selectEl.tagName !== "SELECT") return;

  const preserved = Array.isArray(selectedValues) ? selectedValues : [];
  const preservedSet = new Set(preserved);

  // "Sadece aktif" istiyorsun => isActive false olanları at
  const activeCategories = (categories || []).filter(c => c && c.isActive !== false);

  selectEl.innerHTML = "";
  selectEl.multiple = true;

  if (activeCategories.length === 0) {
    const empty = document.createElement("option");
    empty.value = "";
    empty.textContent = "Kategori bulunamadı";
    empty.disabled = true;
    empty.selected = true;
    selectEl.appendChild(empty);
    return;
  }

  const def = document.createElement("option");
  def.value = "";
  def.textContent = "Kategori Seç";
  def.disabled = true;
  if (preserved.length === 0) def.selected = true;
  selectEl.appendChild(def);

  activeCategories.forEach(cat => {
    const id = getCategoryId(cat);
    if (!id) {
      console.warn("Kategori id bulunamadı:", cat);
      return;
    }
    const opt = document.createElement("option");
    opt.value = id;          // ✅ value her zaman id
    opt.textContent = cat.name;

    // preserved listesi ürünlerde eski sistemden name gelebilir; onu da seçilebilir tut
    if (preservedSet.has(id) || preservedSet.has(cat.name)) {
      opt.selected = true;
    }

    selectEl.appendChild(opt);
  });
}

async function safeJson(res) {
  try {
    return await res.json();
  } catch {
    return null;
  }
}

// ---------- Categories ----------
async function fetchCategoriesPublic() {
  const url = isDev
    ? "/categories" // dev'de aynı origin proxy varsa
    : "https://api.herevemarket.com/categories"; // prod kesin

  try {
    const res = await fetch(url, { cache: "no-store" });
    const payload = await safeJson(res);

    if (!res.ok) {
      console.error("Kategori isteği başarısız:", res.status, payload);
      return [];
    }

    return parseCategoriesPayload(payload);
  } catch (error) {
    console.error("Kategori isteği hata verdi:", error);
    return [];
  }
}

async function loadCategories() {
  cachedCategories = await fetchCategoriesPublic();

  if (!cachedCategories.length) {
    setCategoryStatus("Kategori bulunamadı. ( /categories boş dönüyor olabilir )");
    console.warn("Kategori listesi boş döndü.");
  } else {
    setCategoryStatus("");
  }

  console.debug("[ADMIN PRODUCTS] categories loaded:", cachedCategories.length, cachedCategories[0]);

  // Filter
  const filterSelect = el("categoryFilter");
  if (filterSelect && filterSelect.tagName === "SELECT") {
    const preserved = filterSelect.value || "";

    filterSelect.innerHTML = "";
    const def = document.createElement("option");
    def.value = "";
    def.textContent = "Tüm Kategoriler";
    filterSelect.appendChild(def);

    if (cachedCategories.length === 0) {
      const empty = document.createElement("option");
      empty.value = "";
      empty.textContent = "Kategori bulunamadı";
      empty.disabled = true;
      filterSelect.appendChild(empty);
    }

    cachedCategories.forEach(cat => {
      const id = getCategoryId(cat);
      if (!id) {
        console.warn("Kategori id bulunamadı:", cat);
        return;
      }
      const opt = document.createElement("option");
      opt.value = id;        // ✅ filtrede value=id
      opt.textContent = cat.name;
      filterSelect.appendChild(opt);
    });

    const exists = cachedCategories.some(cat => getCategoryId(cat) === preserved);
    filterSelect.value = exists ? preserved : "";
  }

  // Add/Edit selects
  const addSel = assertSelectExists("addProductCategorySelect", "category_id");
  const editSel = assertSelectExists("editProductCategorySelect", "category_id");

  fillCategorySelect(addSel, cachedCategories, []);
  fillCategorySelect(editSel, cachedCategories, []);
}

// ---------- Products ----------
function renderProductsTable(data) {
  const tbody = document.querySelector("#productList tbody");
  if (!tbody) return;
  tbody.innerHTML = "";

  if (!Array.isArray(data) || data.length === 0) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td class="muted" colspan="3">Ürün yok</td>`;
    tbody.appendChild(tr);
    return;
  }

  data.forEach((p) => {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${p.name || "-"}</td>
      <td>${p.isCampaign ? "Evet" : "Hayır"}</td>
      <td><button type="button" class="small">Düzenle</button></td>
    `;
    tr.querySelector("button").onclick = () => selectProduct(p);
    tbody.appendChild(tr);
  });
}

async function loadProducts() {
  const selected = el("categoryFilter")?.value || "";
  let url = "/admin/api/products";

  // ✅ backend: category=NAME, frontend: select value=id
  if (selected) {
    const matched = cachedCategories.find(cat => getCategoryId(cat) === selected);
    if (matched?.name) {
      url += "?" + new URLSearchParams({ category: matched.name }).toString();
    } else {
      console.warn("Kategori filtresi id eşleşmedi:", selected);
    }
  }

  setStatus("Ürünler yükleniyor...");

  const res = await fetch(url, { headers: authHeaders() });
  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    console.error("Ürünler isteği başarısız:", res.status, payload);
    setStatus("Hata: ürünler getirilemedi");
    return;
  }

  const data = (payload && payload.data) ? payload.data : (payload || []);
  currentProducts = Array.isArray(data) ? data : [];

  renderProductsTable(currentProducts);
  setStatus("");
}

function selectProduct(product) {
  selectedProduct = product;
  const id = getId(product);

  el("editProduct").style.display = "grid";
  el("prodName").innerText = product.name || "-";
  el("prodId").innerText = id ? ("(id: " + id + ")") : "(id yok)";

  const form = el("editProduct");
  form.elements.name.value = product.name || "";
  form.elements.price.value = (product.price ?? "");
  form.elements.isActive.checked = !!product.isActive;

  // Ürün kategorileri (legacy: isim listesi olabilir)
  const selectedCategories = normalizeCategoryValues(product.category);
  fillCategorySelect(el("editProductCategorySelect"), cachedCategories, selectedCategories);
}

async function handleDeleteProduct(product) {
  if (!product) return;
  const id = getId(product);
  if (!id) return alert("Ürün id yok");

  const ok = confirm("Bu ürünü silmek istediğinize emin misiniz?");
  if (!ok) return;

  const res = await fetch("/admin/api/products/" + id, {
    method: "DELETE",
    headers: authHeaders()
  });

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    alert("Silme başarısız: " + (payload?.error || res.statusText));
    return;
  }

  currentProducts = currentProducts.filter(item => getId(item) !== id);

  if (selectedProduct && getId(selectedProduct) === id) {
    selectedProduct = null;
    el("editProduct").style.display = "none";
  }

  renderProductsTable(currentProducts);
  setStatus("Ürün silindi");
}

// ---------- Events ----------
el("categoryFilter")?.addEventListener("change", loadProducts);

el("addProduct")?.addEventListener("submit", async function(event) {
  event.preventDefault();

  const formEl = event.target;
  const fd = new FormData(formEl);

  const price = parseFloat(fd.get("price"));
  if (Number.isNaN(price)) return alert("Fiyat sayı olmalı (örn 24.90)");

  const categorySelect = formEl.querySelector('select[name="category_id"]');
  if (!categorySelect) {
    console.error('Add form içinde select[name="category_id"] bulunamadı. HTML name yanlış.');
    return alert("Kategori alanı bulunamadı (HTML name='category_id' olmalı).");
  }

  const categories = getSelectedCategories(categorySelect);
  if (categories.length === 0) return alert("En az bir kategori seç");

  // select multiple -> aynı field name'i tekrar tekrar append et
  fd.delete("category_id");
  categories.forEach(c => fd.append("category_id", c)); // ✅ id gönder

  fd.set("price", String(price));

  const res = await fetch("/admin/api/products", {
    method: "POST",
    headers: authHeaders(),
    body: fd
  });

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    console.error("Ürün ekleme başarısız:", res.status, payload);
    alert(payload?.error || "Ürün eklenemedi");
    return;
  }

  formEl.reset();
  fillCategorySelect(el("addProductCategorySelect"), cachedCategories, []);
  await loadProducts();
});

el("editProduct")?.addEventListener("submit", async function(event) {
  event.preventDefault();
  if (!selectedProduct) return;

  const id = getId(selectedProduct);
  if (!id) return alert("Ürün id yok");

  const formEl = event.target;
  const fd = new FormData(formEl);

  const price = parseFloat(fd.get("price"));
  if (Number.isNaN(price)) return alert("Fiyat sayı olmalı");

  const categorySelect = formEl.querySelector('select[name="category_id"]');
  if (!categorySelect) {
    console.error('Edit form içinde select[name="category_id"] bulunamadı. HTML name yanlış.');
    return alert("Kategori alanı bulunamadı (HTML name='category_id' olmalı).");
  }

  const categories = getSelectedCategories(categorySelect);
  if (categories.length === 0) return alert("En az bir kategori seç");

  fd.delete("category_id");
  categories.forEach(c => fd.append("category_id", c)); // ✅ id gönder
  fd.set("price", String(price));

  fd.set("isActive", formEl.elements.isActive.checked ? "true" : "false");

  const res = await fetch("/admin/api/products/" + id, {
    method: "PUT",
    headers: authHeaders(),
    body: fd
  });

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    console.error("Ürün güncelleme başarısız:", res.status, payload);
    alert(payload?.error || "Ürün güncellenemedi");
    return;
  }

  await loadProducts();
});

el("deleteProduct")?.addEventListener("click", async function() {
  if (!selectedProduct) return;
  await handleDeleteProduct(selectedProduct);
});


// DOM hazır olmadan çalışmasın
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
window.__adminProducts = {
  loadCategories,
  loadProducts,
  fetchCategoriesPublic,
  fillCategorySelect,
  parseCategoriesPayload,
  getCategoryId,
  cachedCategories: () => cachedCategories
};