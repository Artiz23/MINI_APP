const tg = window.Telegram && window.Telegram.WebApp;
const $ = (id) => document.getElementById(id);

function tgPlatform() {
  return String((tg && tg.platform) || "").toLowerCase();
}
function isDesktopTg() {
  const p = tgPlatform();
  if (p === "android" || p === "ios" || p === "android_x") return false;
  if (p === "tdesktop" || p === "macos" || p === "weba" || p === "web" || p === "webk" || p === "unigram") return true;
  try {
    return !!(window.matchMedia && window.matchMedia("(hover: hover) and (pointer: fine)").matches);
  } catch (_) {
    return false;
  }
}

function viewportPx() {
  const vv = window.visualViewport && window.visualViewport.height;
  const tgH = tg ? Number(tg.viewportHeight || 0) : 0;
  const stable = tg ? Number(tg.viewportStableHeight || 0) : 0;
  const win = Number(window.innerHeight || 0);
  const vis = Number(vv || 0);
  const cands = [tgH, stable, win, vis].filter((n) => n > 80);
  if (!cands.length) return 0;
  if (isDesktopTg()) return Math.round(Math.max.apply(null, cands));
  return Math.round(Math.min.apply(null, cands));
}
function fitTgViewport() {
  if (tg) {
    try { tg.ready(); } catch (_) {}
  }
  const h = viewportPx();
  if (h) {
    document.documentElement.style.setProperty("--app-h", h + "px");
    document.documentElement.style.setProperty("--tg-viewport-stable-height", h + "px");
    document.documentElement.style.setProperty("--tg-viewport-height", h + "px");
  }
  if (tg) {
    const sa = tg.safeAreaInset || {};
    const csa = tg.contentSafeAreaInset || {};
    const top = Number(csa.top || sa.top || 0);
    const bottom = Number(sa.bottom || 0);
    const left = Number(sa.left || 0);
    const right = Number(sa.right || 0);
    const root = document.documentElement;
    if (top) root.style.setProperty("--sat", top + "px");
    if (bottom) root.style.setProperty("--sab", bottom + "px");
    if (left) root.style.setProperty("--sal", left + "px");
    if (right) root.style.setProperty("--sar", right + "px");
    document.documentElement.classList.toggle("tg-fs", !!tg.isFullscreen);
  }
  paintWinBtns();
  measureDock();
}
function lockMiniAppSize() {
  if (!tg) return;
  try { tg.ready(); } catch (_) {}
  try { if (tg.expand) tg.expand(); } catch (_) {}
  try {
    if (isDesktopTg()) {
      if (tg.enableVerticalSwipes) tg.enableVerticalSwipes();
    } else if (tg.disableVerticalSwipes) {
      tg.disableVerticalSwipes();
    }
  } catch (_) {}
  document.documentElement.classList.toggle("desktop", isDesktopTg());
  fitTgViewport();
}
function bindNoZoom() {
  if (window._zoomBound) return;
  window._zoomBound = true;
  if (isDesktopTg()) return;
  const stop = (e) => { e.preventDefault(); };
  document.addEventListener("gesturestart", stop, { passive: false });
  document.addEventListener("gesturechange", stop, { passive: false });
  document.addEventListener("gestureend", stop, { passive: false });
  document.addEventListener("touchmove", (e) => {
    if (e.touches && e.touches.length > 1) e.preventDefault();
  }, { passive: false });
}
function nearestHScroll(el) {
  let n = el;
  while (n && n !== document.body && n !== document.documentElement) {
    if (n.scrollWidth > n.clientWidth + 8) {
      const ox = window.getComputedStyle(n).overflowX;
      if (ox === "auto" || ox === "scroll") return n;
    }
    n = n.parentElement;
  }
  return null;
}
function bindDesktopDragScroll() {
  if (window._dragScrollBound) return;
  window._dragScrollBound = true;
  let ready = false;
  let dragging = false;
  let moved = false;
  let pid = 0;
  let y0 = 0;
  let x0 = 0;
  let top0 = 0;
  let left0 = 0;
  let vBox = null;
  let hBox = null;
  const skip = (t) => !!(t && t.closest && t.closest("input, textarea, select, option, [contenteditable='true']"));
  const end = (e) => {
    if (!ready || (e && e.pointerId != null && e.pointerId !== pid)) return;
    ready = false;
    dragging = false;
    [vBox, hBox].forEach((box) => {
      if (!box) return;
      try { box.releasePointerCapture(pid); } catch (_) {}
      box.classList.remove("drag-scroll");
    });
    vBox = null;
    hBox = null;
  };
  document.addEventListener("pointerdown", (e) => {
    if (!isDesktopTg() || e.button !== 0) return;
    if (e.pointerType && e.pointerType !== "mouse" && e.pointerType !== "pen") return;
    if (skip(e.target)) return;
    const box = $("app");
    if (!box || !box.contains(e.target)) return;
    ready = true;
    dragging = false;
    moved = false;
    pid = e.pointerId;
    y0 = e.clientY;
    x0 = e.clientX;
    vBox = box;
    top0 = box.scrollTop;
    hBox = nearestHScroll(e.target);
    left0 = hBox ? hBox.scrollLeft : 0;
    e.stopPropagation();
  }, true);
  document.addEventListener("pointermove", (e) => {
    if (!ready || e.pointerId !== pid) return;
    const dy = e.clientY - y0;
    const dx = e.clientX - x0;
    if (!dragging) {
      if (Math.abs(dy) < 5 && Math.abs(dx) < 5) return;
      dragging = true;
      moved = true;
      const cap = hBox || vBox;
      if (cap) {
        try { cap.setPointerCapture(e.pointerId); } catch (_) {}
        cap.classList.add("drag-scroll");
      }
      if (hBox && hBox !== cap) hBox.classList.add("drag-scroll");
      if (vBox && vBox !== cap) vBox.classList.add("drag-scroll");
    }
    if (vBox) vBox.scrollTop = top0 - dy;
    if (hBox) hBox.scrollLeft = left0 - dx;
    e.preventDefault();
    e.stopPropagation();
  }, { capture: true, passive: false });
  document.addEventListener("pointerup", end, true);
  document.addEventListener("pointercancel", end, true);
  document.addEventListener("click", (e) => {
    if (!moved) return;
    e.preventDefault();
    e.stopPropagation();
    moved = false;
  }, true);
}

function measureDock() {
  const dock = $("dock");
  if (!dock) return;
  const h = dock.hidden ? 0 : Math.ceil(dock.getBoundingClientRect().height);
  document.documentElement.style.setProperty("--dock-h", h + "px");
}

function canFullscreen() {
  return !!(tg && typeof tg.requestFullscreen === "function");
}

function paintWinBtns() {
  const fs = $("fsBtn");
  const ex = $("expandBtn");
  if (ex) ex.hidden = !tg;
  if (!fs) return;
  if (!canFullscreen()) {
    fs.hidden = true;
    return;
  }
  fs.hidden = false;
  const lbl = fs.querySelector(".glbl");
  if (lbl) lbl.textContent = tg.isFullscreen ? "Окно" : "На весь экран";
  fs.title = tg.isFullscreen ? "Окно" : "На весь экран";
}

function bindWinBtns() {
  const ex = $("expandBtn");
  const fs = $("fsBtn");
  if (ex) ex.onclick = () => {
    lockMiniAppSize();
    toast("Развёрнуто");
  };
  if (fs) fs.onclick = () => {
    try {
      if (tg && tg.isFullscreen && typeof tg.exitFullscreen === "function") tg.exitFullscreen();
      else if (tg && typeof tg.requestFullscreen === "function") tg.requestFullscreen();
    } catch (e) { toast(e.message || "Telegram не дал сменить размер"); }
    setTimeout(fitTgViewport, 200);
  };
  paintWinBtns();
}

document.documentElement.classList.toggle("desktop", isDesktopTg());
if (tg) {
  lockMiniAppSize();
  try {
    tg.setHeaderColor("#f4f6fb");
    tg.setBackgroundColor("#f4f6fb");
  } catch (_) {}
  if (tg.onEvent) {
    tg.onEvent("viewportChanged", () => { lockMiniAppSize(); });
    tg.onEvent("fullscreenChanged", fitTgViewport);
    tg.onEvent("safeAreaChanged", fitTgViewport);
  }
  setTimeout(lockMiniAppSize, 80);
  setTimeout(fitTgViewport, 320);
}
bindNoZoom();
bindDesktopDragScroll();
window.addEventListener("resize", () => { fitTgViewport(); });
if (window.visualViewport) {
  window.visualViewport.addEventListener("resize", fitTgViewport);
}
document.addEventListener("visibilitychange", () => {
  if (!document.hidden && currentSection === "requests" && reqPane === "docs" && reqOpen) renderRequests();
});

const app = $("app");
let me = null;

const SECTIONS = [
  { key: "requests", title: "Заявки", desc: "Тема и статус", ico: "📝" },
  { key: "tasks", title: "Канбан", desc: "Воронка задач: Новые → В работе → Документы → Готово", ico: "🗂" },
  { key: "approvals", title: "Согласование", desc: "Очередь и споры", ico: "✅" },
  { key: "saldo", title: "Сальдо", desc: "Как в боте", ico: "📒" },
  { key: "rates", title: "Курсы", desc: "Живые котировки", ico: "💱" },
  { key: "holidays", title: "Праздники", desc: "Выходные банков", ico: "🏖" },
  { key: "balance", title: "Баланс", desc: "Счета компаний", ico: "🏦" },
  { key: "compliance", title: "Комплаенс", desc: "Проверки", ico: "🛡" },
  { key: "directory", title: "Справочник", desc: "Сотрудники и общности", ico: "📇" },
  { key: "documents", title: "Документы", desc: "Компании, контрагенты и прочее", ico: "📁" },
  { key: "payments", title: "Отправка", desc: "Оплата: что отправить и отправлено", ico: "💸" },
  { key: "appeal_lawyer", title: "Обратиться к юристу", desc: "Чат ВЭД ЮРИСТ", ico: "⚖️" },
  { key: "appeal_docs", title: "Обратиться к документалисту", desc: "Чат документалистов", ico: "📄" },
];

const POS_ACCESS = [
  { key: "tasks", title: "Канбан" },
  { key: "requests", title: "Заявки" },
  { key: "saldo", title: "Сальдо" },
  { key: "approvals", title: "Согласование" },
  { key: "payments", title: "Отправка" },
  { key: "directory", title: "Справочник" },
  { key: "documents", title: "Документы" },
  { key: "rates", title: "Курсы" },
  { key: "holidays", title: "Праздники" },
  { key: "balance", title: "Баланс" },
  { key: "compliance", title: "Комплаенс" },
  { key: "appeals", title: "Обращения" },
  { key: "appeal_lawyer", title: "Обращение к юристу" },
  { key: "appeal_docs", title: "Обращение к документалисту" },
];

const DEFAULT_ACCESS = { requests: true, approvals: true, saldo: true, directory: true, tasks: true };

const TILE_TONE = {
  saldo: "mint", approvals: "amber", payments: "gold",   directory: "lilac",
  directory: "lilac", documents: "sand",
  requests: "sky", tasks: "cyan", rates: "cyan", holidays: "sand", balance: "navy",
  compliance: "rose", appeals: "rose", appeal_lawyer: "navy", appeal_docs: "rose",
  settings: "steel", activity: "steel", chats: "navy", users: "steel", summary: "steel"
};

function tileBtn(key, title, desc, ico, extraClass, badge) {
  const tone = TILE_TONE[key] || "steel";
  const wide = extraClass && extraClass.indexOf("wide") >= 0;
  return `<button class="tile t-${tone} ${extraClass || ""}" data-go="${key}">${badge || ""}<div class="ico">${ico}</div>${wide ? `<div>` : ""}<b>${esc(title)}</b><span>${esc(desc)}</span>${wide ? `</div>` : ""}</button>`;
}

const DIR_KINDS = [
  { key: "employees", title: "Сотрудники", desc: "Кто пользуется Mini App" },
  { key: "managers", title: "Менеджеры", desc: "Общность: две буквы для номера заявки" },
  { key: "clients", title: "Клиенты", desc: "Общность: фирмы, которых ведём" },
  { key: "counterparties", title: "Контрагенты", desc: "Общность: с кем сделка и сальдо" },
  { key: "positions", title: "Должности", desc: "Доступы по роли, несколько на человека" },
];

async function api(path, opt) {
  const headers = { "Content-Type": "application/json" };
  if (tg && tg.initData) headers["X-Telegram-Init-Data"] = tg.initData;
  const res = await fetch(path, Object.assign({ headers }, opt || {}));
  const text = await res.text();
  if (!res.ok) throw new Error(text || res.status);
  try { return JSON.parse(text); } catch { return text; }
}

function esc(s) {
  return String(s == null ? "" : s)
    .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function asList(v) { return Array.isArray(v) ? v : []; }
function dirWarn() { return Number(me && me.dir_warn) || 0; }
function warnDot() { return `<span class="warn-dot" title="Должность удалена, но ещё висит на сотруднике">!</span>`; }
function isAdmin() {
  if (!me) return false;
  if (me.see_all === true || me.staff === true || me.owner === true || me.admin === true || me.can_manage_users === true) return true;
  const r = String(me.role || "").toLowerCase();
  return r === "owner" || r === "admin";
}
function canManageChats() {
  if (!me) return false;
  if (me.owner === true || me.admin === true) return true;
  if (me.can_manage_users === true) return true;
  const r = String(me.role || "").toLowerCase();
  return r === "owner" || r === "admin";
}
function canManage() { return !!(me && (me.can_manage_users || isAdmin())); }
function isLimitedOperator() {
  if (isAdmin()) return false;
  if (!me) return false;
  if (me.limited === true) return true;
  return !!me.employee;
}
function seeAll() { return isAdmin() || !isLimitedOperator(); }
function has(sec) {
  if (isAdmin() || seeAll()) return true;
  if (!me) return false;
  if (Array.isArray(me.sections) && me.sections.indexOf(sec) >= 0) return true;
  const acc = me.access;
  if (Array.isArray(acc)) return acc.indexOf(sec) >= 0;
  if (acc && typeof acc === "object") return !!acc[sec];
  return false;
}
function canEditDir() { return !!(isAdmin() || canManage() || seeAll()); }
function canAddRel() { return !!(canEditDir() || has("directory")); }
function canDelRel(u) { return !!(canEditDir() || isMine(u && u.created_by)); }
function hasAppeals() { return has("appeals") || has("appeal_lawyer") || has("appeal_docs") || has("requests"); }
function showLawyerCard() { return seeAll() || has("appeal_lawyer") || has("appeals"); }
function showDocsCard() { return seeAll() || has("appeal_docs") || has("appeals"); }
function canPay() { return !!(seeAll() || (me && (me.can_payments || (me.access && me.access.payments)))); }
function unread() { return (me && me.unread) || { approvals: 0, disputes: 0, total: 0 }; }
function unreadOf(key) {
  if (!key || key === currentSection) return 0;
  const u = unread();
  if (key === "approvals") return Number(u.total) || ((Number(u.approvals) || 0) + (Number(u.disputes) || 0));
  return Number(u[key]) || 0;
}
function ntf(n) {
  n = Number(n) || 0;
  if (n <= 0) return "";
  return `<span class="ntf">${n > 99 ? "99+" : n}</span>`;
}
async function markSeen(key) {
  if (!key || key === "home" || key === "more") return;
  if (key === "disputes") key = "approvals";
  if (me && me.unread) {
    me.unread[key] = 0;
    if (key === "approvals") {
      me.unread.approvals = 0;
      me.unread.disputes = 0;
      me.unread.total = 0;
    }
    paintDock();
  }
  try {
    const u = await api("/api/inbox", { method: "POST", body: JSON.stringify({ seen: key }) });
    if (me && u && typeof u === "object") {
      me.unread = u;
      paintDock();
    }
  } catch (_) {}
}
function applyBadges() {
  document.querySelectorAll(".tile[data-go], .dock-btn[data-dock]").forEach((el) => {
    const key = el.dataset.go || el.dataset.dock;
    if (!key || key === "home") return;
    const html = ntf(unreadOf(key));
    const old = el.querySelector(":scope > .ntf");
    if (old && !html) old.remove();
    else if (old && html) old.outerHTML = html;
    else if (!old && html) el.insertAdjacentHTML("afterbegin", html);
  });
}
function startUnreadPoll() {
  if (window._unreadPoll) return;
  window._unreadPoll = setInterval(async () => {
    if (document.hidden || !me) return;
    try {
      const u = await api("/api/inbox");
      if (!u || typeof u !== "object") return;
      me.unread = u;
      paintDock();
      if (currentSection === "home") applyBadges();
    } catch (_) {}
  }, 4000);
}
async function refreshMe() {
  me = await api("/api/me");
}
let currentSection = "home";
let showAprForm = false;
let toastTimer = 0;
let navSeq = 0;

function bumpNav() { return ++navSeq; }
function alive(token) { return token === navSeq; }

function buzz(kind) {
  try { tg && tg.HapticFeedback && tg.HapticFeedback.impactOccurred(kind || "light"); } catch (_) {}
}
function toast(msg) {
  const el = $("toast");
  if (!el) return;
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { el.hidden = true; }, 2400);
}
function ask(msg) {
  return new Promise((resolve) => {
    if (tg && tg.showConfirm) tg.showConfirm(msg, resolve);
    else resolve(window.confirm(msg));
  });
}
function recentCPs() {
  try { return JSON.parse(localStorage.getItem("orh3_recent_cp") || "[]"); } catch (_) { return []; }
}
function pushRecent(cp) {
  const a = recentCPs().filter((x) => x !== cp);
  a.unshift(cp);
  localStorage.setItem("orh3_recent_cp", JSON.stringify(a.slice(0, 8)));
}
function sortCPs(cps) {
  const rec = recentCPs();
  return cps.slice().sort((a, b) => {
    const ia = rec.indexOf(a); const ib = rec.indexOf(b);
    if (ia >= 0 && ib >= 0) return ia - ib;
    if (ia >= 0) return -1;
    if (ib >= 0) return 1;
    return String(a).localeCompare(String(b), "ru");
  });
}
function bindSearch(id) {
  const inp = $(id);
  if (!inp) return;
  inp.oninput = () => {
    const q = inp.value.trim().toLowerCase();
    app.querySelectorAll("[data-search]").forEach((el) => {
      el.hidden = !!(q && !(el.dataset.search || "").includes(q));
    });
  };
}
function copyBind() {
  app.querySelectorAll("[data-copy]").forEach((b) => {
    b.onclick = async (e) => {
      e.stopPropagation();
      try { await navigator.clipboard.writeText(b.dataset.copy); toast("Скопировано"); buzz("soft"); } catch (_) { toast(b.dataset.copy); }
    };
  });
}
function paintDock() {
  const dock = $("dock");
  if (!dock || !me) return;
  const items = [
    { key: "home", title: "Главная", ico: "⌂" },
    { key: "requests", title: "Заявки", ico: "📝", badge: unreadOf("requests") },
    { key: "tasks", title: "Канбан", ico: "🗂", badge: unreadOf("tasks") },
    { key: "saldo", title: "Сальдо", ico: "📒", badge: unreadOf("saldo") },
    { key: "approvals", title: "Согл.", ico: "✅", badge: unreadOf("approvals") }
  ];
  const moreOn = currentSection === "activity";
  dock.hidden = false;
  dock.innerHTML = items.filter((it) => it.key === "home" || isAdmin() || !isLimitedOperator() || has(it.key)).map((it) => {
    const on = it.key === currentSection || (it.key === "activity" && moreOn) || (it.key === "home" && currentSection === "home");
    return `<button type="button" class="dock-btn ${on ? "on" : ""}" data-dock="${it.key}">
      ${ntf(it.badge)}<span class="dock-ico">${it.ico}</span><span>${it.title}</span>
    </button>`;
  }).join("");
  dock.querySelectorAll("[data-dock]").forEach((b) => {
    b.onclick = () => {
      buzz();
      const k = b.dataset.dock;
      if (k === "home") return showHome();
      if (k === "activity") return renderActivity();
      return openSection(k);
    };
  });
  measureDock();
}
function onBack() {
  buzz();
  if (aprDspId) { aprDspId = 0; renderApprovals(); return; }
  if (currentSection === "tasks" && taskForm) { taskForm = false; renderTasks(); return; }
  if (currentSection === "directory" && dirTab !== "menu") {
    dirTab = "menu"; renderDirectory(); return;
  }
  if (currentSection === "documents") {
    if (docsTab === "company") {
      if (docsFolder && docsFolder.indexOf(":") >= 0) { docsFolder = docsFolder.split(":")[0]; stopDocsPoll(); renderDocuments(); return; }
      docsTab = "companies"; docsOwner = 0; docsName = ""; stopDocsPoll(); renderDocuments(); return;
    }
    if (docsTab === "counterparty") { docsTab = "counterparties"; docsOwner = 0; docsName = ""; stopDocsPoll(); renderDocuments(); return; }
    if (docsTab === "companies" || docsTab === "counterparties" || docsTab === "misc") { docsTab = ""; stopDocsPoll(); renderDocuments(); return; }
  }
  if (currentSection === "requests" && reqPane) { reqPane = ""; stopDocsPoll(); renderRequests(); return; }
  if (currentSection === "requests" && reqOpen) { reqOpen = 0; stopDocsPoll(); renderRequests(); return; }
  if (currentSection !== "home") { showHome(); return; }
  try { tg && tg.close && tg.close(); } catch (_) {}
}

