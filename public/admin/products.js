requireAuth();

let selectedProduct = null;
let currentProducts = [];

function setProductStatus(text) {
  setText("productStatus", text || "");
}

function normalizeCategoryValues(values) {
  if (Array.isArray(values)) {
    return values.filter(function(value) { return !!value; });
  }

  if (typeof values === "string" && values) {
    return [values];
  }

  return [];
}

function getSelectedCategories(select) {
  if (!select) return [];

  return Array.from(select.selectedOptions || [])
    .map(function(opt) { return opt.value; })
    .filter(function(value) { return !!value; });
}

async function populateProductCategorySelects(selectedValues, preloadedCategories) {
  const desiredSelection = normalizeCategoryValues(selectedValues);
  const categoryData = Array.isArray(preloadedCategories) && preloadedCategories.length > 0
    ? preloadedCategories
    : null;

  let categories = categoryData;

  if (!categories) {
    const res = await fetch("/categories");
    if (handleUnauthorized(res)) return;
    const payload = await safeJson(res);
    categories = (payload && payload.data) ? payload.data : (payload || []);
  }

  const activeCategories = (categories || []).filter(function(category) { return category && category.isActive; });
  const activeNames = new Set(activeCategories.map(function(category) { return category.name; }));
  const selects = document.querySelectorAll(".product-category-select");

  selects.forEach(function(select) {
    const preserved = desiredSelection.length > 0 ? desiredSelection : getSelectedCategories(select);
    select.innerHTML = "";
    select.multiple = true;

    const def = document.createElement("option");
    def.value = "";
    def.textContent = "Kategori Seç";
    def.disabled = true;
    if (preserved.length === 0) def.selected = true;
    select.appendChild(def);

    activeCategories.forEach(function(category) {
      const opt = document.createElement("option");
      opt.value = category.name;
      opt.textContent = category.name;
      select.appendChild(opt);
    });

    preserved.forEach(function(value) {
      if (!activeNames.has(value)) return;
      const opt = Array.from(select.options).find(function(option) { return option.value === value; });
      if (opt) opt.selected = true;
    });
  });
}

async function loadCategories() {
  const filterSelect = document.getElementById("categoryFilter");
  const preserved = filterSelect ? filterSelect.value : "";

  const res = await fetch("/categories");
  if (handleUnauthorized(res)) return;
  const payload = await safeJson(res);
  const data = (payload && payload.data) ? payload.data : (payload || []);

  await populateProductCategorySelects(undefined, data);

  if (filterSelect) {
    filterSelect.innerHTML = "";
    const def = document.createElement("option");
    def.value = "";
    def.textContent = "Tüm Kategoriler";
    filterSelect.appendChild(def);

    (data || []).forEach(function(category) {
      const opt = document.createElement("option");
      opt.value = category.name;
      opt.textContent = category.name;
      filterSelect.appendChild(opt);
    });

    const exists = (data || []).some(function(category){ return category.name === preserved; });
    filterSelect.value = exists ? preserved : "";
  }
}

async function toggleCampaign(checkbox) {
  const checked = checkbox.checked;
  const id = checkbox.dataset.id;

  if (!id) {
    checkbox.checked = !checked;
    alert("Ürün id yok");
    return;
  }

  checkbox.disabled = true;

  try {
    const res = await fetch("/admin/api/products/" + id, {
      method: "PUT",
      headers: authHeaders(),
      body: JSON.stringify({
        isCampaign: checked
      })
    });

    if (handleUnauthorized(res)) {
      checkbox.checked = !checked;
      alert("Kampanya güncellenemedi");
      return;
    }

    if (!res.ok) {
      checkbox.checked = !checked;
      alert("Kampanya güncellenemedi");
      return;
    }

    const updated = currentProducts.find(function(item) { return getId(item) === id; });
    if (updated) {
      updated.isCampaign = checked;
    }
  } catch (err) {
    checkbox.checked = !checked;
    alert("Kampanya güncellenemedi");
  } finally {
    checkbox.disabled = false;
  }
}

function renderProductList(data) {
  const table = document.getElementById("productList");
  const tbody = table.querySelector("tbody");
  tbody.innerHTML = "";

  if (!Array.isArray(data) || data.length === 0) {
    const emptyRow = document.createElement("tr");
    const emptyCell = document.createElement("td");
    emptyCell.colSpan = 3;
    emptyCell.className = "muted";
    emptyCell.textContent = "Ürün yok";
    emptyRow.appendChild(emptyCell);
    tbody.appendChild(emptyRow);
    return;
  }

  data.forEach(function(product) {
    const categoryLabel = Array.isArray(product.category)
      ? (product.category.length ? product.category.join(", ") : "-")
      : (product.category || "-");

    const row = document.createElement("tr");
    row.className = "product-row";

    const info = document.createElement("td");
    info.className = "stacked-text clickable";
    info.innerHTML =
      "<div><strong>" + (product.name || "-") + "</strong></div>" +
      "<div class='muted'>" +
        (product.price ?? "-") + " • " + categoryLabel + " • " + (product.isActive ? "Aktif" : "Pasif") +
      "</div>";
    info.onclick = function() { selectProduct(product); };

    const campaignCell = document.createElement("td");
    const campaignToggle = document.createElement("input");
    campaignToggle.type = "checkbox";
    campaignToggle.className = "campaign-toggle";
    campaignToggle.checked = !!product.isCampaign;
    campaignToggle.dataset.id = getId(product) || "";
    campaignToggle.addEventListener("click", function(event) {
      event.stopPropagation();
    });
    campaignToggle.addEventListener("change", function(event) {
      event.stopPropagation();
      toggleCampaign(campaignToggle);
    });
    campaignCell.appendChild(campaignToggle);

    const actions = document.createElement("td");
    actions.className = "inline-actions";

    const deleteBtn = document.createElement("button");
    deleteBtn.type = "button";
    deleteBtn.className = "danger ghost small";
    deleteBtn.textContent = "Sil";
    deleteBtn.onclick = function(ev) {
      ev.stopPropagation();
      handleDeleteProduct(product);
    };

    actions.appendChild(deleteBtn);

    row.appendChild(info);
    row.appendChild(campaignCell);
    row.appendChild(actions);

    tbody.appendChild(row);
  });
}

