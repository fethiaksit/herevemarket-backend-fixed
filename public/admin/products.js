requireAuth();

let selectedProduct = null;
let currentEditProductId = "";
let lastHydratedEditProductId = "";
let editLoadSequence = 0;
let currentProducts = [];
let cachedCategories = [];
let currentPage = 1;
let pageLimit = 20;
let totalPages = 1;
let lastQueryKey = "";
let searchDebounceTimer = null;
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
      try {
        const rawBody = await res.text();
        if (rawBody) {
          console.error("Kategori isteği ham cevap:", rawBody);
        }
      } catch (error) {
        console.error("Kategori isteği ham cevap okunamadı:", error);
      }
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
  const addSelectCount = document.querySelectorAll("#addProductCategorySelect").length;
  if (addSelectCount > 1) {
    console.error(`[ADMIN PRODUCTS] Duplicate id bulundu: #addProductCategorySelect (${addSelectCount})`);
  }

  fillCategorySelect(addSel, cachedCategories, []);
  fillCategorySelect(editSel, cachedCategories, []);

  if (cachedCategories.length > 0 && addSel) {
    const expectedMinOptions = 2; // placeholder + en az 1 kategori
    if (addSel.options.length < expectedMinOptions) {
      console.error(
        "[ADMIN PRODUCTS] Kategori select doldurulamadı.",
        {
          selectId: "addProductCategorySelect",
          selectHtml: addSel.outerHTML,
          cachedCategoriesCount: cachedCategories.length,
          sampleCategory: cachedCategories[0]
        }
      );
    }
  }
}

