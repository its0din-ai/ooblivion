/* ==========================================================================
   [oob]livion admin — Material 3 interactions driven by anime.js v4
   ========================================================================== */

"use strict";

const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
const desktopQuery = window.matchMedia("(min-width: 1024px)");

function canAnimate() {
  return !motionQuery.matches && !!window.anime;
}

const tsFormat = new Intl.DateTimeFormat("id-ID", {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
});

function formatHumanTime(iso) {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return tsFormat.format(d);
}

/* --------------------------------------------------------------------------
   Theme
   -------------------------------------------------------------------------- */

function currentTheme() {
  const stored = localStorage.getItem("oob_theme");
  if (stored === "light" || stored === "dark") return stored;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function syncThemeIcons() {
  const dark = currentTheme() === "dark";
  document.querySelectorAll(".theme-icon-sun").forEach((el) => {
    el.style.display = dark ? "none" : "inline";
  });
  document.querySelectorAll(".theme-icon-moon").forEach((el) => {
    el.style.display = dark ? "inline" : "none";
  });
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) {
    const surface = getComputedStyle(document.documentElement)
      .getPropertyValue("--md-surface")
      .trim();
    if (surface) meta.setAttribute("content", surface);
  }
}

function animateThemeIcon() {
  if (!canAnimate()) return;
  const dark = currentTheme() === "dark";
  const icons = document.querySelectorAll(
    dark ? ".theme-icon-moon" : ".theme-icon-sun"
  );
  if (!icons.length) return;
  anime.animate(icons, {
    rotate: [-90, 0],
    scale: [0.4, 1],
    opacity: [0, 1],
    duration: 460,
    ease: "outBack",
  });
}

function applyTheme(theme) {
  if (theme === "light" || theme === "dark") {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("oob_theme", theme);
  } else {
    document.documentElement.removeAttribute("data-theme");
    localStorage.removeItem("oob_theme");
  }
  syncThemeIcons();
}

function toggleTheme() {
  applyTheme(currentTheme() === "dark" ? "light" : "dark");
  animateThemeIcon();
}

/* --------------------------------------------------------------------------
   HTTP helpers
   -------------------------------------------------------------------------- */

async function api(path, opts = {}) {
  const headers = Object.assign({}, opts.headers || {});
  if (opts.body && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }
  const res = await fetch(path, Object.assign({}, opts, { headers }));
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || res.statusText || "request failed");
  }
  return data;
}

/* --------------------------------------------------------------------------
   Snackbar
   -------------------------------------------------------------------------- */

let toastTimer = null;
let toastAnimation = null;

function hideToast() {
  const el = document.getElementById("toast");
  if (!el || el.hidden) return;
  if (!canAnimate()) {
    el.hidden = true;
    return;
  }
  toastAnimation = anime.animate(el, {
    opacity: [1, 0],
    translateY: [0, 12],
    duration: 220,
    ease: "inQuad",
    onComplete: () => {
      el.hidden = true;
      el.style.removeProperty("opacity");
      el.style.removeProperty("transform");
    },
  });
}

function toast(msg) {
  const el = document.getElementById("toast");
  if (!el) return;
  el.textContent = msg;
  el.hidden = false;
  if (toastAnimation && toastAnimation.cancel) toastAnimation.cancel();
  if (canAnimate()) {
    toastAnimation = anime.animate(el, {
      opacity: [0, 1],
      translateY: [24, 0],
      scale: [0.96, 1],
      duration: 440,
      ease: "outBack",
    });
  }
  clearTimeout(toastTimer);
  toastTimer = setTimeout(hideToast, 3200);
}

/* --------------------------------------------------------------------------
   Dialog
   -------------------------------------------------------------------------- */