function roleLabel(u, linked) {
  if (u.role === "owner") return "владелец";
  if (u.role === "admin") return "админ";
  if (linked) return "сотрудник";
  return "доступ не выдан";
}
function initials(name) {
  const p = String(name || "?").trim().split(/\s+/);
  return ((p[0] || "?").slice(0, 1) + (p[1] ? p[1].slice(0, 1) : "")).toUpperCase();
}

async function showHome() {
  const token = bumpNav();
  currentSection = "home";
  $("homeBtn").hidden = true;
  try { tg && tg.BackButton && tg.BackButton.hide(); } catch (_) {}
  try { await refreshMe(); } catch (_) {}
  if (!alive(token)) return;
  let dash = {};
  try { dash = await api("/api/home"); } catch (_) {}
  if (!alive(token)) return;
  const hello = (me && me.name) ? me.name.split(" ")[0] : "коллега";
  const total = Number(dash.requests_total) || 0;
  const open = Number(dash.requests_open) || 0;
  const progress = Number(dash.requests_progress) || 0;
  const apr = Number(dash.approvals_pending) || 0;
  const pay = Number(dash.payments_pending) || 0;
  const stuck = apr + pay;
  const work = open + progress;
  const nfmt = (n) => Number(n || 0).toLocaleString("ru-RU");
  const workTiles = [];
  const sysTiles = [];
  const order = ["tasks", "saldo", "approvals", "payments", "directory", "documents", "requests", "rates", "holidays", "balance", "compliance", "appeal_lawyer", "appeal_docs"];
  const byKey = {};
  SECTIONS.forEach((s) => { byKey[s.key] = s; });
  order.forEach((key) => {
    const s = byKey[key];
    if (!s) return;
    if (!isAdmin() && isLimitedOperator()) {
      if (key === "appeal_lawyer" && !showLawyerCard()) return;
      if (key === "appeal_docs" && !showDocsCard()) return;
      if (key !== "appeal_lawyer" && key !== "appeal_docs" && !has(key)) return;
    }
    let badge = ntf(unreadOf(s.key));
    if (!badge && s.key === "directory" && (Number(dash.dir_warn) || dirWarn())) badge = warnDot();
    workTiles.push(tileBtn(s.key, s.title, s.desc, s.ico, "wide", badge));
  });
  if (canManageChats()) {
    sysTiles.push(tileBtn("chats", "Чаты", "ID групп для отправки", "💬", "wide"));
  }
  if (seeAll()) {
    sysTiles.push(tileBtn("users", "Доступы", "Выдать и забрать вход", "🔑", "wide", ntf(unreadOf("users"))));
    sysTiles.push(tileBtn("summary", "Сводка", "Контроль на сейчас", "📊", "wide"));
    sysTiles.push(tileBtn("activity", "Лента", "Что произошло недавно", "🕒", "wide", ntf(unreadOf("activity"))));
  }
  sysTiles.push(tileBtn("settings", "Уведомления", "Что писать в личку", "🔔", "wide"));
  const ava = $("whoAva");
  if (ava) ava.textContent = initials(me && me.name);
  app.innerHTML = `<div class="page home">
    <div class="hello">
      <div>
        <p class="kicker">Вот что происходит сегодня</p>
        <h1>Привет, ${esc(hello)}!</h1>
      </div>
    </div>
    <div class="hero">
      <div class="lbl">В работе и на контроле</div>
      <div class="num">${nfmt(work + stuck)}</div>
      <div class="subd">заявки, согласования и оплаты, которые ждут действия</div>
    </div>
    <div class="kpis">
      <button type="button" class="kpi sky" data-go="${has("requests") ? "requests" : ""}">
        <div class="lbl">Сделок</div>
        <div class="num">${nfmt(total)}</div>
      </button>
      <button type="button" class="kpi sand" data-go="${has("requests") ? "requests" : ""}">
        <div class="lbl">В работе</div>
        <div class="num">${nfmt(work)}</div>
      </button>
      <button type="button" class="kpi blush" data-go="${stuck && canPay() && pay >= apr ? "payments" : "approvals"}">
        <div class="lbl">Зависло</div>
        <div class="num">${nfmt(stuck)}</div>
      </button>
    </div>
    <div class="grid">${workTiles.join("") || (!isAdmin() && isLimitedOperator() ? `<p class="empty">Нет должности — карточки разделов скрыты. Админ назначит должность в Справочник → Сотрудники.</p>` : "")}</div>
    ${sysTiles.length ? `<div class="grid sys">${sysTiles.join("")}</div>` : ""}
  </div>`;
  app.scrollTop = 0;
  app.querySelectorAll("[data-go]").forEach((b) => b.onclick = () => {
    const k = b.dataset.go;
    if (!k) return;
    buzz();
    if (k === "more") return renderMore();
    openSection(k);
  });
  paintDock();
  startUnreadPoll();
}

function activityHTML(acts) {
  return asList(acts).map((a) => `
    <button type="button" class="act" data-go="${esc(a.go || "")}">
      <div class="ava">${esc(initials(a.title))}</div>
      <div>
        <b>${esc(a.title || "Система")}</b>
        <div class="meta">${esc(a.text || "")}${a.at ? " · " + esc(fmtWhen(a.at)) : ""}</div>
      </div>
    </button>`).join("") || `<p class="empty">Пока тихо — новые заявки появятся здесь</p>`;
}

async function renderActivity() {
  const token = bumpNav();
  currentSection = "activity";
  let dash = {};
  try { dash = await api("/api/home"); } catch (_) {}
  if (!alive(token)) return;
  page("Лента", activityHTML(dash.activity), token);
  app.querySelectorAll("[data-go]").forEach((b) => b.onclick = () => {
    const k = b.dataset.go;
    if (!k) return;
    buzz();
    openSection(k);
  });
}

async function renderMore() {
  const token = bumpNav();
  currentSection = "more";
  const items = [];
  const add = (key, title, desc, ico, show) => {
    if (!show) return;
    items.push(`<button type="button" class="more-item" data-go="${key}"><div class="ico">${ico}</div><div><b>${esc(title)}</b><span>${esc(desc)}</span></div></button>`);
  };
  add("requests", "Заявки", "Сделки отдела", "📝", seeAll() || has("requests"));
  add("tasks", "Канбан", "Воронка задач отдела", "🗂", seeAll() || has("tasks"));
  add("saldo", "Сальдо", "Постановки и выгрузка", "📒", seeAll() || has("saldo"));
  add("approvals", "Согласование", "Очередь и споры", "✅", seeAll() || has("approvals"));
  add("payments", "Отправка", "Оплаты к отправке", "💸", seeAll() || has("payments") || canPay());
  add("directory", "Сотрудники и справочники", "Пять списков", "📇", seeAll() || has("directory"));
  add("rates", "Курсы", "Живые котировки", "💱", seeAll() || has("rates"));
  add("holidays", "Праздники", "Выходные банков", "🏖", seeAll() || has("holidays"));
  add("balance", "Баланс", "Счета компаний", "🏦", seeAll() || has("balance"));
  add("compliance", "Комплаенс", "Проверки", "🛡", seeAll() || has("compliance"));
  add("appeal_lawyer", "Обратиться к юристу", "Чат ВЭД ЮРИСТ", "⚖️", showLawyerCard());
  add("appeal_docs", "Обратиться к документалисту", "Чат документалистов", "📄", showDocsCard());
  add("activity", "Лента", "Что произошло недавно", "🕒", seeAll());
  add("chats", "Чаты", "ID групп для отправки", "💬", canManageChats());
  add("users", "Доступы", "Выдать и забрать вход", "🔑", seeAll());
  add("summary", "Сводка", "Контроль на сейчас", "📊", seeAll());
  add("settings", "Уведомления", "Что писать в личку", "🔔", true);
  page("Ещё", `<div class="more-list">${items.join("")}</div>`, token);
  app.querySelectorAll("[data-go]").forEach((b) => b.onclick = () => { buzz(); openSection(b.dataset.go); });
}

function page(title, html, token) {
  if (token != null && !alive(token)) return;
  $("homeBtn").hidden = currentSection === "home";
  $("homeBtn").textContent = "Назад";
  app.innerHTML = `<div class="page"><h2>${esc(title)}</h2>${html}</div>`;
  app.scrollTop = 0;
  try {
    if (tg && tg.BackButton) {
      tg.BackButton.show();
      if (!window._backBound) {
        window._backBound = true;
        tg.BackButton.onClick(onBack);
      }
    }
  } catch (_) {}
  paintDock();
  copyBind();
}

async function openSection(name) {
  currentSection = name === "disputes" ? "approvals" : name;
  await markSeen(currentSection);
  try {
    if (name === "requests") return renderRequests();
    if (name === "tasks") return renderTasks();
    if (name === "payments") return renderPayments();
    if (name === "approvals") { if (aprTab !== "disputes") aprTab = "queue"; aprDspId = 0; return renderApprovals(); }
    if (name === "saldo") {
      if (!saldoFromRequest) {
        saldoDraft.requestId = 0;
        saldoDraft.counterpartyId = 0;
        saldoDraft.reqTitle = "";
      }
      saldoFromRequest = false;
      return renderSaldo();
    }
    if (name === "rates") return renderRates();
    if (name === "holidays") return renderHolidays();
    if (name === "balance") return renderBalance();
    if (name === "compliance") return renderCompliance();
    if (name === "appeals" || name === "appeal_lawyer") return renderAppealKind("lawyer");
    if (name === "appeal_docs") return renderAppealKind("docs");
    if (name === "directory") { dirTab = "menu"; return renderDirectory(); }
    if (name === "documents") { docsTab = ""; docsOwner = 0; docsFolder = "internal"; docsName = ""; return renderDocuments(); }
    if (name === "more") return renderMore();
    if (name === "activity") return renderActivity();
    if (name === "summary") return renderSummary();
    if (name === "users") return renderUsers();
    if (name === "chats") {
      if (!canManageChats()) {
        page("Чаты", `<p class="err">Только для владельца и админов</p>`);
        return;
      }
      return renderChats();
    }
    if (name === "settings") return renderNotify();
    if (name === "disputes") { aprTab = "disputes"; return renderApprovals(); }
  } catch (e) {
    page("Ошибка", `<p class="err">${esc(e.message)}</p>`);
  }
}