// ---------- Products ----------
function renderProductsTable(data) {
  const tbody = document.querySelector("#productList tbody");
  if (!tbody) return;
  tbody.innerHTML = "";

  const searchValue = (el("productSearch")?.value || "").trim();
  if (!Array.isArray(data) || data.length === 0) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td class="muted" colspan="6">${searchValue ? "Sonuç bulunamadı" : "Ürün yok"}</td>`;
    tbody.appendChild(tr);
    return;
  }

  data.forEach((p) => {
    const tr = document.createElement("tr");
    const productId = getId(p);
    tr.innerHTML = `
      <td>${p.name || "-"}</td>
      <td>${p.brand || "-"}</td>
      <td>${p.barcode || "-"}</td>
      <td>${Number.isFinite(Number(p.stock)) ? Number(p.stock) : "-"}</td>
      <td>${parseBooleanValue(p.isCampaign) ? "Evet" : "Hayır"}</td>
      <td><button type="button" class="small" data-product-id="${productId || ""}">Düzenle</button></td>
    `;

    tr.querySelector("button").onclick = () => {
      openEditProductModal(productId);
    };
    tbody.appendChild(tr);
  });
}

function ensurePaginationContainer() {
  let container = el("productPagination");
  if (container) return container;

  const table = el("productList");
  if (!table || !table.parentNode) return null;

  container = document.createElement("div");
  container.id = "productPagination";
  container.className = "pagination";

  const status = el("productStatus");
  if (status && status.parentNode === table.parentNode) {
    status.insertAdjacentElement("afterend", container);
  } else {
    table.insertAdjacentElement("afterend", container);
  }

  return container;
}

function renderPagination() {
  const container = ensurePaginationContainer();
  if (!container) return;

  container.innerHTML = "";

  const safeTotalPages = Math.max(1, totalPages || 1);
  const safePage = Math.min(Math.max(1, currentPage || 1), safeTotalPages);

  const prevBtn = document.createElement("button");
  prevBtn.type = "button";
  prevBtn.textContent = "Önceki";
  prevBtn.disabled = safePage <= 1;
  prevBtn.addEventListener("click", () => loadProducts(safePage - 1));

  const nextBtn = document.createElement("button");
  nextBtn.type = "button";
  nextBtn.textContent = "Sonraki";
  nextBtn.disabled = safePage >= safeTotalPages;
  nextBtn.addEventListener("click", () => loadProducts(safePage + 1));

  const pageInfo = document.createElement("span");
  pageInfo.className = "muted";
  pageInfo.textContent = `Sayfa ${safePage} / ${safeTotalPages}`;

  const limitWrap = document.createElement("label");
  limitWrap.className = "muted";
  limitWrap.textContent = "Sayfa başı ";

  const limitSelect = document.createElement("select");
  [10, 20, 50, 100].forEach((limit) => {
    const opt = document.createElement("option");
    opt.value = String(limit);
    opt.textContent = String(limit);
    if (Number(limit) === Number(pageLimit)) opt.selected = true;
    limitSelect.appendChild(opt);
  });

  limitSelect.addEventListener("change", (event) => {
    const nextLimit = Number(event.target.value);
    if (!Number.isNaN(nextLimit)) {
      pageLimit = nextLimit;
    }
    loadProducts(1);
  });

  limitWrap.appendChild(limitSelect);

  container.appendChild(prevBtn);
  container.appendChild(pageInfo);
  container.appendChild(nextBtn);
  container.appendChild(limitWrap);
}

function setSearchStatus(message) {
  const node = el("productSearchStatus");
  if (node) {
    node.textContent = message || "";
  }
}

function scheduleProductSearch() {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer);
  }

  setSearchStatus("Aranıyor…");
  searchDebounceTimer = setTimeout(() => {
    loadProducts(1);
  }, 400);
}

function setSalePriceVisibility(formEl, enabled) {
  if (!formEl || !formEl.elements?.salePrice) return;
  const salePriceInput = formEl.elements.salePrice;
  const salePriceLabel = salePriceInput.previousElementSibling;
  salePriceInput.disabled = !enabled;
  salePriceInput.style.display = enabled ? "" : "none";
  if (salePriceLabel && salePriceLabel.tagName === "LABEL") {
    salePriceLabel.style.display = enabled ? "" : "none";
  }
}

function parseBooleanValue(value) {
  if (typeof value === "boolean") return value;
  if (typeof value === "number") return value === 1;
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return normalized === "true" || normalized === "1";
  }
  return false;
}


function getSaleEnabledCheckbox(formEl) {
  if (!formEl) return null;
  return formEl.querySelector('input[type="checkbox"][name="saleEnabled"]');
}

function resetSaleFields(formEl) {
  const saleCheckbox = getSaleEnabledCheckbox(formEl);
  if (!formEl || !saleCheckbox || !formEl.elements?.salePrice) return;
  saleCheckbox.checked = false;
  formEl.elements.salePrice.value = "";
  setSalePriceVisibility(formEl, false);
}

function hydrateSaleFields(formEl, product) {
  const saleCheckbox = getSaleEnabledCheckbox(formEl);
  if (!formEl || !saleCheckbox || !formEl.elements?.salePrice) return;
  const saleEnabled = parseBooleanValue(product?.saleEnabled);
  saleCheckbox.checked = saleEnabled;
  if (!saleEnabled) {
    formEl.elements.salePrice.value = "";
    setSalePriceVisibility(formEl, false);
    return;
  }

  const salePrice = Number(product?.salePrice);
  formEl.elements.salePrice.value = Number.isFinite(salePrice) && salePrice > 0 ? String(salePrice) : "";
  setSalePriceVisibility(formEl, true);
}

function bindSaleToggle(formEl) {
  const saleCheckbox = getSaleEnabledCheckbox(formEl);
  if (!formEl || !saleCheckbox || !formEl.elements?.salePrice) return;
  saleCheckbox.addEventListener("change", () => {
    const enabled = !!saleCheckbox.checked;
    setSalePriceVisibility(formEl, enabled);
    if (!enabled) {
      formEl.elements.salePrice.value = "";
      return;
    }
    formEl.elements.salePrice.focus();
  });
}

async function loadProducts(page = 1) {
  const selected = el("categoryFilter")?.value || "";
  const searchInput = el("productSearch") || el("searchInput");
  const isActiveFilter = el("isActiveFilter");
  const params = new URLSearchParams();

  // ✅ backend: category=NAME, frontend: select value=id
  if (selected) {
    const matched = cachedCategories.find(cat => getCategoryId(cat) === selected);
    if (matched?.name) {
      params.set("category", matched.name);
    } else {
      console.warn("Kategori filtresi id eşleşmedi:", selected);
    }
  }

  const searchValue = typeof searchInput?.value === "string" ? searchInput.value.trim() : "";
  if (searchValue) {
    params.set("search", searchValue);
  }

  if (isActiveFilter) {
    if (isActiveFilter.type === "checkbox") {
      if (isActiveFilter.checked) {
        params.set("isActive", "true");
      }
    } else if (isActiveFilter.value !== "") {
      params.set("isActive", isActiveFilter.value);
    }
  }

  const queryKey = JSON.stringify({
    category: params.get("category") || "",
    search: params.get("search") || "",
    isActive: params.get("isActive") || ""
  });
  if (lastQueryKey && lastQueryKey !== queryKey && page !== 1) {
    page = 1;
  }

  params.set("page", String(page));
  params.set("limit", String(pageLimit));
  const url = "/admin/api/products?" + params.toString();

  setStatus("Ürünler yükleniyor...");

  let res;
  try {
    res = await fetch(url, { headers: authHeaders() });
  } catch (error) {
    console.error("Ürünler isteği ağ hatası:", error);
    setStatus("Hata: ürünler getirilemedi");
    return;
  }

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    console.error("Ürünler isteği başarısız:", res.status, payload);
    setStatus("Hata: ürünler getirilemedi");
    return;
  }

  const data = (payload && payload.data) ? payload.data : (payload || []);
  currentProducts = Array.isArray(data) ? data : [];
  const pagination = payload?.pagination || {};
  const total = Number(pagination.total);

  currentPage = Number(pagination.page) || page;
  pageLimit = Number(pagination.limit) || pageLimit;
  if (!Number.isNaN(Number(pagination.totalPages))) {
    totalPages = Number(pagination.totalPages) || 1;
  } else if (!Number.isNaN(total) && total >= 0) {
    totalPages = Math.max(1, Math.ceil(total / pageLimit));
  } else {
    totalPages = 1;
  }

  lastQueryKey = queryKey;

  renderProductsTable(currentProducts);
  renderPagination();
  setStatus("");
  setSearchStatus("");
}

function selectProduct(product) {
  selectedProduct = product;
  const id = getId(product);
  currentEditProductId = id ? String(id) : "";

  el("editProduct").style.display = "grid";
  const deleteButton = el("deleteProduct");
  if (deleteButton) {
    deleteButton.disabled = false;
  }
  el("prodName").innerText = product.name || "-";
  el("prodId").innerText = id ? ("(id: " + id + ")") : "(id yok)";

  const form = el("editProduct");
  if (!form) return;

  form.reset();
  resetSaleFields(form);
  form.elements.productId.value = currentEditProductId;

  form.elements.name.value = product.name || "";
  form.elements.price.value = (product.price ?? "");
  hydrateSaleFields(form, product);
  form.elements.brand.value = product.brand || "";
  form.elements.barcode.value = product.barcode || "";
  form.elements.stock.value = (product.stock ?? "");
  form.elements.description.value = product.description || "";
  form.elements.isActive.checked = parseBooleanValue(product.isActive);
  form.elements.isCampaign.checked = parseBooleanValue(product.isCampaign);

  // Ürün kategorileri (legacy: isim listesi olabilir)
  const selectedCategories = normalizeCategoryValues(product.category);
  fillCategorySelect(el("editProductCategorySelect"), cachedCategories, selectedCategories);

  const preview = el("editProductImagePreview");
  if (preview) {
    const imagePath = (product.imagePath || "").trim();
    if (imagePath) {
      preview.src = "/public/" + imagePath.replace(/^\/+/, "");
      preview.style.display = "block";
    } else {
      preview.removeAttribute("src");
      preview.style.display = "none";
    }
  }
}

function resetEditFormState() {
  const form = el("editProduct");
  if (!form) return;

  form.reset();
  selectedProduct = null;
  currentEditProductId = "";
  lastHydratedEditProductId = "";
  form.elements.productId.value = "";
  form.elements.name.value = "";
  form.elements.price.value = "";
  form.elements.salePrice.value = "0";
  form.elements.brand.value = "";
  form.elements.barcode.value = "";
  form.elements.stock.value = "";
  form.elements.description.value = "";
  form.elements.isActive.checked = false;
  form.elements.isCampaign.checked = false;
  resetSaleFields(form);
  fillCategorySelect(el("editProductCategorySelect"), cachedCategories, []);

  el("prodName").innerText = "-";
  el("prodId").innerText = "(id yok)";

  const preview = el("editProductImagePreview");
  if (preview) {
    preview.removeAttribute("src");
    preview.style.display = "none";
  }
}

function mapProductToEditForm(form, product) {
  if (!form || !product) return;

  const id = String(getId(product) || "").trim();
  selectedProduct = product;
  currentEditProductId = id;
  lastHydratedEditProductId = id;

  const safePrice = Number(product?.price);
  const saleEnabled = parseBooleanValue(product?.saleEnabled);
  const salePrice = Number(product?.salePrice);

  form.elements.productId.value = id;
  form.elements.name.value = String(product?.name || "");
  form.elements.price.value = Number.isFinite(safePrice) ? String(safePrice) : "";
  form.elements.brand.value = String(product?.brand || "");
  form.elements.barcode.value = String(product?.barcode || "");
  form.elements.stock.value = Number.isFinite(Number(product?.stock)) ? String(Number(product.stock)) : "";
  form.elements.description.value = String(product?.description || "");
  form.elements.isActive.checked = parseBooleanValue(product?.isActive);
  form.elements.isCampaign.checked = parseBooleanValue(product?.isCampaign);

  const saleCheckbox = getSaleEnabledCheckbox(form);
  if (saleCheckbox) {
    saleCheckbox.checked = saleEnabled;
  }
  form.elements.salePrice.value = saleEnabled && Number.isFinite(salePrice) && salePrice > 0
    ? String(salePrice)
    : "0";
  setSalePriceVisibility(form, saleEnabled);

  const selectedCategories = normalizeCategoryValues(product?.category);
  fillCategorySelect(el("editProductCategorySelect"), cachedCategories, selectedCategories);

  el("prodName").innerText = product?.name || "-";
  el("prodId").innerText = id ? `(id: ${id})` : "(id yok)";

  const preview = el("editProductImagePreview");
  if (preview) {
    const imagePath = String(product?.imagePath || "").trim();
    if (imagePath) {
      preview.src = "/public/" + imagePath.replace(/^\/+/, "");
      preview.style.display = "block";
    } else {
      preview.removeAttribute("src");
      preview.style.display = "none";
    }
  }
}

async function fetchProductForEdit(productId) {
  const res = await fetch("/admin/api/products/" + productId, {
    headers: authHeaders()
  });

  if (handleUnauthorized(res)) return null;

  const payload = await safeJson(res);
  if (!res.ok) {
    throw new Error(payload?.error || "Ürün getirilemedi");
  }

  return payload;
}

async function openEditProductModal(productId) {
  const id = String(productId || "").trim();
  if (!id) {
    alert("Ürün id yok");
    return;
  }

  const form = el("editProduct");
  if (!form) return;

  form.style.display = "grid";
  const deleteButton = el("deleteProduct");
  if (deleteButton) {
    deleteButton.disabled = true;
  }

  await loadProductForEdit(id);
}

async function loadProductForEdit(productId) {
  const id = String(productId || "").trim();
  if (!id) {
    resetEditFormState();
    return;
  }

  const form = el("editProduct");
  if (!form) return;

  const currentSequence = ++editLoadSequence;
  form.style.display = "grid";
  currentEditProductId = id;
  form.elements.productId.value = id;
  resetEditFormState();
  form.elements.productId.value = id;

  const deleteButton = el("deleteProduct");
  if (deleteButton) {
    deleteButton.disabled = true;
  }

  setStatus("Ürün detayları yükleniyor...");

  try {
    const product = await fetchProductForEdit(id);
    if (!product) {
      if (currentSequence === editLoadSequence) {
        setStatus("");
      }
      return;
    }
    if (currentSequence !== editLoadSequence) {
      return;
    }

    mapProductToEditForm(form, product);
    if (deleteButton) {
      deleteButton.disabled = false;
    }
    setStatus("");
  } catch (error) {
    if (currentSequence !== editLoadSequence) {
      return;
    }
    console.error("Ürün detayı yüklenemedi:", error);
    setStatus("Hata: ürün detayı getirilemedi");
    alert(error?.message || "Ürün detayı getirilemedi");
  }
}

async function handleDeleteProduct(product) {
  if (!product) return;
  const id = getId(product);
  if (!id) return alert("Ürün id yok");

  const ok = confirm("Bu ürünü silmek istediğinize emin misiniz?");
  if (!ok) return;

  let res;
  try {
    res = await fetch("/admin/api/products/" + id, {
      method: "DELETE",
      headers: authHeaders()
    });
  } catch (error) {
    console.error("Ürün silme isteği ağ hatası:", error);
    setStatus("Silme sırasında hata oluştu");
    return;
  }

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    console.error("Ürün silme başarısız:", res.status, payload);
    setStatus("Silme başarısız");
    alert("Silme başarısız: " + (payload?.error || res.statusText));
    return;
  }

  currentProducts = currentProducts.filter(item => getId(item) !== id);

  if (selectedProduct && getId(selectedProduct) === id) {
    selectedProduct = null;
    currentEditProductId = "";
    el("editProduct").style.display = "none";
  }

  renderProductsTable(currentProducts);
  setStatus("Ürün silindi");

  const nextPage = currentProducts.length === 0 && currentPage > 1
    ? currentPage - 1
    : currentPage;
  await loadProducts(nextPage);
}

// ---------- Events ----------
el("categoryFilter")?.addEventListener("change", () => loadProducts(1));
const searchFilterInput = el("productSearch") || el("searchInput");
if (searchFilterInput) {
  searchFilterInput.addEventListener("input", scheduleProductSearch);
}
el("clearProductSearch")?.addEventListener("click", () => {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer);
    searchDebounceTimer = null;
  }

  const input = el("productSearch") || el("searchInput");
  if (input) {
    input.value = "";
  }
  setSearchStatus("");
  loadProducts(1);
});
const isActiveFilterEl = el("isActiveFilter");
if (isActiveFilterEl) {
  isActiveFilterEl.addEventListener("change", () => loadProducts(1));
}

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
  const saleEnabled = !!getSaleEnabledCheckbox(formEl)?.checked;
  fd.set("saleEnabled", saleEnabled ? "true" : "false");
  const parsedPrice = Number(price);
  const salePriceValue = String(formEl.elements.salePrice.value || "").trim();
  if (saleEnabled) {
    if (salePriceValue === "") return alert("İndirimli fiyat giriniz");
    const salePrice = parseFloat(salePriceValue);
    if (Number.isNaN(salePrice)) return alert("İndirimli fiyat sayı olmalı");
    if (salePrice <= 0) return alert("İndirimli fiyat 0'dan büyük olmalı");
    if (salePrice >= parsedPrice) return alert("İndirimli fiyat normal fiyattan düşük olmalı");
    fd.set("salePrice", String(salePrice));
  } else {
    fd.set("salePrice", "0");
  }

  const res = await fetch("/admin/api/products", {
    method: "POST",
    headers: authHeadersMultipart(),
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
  resetSaleFields(formEl);
  fillCategorySelect(el("addProductCategorySelect"), cachedCategories, []);
  await loadProducts(currentPage);
});

el("editProduct")?.addEventListener("submit", async function(event) {
  event.preventDefault();

  const formEl = event.target;
  const id = String(formEl.elements.productId.value || currentEditProductId || "").trim();
  if (!id) return alert("Ürün id yok");
  const fd = new FormData();
  fd.set("name", formEl.elements.name.value || "");
  fd.set("price", formEl.elements.price.value || "");
  fd.set("brand", formEl.elements.brand.value || "");
  fd.set("barcode", formEl.elements.barcode.value || "");
  const stockValue = formEl.elements.stock.value || "";
  fd.set("stock", stockValue);
  fd.set("description", formEl.elements.description.value || "");
  const stockNumber = parseInt(stockValue, 10);
  fd.set("inStock", Number.isFinite(stockNumber) ? String(stockNumber > 0) : "false");

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
  const saleEnabled = !!getSaleEnabledCheckbox(formEl)?.checked;
  fd.set("saleEnabled", saleEnabled ? "true" : "false");
  const parsedPrice = Number(price);
  const salePriceValue = String(formEl.elements.salePrice.value || "").trim();
  if (saleEnabled) {
    if (salePriceValue === "") return alert("İndirimli fiyat giriniz");
    const salePrice = parseFloat(salePriceValue);
    if (Number.isNaN(salePrice)) return alert("İndirimli fiyat sayı olmalı");
    if (salePrice <= 0) return alert("İndirimli fiyat 0'dan büyük olmalı");
    if (salePrice >= parsedPrice) return alert("İndirimli fiyat normal fiyattan düşük olmalı");
    fd.set("salePrice", String(salePrice));
  } else {
    fd.set("salePrice", "0");
  }

  fd.set("isActive", formEl.elements.isActive.checked ? "true" : "false");
  fd.set("isCampaign", formEl.elements.isCampaign.checked ? "true" : "false");

  const imageInput = formEl.querySelector('input[name="image"]');
  if (imageInput && imageInput.files && imageInput.files.length > 0) {
    fd.append("image", imageInput.files[0]);
  }

  const res = await fetch("/admin/api/products/" + id, {
    method: "PUT",
    headers: authHeadersMultipart(),
    body: fd
  });

  if (handleUnauthorized(res)) return;

  const payload = await safeJson(res);
  if (!res.ok) {
    console.error("Ürün güncelleme başarısız:", res.status, payload);
    alert(payload?.error || "Ürün güncellenemedi");
    return;
  }

  await loadProducts(currentPage);
});

el("deleteProduct")?.addEventListener("click", async function() {
  const id = String(el("editProduct")?.elements?.productId?.value || currentEditProductId || "").trim();
  if (!id) return;

  const product = currentProducts.find(item => String(getId(item) || "") === id) || selectedProduct;
  if (!product) return;

  await handleDeleteProduct(product);
});

el("editProductId")?.addEventListener("change", async function(event) {
  const nextId = String(event?.target?.value || "").trim();
  if (!nextId || nextId === lastHydratedEditProductId) return;
  await loadProductForEdit(nextId);
});

async function init() {
  const deleteButton = el("deleteProduct");
  if (deleteButton) {
    deleteButton.disabled = true;
  }
  setSearchStatus("");
  resetSaleFields(el("addProduct"));
  resetSaleFields(el("editProduct"));
  bindSaleToggle(el("addProduct"));
  bindSaleToggle(el("editProduct"));
  await loadCategories();
  await loadProducts(1);
}

// DOM hazır olmadan çalışmasın
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
window.__adminProducts = {
  init,
  loadProductForEdit,
  loadCategories,
  loadProducts,
  fetchCategoriesPublic,
  fillCategorySelect,
  parseCategoriesPayload,
  getCategoryId,
  cachedCategories: () => cachedCategories
};