function confirmDialog(msg, okLabel = "Confirm") {
  return new Promise((resolve) => {
    const overlay = document.getElementById("modal");
    if (!overlay) return resolve(false);
    const dialog = overlay.querySelector(".modal");
    const message = document.getElementById("modal-message");
    const ok = document.getElementById("modal-ok");
    const cancel = document.getElementById("modal-cancel");

    message.textContent = msg;
    ok.textContent = okLabel;

    let settled = false;

    const cleanup = () => {
      overlay.hidden = true;
      overlay.style.removeProperty("opacity");
      dialog.style.removeProperty("opacity");
      dialog.style.removeProperty("transform");
      document.removeEventListener("keydown", onKey);
      ok.onclick = cancel.onclick = overlay.onclick = null;
    };

    const finish = (val) => {
      if (settled) return;
      settled = true;
      document.removeEventListener("keydown", onKey);
      ok.onclick = cancel.onclick = overlay.onclick = null;
      if (!canAnimate()) {
        cleanup();
        resolve(val);
        return;
      }
      anime.animate(dialog, {
        opacity: [1, 0],
        scale: [1, 0.92],
        duration: 150,
        ease: "inQuad",
      });
      anime.animate(overlay, {
        opacity: [1, 0],
        duration: 160,
        ease: "inQuad",
        onComplete: () => {
          cleanup();
          resolve(val);
        },
      });
    };

    const onKey = (e) => {
      if (e.key === "Escape") finish(false);
      if (e.key === "Enter") finish(true);
    };

    ok.onclick = () => finish(true);
    cancel.onclick = () => finish(false);
    overlay.onclick = (e) => {
      if (e.target === overlay) finish(false);
    };
    document.addEventListener("keydown", onKey);

    overlay.hidden = false;
    if (canAnimate()) {
      overlay.style.opacity = "0";
      dialog.style.opacity = "0";
      anime.animate(overlay, {
        opacity: [0, 1],
        duration: 200,
        ease: "outQuad",
      });
      anime.animate(dialog, {
        opacity: [0, 1],
        scale: [0.9, 1],
        translateY: [12, 0],
        duration: 340,
        ease: "outBack",
      });
    }
    ok.focus();
  });
}

/* --------------------------------------------------------------------------
   Atomic animation helpers shared with page scripts
   -------------------------------------------------------------------------- */

function countUp(el, value) {
  if (!el) return;
  const to = Number(value);
  if (!isFinite(to)) {
    el.textContent = String(value);
    return;
  }
  if (!canAnimate()) {
    el.textContent = String(to);
    return;
  }
  const state = { value: 0 };
  anime.animate(state, {
    value: to,
    duration: 900,
    ease: "outExpo",
    onUpdate: () => {
      el.textContent = String(Math.round(state.value));
    },
  });
}

function reveal(el) {
  if (!el) return;
  el.hidden = false;
  if (!canAnimate()) return;
  anime.animate(el, {
    opacity: [0, 1],
    translateY: [-10, 0],
    duration: 340,
    ease: "outQuart",
  });
}

function conceal(el, done) {
  if (!el) return;
  if (!canAnimate()) {
    el.hidden = true;
    if (done) done();
    return;
  }
  anime.animate(el, {
    opacity: [1, 0],
    translateY: [0, -10],
    duration: 200,
    ease: "inQuad",
    onComplete: () => {
      el.hidden = true;
      el.style.removeProperty("opacity");
      el.style.removeProperty("transform");
      if (done) done();
    },
  });
}

function pop(el) {
  if (!el || !canAnimate()) return;
  anime.animate(el, { scale: [1, 1.08, 1], duration: 360, ease: "outBack" });
}

async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.style.position = "fixed";
  ta.style.opacity = "0";
  document.body.appendChild(ta);
  ta.select();
  document.execCommand("copy");
  ta.remove();
}

/* --------------------------------------------------------------------------
   Navigation drawer
   -------------------------------------------------------------------------- */

function initDrawer() {
  const drawer = document.getElementById("drawer");
  const nav = document.getElementById("nav");
  const toggle = document.getElementById("nav-toggle");
  const scrim = document.getElementById("scrim");
  if (!drawer || !toggle || !scrim) return;

  let open = false;

  function setOpen(next) {
    if (desktopQuery.matches) {
      open = false;
      drawer.classList.remove("open");
      drawer.style.removeProperty("transform");
      scrim.hidden = true;
      toggle.setAttribute("aria-expanded", "false");
      return;
    }
    if (next === open) return;
    open = next;
    drawer.classList.toggle("open", open);
    toggle.setAttribute("aria-expanded", open ? "true" : "false");
    document.body.classList.toggle("drawer-open", open);

    const width = drawer.getBoundingClientRect().width || 300;

    if (!canAnimate()) {
      scrim.hidden = !open;
      if (!open) drawer.style.removeProperty("transform");
      return;
    }

    if (open) {
      scrim.hidden = false;
      scrim.style.opacity = "0";
      anime.animate(drawer, {
        translateX: [-width, 0],
        duration: 380,
        ease: "outQuart",
      });
      anime.animate(scrim, {
        opacity: [0, 1],
        duration: 240,
        ease: "outQuad",
      });
    } else {
      anime.animate(drawer, {
        translateX: [0, -width],
        duration: 280,
        ease: "inQuart",
        onComplete: () => {
          if (!open) drawer.style.removeProperty("transform");
        },
      });
      anime.animate(scrim, {
        opacity: [1, 0],
        duration: 220,
        ease: "inQuad",
        onComplete: () => {
          if (!open) {
            scrim.hidden = true;
            scrim.style.removeProperty("opacity");
          }
        },
      });
    }
  }

  toggle.addEventListener("click", () => setOpen(!open));
  scrim.addEventListener("click", () => setOpen(false));
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && open) setOpen(false);
  });
  if (nav) {
    nav.querySelectorAll("a").forEach((link) => {
      link.addEventListener("click", () => setOpen(false));
    });
  }
  const onBreakpoint = () => {
    if (desktopQuery.matches) setOpen(false);
  };
  if (desktopQuery.addEventListener) {
    desktopQuery.addEventListener("change", onBreakpoint);
  }
}