function fmtWhen(v) {
  if (!v) return "";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return String(v);
  return d.toLocaleString("ru-RU", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}
function fmtSize(n) {
  n = Number(n) || 0;
  if (n < 1024) return n + " Б";
  if (n < 1048576) return (n / 1024).toFixed(1).replace(".0", "") + " КБ";
  return (n / 1048576).toFixed(1).replace(".0", "") + " МБ";
}
function isImageFile(f) {
  const mime = String((f && f.mime) || "");
  if (/^image\//i.test(mime)) return true;
  return /\.(png|jpe?g|gif|webp|bmp|svg)$/i.test(String((f && f.name) || ""));
}
async function fillFilePreviews() {
  const imgs = app.querySelectorAll("[data-preview]");
  for (const img of imgs) {
    try {
      const headers = {};
      if (tg && tg.initData) headers["X-Telegram-Init-Data"] = tg.initData;
      const res = await fetch("/api/deal-files?id=" + img.dataset.preview, { headers });
      if (!res.ok) continue;
      const blob = await res.blob();
      img.src = URL.createObjectURL(blob);
      img.onclick = (e) => { e.preventDefault(); e.stopPropagation(); openLightbox(img.src); };
    } catch (_) {}
  }
}

function openLightbox(src) {
  let box = document.getElementById("lightbox");
  if (!box) {
    box = document.createElement("div");
    box.id = "lightbox";
    box.className = "lightbox";
    box.innerHTML = `<button type="button" class="btn lightbox-x" id="lightboxX">Назад</button><img alt="">`;
    document.body.appendChild(box);
    const close = () => { box.hidden = true; };
    box.addEventListener("click", (e) => {
      if (e.target === box || e.target.id === "lightboxX") close();
    });
  }
  const pic = box.querySelector("img");
  if (pic) pic.src = src;
  box.hidden = false;
}

const TASK_COLS = [
  { key: "new", title: "Новые", hint: "Только попали в воронку" },
  { key: "in_progress", title: "В работе", hint: "Сейчас делают" },
  { key: "docs", title: "Документы", hint: "Ждут файлы по сделке" },
  { key: "done", title: "Готово", hint: "Закрыли" }
];

let dirTab = "menu";
let docsTab = "";
let docsOwner = 0;
let docsFolder = "internal";
let docsName = "";
let saldoTab = "menu";
let saldoFromRequest = false;
let saldoDraft = { cp: "", kind: "", action: "", currency: "", requestId: 0, counterpartyId: 0, reqTitle: "" };
let reqOpen = 0;
let reqPane = "";
let docsPoll = 0;
function stopDocsPoll() {
  if (docsPoll) { clearInterval(docsPoll); docsPoll = 0; }
}
function startDocsPoll(n) {
  stopDocsPoll();
  let prev = Number(n) || 0;
  docsPoll = setInterval(async () => {
    if (currentSection !== "requests" || reqPane !== "docs" || !reqOpen) { stopDocsPoll(); return; }
    try {
      const b = await api("/api/requests?id=" + reqOpen);
      const next = asList(b && b.files).length;
      if (next !== prev) {
        prev = next;
        renderRequests();
      }
    } catch (_) {}
  }, 3000);
}
function startVaultPoll(n, qstr) {
  stopDocsPoll();
  let prev = Number(n) || 0;
  docsPoll = setInterval(async () => {
    if (currentSection !== "documents") { stopDocsPoll(); return; }
    try {
      const b = await api("/api/documents?" + qstr);
      const next = asList(b && b.files).length;
      if (next !== prev) {
        prev = next;
        renderDocuments();
      }
    } catch (_) {}
  }, 3000);
}
function openBotChat(uname, start) {
  const u = String(uname || (me && me.bot_username) || "").replace(/^@/, "");
  if (!u) { toast("Откройте личку с ботом Mini App и перешлите туда"); return; }
  let href = "https://t.me/" + u;
  const payload = String(start || "").trim();
  if (payload) href += "?start=" + encodeURIComponent(payload);
  try {
    if (tg && typeof tg.openTelegramLink === "function") { tg.openTelegramLink(href); return; }
  } catch (_) {}
  try { window.open(href, "_blank"); } catch (_) { location.href = href; }
}
let taskForm = false;
let taskDraft = { requestId: 0, title: "", text: "" };
let aprTab = "queue";
let aprDspId = 0;

function kindName(k) {
  if (k === "swift") return "SWIFT";
  if (k === "nerez") return "Нерезидентский рубль";
  return k || "";
}

function kb(items, selected, dataKey, cols) {
  return `<div class="kb ${cols === 3 ? "cols3" : ""}">${(items || []).map((it) => {
    const val = typeof it === "string" ? it : it.id;
    const label = typeof it === "string" ? it : it.name;
    const wide = typeof it === "object" && it.wide;
    return `<button type="button" class="kb-btn ${wide ? "wide" : ""} ${selected === val ? "on" : ""}" data-${dataKey}="${esc(val)}" data-search="${esc(String(label).toLowerCase())}">${esc(label)}</button>`;
  }).join("")}</div>`;
}

function emptyDraft() {
  return { cp: "", kind: "", action: "", currency: "", requestId: 0, counterpartyId: 0, reqTitle: "" };
}

function dirMeta(item, kind) {
  const bits = ["#" + item.id];
  if (kind === "employees") {
    if (item.number) bits.push("№ " + item.number);
    if (item.title) bits.push(item.title);
    if (item.telegram_id) bits.push("TG " + item.telegram_id);
    else bits.push("без Mini App");
  }
  if (kind === "managers" && item.abbrev) bits.push(item.abbrev);
  if ((kind === "clients" || kind === "counterparties") && item.work_id) bits.push("ID " + item.work_id);
  if (item.created_name && (kind === "managers" || kind === "clients" || kind === "counterparties")) bits.push("добавил " + item.created_name);
  return bits.join(" · ");
}

function poolPeople(dirs) {
  return asList(dirs && dirs.access_pool);
}
function userOpt(u) {
  return `${u.name || "без имени"} · ${u.id}`;
}
function dirAddForm(kind, dirs) {
  if (kind === "employees") {
    const pool = poolPeople(dirs);
    const poss = asList(dirs && dirs.positions);
    return `<div class="dir-add">
      <select id="dUser-${kind}"><option value="">Кто из доступов ещё не сотрудник</option>${pool.map((u) => `<option value="${u.id}">${esc(userOpt(u))}</option>`).join("")}</select>
      <input id="dNum-${kind}" placeholder="Номер сотрудника, 1–2 цифры">
      <div class="meta">Должности — можно несколько</div>
      ${posChecks("newemp", [], poss)}
      <p class="meta">${pool.length ? "Человека сначала добавляют карточкой Доступы на главной. Потом он появляется в этом списке." : "Список пуст: сначала выдайте вход в карточке Доступы на главной."}</p>
      <button type="button" class="btn primary" data-diradd="${kind}">Добавить сотрудника</button>
    </div>`;
  }
  if (kind === "positions") {
    return `<div class="dir-add">
      <input id="dName-${kind}" placeholder="Название должности">
      <p class="meta">Галочки — что видит человек с этой должностью. Одинаковые имена нельзя.</p>
      ${posAccessChecks("newpos", {})}
      <button type="button" class="btn primary" data-diradd="${kind}">Добавить</button>
    </div>`;
  }
  if (kind === "managers") {
    return `<div class="dir-add">
      <input id="dName-${kind}" placeholder="Имя / название">
      <input id="dAbbrev-${kind}" placeholder="Аббревиатура, две буквы: ЖЖ">
      <p class="meta">Две буквы — начало номера заявки, например ЖЖ01552.</p>
      <button type="button" class="btn primary" data-diradd="${kind}">Добавить</button>
    </div>`;
  }
  return `<div class="dir-add">
    <input id="dName-${kind}" placeholder="Название">
    <input id="dWork-${kind}" placeholder="Рабочий ID (вручную)">
    <button type="button" class="btn primary" data-diradd="${kind}">Добавить</button>
  </div>`;
}

function posAccessChecks(prefix, access) {
  return `<div class="checks">${POS_ACCESS.map((s) => {
    const on = !!(access && access[s.key]);
    return `<label class="check ${on ? "on" : ""}"><input type="checkbox" data-sec="${s.key}" data-prefix="${prefix}" ${on ? "checked" : ""}> ${esc(s.title)}</label>`;
  }).join("")}</div>`;
}
function posChecks(prefix, selected, poss) {
  const set = {};
  asList(selected).forEach((id) => { set[Number(id)] = true; });
  if (!poss.length) return `<p class="meta">Сначала заведите должности.</p>`;
  return `<div class="checks">${poss.map((p) => {
    const on = !!set[Number(p.id)];
    return `<label class="check ${on ? "on" : ""}"><input type="checkbox" data-pos="${p.id}" data-prefix="${prefix}" ${on ? "checked" : ""}> ${esc(p.name)}</label>`;
  }).join("")}</div>`;
}
function readPosIDs(prefix) {
  return [...app.querySelectorAll(`input[data-prefix="${prefix}"][data-pos]`)].filter((el) => el.checked).map((el) => Number(el.dataset.pos));
}
function empCardClass(u, kind, dirs) {
  if (kind === "employees") {
    const poss = asList(dirs && dirs.positions);
    if (asList(u.position_ids).some((id) => !dirByID(poss, id))) return "warn";
  }
  if (kind === "positions" && u.in_use) return "used";
  return "";
}
function dirByID(list, id) {
  return (list || []).find((x) => Number(x.id) === Number(id));
}
function empLabel(e) {
  if (!e) return "—";
  return (e.number ? "№" + e.number + " · " : "") + (e.name || ("#" + e.id));
}
function mgrLabel(m) {
  if (!m) return "—";
  return (m.abbrev ? m.abbrev + " · " : "") + (m.name || ("#" + m.id));
}
function clLabel(c) {
  if (!c) return "—";
  return (c.work_id ? c.work_id + " · " : "") + (c.name || ("#" + c.id));
}

function dirItemCard(u, kind, dirs) {
  const emps = asList(dirs && dirs.employees);
  const mgrs = asList(dirs && dirs.managers);
  const cls = asList(dirs && dirs.clients);
  const admin = true;
  let extra = "";
  if (kind === "employees") {
    const mid = asList(u.manager_ids);
    const cid = asList(u.client_ids);
    const freeMgr = mgrs.filter((m) => mid.indexOf(m.id) < 0 && mid.indexOf(Number(m.id)) < 0);
    const freeCl = cls.filter((c) => cid.indexOf(c.id) < 0 && cid.indexOf(Number(c.id)) < 0);
    const pool = poolPeople(dirs);
    extra = `<div class="row wrap">
      <input data-empnum="${u.id}" placeholder="Номер, 1–2 цифры" value="${esc(u.number || "")}">
      ${admin ? `<button type="button" class="btn" data-saveemp="${u.id}">Сохранить номер</button>` : ""}
    </div>
    ${u.telegram_id ? `<p class="meta">Mini App: Telegram ${esc(u.telegram_id)}</p>` : `<p class="meta">Человека из доступов ещё не выбрали — Mini App не откроет.</p>`}
    ${admin && !u.telegram_id && pool.length ? `<div class="row wrap">
      <select data-pooluser="${u.id}"><option value="">Выбрать из доступов</option>${pool.map((p) => `<option value="${p.id}">${esc(userOpt(p))}</option>`).join("")}</select>
      <button type="button" class="btn" data-binduser="${u.id}">Привязать</button>
    </div>` : ""}
    <div class="meta">Должности</div>
    ${(() => {
      const poss = asList(dirs && dirs.positions);
      const pids = asList(u.position_ids);
      const chips = pids.length ? pids.map((id) => {
        const p = dirByID(poss, id);
        if (!p) return `<div class="row wrap"><span class="chip bad">должность удалена — доступов нет</span>${admin ? `<button type="button" class="btn bad" data-unlpos="${u.id}" data-pid="${id}">Снять</button>` : ""}</div>`;
        return `<div class="row wrap"><span class="chip">${esc(p.name)}</span>${admin ? `<button type="button" class="btn bad" data-unlpos="${u.id}" data-pid="${id}">Снять</button>` : ""}</div>`;
      }).join("") : `<p class="meta">нет должности — обычный не увидит разделы</p>`;
      const freePos = poss.filter((p) => pids.indexOf(p.id) < 0 && pids.indexOf(Number(p.id)) < 0);
      const add = admin && freePos.length ? `<div class="row wrap">
        <select data-linkpos="${u.id}"><option value="">Добавить должность</option>${optList(freePos, () => "")}</select>
        <button type="button" class="btn" data-gopos="${u.id}">Назначить</button>
      </div>` : "";
      return chips + add;
    })()}
    <div class="meta">Менеджеры</div>
    ${mid.length ? mid.map((id) => {
      const m = dirByID(mgrs, id);
      return `<div class="row wrap"><span class="meta">${esc(mgrLabel(m))}</span>${admin ? `<button type="button" class="btn bad" data-unlmg="${u.id}" data-mid="${id}">Снять</button>` : ""}</div>`;
    }).join("") : `<p class="meta">не назначен</p>`}
    ${admin && freeMgr.length ? `<div class="row wrap">
      <select data-linkmgr="${u.id}"><option value="">Назначить на менеджера</option>${optList(freeMgr, (m) => m.abbrev || "")}</select>
      <button type="button" class="btn" data-gomgr="${u.id}">Назначить</button>
    </div>` : ""}
    <div class="meta">Клиенты</div>
    ${cid.length ? cid.map((id) => {
      const c = dirByID(cls, id);
      return `<div class="row wrap"><span class="meta">${esc(clLabel(c))}</span>${admin ? `<button type="button" class="btn bad" data-unlcl="${u.id}" data-cid="${id}">Снять</button>` : ""}</div>`;
    }).join("") : `<p class="meta">не назначен</p>`}
    ${admin && freeCl.length ? `<div class="row wrap">
      <select data-linkcl="${u.id}"><option value="">Связать с клиентом</option>${optList(freeCl, (c) => c.work_id || "")}</select>
      <button type="button" class="btn" data-gocl="${u.id}">Назначить</button>
    </div>` : ""}
    ${admin && u.telegram_id ? `<div class="row"><button type="button" class="btn bad" data-revemp="${u.id}">Убрать доступ</button></div>` : ""}`;
  }
  if (kind === "managers") {
    const eids = asList(u.employee_ids);
    const free = emps.filter((e) => eids.indexOf(e.id) < 0 && eids.indexOf(Number(e.id)) < 0);
    extra = `<div class="row wrap">
      <input data-mgrab="${u.id}" placeholder="Две буквы" value="${esc(u.abbrev || "")}">
      ${admin ? `<button type="button" class="btn" data-savemgr="${u.id}">Сохранить аббревиатуру</button>` : ""}
    </div>
    <div class="meta">Сотрудники (ID карточки)</div>
    ${eids.length ? eids.map((id) => {
      const e = dirByID(emps, id);
      return `<div class="row wrap"><span class="meta">${esc(empLabel(e))} · id ${id}</span>${admin ? `<button type="button" class="btn bad" data-unlemp="managers" data-oid="${u.id}" data-eid="${id}">Снять</button>` : ""}</div>`;
    }).join("") : `<p class="meta">никто не назначен</p>`}
    ${admin && free.length ? `<div class="row wrap">
      <select data-linkemp="managers" data-oid="${u.id}"><option value="">Назначить сотрудника</option>${optList(free, (e) => e.number ? "№ " + e.number : "")}</select>
      <button type="button" class="btn" data-goemp="managers" data-oid="${u.id}">Назначить</button>
    </div>` : ""}`;
  }
  if (kind === "clients") {
    const eids = asList(u.employee_ids);
    const free = emps.filter((e) => eids.indexOf(e.id) < 0 && eids.indexOf(Number(e.id)) < 0);
    extra = `<div class="row wrap">
      <input data-clname="${u.id}" placeholder="Название" value="${esc(u.name || "")}">
      <input data-clwork="${u.id}" placeholder="Рабочий ID" value="${esc(u.work_id || "")}">
      ${admin ? `<button type="button" class="btn" data-savecl="${u.id}">Сохранить</button>` : ""}
    </div>
    <div class="meta">Сотрудники (ID карточки)</div>
    ${eids.length ? eids.map((id) => {
      const e = dirByID(emps, id);
      return `<div class="row wrap"><span class="meta">${esc(empLabel(e))} · id ${id}</span>${admin ? `<button type="button" class="btn bad" data-unlemp="clients" data-oid="${u.id}" data-eid="${id}">Снять</button>` : ""}</div>`;
    }).join("") : `<p class="meta">никто не назначен</p>`}
    ${admin && free.length ? `<div class="row wrap">
      <select data-linkemp="clients" data-oid="${u.id}"><option value="">Назначить сотрудника</option>${optList(free, (e) => e.number ? "№ " + e.number : "")}</select>
      <button type="button" class="btn" data-goemp="clients" data-oid="${u.id}">Назначить</button>
    </div>` : ""}`;
  }
  if (kind === "counterparties") {
    extra = `<div class="row wrap">
      <input data-cpname="${u.id}" placeholder="Название" value="${esc(u.name || "")}">
      <input data-cpwork="${u.id}" placeholder="Рабочий ID" value="${esc(u.work_id || "")}">
      ${admin ? `<button type="button" class="btn" data-savecp="${u.id}">Сохранить</button>` : ""}
    </div>`;
  }
  if (kind === "positions") {
    const eids = asList(u.employee_ids);
    extra = `${u.in_use ? `<p class="warn-line">Висит на сотрудниках — если удалите, у них загорится красным.</p>` : ""}
      ${posAccessChecks("pos" + u.id, u.access || {})}
      <div class="row"><button type="button" class="btn primary" data-savepos="${u.id}">Сохранить доступы</button></div>
      <div class="meta">Сотрудники с этой должностью</div>
      ${eids.length ? eids.map((id) => {
        const e = dirByID(emps, id);
        return `<div class="row wrap"><span class="meta">${esc(empLabel(e))}</span></div>`;
      }).join("") : `<p class="meta">никто не назначен</p>`}`;
  }
  return `<div class="dir-item">
    <div class="person">
      <div class="ava">${esc(initials(u.name))}</div>
      <div style="flex:1">
        <h3>${esc(u.name)}</h3>
        <div class="meta">${esc(dirMeta(u, kind))}</div>
      </div>
    </div>
    ${extra}
    ${((kind === "managers" || kind === "clients" || kind === "counterparties") ? canDelRel(u) : true) ? `<div class="row"><button type="button" class="btn bad" data-deldir="${u.id}" data-dirkind="${kind}">Удалить</button></div>` : ""}
  </div>`;
}

async function renderDirectory() {
  const token = bumpNav();
  currentSection = "directory";
  if (dirTab === "menu") {
    page("Справочник", `
      <p class="hint">Сотрудники — кто заходит. Должности — какие разделы видит. Остальное — общности. Дубликаты имён нельзя.</p>
      <div class="kb">
        ${DIR_KINDS.map((k) => `<button type="button" class="kb-btn wide ${k.key === "positions" && dirWarn() ? "has-warn" : ""}" data-dirkind="${k.key}">${k.key === "positions" && dirWarn() ? warnDot() : ""}<b>${esc(k.title)}</b><span class="meta">${esc(k.desc)}</span></button>`).join("")}
      </div>
    `, token);
    if (!alive(token)) return;
    app.querySelectorAll("[data-dirkind]").forEach((b) => {
      b.onclick = () => { buzz(); dirTab = b.dataset.dirkind; renderDirectory(); };
    });
    return;
  }
  const kind = DIR_KINDS.find((k) => k.key === dirTab) || DIR_KINDS[0];
  let dirs = {};
  try { dirs = await api("/api/directory"); }
  catch (e) {
    page(kind.title, `<p class="err">${esc(e.message)}</p>`, token);
    return;
  }
  if (!alive(token)) return;
  const list = asList(dirs[kind.key]);
  page(kind.title, `
    <div class="crumbs">
      <button type="button" class="crumb" id="dirBack">Справочник</button>
      <span>›</span>
      <span>${esc(kind.title)}</span>
    </div>
    <div class="card">
      <h3>Добавить</h3>
      ${dirAddForm(kind.key, dirs)}
    </div>
    ${list.map((u) => `<div class="card ${empCardClass(u, kind.key, dirs)}">${dirItemCard(u, kind.key, dirs)}</div>`).join("") || `<p class="empty">Пока пусто — добавьте запись текстом выше.</p>`}
  `, token);
  const back = $("dirBack");
  if (back) back.onclick = () => { buzz(); dirTab = "menu"; renderDirectory(); };
  app.querySelectorAll("[data-diradd]").forEach((b) => {
    b.onclick = async () => {
      const k = b.dataset.diradd;
      const val = (id) => { const el = $(id + "-" + k); return el ? el.value : ""; };
      const body = { kind: k, name: val("dName") };
      if (k === "employees") {
        const sel = $("dUser-" + k);
        body.telegram_id = sel ? Number(sel.value) || 0 : 0;
        body.number = val("dNum");
        body.position_ids = readPosIDs("newemp");
      }
      if (k === "positions") body.access = readAccess("newpos");
      if (k === "managers") body.abbrev = val("dAbbrev");
      if (k === "clients" || k === "counterparties") body.work_id = val("dWork");
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify(body) });
        toast("Добавлено");
        try { await refreshMe(); } catch (_) {}
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-revemp]").forEach((b) => {
    b.onclick = async () => {
      const ok = await ask("Убрать доступ в Mini App? Карточка сотрудника останется. Владельца снять нельзя.");
      if (!ok) return;
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.revemp), revoke_access: true }) });
        toast("Доступ убран");
        try { await refreshMe(); } catch (_) {}
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-saveemp]").forEach((b) => {
    b.onclick = async () => {
      const inp = app.querySelector(`[data-empnum="${b.dataset.saveemp}"]`);
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.saveemp), number: inp ? inp.value : "" }) });
        toast("Номер сохранён");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-binduser]").forEach((b) => {
    b.onclick = async () => {
      const sel = app.querySelector(`[data-pooluser="${b.dataset.binduser}"]`);
      const tgID = sel ? Number(sel.value) : 0;
      if (!tgID) { toast("Выберите человека из доступов"); return; }
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.binduser), telegram_id: tgID }) });
        toast("Сотрудник привязан");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-gomgr]").forEach((b) => {
    b.onclick = async () => {
      const sel = app.querySelector(`[data-linkmgr="${b.dataset.gomgr}"]`);
      const mid = sel ? Number(sel.value) : 0;
      if (!mid) { toast("Выберите менеджера"); return; }
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.gomgr), link_manager_id: mid }) });
        toast("Назначен на менеджера");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-unlmg]").forEach((b) => {
    b.onclick = async () => {
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.unlmg), unlink_manager_id: Number(b.dataset.mid) }) });
        toast("Сняли с менеджера");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-gopos]").forEach((b) => {
    b.onclick = async () => {
      const sel = app.querySelector(`[data-linkpos="${b.dataset.gopos}"]`);
      const pid = sel ? Number(sel.value) : 0;
      if (!pid) { toast("Выберите должность"); return; }
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.gopos), link_position_id: pid }) });
        toast("Должность назначена");
        try { await refreshMe(); } catch (_) {}
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-unlpos]").forEach((b) => {
    b.onclick = async () => {
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.unlpos), unlink_position_id: Number(b.dataset.pid) }) });
        toast("Должность снята");
        try { await refreshMe(); } catch (_) {}
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-savepos]").forEach((b) => {
    b.onclick = async () => {
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "positions", id: Number(b.dataset.savepos), access: readAccess("pos" + b.dataset.savepos) }) });
        toast("Доступы должности сохранены");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-gocl]").forEach((b) => {
    b.onclick = async () => {
      const sel = app.querySelector(`[data-linkcl="${b.dataset.gocl}"]`);
      const cid = sel ? Number(sel.value) : 0;
      if (!cid) { toast("Выберите клиента"); return; }
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.gocl), link_client_id: cid }) });
        toast("Связали с клиентом");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-unlcl]").forEach((b) => {
    b.onclick = async () => {
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "employees", id: Number(b.dataset.unlcl), unlink_client_id: Number(b.dataset.cid) }) });
        toast("Сняли с клиента");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-goemp]").forEach((b) => {
    b.onclick = async () => {
      const sel = app.querySelector(`select[data-linkemp="${b.dataset.goemp}"][data-oid="${b.dataset.oid}"]`);
      const eid = sel ? Number(sel.value) : 0;
      if (!eid) { toast("Выберите сотрудника"); return; }
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: b.dataset.goemp, id: Number(b.dataset.oid), link_employee_id: eid }) });
        toast("Сотрудник назначен");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-unlemp]").forEach((b) => {
    b.onclick = async () => {
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: b.dataset.unlemp, id: Number(b.dataset.oid), unlink_employee_id: Number(b.dataset.eid) }) });
        toast("Сняли сотрудника");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-savemgr]").forEach((b) => {
    b.onclick = async () => {
      const inp = app.querySelector(`[data-mgrab="${b.dataset.savemgr}"]`);
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "managers", id: Number(b.dataset.savemgr), abbrev: inp ? inp.value : "" }) });
        toast("Аббревиатура сохранена");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-savecl]").forEach((b) => {
    b.onclick = async () => {
      const name = app.querySelector(`[data-clname="${b.dataset.savecl}"]`);
      const work = app.querySelector(`[data-clwork="${b.dataset.savecl}"]`);
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "clients", id: Number(b.dataset.savecl), name: name ? name.value : "", work_id: work ? work.value : "" }) });
        toast("Клиент сохранён");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-savecp]").forEach((b) => {
    b.onclick = async () => {
      const name = app.querySelector(`[data-cpname="${b.dataset.savecp}"]`);
      const work = app.querySelector(`[data-cpwork="${b.dataset.savecp}"]`);
      try {
        await api("/api/directory", { method: "POST", body: JSON.stringify({ kind: "counterparties", id: Number(b.dataset.savecp), name: name ? name.value : "", work_id: work ? work.value : "" }) });
        toast("Контрагент сохранён");
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-deldir]").forEach((b) => {
    b.onclick = async () => {
      const ok = await ask(b.dataset.dirkind === "positions" ? "Удалить должность? Она снимется у всех сотрудников, разделы пропадут." : "Удалить запись из справочника?");
      if (!ok) return;
      try {
        await api("/api/directory?kind=" + encodeURIComponent(b.dataset.dirkind) + "&id=" + b.dataset.deldir, { method: "DELETE" });
        toast("Удалено");
        try { await refreshMe(); } catch (_) {}
        renderDirectory();
      } catch (e) { toast(e.message); }
    };
  });
  bindChecks();
}

function dirCrumbs(text) {
  return `<div class="crumbs"><span>${esc(text)}</span></div>`;
}

function crumbs(d) {
  const parts = [];
  if (d.requestId) parts.push(`<span class="crumb">${esc(d.reqTitle || ("заявка #" + d.requestId))}</span>`);
  if (d.cp) {
    if (d.requestId) parts.push(`<span class="crumb">${esc(d.cp)}</span>`);
    else parts.push(`<button type="button" class="crumb" data-step="cp">${esc(d.cp)}</button>`);
  }
  if (d.kind) parts.push(`<button type="button" class="crumb" data-step="kind">${esc(kindName(d.kind))}</button>`);
  if (d.action) parts.push(`<button type="button" class="crumb" data-step="act">${d.action === "minus" ? "Закрыл" : "Поставил"}</button>`);
  if (d.currency) parts.push(`<button type="button" class="crumb" data-step="cur">${esc(d.currency)}</button>`);
  if (!parts.length) return "";
  return `<div class="crumbs">${parts.join("<span>›</span>")}</div>`;
}

function downloadText(name, content, mime) {
  const a = document.createElement("a");
  a.href = URL.createObjectURL(new Blob([content], { type: mime || "text/plain;charset=utf-8" }));
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1500);
}

function payBadge(st) {
  if (st === "pending") return `<span class="badge">ожидает отправки</span>`;
  if (st === "sent") return `<span class="badge green">отправлено</span>`;
  return "";
}
function aprBadge(list) {
  const a = asList(list);
  if (!a.length) return "";
  const last = a[0];
  if (last.status === "pending") return `<span class="badge">согласование</span>`;
  if (last.status === "approved") return `<span class="badge green">согласовано</span>`;
  if (last.status === "rejected") return `<span class="badge">отклонено</span>`;
  return "";
}

