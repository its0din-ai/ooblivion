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
  if (window.oobParticles) window.oobParticles.refresh();
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
window.formatHumanTime = formatHumanTime;

(function () {
  const canvas = document.getElementById("particles");
  if (!canvas || !canvas.getContext) return;
  if (window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

  const ctx = canvas.getContext("2d");
  const DPR = Math.min(window.devicePixelRatio || 1, 2);
  let items = [];
  let raf = 0;

  function palette() {
    const s = getComputedStyle(document.documentElement);
    const accent = (s.getPropertyValue("--accent") || "#58a6ff").trim();
    const muted = (s.getPropertyValue("--fg-muted") || "#7d8794").trim();
    return [accent, accent, muted];
  }

  function spawn() {
    const p = palette();
    return {
      x: Math.random() * canvas.width,
      y: Math.random() * canvas.height,
      r: 0.6 + Math.random() * 1.8,
      vx: (Math.random() - 0.5) * 0.35,
      vy: (Math.random() - 0.5) * 0.35,
      a: 0.15 + Math.random() * 0.5,
      tw: Math.random() * Math.PI * 2,
      tws: 0.008 + Math.random() * 0.02,
      color: p[Math.floor(Math.random() * p.length)],
    };
  }

  function resize() {
    canvas.width = window.innerWidth * DPR;
    canvas.height = window.innerHeight * DPR;
    canvas.style.width = window.innerWidth + "px";
    canvas.style.height = window.innerHeight + "px";
    ctx.setTransform(DPR, 0, 0, DPR, 0, 0);
    const count = window.innerWidth < 640 ? 45 : 120;
    items = [];
    for (let i = 0; i < count; i++) items.push(spawn());
  }

  function drawLinks() {
    const link = 130;
    const maxD = link * link;
    for (let i = 0; i < items.length; i++) {
      const a = items[i];
      for (let j = i + 1; j < items.length; j++) {
        const b = items[j];
        const dx = a.x - b.x;
        const dy = a.y - b.y;
        const d = dx * dx + dy * dy;
        if (d > maxD) continue;
        const alpha = (1 - Math.sqrt(d) / link) * 0.3;
        ctx.globalAlpha = alpha;
        ctx.strokeStyle = a.color;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.stroke();
      }
    }
    ctx.globalAlpha = 1;
  }

  function tick() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    for (const p of items) {
      p.x += p.vx;
      p.y += p.vy;
      p.tw += p.tws;
      if (p.x < -10) p.x = canvas.width + 10;
      if (p.x > canvas.width + 10) p.x = -10;
      if (p.y < -10) p.y = canvas.height + 10;
      if (p.y > canvas.height + 10) p.y = -10;
      ctx.globalAlpha = Math.max(0.05, p.a * (0.5 + 0.5 * Math.sin(p.tw)));
      ctx.fillStyle = p.color;
      ctx.beginPath();
      ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
      ctx.fill();
    }
    drawLinks();
    raf = requestAnimationFrame(tick);
  }

  window.addEventListener("resize", resize);
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      cancelAnimationFrame(raf);
    } else {
      raf = requestAnimationFrame(tick);
    }
  });
  resize();
  tick();

  window.oobParticles = {
    refresh: () => {
      for (const p of items) p.color = palette()[Math.floor(Math.random() * 3)];
    },
  };
})();