/* --------------------------------------------------------------------------
   Ripple micro-interaction
   -------------------------------------------------------------------------- */

function initRipples() {
  document.addEventListener("pointerdown", (event) => {
    if (!canAnimate() || event.button !== 0) return;
    const host = event.target.closest(".btn, .icon-btn, .nav a, .live-btn");
    if (!host || host.disabled || host.classList.contains("disabled")) return;
    const rect = host.getBoundingClientRect();
    if (!rect.width || !rect.height) return;
    const size = Math.max(rect.width, rect.height) * 2.2;
    const ripple = document.createElement("span");
    ripple.className = "ripple";
    ripple.style.width = size + "px";
    ripple.style.height = size + "px";
    ripple.style.left = event.clientX - rect.left + "px";
    ripple.style.top = event.clientY - rect.top + "px";
    host.appendChild(ripple);
    anime.animate(ripple, {
      scale: [0, 1],
      opacity: [0.22, 0],
      duration: 640,
      ease: "outCubic",
      onComplete: () => ripple.remove(),
    });
  });
}

/* --------------------------------------------------------------------------
   Page enter choreography
   -------------------------------------------------------------------------- */

function animatePageIn() {
  const root = document.documentElement;
  const panels = document.querySelectorAll(".panel, .auth-card");
  const bar = document.querySelector(".topbar, .auth-topbar");

  if (!canAnimate()) {
    root.classList.remove("motion-init");
    return;
  }

  if (bar) {
    anime.animate(bar, {
      opacity: [0, 1],
      translateY: [-8, 0],
      duration: 460,
      ease: "outQuad",
    });
  }

  if (!panels.length) {
    root.classList.remove("motion-init");
    return;
  }

  anime.animate(panels, {
    opacity: [0, 1],
    translateY: [18, 0],
    scale: [0.985, 1],
    duration: 620,
    delay: anime.stagger(80, { start: 30 }),
    ease: "outQuart",
    onComplete: () => root.classList.remove("motion-init"),
  });
  setTimeout(() => root.classList.remove("motion-init"), 1800);
}

/* --------------------------------------------------------------------------
   Boot
   -------------------------------------------------------------------------- */

document.addEventListener("DOMContentLoaded", () => {
  const flash = new URLSearchParams(location.search).get("flash");
  if (flash === "success") {
    toast("saved successfully");
  } else if (flash === "error") {
    toast("operation failed");
  }

  for (const el of document.querySelectorAll("[data-ts]")) {
    const local = formatHumanTime(el.dataset.ts);
    if (local) el.textContent = local;
  }

  initDrawer();
  initRipples();
  syncThemeIcons();

  for (const btn of document.querySelectorAll("[data-theme-toggle]")) {
    btn.addEventListener("click", toggleTheme);
  }

  if (window.matchMedia) {
    const scheme = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      syncThemeIcons();
      animateThemeIcon();
    };
    if (scheme.addEventListener) scheme.addEventListener("change", onChange);
  }

  animatePageIn();
});

window.api = api;
window.toast = toast;
window.confirmDialog = confirmDialog;
window.toggleTheme = toggleTheme;
window.applyTheme = applyTheme;
window.formatHumanTime = formatHumanTime;
window.countUp = countUp;
window.reveal = reveal;
window.conceal = conceal;
window.pop = pop;
window.copyText = copyText;

/* --------------------------------------------------------------------------
   Country flag helper
   -------------------------------------------------------------------------- */