async function downloadDeal(id, name) {
  const headers = {};
  if (tg && tg.initData) headers["X-Telegram-Init-Data"] = tg.initData;
  const res = await fetch("/api/deal-files?id=" + id, { headers });
  if (!res.ok) throw new Error(await res.text() || res.status);
  const blob = await res.blob();
  const safe = String(name || "file").replace(/[/\\?%*:|"<>]/g, "_") || "file";
  const file = new File([blob], safe, { type: blob.type || "application/octet-stream" });
  try {
    if (navigator.canShare && navigator.canShare({ files: [file] })) {
      await navigator.share({ files: [file], title: safe });
      return;
    }
  } catch (e) {
    if (e && e.name === "AbortError") return;
  }
  const out = new Blob([blob], { type: "application/octet-stream" });
  const url = URL.createObjectURL(out);
  const a = document.createElement("a");
  a.href = url;
  a.setAttribute("download", safe);
  a.rel = "noopener";
  a.style.display = "none";
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 2500);
  toast("Если файл не сохранился — «Себе в личку»: так Mini App не затрётся.");
}

async function downloadVault(id, name, entry = "") {
  const headers = {};
  if (tg && tg.initData) headers["X-Telegram-Init-Data"] = tg.initData;
  const res = await fetch("/api/documents?id=" + id + (entry ? "&entry=" + encodeURIComponent(entry) : ""), { headers });
  if (!res.ok) throw new Error(String(await res.text() || res.status).trim());
  const blob = await res.blob();
  const safe = String(name || "file").replace(/[/\\?%*:|"<>]/g, "_") || "file";
  const file = new File([blob], safe, { type: blob.type || "application/octet-stream" });
  try {
    if (navigator.canShare && navigator.canShare({ files: [file] })) {
      await navigator.share({ files: [file], title: safe });
      return;
    }
  } catch (e) {
    if (e && e.name === "AbortError") return;
  }
  const out = new Blob([blob], { type: "application/octet-stream" });
  const url = URL.createObjectURL(out);
  const a = document.createElement("a");
  a.href = url;
  a.setAttribute("download", safe);
  a.rel = "noopener";
  a.style.display = "none";
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 2500);
  toast("Если файл не сохранился — «Себе в личку»: так Mini App не затрётся.");
}

async function uploadVault(fields, fileList) {
  const headers = {};
  if (tg && tg.initData) headers["X-Telegram-Init-Data"] = tg.initData;
  const fd = new FormData();
  Object.keys(fields || {}).forEach((k) => fd.append(k, fields[k]));
  for (const f of fileList) fd.append("files", f);
  const res = await fetch("/api/documents", { method: "POST", headers, body: fd });
  const text = await res.text();
  if (!res.ok) throw new Error(String(text || res.status).trim());
  try { return JSON.parse(text); } catch { return text; }
}

function optList(items, extra) {
  return (items || []).map((it) => {
    const add = extra ? extra(it) : "";
    return `<option value="${it.id}">${esc(it.name)}${add ? " · " + esc(add) : ""}</option>`;
  }).join("");
}

function myIDs() {
  const ids = [];
  if (me && me.id) ids.push(Number(me.id));
  try {
    const u = tg && tg.initDataUnsafe && tg.initDataUnsafe.user;
    if (u && u.id) ids.push(Number(u.id));
  } catch (_) {}
  return ids.filter((n) => n);
}
function isMine(by) {
  const n = Number(by);
  if (!n) return false;
  return myIDs().some((id) => id === n);
}
function canDelFile(f) {
  return !!(f && (f.mine || isMine(f.created_by)));
}

function reqUndoRow(r) {
  if (!isMine(r.created_by)) return "";
  const ret = r.status && r.status !== "open" && r.status !== "deleted"
    ? `<button type="button" class="btn" data-retreq="${r.id}">Вернуть</button>` : "";
  return `<div class="row wrap">
    ${ret}
    <button type="button" class="btn bad" data-delreq="${r.id}">Удалить</button>
  </div>`;
}

function payUndoRow(p) {
  if (!p || !isMine(p.created_by)) return "";
  return `<div class="row wrap">
    ${p.status === "sent" ? `<button type="button" class="btn" data-retpay="${p.id}">Вернуть</button>` : ""}
    <button type="button" class="btn bad" data-delpay="${p.id}">Удалить</button>
  </div>`;
}

function aprUndoRow(a) {
  if (!a || !isMine(a.manager_id)) return "";
  return a.status === "pending"
    ? `<button type="button" class="btn bad" data-delapr="${a.id}">Удалить</button>`
    : `<button type="button" class="btn" data-retapr="${a.id}">Вернуть</button>`;
}

function bindUndo() {
  const after = () => {
    if (currentSection === "requests") return renderRequests();
    if (currentSection === "payments") return renderPayments();
    if (currentSection === "approvals") return renderApprovals();
    if (currentSection === "tasks") return renderTasks();
    openSection(currentSection);
  };
  app.querySelectorAll("[data-delreq]").forEach((b) => b.onclick = async () => {
    const ok = await ask("Удалить заявку? Её не будет в списке.");
    if (!ok) return;
    try {
      await api("/api/requests?id=" + b.dataset.delreq, { method: "DELETE" });
      toast("Заявка удалена");
      reqOpen = 0; reqPane = "";
      renderRequests();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-retreq]").forEach((b) => b.onclick = async () => {
    try {
      await api("/api/requests", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.retreq), status: "open" }) });
      toast("Заявка возвращена");
      after();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-delpay]").forEach((b) => b.onclick = async () => {
    const ok = await ask("Удалить эту оплату из отправки?");
    if (!ok) return;
    try {
      await api("/api/payments", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.delpay), status: "deleted" }) });
      toast("Оплата удалена");
      after();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-retpay]").forEach((b) => b.onclick = async () => {
    try {
      await api("/api/payments", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.retpay), status: "pending" }) });
      toast("Оплата возвращена в очередь");
      after();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-delapr]").forEach((b) => b.onclick = async () => {
    const ok = await ask("Снять это согласование?");
    if (!ok) return;
    try {
      await api("/api/approvals", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.delapr), status: "withdrawn" }) });
      toast("Согласование снято");
      after();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-retapr]").forEach((b) => b.onclick = async () => {
    try {
      await api("/api/approvals", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.retapr), status: "pending" }) });
      toast("Согласование возвращено в очередь");
      after();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-delfile]").forEach((b) => b.onclick = async () => {
    const ok = await ask("Удалить это из сделки? У всех пропадёт.");
    if (!ok) return;
    try {
      await api("/api/deal-files?id=" + b.dataset.delfile, { method: "DELETE" });
      toast("Удалено");
      after();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-deltask]").forEach((b) => b.onclick = async () => {
    const ok = await ask("Удалить задачу из воронки?");
    if (!ok) return;
    try {
      await api("/api/tasks?id=" + b.dataset.deltask, { method: "DELETE" });
      toast("Задача удалена");
      after();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-movetask]").forEach((b) => b.onclick = async () => {
    try {
      await api("/api/tasks", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.movetask), status: b.dataset.v }) });
      toast("Перенесли");
      after();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-delap]").forEach((b) => b.onclick = async () => {
    const ok = await ask("Убрать это обращение из заявки?");
    if (!ok) return;
    try {
      await api("/api/appeals?id=" + b.dataset.delap, { method: "DELETE" });
      toast("Обращение снято");
      after();
    } catch (e) { toast(e.message); }
  });
}

function reqPeople(r) {
  const emp = r.employee_name || r.created_name || "—";
  const mgr = r.manager_abbrev ? `${r.manager_name} (${r.manager_abbrev})` : (r.manager_name || "—");
  const cl = r.client_work_id ? `${r.client_name} · ${r.client_work_id}` : (r.client_name || "—");
  const cp = r.counterparty_work_id ? `${r.counterparty_name} · ${r.counterparty_work_id}` : (r.counterparty_name || "—");
  return `<div class="meta">Сотрудник: ${esc(emp)}</div>
    <div class="meta">Менеджер: ${esc(mgr)}</div>
    <div class="meta">Клиент: ${esc(cl)}</div>
    <div class="meta">Контрагент: ${esc(cp)}</div>`;
}

function reqStatusBtns(r) {
  const st = String(r.status || "");
  return `<div class="kb req-status">
      <button type="button" class="kb-btn ${st === "in_progress" ? "on" : ""}" data-st="${r.id}" data-v="in_progress">В работе</button>
      <button type="button" class="kb-btn ${st === "done" ? "on" : ""}" data-st="${r.id}" data-v="done">В таблицу</button>
    </div>`;
}

function archiveDialog(title) {
  const dialog = document.createElement("dialog");
  dialog.className = "archive-modal";
  dialog.innerHTML = `<header><h3>${esc(title)}</h3><button type="button" class="btn" data-close>Закрыть</button></header><div class="archive-body"></div>`;
  document.body.appendChild(dialog);
  dialog.querySelector("[data-close]").onclick = () => dialog.close();
  dialog.onclose = () => dialog.remove();
  dialog.showModal();
  return dialog;
}

function requestDocumentButtons(r) {
  if (!has("documents")) return "";
  if (!r.document_id) return `<button class="btn primary" data-arc="${r.id}">Сохранить в документы</button>`;
  return `<button class="btn" data-document-view="${r.document_id}">Открыть в документах</button>
    ${(isAdmin() || isMine(r.document_by)) ? `<button class="btn bad" data-document-delete="${r.document_id}">Удалить из документов</button>` : ""}
    <span class="meta">${esc(r.document_place || "Сохранено в документы")}</span>`;
}

async function removeSavedRequest(id, refresh) {
  if (!await ask("Удалить сохранённую копию со всеми вложениями из документов? Исходная заявка останется.")) return false;
  await api("/api/documents?id=" + id, { method: "DELETE" });
  toast("Удалено из документов");
  await refresh();
  return true;
}

function bindArchiveButtons() {
  app.querySelectorAll("[data-arc]").forEach((b) => b.onclick = (e) => {
    e.stopPropagation(); showArchiveModal(Number(b.dataset.arc));
  });
  app.querySelectorAll("[data-document-view]").forEach((b) => b.onclick = () => showSavedRequest(Number(b.dataset.documentView), renderRequests));
  app.querySelectorAll("[data-document-delete]").forEach((b) => b.onclick = async () => {
    b.disabled = true;
    try { await removeSavedRequest(Number(b.dataset.documentDelete), renderRequests); }
    catch (e) { toast(e.message); }
    finally { b.disabled = false; }
  });
}

async function showArchiveModal(reqId) {
  const dialog = archiveDialog("Сохранить заявку в документы");
  const body = dialog.querySelector(".archive-body");
  body.innerHTML = `<p class="hint">Загружаем существующие вкладки…</p>`;
  try {
    const destinations = asList(await api("/api/documents?destinations=1"));
    if (!dialog.isConnected) return;
    body.innerHTML = `<p class="hint">Сохранятся все данные заявки на этот момент: файлы, сообщения и связанные операции.</p>
      <label>Куда сохранить<select id="archiveDestination">${destinations.map((d, i) => `<option value="${i}">${esc((d.scope === "company" ? "Компании · " : d.scope === "counterparty" ? "Контрагенты · " : "") + d.label)}</option>`).join("")}</select></label>
      <p class="err" data-error hidden></p><button class="btn primary" data-save>Сохранить всё</button>`;
    const save = body.querySelector("[data-save]");
    save.disabled = !destinations.length;
    save.onclick = async () => {
      save.disabled = true;
      const error = body.querySelector("[data-error]");
      error.hidden = true;
      try {
        const dest = destinations[Number(body.querySelector("select").value)];
        await api("/api/requests/archive", { method: "POST", body: JSON.stringify({ request_id: reqId, ...dest }) });
        dialog.close(); toast("Заявка целиком сохранена в документы");
        if (currentSection === "requests") await renderRequests();
      } catch (e) { error.textContent = e.message; error.hidden = false; save.disabled = false; }
    };
  } catch (e) { body.innerHTML = `<p class="err">${esc(e.message)}</p>`; }
}

async function showSavedRequest(id, refresh = renderDocuments) {
  const dialog = archiveDialog("Сохранённая заявка");
  const body = dialog.querySelector(".archive-body");
  body.textContent = "Загрузка…";
  try {
    const data = await api("/api/documents?id=" + id + "&archive=1");
    if (!dialog.isConnected) return;
    const f = data.file;
    const entries = asList(data.entries);
    body.innerHTML = `<h3>${esc(f.request_uid)} · ${esc(f.request_title || "")}</h3>
      <p class="meta">Сохранено ${esc(fmtWhen(f.created_at))} · ${esc(f.created_name)}</p>
      <div class="row wrap"><button class="btn primary" data-download-all>Скачать всё ZIP</button>
      <button class="btn" data-report>Скачать отчёт</button>
      ${(isAdmin() || isMine(f.created_by)) ? `<button class="btn bad" data-remove>Удалить из документов</button>` : ""}</div>
      <h3>Вложения · ${entries.length}</h3>
      ${entries.map((entry, i) => `<div class="card"><b>${esc(entry.name)}</b><div class="meta">${fmtSize(entry.size)}</div>
        <div class="row wrap"><button class="btn" data-entry-view="${i}">Открыть</button><button class="btn" data-entry-download="${i}">Скачать</button></div></div>`).join("")}
      <h3>Карточка, сообщения и операции</h3><pre class="archive-report">${esc(data.report)}</pre>`;
    body.querySelector("[data-download-all]").onclick = async () => { try { await downloadVault(id, f.name); } catch (e) { toast(e.message); } };
    body.querySelector("[data-report]").onclick = async () => { try { await downloadVault(id, "Заявка_" + f.request_uid + ".txt", "report.txt"); } catch (e) { toast(e.message); } };
    body.querySelectorAll("[data-entry-download]").forEach((b) => b.onclick = async () => {
      const entry = entries[Number(b.dataset.entryDownload)];
      try { await downloadVault(id, entry.name, entry.path); } catch (e) { toast(e.message); }
    });
    body.querySelectorAll("[data-entry-view]").forEach((b) => b.onclick = () => {
      const entry = entries[Number(b.dataset.entryView)];
      previewVaultFile(id, entry.name, entry.path);
    });
    const remove = body.querySelector("[data-remove]");
    if (remove) remove.onclick = async () => {
      remove.disabled = true;
      try { if (await removeSavedRequest(id, refresh)) dialog.close(); } catch (e) { toast(e.message); }
      finally { remove.disabled = false; }
    };
  } catch (e) { body.innerHTML = `<p class="err">${esc(e.message)}</p>`; }
}

async function previewVaultFile(id, name, entry = "") {
  const dialog = archiveDialog(name || "Просмотр файла");
  const body = dialog.querySelector(".archive-body");
  body.textContent = "Загрузка…";
  let url;
  dialog.addEventListener("close", () => { if (url) URL.revokeObjectURL(url); });
  try {
    const headers = {};
    if (tg && tg.initData) headers["X-Telegram-Init-Data"] = tg.initData;
    const res = await fetch("/api/documents?id=" + id + (entry ? "&entry=" + encodeURIComponent(entry) : ""), { headers });
    if (!res.ok) throw new Error(await res.text());
    const blob = await res.blob();
    if (!dialog.isConnected) return;
    const mime = blob.type.split(";")[0];
    body.innerHTML = `<button class="btn" data-download>Скачать</button><div class="archive-preview"></div>`;
    body.querySelector("[data-download]").onclick = async () => { try { await downloadVault(id, name, entry); } catch (e) { toast(e.message); } };
    const preview = body.querySelector(".archive-preview");
    if (mime.startsWith("text/") || mime === "application/json") {
      const pre = document.createElement("pre"); pre.className = "archive-report"; pre.textContent = await blob.text(); preview.appendChild(pre);
    } else if (mime.startsWith("image/") || mime === "application/pdf" || mime.startsWith("audio/") || mime.startsWith("video/")) {
      url = URL.createObjectURL(blob);
      const el = document.createElement(mime.startsWith("image/") ? "img" : mime === "application/pdf" ? "iframe" : mime.startsWith("audio/") ? "audio" : "video");
      if (el.tagName === "IFRAME") { el.setAttribute("sandbox", ""); el.title = name; }
      if (el.tagName === "IMG") el.alt = name;
      el.controls = true; el.src = url; preview.appendChild(el);
    } else { preview.textContent = "Этот формат можно скачать и открыть на устройстве."; }
  } catch (e) { body.innerHTML = `<p class="err">${esc(e.message)}</p>`; }
}

async function renderRequests() {
  const token = bumpNav();
  currentSection = "requests";
  if (reqOpen) return renderRequestCard(token);
  const [list, dirs] = await Promise.all([api("/api/requests"), api("/api/directory")]);
  if (!alive(token)) return;
  const managers = asList(dirs.managers);
  const clients = asList(dirs.clients);
  const cps = asList(dirs.counterparties);
  const emp = me && me.employee;
  page("Заявки", `
    <div class="card">
      <p class="hint">Номер можно вписать самому. Если поле пустое — уйдёт сам: две буквы + номер сотрудника + месяц без нуля (1,2,3…10) + порядковый. Пример ЖЖ01552. Удалить или вернуть заявку может только тот, кто её отправил.</p>
      ${emp ? `<div class="meta">Вы сотрудник: ${esc(emp.name)}${emp.number ? " · № " + esc(emp.number) : " · нет номера"}${emp.title ? " · " + esc(emp.title) : ""}</div>` : (isAdmin() ? `<div class="meta">Админ может всё без карточки сотрудника. В автономере будет 00.</div>` : `<div class="meta">Вас ещё нет в Справочник → Сотрудники. Владелец выбирает вас из Доступов.</div>`)}
      <input id="reqUID" placeholder="Номер заявки (если пусто — сам)" autocomplete="off" inputmode="text">
      <input id="reqTitle" placeholder="Комментарий (необязательно)">
      <select id="reqMgr"><option value="">Менеджер</option>${optList(managers, (m) => m.abbrev || "")}</select>
      <select id="reqClient"><option value="">Клиент</option>${optList(clients, (c) => c.work_id || "")}</select>
      <select id="reqCP"><option value="">Контрагент</option>${optList(cps, (c) => c.work_id || "")}</select>
      <button class="btn primary" id="reqCreate">Создать заявку</button>
    </div>
    ${asList(list).map((r) => `
      <div class="card">
        <h3>${esc(r.uid)} <span class="badge">${esc(r.status)}</span> ${payBadge(r.payment_status)}</h3>
        ${r.title && r.title !== r.uid ? `<p>${esc(r.title)}</p>` : ""}
        <div class="meta">${esc(fmtWhen(r.created_at))}${r.files_count ? " · файлов: " + r.files_count : ""}${r.messages_count ? " · сообщ.: " + r.messages_count : ""}</div>
        ${reqPeople(r)}
        <div class="kb">
          <button type="button" class="kb-btn wide acc" data-opendocs="${r.id}"><b>Файлы и сообщения</b><span class="meta">${r.files_count || r.messages_count ? ((r.files_count || 0) + " файлов · " + (r.messages_count || 0) + " сообщ.") : "Переслать из Telegram в эту сделку"}</span></button>
        </div>
         <div class="row wrap">
           <button class="btn primary" data-openreq="${r.id}">Открыть</button>
           ${requestDocumentButtons(r)}
         </div>
         ${reqStatusBtns(r)}

        ${reqUndoRow(r)}
      </div>`).join("") || `<p class="empty">Пока нет заявок</p>`}
  `);
  $("reqCreate").onclick = async () => {
    try {
      const out = await api("/api/requests", { method: "POST", body: JSON.stringify({
        uid: $("reqUID").value,
        title: $("reqTitle").value,
        manager_id: Number($("reqMgr").value) || 0,
        client_id: Number($("reqClient").value) || 0,
        counterparty_id: Number($("reqCP").value) || 0
      }) });
      toast(out && out.uid ? ("Заявка " + out.uid) : "Заявка создана");
      renderRequests();
    } catch (e) { toast(e.message); }
  };
  app.querySelectorAll("[data-openreq]").forEach((b) => b.onclick = (e) => {
    e.stopPropagation();
    reqOpen = Number(b.dataset.openreq);
    reqPane = "";
    buzz();
    renderRequests();
  });
  app.querySelectorAll("[data-opendocs]").forEach((b) => b.onclick = (e) => {
    e.stopPropagation();
    reqOpen = Number(b.dataset.opendocs);
    reqPane = "docs";
    buzz();
    renderRequests();
  });
  bindArchiveButtons();
  bindStatus("/api/requests");
  bindUndo();
}

