function csrf() {
  const m = document.cookie.match(/(?:^|;\s*)oob_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : "";
}

function currentTheme() {
  const stored = localStorage.getItem("oob_theme");
  if (stored === "light" || stored === "dark") return stored;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
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
}

function syncThemeIcons() {
  const dark = currentTheme() === "dark";
  document.querySelectorAll(".theme-icon-sun").forEach((el) => {
    el.style.display = dark ? "none" : "inline";
  });
  document.querySelectorAll(".theme-icon-moon").forEach((el) => {
    el.style.display = dark ? "inline" : "none";
  });
}

async function api(path, opts = {}) {
  const headers = Object.assign({ "X-CSRF-Token": csrf() }, opts.headers || {});
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

let toastTimer;

function toast(msg) {
  const el = document.getElementById("toast");
  if (!el) return;
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (el.hidden = true), 3000);
}

function confirmDialog(msg, okLabel = "Confirm") {
  return new Promise((resolve) => {
    const overlay = document.getElementById("modal");
    const message = document.getElementById("modal-message");
    const ok = document.getElementById("modal-ok");
    const cancel = document.getElementById("modal-cancel");
    if (!overlay) return resolve(false);
    message.textContent = msg;
    ok.textContent = okLabel;

    const onKey = (e) => {
      if (e.key === "Escape") done(false);
      if (e.key === "Enter") done(true);
    };
    const done = (val) => {
      overlay.hidden = true;
      document.removeEventListener("keydown", onKey);
      ok.onclick = cancel.onclick = overlay.onclick = null;
      resolve(val);
    };
    ok.onclick = () => done(true);
    cancel.onclick = () => done(false);
    overlay.onclick = (e) => {
      if (e.target === overlay) done(false);
    };
    document.addEventListener("keydown", onKey);
    overlay.hidden = false;
    ok.focus();
  });
}

document.addEventListener("DOMContentLoaded", () => {
  const toggle = document.getElementById("nav-toggle");
  const nav = document.getElementById("nav");
  if (toggle && nav) {
    toggle.addEventListener("click", () => {
      const open = nav.classList.toggle("open");
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    });
    for (const link of nav.querySelectorAll("a")) {
      link.addEventListener("click", () => nav.classList.remove("open"));
    }
  }
  for (const btn of document.querySelectorAll("[data-theme-toggle]")) {
    btn.addEventListener("click", toggleTheme);
  }
  syncThemeIcons();
  if (window.matchMedia) {
    window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", syncThemeIcons);
  }
});

window.api = api;
window.toast = toast;
window.confirmDialog = confirmDialog;
window.toggleTheme = toggleTheme;
window.applyTheme = applyTheme;