function countryFlag(code) {
  if (!code || code.length !== 2) return "";
  const c = code.toUpperCase();
  return (
    String.fromCodePoint(0x1f1e6 + c.charCodeAt(0) - 65) +
    String.fromCodePoint(0x1f1e6 + c.charCodeAt(1) - 65)
  );
}

/* --------------------------------------------------------------------------
   Live request list
   -------------------------------------------------------------------------- */

(function () {
  const list = document.getElementById("req-list");
  const toggle = document.getElementById("live-toggle");
  const panel = document.getElementById("req-panel");
  if (!list || !toggle || !panel) return;

  const totalEl = document.getElementById("req-total");
  const emptyEl = document.getElementById("req-empty");
  const intervalMs = 2000;
  let timer = null;
  let inFlight = false;
  let lastKey = "";

  function renderRow(it) {
    const row = document.createElement("div");
    row.className = "req-row" + (it.Saved ? " req-saved" : "");
    row.dataset.id = String(it.ID);

    const checkLabel = document.createElement("label");
    checkLabel.className = "row-check";
    checkLabel.setAttribute("aria-label", "select request " + it.ID);
    const check = document.createElement("input");
    check.type = "checkbox";
    check.className = "row-select";
    check.value = String(it.ID);
    checkLabel.appendChild(check);
    row.appendChild(checkLabel);

    const a = document.createElement("a");
    a.className = "req-main";
    a.href = "/admin/requests/" + it.ID;
    const m = document.createElement("span");
    m.className = "req-method";
    m.textContent = it.Method;
    const t = document.createElement("span");
    t.className = "req-target";
    t.textContent = it.Host + it.Path;
    if (it.Query) {
      t.appendChild(document.createTextNode("?"));
      const q = document.createElement("span");
      q.className = "req-query";
      q.textContent = it.Query;
      t.appendChild(q);
    }
    a.appendChild(m);
    a.appendChild(t);
    row.appendChild(a);

    const meta = document.createElement("span");
    meta.className = "req-meta";
    const tag = document.createElement("span");
    tag.className = "tag" + (it.Saved ? " tag-saved" : "");
    tag.textContent = it.Saved ? "saved" : "raw";
    meta.appendChild(tag);
    if (it.SourceIP) {
      const ip = document.createElement("a");
      ip.className = "ip-link";
      ip.href = "https://ipinfo.io/" + it.SourceIP;
      ip.target = "_blank";
      ip.rel = "noopener noreferrer";
      ip.textContent = it.SourceIP;
      meta.appendChild(ip);
    }
    if (it.IPCountry) {
      const fl = document.createElement("span");
      fl.className = "flag";
      fl.title = it.IPCountry;
      fl.textContent = countryFlag(it.IPCountry);
      meta.appendChild(fl);
    }
    const time = document.createElement("time");
    time.dataset.ts = it.CreatedAt;
    time.textContent = it.CreatedAt;
    meta.appendChild(time);
    row.appendChild(meta);
    return row;
  }

  function renderPagination(data) {
    const nav = document.createElement("nav");
    nav.className = "pagination";
    nav.setAttribute("aria-label", "pagination");
    const perPage = data.per_page || 20;
    const pages = Math.max(1, Math.ceil(data.total / perPage));
    const page = Math.min(Math.max(data.page || 1, 1), pages);
    const href = (p) => {
      const params = new URLSearchParams(location.search);
      params.delete("flash");
      params.set("page", String(p));
      return location.pathname + "?" + params.toString();
    };
    const add = (label, p, active, disabled) => {
      const el = document.createElement(disabled ? "span" : "a");
      el.className = "btn" + (active ? " active" : "") + (disabled ? " disabled" : "");
      el.textContent = label;
      if (!disabled) el.href = href(p);
      nav.appendChild(el);
    };
    add("first", 1, false, page === 1);
    add("prev", page - 1, false, page === 1);
    let start = page - 2;
    if (start < 1) start = 1;
    let end = start + 4;
    if (end > pages) {
      end = pages;
      start = end - 4;
      if (start < 1) start = 1;
    }
    for (let n = start; n <= end; n++) add(String(n), n, n === page, false);
    add("next", page + 1, false, page === pages);
    add("last", pages, false, page === pages);
    const label = document.createElement("span");
    label.className = "muted";
    label.textContent = "page " + page + " / " + pages;
    nav.appendChild(label);
    return nav;
  }

  function render(data) {
    const key = data.total + ":" + (data.items.length ? data.items[0].ID : "");
    if (key === lastKey) return;
    lastKey = key;
    if (totalEl) totalEl.textContent = String(data.total);

    list.textContent = "";
    const fragment = document.createDocumentFragment();
    for (const it of data.items) fragment.appendChild(renderRow(it));
    list.appendChild(fragment);
    if (emptyEl) emptyEl.hidden = data.items.length > 0;

    const navs = Array.from(document.querySelectorAll("nav.pagination"));
    if (data.items.length > 0) {
      const nav = renderPagination(data);
      if (navs.length) {
        navs.forEach((n) => n.replaceWith(nav.cloneNode(true)));
      } else {
        panel.insertBefore(nav.cloneNode(true), list);
        panel.insertBefore(nav.cloneNode(true), list.nextSibling);
      }
    } else if (navs.length) {
      navs.forEach((n) => n.remove());
    }

    list.querySelectorAll("[data-ts]").forEach((el) => {
      const local = formatHumanTime(el.dataset.ts);
      if (local) el.textContent = local;
    });

    if (canAnimate()) {
      anime.animate(list.querySelectorAll(".req-row"), {
        opacity: [0, 1],
        translateY: [10, 0],
        duration: 420,
        delay: anime.stagger(35),
        ease: "outQuart",
      });
    }

    document.dispatchEvent(new Event("oob:list-updated"));
  }

  function poll() {
    if (inFlight || document.hidden) return;
    inFlight = true;
    fetch("/admin/api/requests" + location.search, {
      headers: { Accept: "application/json" },
    })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error("bad response"))))
      .then(render)
      .catch(() => {})
      .finally(() => (inFlight = false));
  }

  function start() {
    panel.classList.add("live-active");
    toggle.classList.add("on");
    toggle.setAttribute("aria-pressed", "true");
    if (canAnimate()) pop(toggle);
    if (!timer) timer = setInterval(poll, intervalMs);
  }

  function stop() {
    panel.classList.remove("live-active");
    toggle.classList.remove("on");
    toggle.setAttribute("aria-pressed", "false");
    clearInterval(timer);
    timer = null;
  }

  toggle.addEventListener("click", () => {
    const on = !toggle.classList.contains("on");
    localStorage.setItem("oob_live", on ? "1" : "0");
    on ? start() : stop();
  });

  document.addEventListener("visibilitychange", () => {
    if (toggle.classList.contains("on") && !document.hidden && !timer) start();
  });

  if (localStorage.getItem("oob_live") === "1") {
    start();
  }
})();