async function renderRequestCard(token) {
  const b = await api("/api/requests?id=" + reqOpen);
  if (token != null && !alive(token)) return;
  const r = b;
  if (reqPane === "pay") return renderPayPane(r);
  if (reqPane === "docs") return await renderDocsPane(r);
  if (reqPane === "appeal" || reqPane === "appeal_docs" || reqPane === "appeal_lawyer") return renderAppealPane(r);
  const pays = asList(r.payments);
  const lastPay = pays[0];
  page(r.uid || r.title || "Заявка", `
    <div class="card">
      <h3>${esc(r.uid || r.title)} <span class="badge">${esc(r.status)}</span> ${payBadge(r.payment_status)} ${aprBadge(r.approvals)}</h3>
      ${r.title && r.title !== r.uid ? `<p>${esc(r.title)}</p>` : ""}
      ${reqPeople(r)}
      <p class="hint">Контекст заявки наследуется: сальдо, согласование, оплата, документы и обращения не заполняют ФИО заново.</p>
    </div>
    <div class="kb">
      <button type="button" class="kb-btn wide acc" id="rqDocs"><b>Файлы и сообщения</b><span class="meta">${(r.files_count || 0) + (r.messages_count || 0) ? ((r.files_count || 0) + " файлов · " + (r.messages_count || 0) + " сообщ.") : "Переслать из Telegram. В таблицу не уходит."}</span></button>
      <button type="button" class="kb-btn" id="rqSaldo"><b>Сальдо</b><span class="meta">Постановка по этой заявке</span></button>
      <button type="button" class="kb-btn" id="rqApr"><b>Согласование</b><span class="meta">В очередь админу</span></button>
      <button type="button" class="kb-btn" id="rqPay"><b>Оплата</b><span class="meta">Что отправить → Отправка</span></button>
      <button type="button" class="kb-btn" id="rqTask"><b>Ещё задача</b><span class="meta">В канбан с новым номером</span></button>
      <button type="button" class="kb-btn wide" id="rqAppeal"><b>Обращение</b><span class="meta">Документалист или юрист</span></button>
    </div>
      <div class="row wrap">
        <button class="btn" data-dsp="requests" data-uid="${esc(r.uid)}">Спор</button>
        ${requestDocumentButtons(r)}
      </div>
      ${reqStatusBtns(r)}

    ${reqUndoRow(r)}
    ${asList(r.approvals).map((a) => `
      <div class="card">
        <div class="meta">Согласование</div>
        <h3>${esc(a.status)}</h3>
        <div class="meta">${esc(a.manager_name)} · ${esc(fmtWhen(a.created_at))}</div>
        ${aprUndoRow(a)}
      </div>`).join("")}
    ${lastPay ? `<div class="card"><div class="meta">Оплата</div><h3>${esc(lastPay.status === "sent" ? "Отправлено" : "Ждёт отправки")}</h3><p>${esc(lastPay.text)}</p><div class="meta">${esc(fmtWhen(lastPay.created_at))}${lastPay.sent_at ? " · отправлено " + esc(fmtWhen(lastPay.sent_at)) : ""}</div>
      ${payUndoRow(lastPay)}
    </div>` : ""}
  `);
  $("rqSaldo").onclick = () => {
    saldoFromRequest = true;
    saldoTab = "op";
    saldoDraft = {
      cp: r.counterparty_name || "",
      kind: "", action: "", currency: "",
      requestId: r.id,
      counterpartyId: r.counterparty_id || 0,
      reqTitle: r.title || ""
    };
    buzz();
    openSection("saldo");
  };
  $("rqApr").onclick = async () => {
    try {
      await api("/api/approvals", { method: "POST", body: JSON.stringify({ request_id: r.id }) });
      toast("Отправлено на согласование");
      if (has("approvals")) { aprTab = "queue"; openSection("approvals"); }
      else renderRequests();
    } catch (e) { toast(e.message); }
  };
  $("rqPay").onclick = () => { reqPane = "pay"; renderRequests(); };
  $("rqDocs").onclick = () => { reqPane = "docs"; renderRequests(); };
  $("rqTask").onclick = () => {
    taskForm = true;
    taskDraft = { requestId: r.id, title: "", text: "" };
    buzz();
    openSection("tasks");
  };
  $("rqAppeal").onclick = () => { reqPane = "appeal"; renderRequests(); };
  bindArchiveButtons();
  bindStatus("/api/requests");
  bindDispute();
  bindUndo();
}

function renderPayPane(r) {
  page("Оплата · " + (r.title || ""), `
    <div class="card">
      <p class="hint">Вся информация заявки уже здесь. Напишите, что нужно отправить — карточка придёт в раздел «Отправка».</p>
      ${reqPeople(r)}
      <div class="meta">${esc(r.uid)}</div>
    </div>
    <div class="card">
      <textarea id="payText" rows="5" placeholder="Что отправить: реквизиты, сумма, банк, комментарий"></textarea>
      <button class="btn primary" id="payGo">Отправить в отправку</button>
    </div>
    ${asList(r.payments).map((p) => `
      <div class="card">
        <h3>${p.status === "sent" ? "Отправлено" : "Ждёт отправки"}</h3>
        <p>${esc(p.text)}</p>
        <div class="meta">${esc(p.created_name)} · ${esc(fmtWhen(p.created_at))}${p.sent_name ? " · " + esc(p.sent_name) : ""}</div>
        ${payUndoRow(p)}
      </div>`).join("")}
  `);
  $("payGo").onclick = async () => {
    try {
      await api("/api/payments", { method: "POST", body: JSON.stringify({ request_id: r.id, text: $("payText").value }) });
      toast("Ушло в Отправку");
      reqPane = "";
      renderRequests();
    } catch (e) { toast(e.message); }
  };
  bindUndo();
}

async function renderDocsPane(r) {
  const files = asList(r.files);
  const docs = files.filter((f) => f.kind === "file");
  const msgs = files.filter((f) => f.kind !== "file");
  const own = files.filter(canDelFile);
  let att = {};
  try { att = await api("/api/attach"); } catch (_) {}
  const waiting = !!(att && att.waiting && Number(att.request_id) === Number(r.id));
  const botName = (att && att.bot_username) || (me && me.bot_username) || "";
  const fileCards = docs.map((f) => `
      <div class="card">
        ${isImageFile(f) ? `<img class="doc-preview" alt="" data-preview="${f.id}">` : ""}
        <h3>${esc(f.name || "Файл")}</h3>
        ${f.text ? `<p>${esc(f.text)}</p>` : ""}
        <div class="meta">${esc(f.created_name)} · ${esc(fmtWhen(f.created_at))}${f.size ? " · " + fmtSize(f.size) : ""}</div>
        <div class="row wrap">
          <button class="btn" data-dl="${f.id}" data-name="${esc(f.name)}">Скачать</button>
          <button type="button" class="btn" data-togs="${f.id}">Себе в личку</button>
          ${canDelFile(f) ? `<button type="button" class="btn bad" data-delfile="${f.id}">Удалить этот файл</button>` : ""}
        </div>
      </div>`).join("");
  const msgCards = msgs.map((f) => `
      <div class="card msg-card">
        <div class="meta">${esc(f.created_name)} · ${esc(fmtWhen(f.created_at))}</div>
        <p>${esc(f.text || "")}</p>
        <div class="row wrap">
          <button type="button" class="btn" data-copymsg="${f.id}">Копировать</button>
          <button type="button" class="btn" data-togs="${f.id}">Себе в личку</button>
          ${canDelFile(f) ? `<button type="button" class="btn bad" data-delfile="${f.id}">Удалить это сообщение</button>` : ""}
        </div>
      </div>`).join("");
  page("Файлы сделки · " + (r.title || ""), `
    <p class="hint">Файлы и сообщения сюда только через бота в личке. Каждое своё можно убрать отдельно — у всех пропадёт. Чужое не трогается.</p>
    <div class="card">
      ${reqPeople(r)}
      <div class="meta">${esc(r.uid)} · файлов ${docs.length} · сообщений ${msgs.length}</div>
    </div>
    ${docs.length ? `<h3>Файлы</h3>` : ""}${fileCards}
    ${msgs.length ? `<h3>Сообщения</h3>` : ""}${msgCards}
    ${!docs.length && !msgs.length ? `<p class="empty">Пока пусто — перешлите из Telegram</p>` : ""}
    <div class="card">
      <h3>Переслать из Telegram</h3>
      <p class="hint">Нажмите кнопку — откроется личка с ботом. Перешлите туда файл или сообщение. Можно несколько подряд. Они сами лягут в эту сделку.</p>
      ${waiting ? `<p class="hint">Сейчас жду пересылку${botName ? " @" + esc(botName) : ""}. Кинули — появятся здесь, можно не закрывать Mini App.</p>` : ""}
      <button class="btn primary" id="docTg">${waiting ? "Открыть личку бота" : "Переслать из Telegram"}</button>
      ${waiting ? `<button class="btn" id="docTgStop">Больше не ждать</button>` : ""}
      ${own.length ? `<button class="btn bad" id="docClearMine">Убрать отправленное</button>` : ""}
      ${own.length ? `<button class="btn" id="docUndo">Отменить последнее</button>` : ""}
    </div>
    <div class="card">
      <h3>Текст без файла</h3>
      <textarea id="docNote" rows="4" placeholder="Вставить текст или комментарий по сделке"></textarea>
      <button class="btn primary" id="docMsgBtn">Сохранить в сделку</button>
    </div>
  `);
  const goTg = async () => {
    try {
      const out = await api("/api/attach", { method: "POST", body: JSON.stringify({ request_id: r.id }) });
      toast("Перешлите файлы боту в личку");
      openBotChat((out && out.bot_username) || botName);
      startDocsPoll(files.length);
    } catch (e) { toast(e.message); }
  };
  $("docTg").onclick = goTg;
  const stopBtn = $("docTgStop");
  if (stopBtn) stopBtn.onclick = async () => {
    try {
      await api("/api/attach", { method: "POST", body: JSON.stringify({ cancel: true }) });
      stopDocsPoll();
      toast("Больше не жду");
      renderRequests();
    } catch (e) { toast(e.message); }
  };
  const clearBtn = $("docClearMine");
  if (clearBtn) clearBtn.onclick = async () => {
    const ok = await ask("Убрать из этой заявки всё, что отправили вы? Чужое останется.");
    if (!ok) return;
    try {
      const out = await api("/api/attach", { method: "POST", body: JSON.stringify({ request_id: r.id, clear_mine: true }) });
      toast(out && out.cleared ? ("Убрали отправленное: " + out.cleared) : "Убрано");
      renderRequests();
    } catch (e) { toast(e.message); }
  };
  const undoBtn = $("docUndo");
  if (undoBtn) undoBtn.onclick = async () => {
    const ok = await ask("Убрать из заявки то, что вы отправили последним?");
    if (!ok) return;
    try {
      const out = await api("/api/attach", { method: "POST", body: JSON.stringify({ undo: true }) });
      toast(out && out.undone ? ("Убрали: " + out.undone) : "Отменено");
      renderRequests();
    } catch (e) { toast(e.message); }
  };
  $("docMsgBtn").onclick = async () => {
    try {
      await api("/api/deal-files", { method: "POST", body: JSON.stringify({ request_id: r.id, text: $("docNote").value, kind: "message" }) });
      toast("Сообщение в сделке");
      renderRequests();
    } catch (e) { toast(e.message); }
  };
  app.querySelectorAll("[data-dl]").forEach((b) => b.onclick = async (e) => {
    e.preventDefault();
    e.stopPropagation();
    try { await downloadDeal(Number(b.dataset.dl), b.dataset.name); } catch (err) { toast(err.message); }
  });
  app.querySelectorAll("[data-togs]").forEach((b) => b.onclick = async () => {
    try {
      await api("/api/deal-files", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.togs), send_self: true }) });
      toast("Отправил в личку бота");
      openBotChat(botName);
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-copymsg]").forEach((b) => b.onclick = async () => {
    const rec = msgs.find((x) => String(x.id) === String(b.dataset.copymsg));
    const text = rec && rec.text ? rec.text : "";
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) await navigator.clipboard.writeText(text);
      toast("Скопировано");
    } catch (_) { toast("Не удалось скопировать"); }
  });
  bindUndo();
  fillFilePreviews();
  if (waiting) startDocsPoll(files.length);
}

async function renderTasks() {
  const token = bumpNav();
  currentSection = "tasks";
  const [list, dirs] = await Promise.all([
    api("/api/tasks"),
    api("/api/directory").catch(() => ({}))
  ]);
  let reqs = [];
  try { reqs = asList(await api("/api/requests")); } catch (_) {}
  if (!alive(token)) return;
  const tasks = asList(list);
  const employees = asList(dirs.employees);
  const managers = asList(dirs.managers);
  if (taskForm) {
    page("Новая задача", `
      <p class="hint">Номер сам: две буквы + номер сотрудника + месяц без нуля (1,2,3…10) + порядковый без нуля. Пример ЖЖ01552. Если привязать заявку — аббревиатура берётся с неё.</p>
      <div class="card">
        <input id="taskTitle" placeholder="Комментарий (необязательно)" value="${esc(taskDraft.title || "")}">
        <textarea id="taskText" rows="4" placeholder="Что нужно сделать">${esc(taskDraft.text || "")}</textarea>
        <select id="taskReq"><option value="">Без заявки</option>${asList(reqs).map((r) => `<option value="${r.id}" ${Number(taskDraft.requestId) === Number(r.id) ? "selected" : ""}>${esc(r.uid || r.title)}</option>`).join("")}</select>
        <select id="taskMgr"><option value="">Менеджер (если без заявки)</option>${optList(managers, (m) => m.abbrev || "")}</select>
        <select id="taskEmp"><option value="">Исполнитель</option>${optList(employees, (e) => (e.number ? "№ " + e.number : "") || e.title || "")}</select>
        <button class="btn primary" id="taskSave">Поставить в воронку</button>
      </div>
    `);
    $("taskSave").onclick = async () => {
      try {
        const out = await api("/api/tasks", { method: "POST", body: JSON.stringify({
          title: $("taskTitle").value,
          text: $("taskText").value,
          request_id: Number($("taskReq").value) || 0,
          manager_id: Number($("taskMgr").value) || 0,
          employee_id: Number($("taskEmp").value) || 0,
          status: "new"
        }) });
        toast(out && out.uid ? ("Задача " + out.uid) : "Задача в воронке");
        taskForm = false;
        taskDraft = { requestId: 0, title: "", text: "" };
        renderTasks();
      } catch (e) { toast(e.message); }
    };
    return;
  }
  const by = { new: [], in_progress: [], docs: [], done: [] };
  tasks.forEach((t) => {
    const k = t.status === "in_progress" || t.status === "docs" || t.status === "done" ? t.status : "new";
    by[k].push(t);
  });
  const cols = TASK_COLS.map((c) => {
    const items = by[c.key] || [];
    return `<div class="kanban-col">
      <h3>${esc(c.title)} <span class="badge">${items.length}</span></h3>
      <div class="meta">${esc(c.hint)}</div>
      ${items.map((t) => taskCardHTML(t)).join("") || `<p class="empty">Пусто</p>`}
    </div>`;
  }).join("");
  page("Канбан", `
    <p class="hint">Это воронка задач: Новые → В работе → Документы → Готово. Колонка «Документы» — этап, когда ждут бумаги. Сами файлы сделки: Заявки → Файлы на карточке.</p>
    <button class="btn primary" id="taskNew">Ещё задача</button>
    <div class="kanban">${cols}</div>
  `);
  $("taskNew").onclick = () => { taskForm = true; taskDraft = { requestId: 0, title: "", text: "" }; renderTasks(); };
  app.querySelectorAll("[data-opentaskreq]").forEach((b) => b.onclick = () => {
    reqOpen = Number(b.dataset.opentaskreq);
    reqPane = "";
    buzz();
    openSection("requests");
  });
  app.querySelectorAll("[data-opentaskdocs]").forEach((b) => b.onclick = () => {
    reqOpen = Number(b.dataset.opentaskdocs);
    reqPane = "docs";
    buzz();
    openSection("requests");
  });
  bindUndo();
  paintDock();
}

function taskCardHTML(t) {
  const st = t.status === "in_progress" || t.status === "docs" || t.status === "done" ? t.status : "new";
  const chips = TASK_COLS.map((c) => `<button type="button" class="chip ${c.key === st ? "on" : ""}" data-movetask="${t.id}" data-v="${c.key}">${esc(c.title)}</button>`).join("");
  const who = [t.employee_name, t.client_name, t.manager_name].filter(Boolean).join(" · ");
  return `<div class="task-card">
    <h3>${esc(t.uid || t.title)}</h3>
    ${t.title && t.title !== t.uid ? `<p>${esc(t.title)}</p>` : ""}
    ${t.text ? `<p>${esc(t.text)}</p>` : ""}
    <div class="meta">${t.request_uid || t.request_title ? esc(t.request_uid || t.request_title) : "без заявки"}${who ? " · " + esc(who) : ""}</div>
    <div class="meta">${esc(t.created_name || "")} · ${esc(fmtWhen(t.moved_at || t.created_at))}</div>
    <div class="chips">${chips}</div>
    <div class="row wrap">
      ${t.request_id ? `<button type="button" class="btn" data-opentaskreq="${t.request_id}">Заявка</button>` : ""}
      ${t.request_id ? `<button type="button" class="btn primary" data-opentaskdocs="${t.request_id}">Файлы сделки</button>` : ""}
      ${isMine(t.created_by) ? `<button type="button" class="btn bad" data-deltask="${t.id}">Удалить</button>` : ""}
    </div>
  </div>`;
}

function renderAppealPane(r) {
  if (reqPane === "appeal") {
    page("Обращение · " + (r.title || ""), `
      <p class="hint">Заявка уже выбрана. Обращение уйдёт в рабочий чат с контекстом сделки.</p>
      <div class="kb">
        <button type="button" class="kb-btn wide" id="apDocs"><b>Документалист</b><span class="meta">Чат ВЭД документалисты</span></button>
        <button type="button" class="kb-btn wide" id="apLaw"><b>Юрист</b><span class="meta">Чат ВЭД юрист</span></button>
      </div>
      ${asList(r.appeals).map((a) => `
        <div class="card">
          <h3>${a.kind === "lawyer" ? "Юрист" : "Документалист"} <span class="badge">${esc(a.status)}</span></h3>
          <p>${esc(a.text)}</p>
          <div class="meta">${esc(a.created_name)} · ${esc(fmtWhen(a.created_at))}${a.error ? " · " + esc(a.error) : ""}</div>
          ${a.status !== "cancelled" && isMine(a.created_by) ? `<button type="button" class="btn bad" data-delap="${a.id}">Удалить</button>` : ""}
        </div>`).join("")}
    `);
    $("apDocs").onclick = () => { reqPane = "appeal_docs"; renderRequests(); };
    $("apLaw").onclick = () => { reqPane = "appeal_lawyer"; renderRequests(); };
    bindUndo();
    return;
  }
  const lawyer = reqPane === "appeal_lawyer";
  page((lawyer ? "Юрист" : "Документалист") + " · " + (r.title || ""), `
    <div class="card">
      ${reqPeople(r)}
      <textarea id="apText" rows="5" placeholder="Текст обращения"></textarea>
      <button class="btn primary" id="apGo">Отправить в чат</button>
    </div>
  `);
  $("apGo").onclick = async () => {
    try {
      await api("/api/appeals", { method: "POST", body: JSON.stringify({ request_id: r.id, kind: lawyer ? "lawyer" : "docs", text: $("apText").value }) });
      toast("Отправлено в чат");
      reqPane = "appeal";
      renderRequests();
    } catch (e) { toast(e.message); }
  };
}