async function loadProducts() {
  const selected = document.getElementById("categoryFilter").value;
  let url = "/admin/api/products";

  if (selected) {
    url += "?" + new URLSearchParams({ category: selected }).toString();
  }

  setProductStatus("Ürünler yükleniyor...");

  const res = await fetch(url, { headers: authHeaders() });
  if (handleUnauthorized(res)) return;
  const payload = await safeJson(res);
  if (!res.ok) {
    setProductStatus("Hata: ürünler getirilemedi");
    return;
  }

  const data = (payload && payload.data) ? payload.data : (payload || []);
  currentProducts = Array.isArray(data) ? data : [];

  renderProductList(currentProducts);
  setProductStatus("");
}

async function selectProduct(product) {
  selectedProduct = product;
  const id = getId(product);

  const categories = normalizeCategoryValues(product.category);

  document.getElementById("editProduct").style.display = "grid";
  document.getElementById("prodName").innerText = product.name || "-";
  document.getElementById("prodId").innerText = id ? ("(id: " + id + ")") : "(id yok)";

  await populateProductCategorySelects(categories);

  const form = document.getElementById("editProduct");
  form.elements.name.value = product.name || "";
  form.elements.price.value = (product.price ?? "");
  form.elements.imageUrl.value = product.imageUrl || "";
  form.elements.isActive.checked = !!product.isActive;
}

async function handleDeleteProduct(product) {
  if (!product) return;

  const id = getId(product);
  if (!id) {
    alert("Ürün id yok");
    return;
  }

  const confirmed = confirm("Bu ürünü silmek istediğinize emin misiniz?");
  if (!confirmed) return;

  const res = await fetch("/admin/api/products/" + id, {
    method: "DELETE",
    headers: authHeaders()
  });
  if (handleUnauthorized(res)) return;
  const payload = await safeJson(res);

  if (!res.ok) {
    alert("Silme başarısız: " + ((payload && payload.error) ? payload.error : res.statusText));
    return;
  }

  currentProducts = currentProducts.filter(function(item) { return getId(item) !== id; });
  if (selectedProduct && getId(selectedProduct) === id) {
    selectedProduct = null;
    document.getElementById("editProduct").style.display = "none";
  }

  renderProductList(currentProducts);
  setProductStatus("Ürün silindi");
}

document.getElementById("categoryFilter").addEventListener("change", function() {
  loadProducts();
});

document.getElementById("addProduct").addEventListener("submit", async function(event) {
  event.preventDefault();

  const form = new FormData(event.target);
  const price = parseFloat(form.get("price"));
  if (Number.isNaN(price)) {
    alert("Fiyat sayı olmalı (örn 24.90)");
    return;
  }

  const categories = getSelectedCategories(event.target.querySelector('select[name="category"]'));
  if (categories.length === 0) {
    alert("En az bir kategori seç");
    return;
  }

  const res = await fetch("/admin/api/products", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({
      name: form.get("name"),
      price: price,
      category: categories,
      imageUrl: form.get("imageUrl"),
      isActive: true
    })
  });

  if (handleUnauthorized(res)) return;

  event.target.reset();
  loadProducts();
});

document.getElementById("editProduct").addEventListener("submit", async function(event) {
  event.preventDefault();
  if (!selectedProduct) return;

  const id = getId(selectedProduct);
  if (!id) {
    alert("Ürün id yok");
    return;
  }

  const form = new FormData(event.target);
  const price = parseFloat(form.get("price"));
  if (Number.isNaN(price)) {
    alert("Fiyat sayı olmalı");
    return;
  }

  const categories = getSelectedCategories(event.target.querySelector('select[name="category"]'));
  if (categories.length === 0) {
    alert("En az bir kategori seç");
    return;
  }

  const res = await fetch("/admin/api/products/" + id, {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify({
      name: form.get("name"),
      price: price,
      category: categories,
      imageUrl: form.get("imageUrl"),
      isActive: form.get("isActive") === "on"
    })
  });

  if (handleUnauthorized(res)) return;

  loadProducts();
});

document.getElementById("deleteProduct").addEventListener("click", async function() {
  if (!selectedProduct) return;

  await handleDeleteProduct(selectedProduct);
});

loadCategories();
loadProducts();