/* --------------------------------------------------------------------------
   Bulk selection & delete
   -------------------------------------------------------------------------- */

(function () {
  const list = document.getElementById("req-list");
  const selectAll = document.getElementById("select-all");
  const bulkBtn = document.getElementById("bulk-delete");
  const countEl = document.getElementById("bulk-count");
  if (!list || !selectAll || !bulkBtn) return;

  function checkedBoxes() {
    return Array.from(list.querySelectorAll(".row-select:checked"));
  }

  function selected() {
    return checkedBoxes().map((cb) => cb.value);
  }

  function update() {
    const ids = selected();
    const total = list.querySelectorAll(".row-select").length;
    selectAll.checked = ids.length > 0 && ids.length === total;
    selectAll.indeterminate = ids.length > 0 && ids.length < total;
    bulkBtn.disabled = ids.length === 0;
    if (countEl) countEl.textContent = ids.length ? ids.length + " selected" : "";
  }

  list.addEventListener("change", (e) => {
    if (e.target.classList && e.target.classList.contains("row-select")) update();
  });

  selectAll.addEventListener("change", () => {
    list.querySelectorAll(".row-select").forEach((cb) => {
      cb.checked = selectAll.checked;
    });
    update();
  });

  bulkBtn.addEventListener("click", async () => {
    const ids = selected();
    if (!ids.length) return;
    if (!(await confirmDialog("delete " + ids.length + " selected request(s)?"))) return;
    try {
      await api("/admin/api/requests/bulk_delete", {
        method: "POST",
        body: JSON.stringify({ ids: ids.map(Number) }),
      });
      const rows = checkedBoxes()
        .map((cb) => cb.closest(".req-row"))
        .filter(Boolean);
      if (canAnimate() && rows.length) {
        anime.animate(rows, {
          opacity: [1, 0],
          translateX: [0, 28],
          scale: [1, 0.96],
          duration: 260,
          delay: anime.stagger(18),
          ease: "inQuad",
          onComplete: () => location.reload(),
        });
      } else {
        location.reload();
      }
    } catch (err) {
      toast(err.message);
    }
  });

  document.addEventListener("oob:list-updated", update);
})();