function appealStatus(a) {
  if (a.status === "taken") return "в работе";
  if (a.status === "closed") return "закрыта";
  if (a.status === "cancelled") return "снята";
  return "новая";
}
async function renderAppealKind(kind) {
  const token = bumpNav();
  const lawyer = kind === "lawyer";
  const title = lawyer ? "Обратиться к юристу" : "Обратиться к документалисту";
  currentSection = lawyer ? "appeal_lawyer" : "appeal_docs";
  let pack = { items: [], stats: {} };
  let reqs = [];
  try { pack = await api("/api/appeals"); } catch (e) {
    page(title, `<p class="err">${esc(e.message)}</p>`, token);
    return;
  }
  if (has("requests") || isAdmin()) {
    try { reqs = asList(await api("/api/requests")); } catch (_) {}
  }
  if (!alive(token)) return;
  const items = asList(pack.items).filter((a) => lawyer ? a.kind === "lawyer" : a.kind !== "lawyer");
  const st = pack.stats || {};
  const canCreate = has("appeals") || has("requests") || isAdmin();
  const inbox = isAdmin() || (lawyer ? has("appeal_lawyer") : has("appeal_docs"));
  const today = lawyer ? (st.lawyer_today || 0) : (st.docs_today || 0);
  const openN = lawyer ? (st.lawyer_open || 0) : (st.docs_open || 0);
  const takenN = lawyer ? (st.lawyer_taken || 0) : (st.docs_taken || 0);
  const chatID = lawyer ? pack.lawyer_chat_id : pack.docs_chat_id;
  const canEdit = !!(pack.can_edit_chats || canManageChats());
  const chatName = lawyer ? "ВЭД ЮРИСТ" : "документалистов";
  const chats = asList(pack.chats);
  const selectedChat = chats.find((c) => Number(c.chat_id) === Number(chatID));
  const chatLabel = selectedChat ? selectedChat.name : (chatID ? String(chatID) : "не задан");
  const showOpenChats = canManageChats();
  const card = (a) => {
    const open = a.status !== "taken" && a.status !== "closed" && a.status !== "cancelled";
    return `<div class="card">
      <h3>${esc(a.uid || "")} <span class="badge">${esc(appealStatus(a))}</span></h3>
      <p>${esc(a.text)}</p>
      <div class="meta">${esc(a.created_name)} · ${esc(fmtWhen(a.created_at))}${a.request_uid ? " · " + esc(a.request_uid) : ""}${a.taken_name ? " · взял " + esc(a.taken_name) : ""}${a.closed_name ? " · закрыл " + esc(a.closed_name) : ""}${a.error ? " · не ушло в чат: " + esc(a.error) : ""}</div>
      <div class="row wrap">
        ${open && inbox ? `<button type="button" class="btn primary" data-aptake="${a.id}">Взять в работу</button>` : ""}
        ${(open || a.status === "taken") && inbox ? `<button type="button" class="btn ok" data-apclose="${a.id}">Закрыть заявку</button>` : ""}
        ${a.status !== "closed" && a.status !== "cancelled" && isMine(a.created_by) ? `<button type="button" class="btn bad" data-delap="${a.id}">Снять</button>` : ""}
      </div>
    </div>`;
  };
  page(title, `
    <p class="hint">Заявка уходит в Telegram-чат ${esc(chatName)}. Там берут в работу, отвечают реплаем автору и жмут «Закрыть». Сколько закрыли за день — видно здесь.</p>
    <div class="kpis">
      <div class="kpi sand"><div class="lbl">Закрыто сегодня</div><div class="num">${today}</div></div>
      <div class="kpi blush"><div class="lbl">В очереди</div><div class="num">${openN}</div></div>
      <div class="kpi mint"><div class="lbl">В работе</div><div class="num">${takenN}</div></div>
    </div>
    ${canEdit ? `<div class="card">
      <h3>Куда слать</h3>
      <p class="meta">Выберите группу из карточки «Чаты». Сейчас: ${esc(chatLabel)}.</p>
      <select id="apChatSel">
        <option value="">Не выбран</option>
        ${chats.map((c) => `<option value="${c.chat_id}" ${Number(c.chat_id) === Number(chatID) ? "selected" : ""}>${esc(c.name)} · ${esc(c.chat_id)}</option>`).join("")}
      </select>
      <div class="row wrap">
        <button type="button" class="btn primary" id="apChatSave">Сохранить</button>
        ${showOpenChats ? `<button type="button" class="btn" id="apChatGo">Открыть Чаты</button>` : ""}
      </div>
      ${!chats.length ? `<p class="hint">Список пуст — сначала добавьте чат в карточке «Чаты».</p>` : ""}
    </div>` : `<p class="meta">${chatID ? "Чат задан, заявки уходят в Telegram." : "Чат ещё не задан — админ выберет группу в карточке Чаты."}</p>`}
    ${canCreate ? `<div class="card">
      <h3>Новое обращение</h3>
      <select id="apReq"><option value="">Без заявки</option>${reqs.map((r) => `<option value="${r.id}">${esc(r.uid || r.title)}</option>`).join("")}</select>
      <textarea id="apText" rows="4" placeholder="Что нужно"></textarea>
      <button type="button" class="btn primary" id="apCreate">Отправить в чат</button>
    </div>` : ""}
    ${items.map(card).join("") || `<p class="empty">Пока нет обращений</p>`}
  `, token);
  const saveChat = $("apChatSave");
  if (saveChat) saveChat.onclick = async () => {
    try {
      await api("/api/appeals", { method: "POST", body: JSON.stringify({
        set_chat: true, kind: lawyer ? "lawyer" : "docs", chat_id: Number($("apChatSel").value) || 0
      }) });
      toast("Чат сохранён");
      renderAppealKind(kind);
    } catch (e) { toast(e.message); }
  };
  const goChats = $("apChatGo");
  if (goChats) goChats.onclick = () => { buzz(); openSection("chats"); };
  const go = $("apCreate");
  if (go) go.onclick = async () => {
    try {
      await api("/api/appeals", { method: "POST", body: JSON.stringify({
        kind: lawyer ? "lawyer" : "docs",
        request_id: Number($("apReq").value) || 0,
        text: $("apText").value
      }) });
      toast(chatID ? "Ушло в чат" : "Записано. Чат ещё не задан");
      renderAppealKind(kind);
    } catch (e) { toast(e.message); }
  };
  app.querySelectorAll("[data-aptake]").forEach((b) => {
    b.onclick = async () => {
      try {
        await api("/api/appeals", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.aptake), status: "taken" }) });
        toast("В работе");
        renderAppealKind(kind);
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-apclose]").forEach((b) => {
    b.onclick = async () => {
      try {
        await api("/api/appeals", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.apclose), status: "closed" }) });
        toast("Закрыто");
        renderAppealKind(kind);
      } catch (e) { toast(e.message); }
    };
  });
  bindUndo();
}

async function renderPayments() {
  const token = bumpNav();
  const list = asList(await api("/api/payments"));
  if (!alive(token)) return;
  const pending = list.filter((p) => p.status !== "sent");
  const done = list.filter((p) => p.status === "sent");
  const card = (p) => `
    <div class="card">
      <h3>${esc(p.request_title || ("заявка #" + p.request_id))} ${payBadge(p.status)}</h3>
      <pre class="report">${esc(p.context || "")}</pre>
      <p><b>Что отправить</b><br>${esc(p.text)}</p>
      <div class="meta">${esc(p.created_name)} · ${esc(fmtWhen(p.created_at))}${p.sent_name ? " · " + esc(p.sent_name) + " " + esc(fmtWhen(p.sent_at)) : ""}</div>
      ${p.status !== "sent" && canPay() ? `<button class="btn ok" data-sent="${p.id}">Отправлено</button>` : ""}
      ${payUndoRow(p)}
    </div>`;
  page("Отправка", `
    <p class="hint">Сюда падает оплата с заявки. Смотрите контекст, отправляете, жмёте «Отправлено».</p>
    <h3>Ждут</h3>
    ${pending.map(card).join("") || `<p class="empty">Пусто</p>`}
    ${done.length ? `<h3>Отправленные</h3>${done.map(card).join("")}` : ""}
  `);
  app.querySelectorAll("[data-sent]").forEach((b) => b.onclick = async () => {
    try {
      await api("/api/payments", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.sent), status: "sent" }) });
      toast("Отмечено");
      renderPayments();
    } catch (e) { toast(e.message); }
  });
  bindUndo();
}

async function renderApprovals() {
  const token = bumpNav();
  const admin = isAdmin();
  await refreshMe();
  if (!alive(token)) return;
  const u = unread();
  const menu = `<div class="kb">
    <button type="button" class="kb-btn ${aprTab === "queue" ? "on" : ""}" data-atab="queue">Очередь${ntf(u.approvals)}</button>
    <button type="button" class="kb-btn ${aprTab === "disputes" ? "on" : ""}" data-atab="disputes">Споры${ntf(u.disputes)}</button>
  </div>`;
  const bindTabs = () => {
    app.querySelectorAll("[data-atab]").forEach((b) => {
      b.onclick = () => { aprTab = b.dataset.atab; aprDspId = 0; renderApprovals(); };
    });
  };
  if (aprTab === "disputes") {
    const disputes = asList(await api("/api/disputes"));
    if (!alive(token)) return;
    if (aprDspId) {
      const d = disputes.find((x) => x.id === aprDspId);
      if (!d) { aprDspId = 0; return renderApprovals(); }
      await api("/api/inbox", { method: "POST", body: JSON.stringify({ seen: "dsp", id: d.id }) });
      if (!alive(token)) return;
      const msgs = asList(d.messages);
      page("Согласование", menu + `
        <button type="button" class="btn" data-atab="disputes">← К спорам</button>
        <div class="card">
          <h3>${esc(d.ref_uid || d.section)} <span class="badge">${esc(d.status)}</span></h3>
          <div class="meta">${esc(d.uid)} · ${esc(d.target_name || "")}</div>
        </div>
        ${msgs.map((m) => `
          <div class="msg ${m.by === me.id ? "mine" : ""}">
            <b>${esc(m.by_name || m.by)}</b>
            <div>${esc(m.text)}</div>
            <div class="meta">${esc(fmtWhen(m.at))}</div>
          </div>`).join("")}
        ${d.status === "open" ? `<div class="card">
          <textarea id="dspReply" rows="3" placeholder="Ответ по спору"></textarea>
          <button class="btn primary" id="dspSend">Ответить</button>
          ${admin ? `<button class="btn" data-dst="${d.id}">Закрыть спор</button>` : ""}
        </div>` : `<div class="card">
          <p class="meta">Спор закрыт.</p>
          <button class="btn" data-hide="dsp:${d.id}">Убрать у себя</button>
        </div>`}
      `);
      bindTabs();
      const send = $("dspSend");
      if (send) send.onclick = async () => {
        try {
          await api("/api/disputes", { method: "POST", body: JSON.stringify({ id: d.id, reply: $("dspReply").value }) });
          toast("Ответ отправлен");
          renderApprovals();
        } catch (e) { alert(e.message); }
      };
      app.querySelectorAll("[data-dst]").forEach((b) => b.onclick = async () => {
        await api("/api/disputes", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.dst), status: "closed" }) });
        renderApprovals();
      });
      app.querySelectorAll("[data-hide]").forEach((b) => b.onclick = async () => {
        await api("/api/inbox", { method: "POST", body: JSON.stringify({ hide: b.dataset.hide }) });
        aprDspId = 0;
        renderApprovals();
      });
      return;
    }
    page("Согласование", menu + `
      <button class="btn" id="aprHide">Убрать закрытые у себя</button>
      ${disputes.map((d) => {
        const msgs = asList(d.messages);
        const last = msgs[msgs.length - 1];
        const fresh = last && last.by !== me.id;
        return `<div class="card" style="position:relative">
          ${fresh ? ntf(1) : ""}
          <h3>${esc(d.ref_uid || d.section)} <span class="badge">${esc(d.status)}</span></h3>
          <p>${esc((last && last.text) || d.text)}</p>
          <div class="meta">${esc(d.uid)} · ${esc(d.target_name || d.created_name || "")}</div>
          <div class="row">
            <button class="btn primary" data-open="${d.id}">${d.status === "open" ? "Ответить" : "Открыть"}</button>
            ${d.status === "closed" ? `<button class="btn" data-hide="dsp:${d.id}">Убрать у себя</button>` : ""}
            ${d.status === "open" && admin ? `<button class="btn" data-dst="${d.id}">Закрыть</button>` : ""}
          </div>
        </div>`;
      }).join("") || `<p class="empty">Споров нет</p>`}
    `);
    bindTabs();
    $("aprHide").onclick = async () => {
      await api("/api/inbox", { method: "POST", body: JSON.stringify({ hide_closed: "disputes" }) });
      renderApprovals();
    };
    app.querySelectorAll("[data-open]").forEach((b) => { b.onclick = () => { aprDspId = Number(b.dataset.open); renderApprovals(); }; });
    app.querySelectorAll("[data-dst]").forEach((b) => b.onclick = async () => {
      await api("/api/disputes", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.dst), status: "closed" }) });
      renderApprovals();
    });
    app.querySelectorAll("[data-hide]").forEach((b) => b.onclick = async () => {
      await api("/api/inbox", { method: "POST", body: JSON.stringify({ hide: b.dataset.hide }) });
      renderApprovals();
    });
    return;
  }
  await api("/api/inbox", { method: "POST", body: JSON.stringify({ seen: "approvals" }) });
  if (!alive(token)) return;
  const list = asList(await api("/api/approvals"));
  if (!alive(token)) return;
  const form = showAprForm ? `
    <div class="card">
      <textarea id="aprText" rows="3" placeholder="Текст на согласование"></textarea>
      <input id="aprReq" placeholder="Номер заявки (необязательно)">
      <div class="row">
        <button class="btn primary" id="aprAdd">Отправить</button>
        <button class="btn" id="aprFold">Скрыть</button>
      </div>
    </div>` : `<button class="btn primary fold" id="aprOpen">＋ Новое согласование</button>`;
  page("Согласование", menu + form + `
    <button class="btn" id="aprHide">Убрать закрытые у себя</button>
    ${list.map((a) => `
      <div class="card">
        <h3>${esc(a.preview)} <span class="badge">${esc(a.status)}</span></h3>
        <div class="meta"><button class="copy" data-copy="${esc(a.uid)}">${esc(a.uid)}</button> · ${esc(a.manager_name)} · ${esc(fmtWhen(a.created_at))}</div>
        ${a.status === "pending" && admin ? `<div class="row">
          <button class="btn ok" data-st="${a.id}" data-v="approved">Принять</button>
          <button class="btn bad" data-st="${a.id}" data-v="rejected">Отклонить</button>
        </div>` : `<p class="meta">${esc(a.decided_by || (admin ? "" : "Ждёт администратора"))}</p>`}
        ${admin ? `<button class="btn" data-dsp="approvals" data-uid="${esc(a.uid)}">Спор</button>` : ""}
        ${aprUndoRow(a)}
        ${a.status !== "pending" ? `<button class="btn" data-hide="apr:${a.id}">Убрать у себя</button>` : ""}
      </div>`).join("") || `<p class="empty">Очередь пуста</p>`}
  `);
  const openF = $("aprOpen");
  if (openF) openF.onclick = () => { showAprForm = true; renderApprovals(); };
  const fold = $("aprFold");
  if (fold) fold.onclick = () => { showAprForm = false; renderApprovals(); };
  if ($("aprAdd")) $("aprAdd").onclick = async () => {
    await api("/api/approvals", { method: "POST", body: JSON.stringify({ preview: $("aprText").value, request_uid: $("aprReq").value }) });
    showAprForm = false;
    toast("Отправлено");
    renderApprovals();
  };
  $("aprHide").onclick = async () => {
    await api("/api/inbox", { method: "POST", body: JSON.stringify({ hide_closed: "approvals" }) });
    renderApprovals();
  };
  bindStatus("/api/approvals");
  bindDispute();
  bindUndo();
  bindTabs();
  app.querySelectorAll("[data-hide]").forEach((b) => b.onclick = async () => {
    await api("/api/inbox", { method: "POST", body: JSON.stringify({ hide: b.dataset.hide }) });
    renderApprovals();
  });
}

async function renderSaldo() {
  const token = bumpNav();
  const data = await api("/api/saldo");
  if (!alive(token)) return;
  const cps = sortCPs(asList(data.counterparties));
  const curs = asList(data.currencies);
  const lots = data.lots || [];
  const back = saldoTab === "menu" ? "" : `<button type="button" class="btn" data-stab="menu">← Меню сальдо</button>`;
  let body = "";
  if (saldoTab === "edit") saldoTab = "menu";
  if (saldoTab === "menu") {
    body = `<div class="kb">
      <button type="button" class="kb-btn" data-stab="mine">📌 Мои заявки</button>
      <button type="button" class="kb-btn" data-stab="op">📒 Создать заявку</button>
      <button type="button" class="kb-btn" data-stab="rep">📤 Выгрузка</button>
      <button type="button" class="kb-btn" data-stab="cp">📋 По контрагенту</button>
      <button type="button" class="kb-btn" data-stab="files">📊 Файлы</button>
    </div>`;
  } else if (saldoTab === "op") {
    const d = saldoDraft;
    body = `<div class="card">
      ${crumbs(d)}
      ${d.requestId ? `<p class="hint">Контекст заявки уже выбран. Повторно вводить сотрудника, клиента, менеджера и контрагента не нужно — backend берёт ID.</p>` : ""}
      ${d.cp ? "" : `<h3>Контрагент</h3>
        ${cps.length ? `<input id="sFind" class="search" placeholder="Найти контрагента" autocomplete="off">` + kb(cps, d.cp, "cp") : `<p class="empty">Список пуст. Админ добавляет в Справочник → Контрагенты.</p>`}`}
      ${d.cp && !d.kind ? `<h3>Тип</h3>${kb([{ id: "swift", name: "SWIFT" }, { id: "nerez", name: "Нерезидентский рубль" }], d.kind, "kind")}` : ""}
      ${d.kind && !d.action ? `<h3>Действие</h3>${kb([{ id: "plus", name: "➕ Поставил (+)" }, { id: "minus", name: "➖ Закрыл (−)" }], d.action, "act")}` : ""}
      ${d.action && !d.currency ? `<h3>Валюта</h3>${kb(curs, d.currency, "cur", 3)}` : ""}
      ${d.currency ? `<h3>Сумма</h3>
        <input id="sAmt" type="number" inputmode="decimal" placeholder="Сумма">
        <div class="row"><button class="btn primary" id="sGo">Записать</button><button class="btn" id="sReset">Сначала</button></div>` : ""}
    </div>`;
  } else if (saldoTab === "mine") {
    body = lots.length ? lots.map((l) => `
      <div class="card">
        <h3>${esc(l.cp)}</h3>
        <div class="amt">${l.remaining} ${esc(l.currency)}</div>
        <div class="meta">${esc(l.request_title || "без заявки")} · ${esc(kindName(l.kind))} · ${esc(l.uid)} · ${esc(l.manager)}</div>
        <button class="btn bad" data-close="${l.op_id}">Закрыть</button>
      </div>`).join("") : `<p class="empty">Открытых постановок нет</p>`;
  } else if (saldoTab === "rep") {
    const lines = asList(data.report_lines).filter((r) => Number(r.amount) !== 0);
    body = `<div class="card"><pre class="report">${esc(data.report || "Пусто")}</pre></div>` +
      (lines.length ? lines.map((r) => `
        <div class="card">
          <h3>${esc(r.cp)}</h3>
          <div class="meta">${esc(r.request || "без заявки")} · ${esc(r.kind)} · ${esc(r.currency)}</div>
          <div class="amt">${r.amount}</div>
        </div>`).join("") : (Object.entries(data.totals || {}).filter(([, v]) => Number(v) !== 0).map(([k, v]) => {
        const [cp, kind, cur] = k.split("|");
        return `<div class="card"><h3>${esc(cp)}</h3><div class="meta">${esc(kindName(kind))} · ${esc(cur)}</div><div class="amt">${v}</div></div>`;
      }).join("")));
  } else if (saldoTab === "cp") {
    body = `<div class="card"><h3>Контрагент</h3>
      <input id="sFind" class="search" placeholder="Найти контрагента" autocomplete="off">
      ${kb(cps, saldoDraft.cp, "dcp")}</div>
      <div class="card" id="cpDetail"><p class="meta">Выберите контрагента</p></div>`;
  } else {
    body = `<div class="card">
      <p class="meta">Файлы придут вам в личку с ботом Mini App, не скачиваются здесь.</p>
      <button class="btn primary" id="sSendFiles">Отправить файлы в личку</button>
      <p class="meta" id="sFilesMsg"></p>
    </div>`;
  }
  page("Сальдо", back + body, token);
  if (!alive(token)) return;
  bindSearch("sFind");
  app.querySelectorAll("[data-stab]").forEach((b) => { b.onclick = () => { const next = b.dataset.stab; if (next === "op" && saldoTab !== "op") saldoDraft = emptyDraft(); saldoTab = next; renderSaldo(); }; });
  app.querySelectorAll("[data-step]").forEach((b) => {
    b.onclick = () => {
      const s = b.dataset.step;
      if (s === "cp") {
        if (saldoDraft.requestId) return;
        saldoDraft = Object.assign(emptyDraft(), { requestId: saldoDraft.requestId, reqTitle: saldoDraft.reqTitle, counterpartyId: 0 });
      }
      else if (s === "kind") { saldoDraft.kind = ""; saldoDraft.action = ""; saldoDraft.currency = ""; }
      else if (s === "act") { saldoDraft.action = ""; saldoDraft.currency = ""; }
      else if (s === "cur") saldoDraft.currency = "";
      renderSaldo();
    };
  });
  app.querySelectorAll("[data-cp]").forEach((b) => { b.onclick = () => { saldoDraft = Object.assign(emptyDraft(), { cp: b.dataset.cp, requestId: saldoDraft.requestId, reqTitle: saldoDraft.reqTitle, counterpartyId: saldoDraft.counterpartyId }); pushRecent(b.dataset.cp); renderSaldo(); }; });
  app.querySelectorAll("[data-kind]").forEach((b) => { b.onclick = () => { saldoDraft.kind = b.dataset.kind; saldoDraft.action = ""; saldoDraft.currency = ""; renderSaldo(); }; });
  app.querySelectorAll("[data-act]").forEach((b) => { b.onclick = () => { saldoDraft.action = b.dataset.act; saldoDraft.currency = ""; renderSaldo(); }; });
  app.querySelectorAll("[data-cur]").forEach((b) => { b.onclick = () => { saldoDraft.currency = b.dataset.cur; renderSaldo(); }; });
  const go = $("sGo");
  if (go) go.onclick = async () => {
    try {
      await api("/api/saldo", { method: "POST", body: JSON.stringify({
        cp: saldoDraft.cp, kind: saldoDraft.kind, currency: saldoDraft.currency,
        amount: Number($("sAmt").value), action: saldoDraft.action,
        request_id: saldoDraft.requestId || 0,
        counterparty_id: saldoDraft.counterpartyId || 0
      })});
      pushRecent(saldoDraft.cp);
      toast("Записано");
      saldoDraft = emptyDraft();
      saldoTab = "mine";
      renderSaldo();
    } catch (e) { toast(e.message); }
  };
  const amt = $("sAmt");
  if (amt) {
    amt.focus();
    amt.onkeydown = (e) => { if (e.key === "Enter") go && go.click(); };
  }
  const rst = $("sReset");
  if (rst) rst.onclick = () => { saldoDraft = emptyDraft(); renderSaldo(); };
  app.querySelectorAll("[data-close]").forEach((b) => b.onclick = async () => {
    await api("/api/saldo", { method: "POST", body: JSON.stringify({ close_id: Number(b.dataset.close) }) });
    renderSaldo();
  });
  app.querySelectorAll("[data-dcp]").forEach((b) => b.onclick = async () => {
    saldoDraft.cp = b.dataset.dcp;
    const d = await api("/api/saldo?view=detail&cp=" + encodeURIComponent(saldoDraft.cp));
    const box = $("cpDetail");
    if (box) box.innerHTML = `<h3>${esc(saldoDraft.cp)}</h3><pre class="report">${esc(d.detail || "")}</pre>`;
    app.querySelectorAll("[data-dcp]").forEach((x) => x.classList.toggle("on", x.dataset.dcp === saldoDraft.cp));
  });
  const sendF = $("sSendFiles");
  if (sendF) sendF.onclick = async () => {
    try {
      await api("/api/saldo", { method: "POST", body: JSON.stringify({ send_files: true }) });
      const msg = $("sFilesMsg");
      if (msg) msg.textContent = "Файлы отправлены вам в личку с ботом.";
      toast("Файлы отправлены в личку");
    } catch (e) { alert(e.message); }
  };
}

async function renderRates() {
  const token = bumpNav();
  page("Курсы", `<p class="meta">Загрузка…</p>`, token);
  const s = await api("/api/rates");
  if (!alive(token)) return;
  const line = (label, v, err) => `<div class="card"><h3>${esc(label)}</h3><div class="amt">${err ? esc(err) : v}</div></div>`;
  page("Курсы", `<button class="btn fold" id="rReload">Обновить курсы</button>` + [
    line("Rapira USDT/RUB", s.rapira, s.rapira_err),
    line("ЦБ " + (s.cbr_date || ""), (s.cbr || []).map((x) => x.code + " " + x.per_unit).join(" · "), s.cbr_err),
    line("ProFinance руб", (s.rub || []).map((x) => x.pair + " " + x.bid).join(" · "), s.pf_err),
    line("ProFinance forex", (s.forex || []).map((x) => x.pair + " " + x.bid).join(" · "), s.pf_err),
    line("XE EUR/USD", s.xe_eurusd, s.xe_err),
    line("Investing USD/RUB", s.investing, s.investing_err),
  ].join(""), token);
  if (!alive(token)) return;
  $("rReload").onclick = () => renderRates();
}

async function renderHolidays() {
  const token = bumpNav();
  page("Праздники", `<p class="meta">Загрузка…</p>`, token);
  let list = [];
  try {
    list = asList(await api("/api/holidays"));
  } catch (e) {
    if (!alive(token)) return;
    page("Праздники", `<p class="err">${esc(e.message)}</p><button class="btn fold" id="hReload">Обновить</button>`, token);
    const again = $("hReload");
    if (again) again.onclick = () => renderHolidays();
    return;
  }
  if (!alive(token)) return;
  page("Праздники", `<button class="btn fold" id="hReload">Обновить</button>` + ((list || []).map((h) => `
    <div class="card">
      <h3>${esc(h.Name || h.name)}</h3>
      <div class="meta">${esc(h.CountryName || h.country_name)} · через ${h.days_left} дн.</div>
      <div class="meta">${esc(h.Date || h.date)} ${h.DateTo || h.date_to ? " — " + esc(h.DateTo || h.date_to) : ""}</div>
    </div>`).join("") || `<p class="empty">Нет ближайших</p>`), token);
  if (!alive(token)) return;
  const reload = $("hReload");
  if (reload) reload.onclick = () => renderHolidays();
}

async function renderBalance() {
  const token = bumpNav();
  const data = await api("/api/balance");
  if (!alive(token)) return;
  const names = {};
  (data.companies || []).forEach((c) => names[c.id] = c.name);
  const opts = (data.companies || []).map((c) => `<option value="${c.id}">${esc(c.name)}</option>`).join("");
  page("Баланс компаний", `
    <div class="card">
      <select id="bCo">${opts}</select>
      <input id="bBank" placeholder="Банк">
      <input id="bCur" value="EUR" placeholder="Валюта">
      <input id="bAmt" type="number" placeholder="Сумма">
      <button class="btn primary" id="bSave">Сохранить баланс</button>
      <p class="meta">Утро: ${data.morning_ok ? "обновлён" : "нет"} · Вечер: ${data.evening_ok ? "обновлён" : "нет"}</p>
    </div>
    ${(data.accounts || []).map((a) => `
      <div class="card">
        <h3>${esc(names[a.company_id] || a.company_id)}</h3>
        <div class="amt">${a.amount} ${esc(a.currency)}</div>
        <div class="meta">${esc(a.uid)} · ${esc(a.bank)} · ${esc(a.updated_by)} · ${esc(fmtWhen(a.updated_at))}</div>
      </div>`).join("") || `<p class="empty">Счетов пока нет</p>`}
  `);
  $("bSave").onclick = async () => {
    await api("/api/balance", { method: "POST", body: JSON.stringify({
      company_id: Number($("bCo").value), bank: $("bBank").value,
      currency: $("bCur").value, amount: Number($("bAmt").value)
    })});
    renderBalance();
  };
}

async function renderDocuments() {
  const token = bumpNav();
  currentSection = "documents";
  if (!docsTab) {
    page("Документы", `
      <p class="hint">Файлы компаний, контрагентов и всё, что не легло в сделку.</p>
      <div class="kb">
        <button type="button" class="kb-btn wide" id="docCo"><b>Наши компании</b><span class="meta">Внутренний и внешний контур, прочее</span></button>
        <button type="button" class="kb-btn wide" id="docCp"><b>Контрагенты</b><span class="meta">Документы по контрагенту</span></button>
        <button type="button" class="kb-btn wide" id="docMisc"><b>Прочее</b><span class="meta">Общий склад файлов</span></button>
      </div>
    `, token);
    if (!alive(token)) return;
    $("docCo").onclick = () => { docsTab = "companies"; renderDocuments(); };
    $("docCp").onclick = () => { docsTab = "counterparties"; renderDocuments(); };
    $("docMisc").onclick = () => { docsTab = "misc"; renderDocuments(); };
    return;
  }
  let pack = { companies: [], counterparties: [], files: [], can_edit: false };
  const q = new URLSearchParams();
  if (docsTab === "company") {
    q.set("scope", "company"); q.set("owner_id", String(docsOwner)); q.set("folder", docsFolder || "internal");
  } else if (docsTab === "counterparty") {
    q.set("scope", "counterparty"); q.set("owner_id", String(docsOwner)); q.set("folder", "other");
  } else if (docsTab === "misc") {
    q.set("scope", "misc"); q.set("folder", "other");
  }
  try { pack = await api("/api/documents" + (q.toString() ? "?" + q.toString() : "")); } catch (e) {
    page("Документы", `<p class="err">${esc(e.message)}</p>`, token);
    return;
  }
  if (!alive(token)) return;
  let att = {};
  if (docsTab === "company" || docsTab === "counterparty" || docsTab === "misc") {
    try { att = await api("/api/attach"); } catch (_) {}
    if (!alive(token)) return;
  }
  const canEdit = !!(pack.can_edit || isAdmin());
  const files = asList(pack.files);
  const fileCards = files.map((f) => `
    <div class="card">
      ${isImageFile(f) ? `<img class="doc-preview" alt="" data-vpreview="${f.id}">` : ""}
      <h3>${f.request_id ? "📁 " + esc(f.request_uid) + " · " + esc(f.request_title || "Заявка") : esc(f.name || "Файл")}</h3>
      <div class="meta">${esc(f.created_name)} · ${esc(fmtWhen(f.created_at))}${f.size ? " · " + fmtSize(f.size) : ""}</div>
      <div class="row wrap">
        <button class="btn" data-vopen="${f.id}" data-archive="${f.request_id ? "1" : ""}" data-name="${esc(f.name)}">Открыть</button>
        <button class="btn" data-vdl="${f.id}" data-name="${esc(f.name)}">${f.request_id ? "Скачать всё ZIP" : "Скачать"}</button>
        <button type="button" class="btn" data-vtogs="${f.id}">Себе в личку</button>
        ${(f.mine || canEdit) ? `<button type="button" class="btn bad" data-vdel="${f.id}">Удалить</button>` : ""}
      </div>
    </div>`).join("") || `<p class="empty">Пока пусто — загрузите с устройства или перешлите из Telegram</p>`;
  const vaultWaiting = (scope, owner, folder) => !!(att && att.waiting && att.kind === "vault"
    && String(att.scope || "") === String(scope || "")
    && Number(att.owner_id || 0) === Number(owner || 0)
    && String(att.folder || "") === String(folder || ""));
  const bindFiles = (scope, owner, folder) => {
    const pick = $("vfPick");
    const go = $("vfGo");
    if (go && pick) go.onclick = async () => {
      if (!pick.files || !pick.files.length) { toast("Выберите файлы"); return; }
      try {
        await uploadVault({ scope, owner_id: String(owner || 0), folder: folder || "other" }, pick.files);
        toast("Загружено");
        renderDocuments();
      } catch (e) { toast(e.message); }
    };
    const qstr = q.toString();
    const waiting = vaultWaiting(scope, owner, folder);
    const botName = (att && att.bot_username) || (me && me.bot_username) || "";
    const goTg = async () => {
      try {
        const out = await api("/api/attach", { method: "POST", body: JSON.stringify({ vault: true, scope, owner_id: Number(owner || 0), folder: folder || "other" }) });
        toast("Перешлите файлы боту в личку");
        openBotChat((out && out.bot_username) || botName, out && out.start);
        startVaultPoll(files.length, qstr);
      } catch (e) { toast(e.message); }
    };
    const tgBtn = $("vfTg");
    if (tgBtn) tgBtn.onclick = goTg;
    const stopBtn = $("vfTgStop");
    if (stopBtn) stopBtn.onclick = async () => {
      try {
        await api("/api/attach", { method: "POST", body: JSON.stringify({ cancel: true }) });
        stopDocsPoll();
        toast("Больше не жду");
        renderDocuments();
      } catch (e) { toast(e.message); }
    };
    const clearBtn = $("vfClearMine");
    if (clearBtn) clearBtn.onclick = async () => {
      const ok = await ask("Убрать отсюда всё, что отправили вы? Чужое останется.");
      if (!ok) return;
      try {
        const out = await api("/api/attach", { method: "POST", body: JSON.stringify({ vault: true, scope, owner_id: Number(owner || 0), folder: folder || "other", clear_mine: true }) });
        toast(out && out.cleared ? ("Убрали отправленное: " + out.cleared) : "Убрано");
        renderDocuments();
      } catch (e) { toast(e.message); }
    };
    const undoBtn = $("vfUndo");
    if (undoBtn) undoBtn.onclick = async () => {
      const ok = await ask("Убрать из документов то, что вы отправили последним?");
      if (!ok) return;
      try {
        const out = await api("/api/attach", { method: "POST", body: JSON.stringify({ undo: true }) });
        toast(out && out.undone ? ("Убрали: " + out.undone) : "Отменено");
        renderDocuments();
      } catch (e) { toast(e.message); }
    };
    if (waiting) startVaultPoll(files.length, qstr);
    app.querySelectorAll("[data-vopen]").forEach((b) => b.onclick = () => {
      if (b.dataset.archive) showSavedRequest(Number(b.dataset.vopen), renderDocuments);
      else previewVaultFile(Number(b.dataset.vopen), b.dataset.name);
    });
    app.querySelectorAll("[data-vdl]").forEach((b) => {
      b.onclick = async (e) => {
        e.preventDefault();
        try { await downloadVault(Number(b.dataset.vdl), b.dataset.name); } catch (err) { toast(err.message); }
      };
    });
    app.querySelectorAll("[data-vtogs]").forEach((b) => {
      b.onclick = async () => {
        try {
          await api("/api/documents", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.vtogs), send_self: true }) });
          toast("Отправил в личку");
        } catch (e) { toast(e.message); }
      };
    });
    app.querySelectorAll("[data-vdel]").forEach((b) => {
      b.onclick = async () => {
        const ok = await ask("Удалить документ? Сохранённая заявка удаляется целиком с копиями вложений. Исходные данные останутся.");
        if (!ok) return;
        try {
          await api("/api/documents?id=" + b.dataset.vdel, { method: "DELETE" });
          toast("Удалено");
          renderDocuments();
        } catch (e) { toast(e.message); }
      };
    });
    const imgs = app.querySelectorAll("[data-vpreview]");
    for (const img of imgs) {
      (async () => {
        try {
          const headers = {};
          if (tg && tg.initData) headers["X-Telegram-Init-Data"] = tg.initData;
          const res = await fetch("/api/documents?id=" + img.dataset.vpreview, { headers });
          if (!res.ok) return;
          const blob = await res.blob();
          img.src = URL.createObjectURL(blob);
          img.onclick = (e) => { e.preventDefault(); e.stopPropagation(); openLightbox(img.src); };
        } catch (_) {}
      })();
    }
  };
  const uploadBlock = (scope, owner, folder, hint) => {
    const waiting = vaultWaiting(scope, owner, folder);
    const botName = (att && att.bot_username) || (me && me.bot_username) || "";
    const own = files.filter((f) => f.mine);
    return `
    <div class="card">
      <h3>С устройства</h3>
      <p class="hint">Файлы с телефона или компьютера, можно несколько сразу, до 25 МБ каждый.</p>
      <input id="vfPick" type="file" multiple>
      <button type="button" class="btn primary" id="vfGo">Загрузить</button>
    </div>
    <div class="card">
      <h3>Переслать из Telegram</h3>
      <p class="hint">${esc(hint)}</p>
      ${waiting ? `<p class="hint">Сейчас жду пересылку${botName ? " @" + esc(botName) : ""}. Кинули — появятся здесь, можно не закрывать Mini App.</p>` : ""}
      <button class="btn primary" id="vfTg">${waiting ? "Открыть личку бота" : "Переслать из Telegram"}</button>
      ${waiting ? `<button class="btn" id="vfTgStop">Больше не ждать</button>` : ""}
      ${own.length ? `<button class="btn bad" id="vfClearMine">Убрать отправленное</button>` : ""}
      ${own.length ? `<button class="btn" id="vfUndo">Отменить последнее</button>` : ""}
    </div>`;
  };

  if (docsTab === "companies") {
    const list = asList(pack.companies);
    page("Наши компании", `
      <p class="hint">Выберите компанию. Внутри — внутренний контур, внешний контур и прочее.</p>
      ${canEdit ? `<div class="card">
        <h3>Добавить компанию</h3>
        <input id="coName" placeholder="Название">
        <button type="button" class="btn primary" id="coAdd">Добавить</button>
      </div>` : ""}
      ${list.map((c) => `<div class="card">
        <button type="button" class="kb-btn wide" data-co="${c.id}" data-name="${esc(c.name)}"><b>${esc(c.name)}</b><span class="meta">Открыть документы</span></button>
        ${canEdit ? `<div class="row wrap">
          <button type="button" class="btn" data-corename="${c.id}" data-name="${esc(c.name)}">Переименовать</button>
          <button type="button" class="btn bad" data-codel="${c.id}">Удалить</button>
        </div>` : ""}
      </div>`).join("") || `<p class="empty">Компаний пока нет — админ добавит</p>`}
    `, token);
    if (!alive(token)) return;
    const add = $("coAdd");
    if (add) add.onclick = async () => {
      try {
        await api("/api/documents", { method: "POST", body: JSON.stringify({ action: "add_company", name: $("coName").value }) });
        toast("Компания добавлена");
        renderDocuments();
      } catch (e) { toast(e.message); }
    };
    app.querySelectorAll("[data-co]").forEach((b) => {
      b.onclick = () => { docsTab = "company"; docsOwner = Number(b.dataset.co); docsName = b.dataset.name || ""; docsFolder = "internal"; renderDocuments(); };
    });
    app.querySelectorAll("[data-corename]").forEach((b) => {
      b.onclick = async () => {
        const name = window.prompt("Новое название", b.dataset.name || "");
        if (name == null) return;
        try {
          await api("/api/documents", { method: "POST", body: JSON.stringify({ action: "rename_company", id: Number(b.dataset.corename), name }) });
          toast("Переименовано");
          renderDocuments();
        } catch (e) { toast(e.message); }
      };
    });
    app.querySelectorAll("[data-codel]").forEach((b) => {
      b.onclick = async () => {
        const ok = await ask("Удалить компанию и её файлы в Документах?");
        if (!ok) return;
        try {
          await api("/api/documents", { method: "POST", body: JSON.stringify({ action: "delete_company", id: Number(b.dataset.codel) }) });
          toast("Удалено");
          renderDocuments();
        } catch (e) { toast(e.message); }
      };
    });
    return;
  }

  if (docsTab === "company") {
    const folder = docsFolder || "internal";
    const contour = folder.startsWith("external") ? "external" : (folder.startsWith("other") ? "other" : "internal");
    const tabId = (folder.indexOf(":") >= 0) ? Number(folder.split(":")[1] || 0) : 0;
    const tabs = asList(pack.tabs).filter((t) => String(t.contour || "") === contour);
    const activeTab = tabs.find((t) => Number(t.id) === tabId);
    const contourTitle = contour === "external" ? "Внешний контур" : (contour === "other" ? "Прочее" : "Внутренний контур");
    const folderTitle = activeTab ? (contourTitle + " · " + activeTab.name) : contourTitle;
    const subTabs = (contour === "internal" || contour === "external") ? `
      <div class="kb">
        <button type="button" class="kb-btn ${!tabId ? "on" : ""}" data-subfold="${contour}"><b>Общее</b></button>
        ${tabs.map((t) => `<button type="button" class="kb-btn ${Number(t.id) === tabId ? "on" : ""}" data-subfold="${contour}:${t.id}" data-tabid="${t.id}"><b>${esc(t.name)}</b></button>`).join("")}
        <button type="button" class="kb-btn" id="tabAdd"><b>+ Вкладка</b></button>
      </div>
      ${tabId ? `<div class="row wrap">
        <button type="button" class="btn" id="tabRename">Переименовать вкладку</button>
        <button type="button" class="btn bad" id="tabDel">Удалить вкладку</button>
      </div>` : ""}
    ` : "";
    page(docsName || "Компания", `
      <div class="kb">
        <button type="button" class="kb-btn ${contour === "internal" ? "on" : ""}" data-fold="internal"><b>Внутренний контур</b></button>
        <button type="button" class="kb-btn ${contour === "external" ? "on" : ""}" data-fold="external"><b>Внешний контур</b></button>
        <button type="button" class="kb-btn ${contour === "other" ? "on" : ""}" data-fold="other"><b>Прочее</b></button>
      </div>
      ${subTabs}
      <p class="hint">${esc(folderTitle)} — файлы с устройства или через бота в личке.</p>
      ${uploadBlock("company", docsOwner, folder, "Нажмите кнопку — откроется личка с ботом. Перешлите туда файл. Можно несколько подряд.")}
      ${fileCards}
    `, token);
    if (!alive(token)) return;
    app.querySelectorAll("[data-fold]").forEach((b) => {
      b.onclick = () => { docsFolder = b.dataset.fold; renderDocuments(); };
    });
    app.querySelectorAll("[data-subfold]").forEach((b) => {
      b.onclick = () => { docsFolder = b.dataset.subfold; renderDocuments(); };
    });
    const addTab = $("tabAdd");
    if (addTab) addTab.onclick = async () => {
      const name = window.prompt("Название вкладки", "");
      if (name == null) return;
      try {
        const t = await api("/api/documents", { method: "POST", body: JSON.stringify({ action: "add_tab", company_id: docsOwner, contour, name }) });
        toast("Вкладка добавлена");
        if (t && t.id) docsFolder = contour + ":" + t.id;
        renderDocuments();
      } catch (e) { toast(e.message); }
    };
    const renTab = $("tabRename");
    if (renTab) renTab.onclick = async () => {
      const name = window.prompt("Новое название", activeTab && activeTab.name || "");
      if (name == null) return;
      try {
        await api("/api/documents", { method: "POST", body: JSON.stringify({ action: "rename_tab", id: tabId, name }) });
        toast("Переименовано");
        renderDocuments();
      } catch (e) { toast(e.message); }
    };
    const delTab = $("tabDel");
    if (delTab) delTab.onclick = async () => {
      const ok = await ask("Удалить вкладку и все файлы в ней?");
      if (!ok) return;
      try {
        await api("/api/documents", { method: "POST", body: JSON.stringify({ action: "delete_tab", id: tabId }) });
        toast("Удалено");
        docsFolder = contour;
        renderDocuments();
      } catch (e) { toast(e.message); }
    };
    bindFiles("company", docsOwner, folder);
    return;
  }
  if (docsTab === "counterparties") {
    const list = asList(pack.counterparties);
    page("Контрагенты", `
      <p class="hint">Контрагенты из Справочника. Откройте карточку и закиньте файлы.</p>
      ${list.map((c) => `<button type="button" class="kb-btn wide" data-cp="${c.id}" data-name="${esc(c.name)}"><b>${esc(c.name)}</b><span class="meta">${esc(c.work_id || "документы")}</span></button>`).join("") || `<p class="empty">Контрагентов нет — добавьте в Справочник</p>`}
    `, token);
    if (!alive(token)) return;
    app.querySelectorAll("[data-cp]").forEach((b) => {
      b.onclick = () => { docsTab = "counterparty"; docsOwner = Number(b.dataset.cp); docsName = b.dataset.name || ""; renderDocuments(); };
    });
    return;
  }

  if (docsTab === "counterparty") {
    page(docsName || "Контрагент", `
      <p class="hint">Файлы по этому контрагенту — с устройства или через бота в личке.</p>
      ${uploadBlock("counterparty", docsOwner, "other", "Нажмите кнопку — откроется личка с ботом. Перешлите туда файл. Можно несколько подряд.")}
      ${fileCards}
    `, token);
    if (!alive(token)) return;
    bindFiles("counterparty", docsOwner, "other");
    return;
  }

  page("Прочее", `
    <p class="hint">Документы, которые не относятся к компании или контрагенту — с устройства или через бота в личке.</p>
    ${uploadBlock("misc", 0, "other", "Нажмите кнопку — откроется личка с ботом. Перешлите туда файл. Можно несколько подряд.")}
    ${fileCards}
  `, token);
  if (!alive(token)) return;
  bindFiles("misc", 0, "other");
}

async function renderCompliance() {
  const token = bumpNav();
  const [list, reqs] = await Promise.all([api("/api/compliance"), has("requests") ? api("/api/requests") : Promise.resolve([])]);
  if (!alive(token)) return;
  page("Комплаенс", `
    <div class="card">
      <p class="hint">Свободный текст, без вариантов ответа. Можно привязать заявку — тогда подтянется контекст сделки.</p>
      <select id="cReq"><option value="">Заявка (необязательно)</option>${asList(reqs).map((r) => `<option value="${r.id}">${esc(r.uid || r.title)}</option>`).join("")}</select>
      <input id="cSub" placeholder="Тема (необязательно)">
      <textarea id="cText" rows="6" placeholder="Текст проверки — пишите свободно"></textarea>
      <button class="btn primary" id="cAdd">Новая проверка</button>
    </div>
    ${(list || []).map((c) => `
      <div class="card">
        <h3>${esc(c.subject)} <span class="badge">${esc(c.status)}</span></h3>
        <div class="meta">${esc(c.uid)} · ${esc(c.created_name)} · ${esc(c.request_uid || "")}</div>
        ${(c.questions || []).filter((q) => q.answer).map((q) => `<p><b>${esc(q.text)}</b><br>${esc(q.answer)}</p>`).join("")}
        <textarea data-ctext="${c.id}" rows="5" placeholder="Текст проверки">${esc(c.text || "")}</textarea>
        <div class="row wrap">
          <button type="button" class="btn" data-savec="${c.id}">Сохранить текст</button>
          ${c.status !== "done" ? `<button type="button" class="btn ok" data-donec="${c.id}">Готово</button>` : `<button type="button" class="btn" data-openc="${c.id}">Вернуть в работу</button>`}
          <button class="btn" data-dsp="compliance" data-uid="${esc(c.uid)}">Спор</button>
        </div>
      </div>`).join("")}
  `);
  $("cAdd").onclick = async () => {
    try {
      await api("/api/compliance", { method: "POST", body: JSON.stringify({ subject: $("cSub").value, text: $("cText").value, request_id: Number($("cReq").value) || 0 }) });
      toast("Проверка создана");
      renderCompliance();
    } catch (e) { toast(e.message); }
  };
  app.querySelectorAll("[data-savec]").forEach((b) => b.onclick = async () => {
    const ta = app.querySelector(`[data-ctext="${b.dataset.savec}"]`);
    try {
      await api("/api/compliance", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.savec), text: ta ? ta.value : "" }) });
      toast("Сохранено");
      renderCompliance();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-donec]").forEach((b) => b.onclick = async () => {
    try {
      await api("/api/compliance", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.donec), status: "done" }) });
      toast("Готово");
      renderCompliance();
    } catch (e) { toast(e.message); }
  });
  app.querySelectorAll("[data-openc]").forEach((b) => b.onclick = async () => {
    try {
      await api("/api/compliance", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.openc), status: "open" }) });
      renderCompliance();
    } catch (e) { toast(e.message); }
  });
  bindDispute();
}

async function renderSummary() {
  const token = bumpNav();
  const s = await api("/api/summary");
  if (!alive(token)) return;
  page("Сводка", Object.entries(s).map(([k, v]) => `<div class="card"><div class="meta">${esc(k)}</div><div class="amt">${esc(v)}</div></div>`).join(""));
}

function checksHTML(prefix, access, locked, keys) {
  return `<div class="checks">${(keys || SECTIONS).map((s) => {
    const on = !!(access && access[s.key]);
    return `<label class="check ${on ? "on" : ""}">
      <input type="checkbox" data-sec="${s.key}" data-prefix="${prefix}" ${on ? "checked" : ""} ${locked ? "disabled" : ""}>
      ${esc(s.title)}
    </label>`;
  }).join("")}</div>`;
}
function accessFromEmp(emp, poss) {
  const out = {};
  asList(emp && emp.position_ids).forEach((id) => {
    const p = dirByID(poss, id);
    const acc = p && p.access;
    if (!acc) return;
    Object.keys(acc).forEach((k) => { if (acc[k]) out[k] = true; });
  });
  return out;
}
function posNames(emp, poss) {
  return asList(emp && emp.position_ids).map((id) => {
    const p = dirByID(poss, id);
    return p ? p.name : "должность удалена";
  }).filter(Boolean);
}

function readAccess(prefix) {
  const out = {};
  app.querySelectorAll(`input[data-prefix="${prefix}"][data-sec]`).forEach((el) => {
    out[el.dataset.sec] = !!el.checked;
  });
  return out;
}

function paintChecks() {
  app.querySelectorAll(".check").forEach((lab) => {
    const inp = lab.querySelector("input");
    lab.classList.toggle("on", !!(inp && inp.checked));
  });
}

function bindChecks() {
  app.querySelectorAll("label.check").forEach((lab) => {
    const inp = lab.querySelector("input[type=checkbox]");
    if (!inp) return;
    lab.addEventListener("click", (e) => {
      if (inp.disabled) return;
      if (e.target !== inp) {
        e.preventDefault();
        inp.checked = !inp.checked;
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
      paintChecks();
    });
  });
  app.querySelectorAll(".check input").forEach((el) => {
    el.addEventListener("change", paintChecks);
  });
  paintChecks();
}

async function renderChats() {
  const token = bumpNav();
  currentSection = "chats";
  try { await refreshMe(); } catch (_) {}
  if (!alive(token)) return;
  if (!canManageChats()) {
    page("Чаты", `<p class="err">Только для владельца и админов</p>`, token);
    return;
  }
  let pack = { items: [], lawyer_chat_id: 0, docs_chat_id: 0 };
  try { pack = await api("/api/chats"); } catch (e) {
    page("Чаты", `<p class="err">${esc(e.message)}</p>`, token);
    return;
  }
  if (!alive(token)) return;
  const items = asList(pack.items);
  const roleOf = (chatID) => {
    const bits = [];
    if (Number(chatID) && Number(chatID) === Number(pack.lawyer_chat_id)) bits.push("юрист");
    if (Number(chatID) && Number(chatID) === Number(pack.docs_chat_id)) bits.push("документалист");
    return bits.length ? bits.join(" · ") : "";
  };
  page("Чаты", `
    <p class="hint">Здесь владелец и админы заводят группы Telegram. Потом в обращениях выбираете чат из этого списка — куда слать заявки.</p>
    <div class="card">
      <h3>Добавить чат</h3>
      <input id="chName" placeholder="Название (например ВЭД ЮРИСТ)">
      <input id="chID" inputmode="numeric" placeholder="ID чата (-100…)">
      <p class="meta">В нужной группе напишите боту /id — он покажет число.</p>
      <button type="button" class="btn primary" id="chAdd">Добавить</button>
    </div>
    ${items.map((c) => `
      <div class="card">
        <input data-chname="${c.id}" value="${esc(c.name)}" placeholder="Название">
        <input data-chid="${c.id}" inputmode="numeric" value="${esc(c.chat_id)}" placeholder="ID чата">
        <div class="meta">${esc(c.uid || "")}${roleOf(c.chat_id) ? " · используется: " + esc(roleOf(c.chat_id)) : ""}</div>
        <div class="row wrap">
          <button type="button" class="btn" data-chsave="${c.id}">Сохранить</button>
          <button type="button" class="btn bad" data-chdel="${c.id}">Удалить</button>
        </div>
      </div>`).join("") || `<p class="empty">Чатов пока нет</p>`}
  `, token);
  if (!alive(token)) return;
  $("chAdd").onclick = async () => {
    try {
      await api("/api/chats", { method: "POST", body: JSON.stringify({ name: $("chName").value, chat_raw: $("chID").value }) });
      toast("Чат добавлен");
      renderChats();
    } catch (e) { toast(e.message); }
  };
  app.querySelectorAll("[data-chsave]").forEach((b) => {
    b.onclick = async () => {
      const id = Number(b.dataset.chsave);
      const name = app.querySelector(`[data-chname="${id}"]`);
      const chat = app.querySelector(`[data-chid="${id}"]`);
      try {
        await api("/api/chats", { method: "POST", body: JSON.stringify({
          id, name: name ? name.value : "", chat_raw: chat ? chat.value : ""
        }) });
        toast("Сохранено");
        renderChats();
      } catch (e) { toast(e.message); }
    };
  });
  app.querySelectorAll("[data-chdel]").forEach((b) => {
    b.onclick = async () => {
      const ok = await ask("Удалить этот чат из списка? Если он был выбран для юриста/документалиста — привязка сбросится.");
      if (!ok) return;
      try {
        await api("/api/chats?id=" + b.dataset.chdel, { method: "DELETE" });
        toast("Удалено");
        renderChats();
      } catch (e) { toast(e.message); }
    };
  });
}

async function renderNotify() {
  const token = bumpNav();
  await refreshMe();
  if (!alive(token)) return;
  const n = me.notify || {};
  const rows = [];
  const add = (key, title, desc, show) => {
    if (!show) return;
    const on = n[key] !== false;
    rows.push(`<label class="check ${on ? "on" : ""}">
      <input type="checkbox" data-ntf="${key}" ${on ? "checked" : ""}>
      <span><b>${esc(title)}</b><br><span class="meta">${esc(desc)}</span></span>
    </label>`);
  };
  add("rates", "Курсы", "Будни в 10:00, 13:00 и 16:00 МСК", true);
  add("holidays", "Праздники", "Утром, если ближайший банковский выходной близко", true);
  add("saldo", "Сальдо", "В 18:30 открытые постановки", true);
  add("requests", "Заявки", "Новая заявка и смена статуса", true);
  add("approvals", "Согласование", "Очередь, решение и ответы в споре", true);
  add("payments", "Отправка", "Новая оплата в очередь на отправку", true);
  add("balance", "Баланс", "Напоминание утром и вечером", true);
  page("Уведомления", `
    <p class="meta">Приходят в личку с ботом Mini App, только по разделам с доступом. Выключите, что не нужно.</p>
    <div class="card">${rows.join("") || `<p class="empty">Нет разделов для уведомлений</p>`}
      ${rows.length ? `<div class="row"><button class="btn primary" id="ntfSave">Сохранить</button></div>` : ""}
    </div>
  `);
  if (!alive(token)) return;
  bindChecks();
  const save = $("ntfSave");
  if (save) save.onclick = async () => {
    const body = {};
    app.querySelectorAll("[data-ntf]").forEach((el) => { body[el.dataset.ntf] = !!el.checked; });
    try {
      me.notify = await api("/api/notify", { method: "POST", body: JSON.stringify(body) });
      toast("Сохранено");
    } catch (e) { toast(e.message); }
  };
}

async function renderUsers() {
  const token = bumpNav();
  const [list, dirs] = await Promise.all([api("/api/users"), api("/api/directory").catch(() => ({}))]);
  if (!alive(token)) return;
  const emps = asList(dirs.employees);
  const poss = asList(dirs.positions);
  const empOf = (uid) => emps.find((e) => Number(e.telegram_id) === Number(uid));
  const allOn = Object.fromEntries(POS_ACCESS.map((s) => [s.key, true]));
  page("Доступы", `
    <div class="hint">Нажмите на человека — откроются галочки с должностей. Снять их здесь нельзя, только в Справочнике. Админу должности не нужны.</div>
    <div class="card">
      <h3>Выдать доступ</h3>
      <input id="uId" placeholder="Telegram ID">
      <input id="uName" placeholder="Имя">
      <label class="check" style="margin-top:8px"><input type="checkbox" id="uAdm"> Админ — всё можно, сотрудником быть не обязан</label>
      <div class="row"><button class="btn primary" id="uAdd">Выдать доступ</button></div>
    </div>
    ${asList(list).map((u) => {
      const owner = u.role === "owner";
      const badge = owner ? "gold" : (u.role === "admin" ? "green" : "");
      const linked = empOf(u.id);
      const staff = owner || u.role === "admin" || u.role === "owner" || u.admin || u.staff;
      const acc = staff ? allOn : accessFromEmp(linked, poss);
      const jobs = posNames(linked, poss);
      return `<div class="card" data-user="${u.id}">
        <button type="button" class="user-head" data-openuser="${u.id}">
          <div class="person">
            <div class="ava">${esc(initials(u.name))}</div>
            <div style="flex:1">
              <h3>${esc(u.name)} <span class="badge ${badge}">${esc(roleLabel(u, linked))}</span></h3>
              <div class="meta">${u.id}${linked ? " · сотрудник " + esc(empLabel(linked)) : " · в пуле, ещё не сотрудник"}${jobs.length ? " · " + esc(jobs.join(", ")) : ""}</div>
              <div class="meta">Нажмите, чтобы увидеть разделы</div>
            </div>
          </div>
        </button>
        <div class="user-more" hidden>
          ${checksHTML("u" + u.id, acc, true, POS_ACCESS)}
          <p class="meta">${staff ? "Админ и владелец видят все разделы, должности не обязательны." : (linked ? "Эти галочки из справочника, здесь их не снять." : "Пока не сотрудник — разделов нет. Назначьте в Справочнике.")}</p>
          ${owner ? `<p class="meta">Владельца нельзя снять и нельзя урезать.</p>
            <div class="row"><button class="btn primary" data-save="${u.id}">Сохранить</button></div>` : `
            <label class="check"><input type="checkbox" data-adm="${u.id}" ${u.role === "admin" ? "checked" : ""}> Админ</label>
            <div class="row">
              <button class="btn primary" data-save="${u.id}">Сохранить</button>
              <button class="btn bad" data-del="${u.id}">Забрать доступ</button>
            </div>`}
        </div>
      </div>`;
    }).join("")}
  `);
  bindChecks();
  app.querySelectorAll("[data-openuser]").forEach((b) => {
    b.onclick = () => {
      hideKeyboard();
      const card = b.closest("[data-user]");
      const more = card && card.querySelector(".user-more");
      if (!more) return;
      const open = more.hidden;
      app.querySelectorAll(".user-more").forEach((m) => { m.hidden = true; });
      more.hidden = !open;
      buzz();
    };
  });
  $("uAdd").onclick = async () => {
    await api("/api/users", { method: "POST", body: JSON.stringify({
      id: Number($("uId").value),
      name: $("uName").value,
      role: $("uAdm").checked ? "admin" : "operator"
    })});
    renderUsers();
  };
  app.querySelectorAll("[data-save]").forEach((b) => b.onclick = async () => {
    const id = Number(b.dataset.save);
    const card = app.querySelector(`[data-user="${id}"]`);
    const name = card.querySelector("h3").childNodes[0].textContent.trim();
    const adm = card.querySelector(`[data-adm="${id}"]`);
    await api("/api/users", { method: "POST", body: JSON.stringify({
      id,
      name,
      role: adm ? (adm.checked ? "admin" : "operator") : "owner"
    })});
    renderUsers();
  });
  app.querySelectorAll("[data-del]").forEach((b) => b.onclick = async () => {
    await api("/api/users?id=" + b.dataset.del, { method: "DELETE" });
    renderUsers();
  });
}

async function renderDisputes() {
  const token = bumpNav();
  const list = asList(await api("/api/disputes"));
  if (!alive(token)) return;
  page("Споры", (list || []).map((d) => `
    <div class="card">
      <h3>${esc(d.section)} · ${esc(d.ref_uid)}</h3>
      <p>${esc(d.text)}</p>
      <div class="meta">${esc(d.status)} · ${esc(d.uid)}</div>
      ${d.status === "open" ? `<button class="btn ok" data-st="${d.id}" data-v="closed">Закрыть</button>` : ""}
    </div>`).join("") || `<p class="empty">Нет споров</p>`);
  app.querySelectorAll("[data-st]").forEach((b) => b.onclick = async () => {
    await api("/api/disputes", { method: "POST", body: JSON.stringify({ id: Number(b.dataset.st), status: b.dataset.v }) });
    renderDisputes();
  });
}

function bindStatus(path) {
  app.querySelectorAll("[data-st]").forEach((b) => b.onclick = async () => {
    const body = { id: Number(b.dataset.st), status: b.dataset.v };
    if (path === "/api/requests" && b.dataset.v === "done") {
      body.table_ref = "row:" + b.dataset.st;
    }
    await api(path, { method: "POST", body: JSON.stringify(body) });
    openSection(path.split("/").pop());
  });
}

function bindDispute() {
  app.querySelectorAll("[data-dsp]").forEach((b) => {
    b.onclick = () => {
      let box = b.parentElement.querySelector(".inline-dsp");
      if (box) { box.remove(); return; }
      box = document.createElement("div");
      box.className = "inline-dsp";
      box.innerHTML = `<textarea rows="2" placeholder="Что спорно?"></textarea><button type="button" class="btn primary">Отправить спор</button>`;
      b.after(box);
      box.querySelector("textarea").focus();
      box.querySelector("button").onclick = async () => {
        const text = box.querySelector("textarea").value.trim();
        if (!text) return;
        try {
          await api("/api/disputes", { method: "POST", body: JSON.stringify({ section: b.dataset.dsp, ref_uid: b.dataset.uid, text }) });
          toast("Спор отправлен. Ответ — во вкладке Споры");
          aprTab = "disputes";
          renderApprovals();
        } catch (e) { toast(e.message); }
      };
    };
  });
}

function hideKeyboard() {
  const el = document.activeElement;
  if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT")) {
    el.blur();
  }
  try { if (document.body) document.body.focus(); } catch (_) {}
}
function bindHideKeyboard() {
  if (window._kbBound) return;
  window._kbBound = true;
  document.addEventListener("pointerdown", (e) => {
    const t = e.target;
    if (!t || !t.closest) return;
    if (t.closest("input, textarea, select, label.check, button, a, .kb-btn, .tile, .dock-btn")) return;
    hideKeyboard();
  }, true);
}

$("homeBtn").onclick = onBack;
bindWinBtns();
bindHideKeyboard();

function launchGo() {
  let go = "";
  try { go = new URLSearchParams(location.search).get("go") || ""; } catch (_) {}
  if (!go) {
    try { go = (tg && tg.initDataUnsafe && tg.initDataUnsafe.start_param) || ""; } catch (_) {}
  }
  go = String(go || "").toLowerCase().trim();
  const allow = { requests: 1, tasks: 1, saldo: 1, approvals: 1, payments: 1, directory: 1 };
  if (!allow[go]) return false;
  openSection(go);
  return true;
}

(async function init() {
  try {
    me = await api("/api/me");
    const tag = me.owner ? "владелец" : (me.admin || me.can_manage_users || me.role === "admin" ? "админ" : (me.employee ? "сотрудник" : "доступ не выдан"));
    $("who").textContent = (me.name || "") + " · " + tag;
    const ava = $("whoAva");
    if (ava) ava.textContent = initials(me.name);
    const menu = $("menuBtn");
    if (menu) menu.onclick = () => { buzz(); renderMore(); };
    if (!launchGo()) showHome();
  } catch (e) {
    app.innerHTML = `<p class="err">Откройте приложение из главного бота Telegram. ${esc(e.message)}</p>`;
  }
})();
