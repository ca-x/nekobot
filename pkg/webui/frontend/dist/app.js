/* ========== i18n ========== */
const SUPPORTED_LANGS = ["en", "zh-CN", "ja"];
const messages = {};

async function loadI18n() {
  await Promise.all(SUPPORTED_LANGS.map(async function(lang) {
    try {
      var resp = await fetch("/i18n/" + lang + ".json");
      if (resp.ok) messages[lang] = await resp.json();
    } catch (_) {}
    if (!messages[lang]) messages[lang] = {};
  }));
}

let currentLang = localStorage.getItem("nekobot_lang") || "en";

function t(key, ...args) {
  let str = (messages[currentLang] && messages[currentLang][key]) || messages.en[key] || key;
  args.forEach((a, i) => { str = str.replace("{" + i + "}", a); });
  return str;
}

function renderI18n() {
  document.querySelectorAll("[data-i18n]").forEach(el => {
    const key = el.getAttribute("data-i18n");
    if (key) el.textContent = t(key);
  });
  document.querySelectorAll("[data-i18n-ph]").forEach(el => {
    const key = el.getAttribute("data-i18n-ph");
    if (key) el.placeholder = t(key);
  });
  document.title = "Nekobot - " + t("appSubtitle");
  document.documentElement.lang = currentLang === "zh-CN" ? "zh-CN" : "en";
  setToolAccessIconButtonLabels();
}

function setToolAccessIconButtonLabels() {
  var pairs = [
    ["toolAccessCopyUrlBtn", "copyUrl"],
    ["toolAccessCopyPasswordBtn", "copyPassword"],
    ["toolAccessCopyOtpBtn", "copyOtp"],
    ["toolAccessRefreshBtn", "refreshAccess"],
    ["toolAccessOtpRefreshBtn", "refreshOtp"]
  ];
  pairs.forEach(function(pair) {
    var el = $(pair[0]);
    if (!el) return;
    var text = t(pair[1]);
    el.title = text;
    el.setAttribute("aria-label", text);
  });
}

function switchLang(lang) {
  currentLang = lang;
  localStorage.setItem("nekobot_lang", lang);
  renderI18n();
  renderChannelsTable();
  renderModelSelect();
  renderProviderSelect();
  renderChatProviderSelect();
  renderProviderModelPicker();
  renderToolSessionsList();
  renderToolTabs();
  renderToolPanels();
  renderConfigSectionSelect();
  if (state.loadedConfig) renderConfigEditorForSection();
  if (!$("providerDialog").classList.contains("hidden")) {
    $("providerDialogTitle").textContent = t(state.providerDialogMode === "edit" ? "editProviderDialogTitle" : "newProviderDialogTitle");
  }
  if (!$("toolSessionDialog").classList.contains("hidden")) {
    $("toolSessionDialogTitle").textContent = t(state.toolSessionDialogMode === "edit" ? "editToolSessionTitle" : "newToolSessionTitle");
    $("toolSessionDialogCreateBtn").textContent = t(state.toolSessionDialogMode === "edit" ? "save" : "createSession");
  }
}

/* ========== Theme ========== */
function getTheme() { return localStorage.getItem("nekobot_theme") || "light"; }

function applyTheme(theme) {
  document.documentElement.setAttribute("data-theme", theme);
  localStorage.setItem("nekobot_theme", theme);
  var sun = document.getElementById("themeIconSun");
  var moon = document.getElementById("themeIconMoon");
  if (theme === "dark") {
    sun.classList.remove("hidden");
    moon.classList.add("hidden");
  } else {
    sun.classList.add("hidden");
    moon.classList.remove("hidden");
  }
  applyToolTerminalTheme();
}

function toggleTheme() {
  applyTheme(getTheme() === "light" ? "dark" : "light");
}

function loadToolAccessRecords() {
  try {
    var raw = localStorage.getItem("nekobot_tool_access_records") || "{}";
    var parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch (_) {
    return {};
  }
}

function saveToolAccessRecords() {
  try {
    localStorage.setItem("nekobot_tool_access_records", JSON.stringify(state.toolAccessRecords || {}));
  } catch (_) {}
}

const TOOL_SESSION_DRAFT_KEY = "nekobot_tool_session_draft";

function loadToolSessionDraft() {
  try {
    var raw = localStorage.getItem(TOOL_SESSION_DRAFT_KEY) || "{}";
    var parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch (_) {
    return {};
  }
}

function saveToolSessionDraft(draft) {
  try {
    localStorage.setItem(TOOL_SESSION_DRAFT_KEY, JSON.stringify(draft || {}));
  } catch (_) {}
}

/* ========== State & API ========== */
const state = {
  token: localStorage.getItem("nekobot_webui_token") || "",
  ws: null,
  models: [],
  channels: {},
  editingChannel: "",
  providers: [],
  editingProviderName: "",
  defaultProvider: "",
  chatProvider: "",
  providerModelCatalog: {},
  providerModelOptions: [],
  providerModelSelected: [],
  providerModelFilter: "",
  providerDialogMode: "new",
  toolSessionsRaw: [],
  toolSessions: [],
  terminatedToolSessions: 0,
  toolSessionDialogMode: "new",
  editingToolSessionID: "",
  toolTabs: [],
  activeToolTab: "",
  splitToolTab: "",
  toolMaximized: "",
  toolBuffers: {},
  toolSockets: {},
  toolTerminalPanels: { Primary: null, Split: null },
  toolTerminalEnabled: false,
  toolAccessRecords: loadToolAccessRecords(),
  toolAccessDialogSessionID: "",
  toolAccessOTP: null,
  toolAccessOTPTimer: null,
  toolSessionDraft: loadToolSessionDraft(),
  loadedConfig: null,
  toolInputQueue: {},
  toolInputSending: {},
  toolReloadTicker: 0,
  toolPollBusy: false,
  toolPollTimer: null,
  toolSessionRefreshTimer: null,
  pendingToolSessionID: "",
  configSection: "agents"
};

const $ = (id) => document.getElementById(id);

function setAuthError(msg) { $("authError").textContent = msg || ""; }

function showToast(text, type) {
  var message = String(text || "").trim();
  if (!message) return;
  var wrap = $("toastContainer");
  if (!wrap) return;
  var item = document.createElement("div");
  item.className = "toast-item " + (type || "info");
  item.textContent = message;
  wrap.appendChild(item);
  setTimeout(function() {
    item.style.opacity = "0";
    item.style.transform = "translateY(4px)";
  }, 2200);
  setTimeout(function() {
    if (item.parentNode) item.parentNode.removeChild(item);
  }, 2500);
}

async function api(path, options) {
  options = options || {};
  const headers = Object.assign({ "Content-Type": "application/json" }, options.headers || {});
  if (state.token) headers["Authorization"] = "Bearer " + state.token;
  const resp = await fetch(path, Object.assign({}, options, { headers: headers }));
  const text = await resp.text();
  var payload = {};
  try { payload = text ? JSON.parse(text) : {}; } catch (_) { payload = { raw: text }; }
  if (resp.status === 401 && path !== "/api/status") {
    state.token = "";
    localStorage.removeItem("nekobot_webui_token");
    showAuth();
    checkInitAndAuth();
    throw new Error(t("sessionExpired") || "Session expired, please login again");
  }
  if (!resp.ok) throw new Error(payload.error || "HTTP " + resp.status);
  return payload;
}

function showMain() {
  $("authWrapper").classList.add("hidden");
  $("mainCard").classList.remove("hidden");
}

function showAuth() {
  $("mainCard").classList.add("hidden");
  $("authWrapper").classList.remove("hidden");
}

function setWSState(connected, key) {
  var wrap = $("wsState");
  wrap.querySelector(".ws-dot").className = "ws-dot " + (connected ? "ok" : "err");
  wrap.querySelector("span:last-child").textContent = t(key);
}

function pushChat(kind, text) {
  var log = $("chatLog");
  var empty = log.querySelector(".chat-empty");
  if (empty) empty.remove();
  var item = document.createElement("div");
  item.className = "chat-msg " + kind;
  item.textContent = text;
  log.appendChild(item);
  log.scrollTop = log.scrollHeight;
}

/* ========== Auth ========== */
async function bootstrapToolAccessFromURL() {
  var params = new URLSearchParams(location.search || "");
  var sessionID = (params.get("tool_session") || "").trim();
  var password = (params.get("tool_password") || "").trim();
  if (!sessionID) return;

  state.pendingToolSessionID = sessionID;
  if (!password) return;

  try {
    var resp = await fetch("/api/tool-sessions/access-login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session_id: sessionID, password: password })
    }).then(function(r) { return r.json(); });
    if (resp && resp.token) {
      state.token = resp.token;
      localStorage.setItem("nekobot_webui_token", state.token);
    }
  } catch (_) {}

  params.delete("tool_password");
  params.delete("tool_session");
  var next = location.pathname + (params.toString() ? ("?" + params.toString()) : "");
  history.replaceState(null, "", next);
}

async function checkInitAndAuth() {
  setAuthError("");
  try {
    await api("/api/status");
    showMain();
    await initMain();
    return;
  } catch (_) {}

  showAuth();
  var init = await fetch("/api/auth/init-status").then(function(r) { return r.json(); });
  if (init.initialized) {
    $("authHint").setAttribute("data-i18n", "loginHint");
    $("authHint").textContent = t("loginHint");
    $("authLogin").classList.remove("hidden");
    $("authInit").classList.add("hidden");
  } else {
    $("authHint").setAttribute("data-i18n", "firstRunHint");
    $("authHint").textContent = t("firstRunHint");
    $("authInit").classList.remove("hidden");
    $("authLogin").classList.add("hidden");
  }
}

async function initMain() {
  await Promise.all([loadModels(), loadProviders(), loadChannels(), loadConfig(), loadStatus(), loadToolSessions()]);
  startToolPoller();
  connectWS();
}

/* ========== Models ========== */
async function loadModels() {
  try {
    var results = await Promise.all([api("/api/providers"), api("/api/config")]);
    var providers = results[0], cfg = results[1];
    var models = [];
    var seen = new Set();
    var addModel = function(provider, model) {
      var m = String(model || "").trim();
      if (!m) return;
      var p = String(provider || "default").trim() || "default";
      var key = p + "::" + m;
      if (seen.has(key)) return;
      seen.add(key);
      models.push({ provider: p, model: m });
    };
    var currentProvider = (cfg && cfg.agents && cfg.agents.defaults && cfg.agents.defaults.provider) || "";
    state.defaultProvider = currentProvider;
    if (!state.chatProvider) state.chatProvider = currentProvider;
    if (cfg && cfg.agents && cfg.agents.defaults && Array.isArray(cfg.agents.defaults.fallback)) {
      $("fallbackInput").value = cfg.agents.defaults.fallback.join(", ");
    }
    addModel(currentProvider, (cfg && cfg.agents && cfg.agents.defaults && cfg.agents.defaults.model) || "");
    for (var i = 0; i < providers.length; i++) {
      var p = providers[i];
      if (p.default_model) addModel(p.name, p.default_model);
      if (Array.isArray(p.models)) {
        for (var j = 0; j < p.models.length; j++) addModel(p.name, p.models[j]);
      }
    }
    state.models = models;
  } catch (_) {
    state.models = [];
  }
  renderModelSelect();
  renderChatProviderSelect();
  var selectedModel = $("modelSelect").value;
  if (selectedModel) $("modelInput").value = selectedModel;
}

function renderModelSelect() {
  var sel = $("modelSelect");
  var previousValue = sel.value;
  var providerFilter = String(state.chatProvider || "").trim();
  var models = providerFilter
    ? state.models.filter(function(item) { return item.provider === providerFilter; })
    : state.models.slice();

  sel.innerHTML = "";
  var auto = document.createElement("option");
  auto.value = "";
  auto.textContent = t("defaultModel");
  sel.appendChild(auto);
  for (var i = 0; i < models.length; i++) {
    var item = models[i];
    var opt = document.createElement("option");
    opt.value = item.model;
    opt.dataset.provider = item.provider;
    opt.textContent = providerFilter ? item.model : (item.model + " (" + item.provider + ")");
    sel.appendChild(opt);
  }

  if (previousValue && models.some(function(item) { return item.model === previousValue; })) {
    sel.value = previousValue;
  } else if (models.length > 0) {
    sel.value = models[0].model;
  } else {
    sel.value = "";
  }
}

function renderChatProviderSelect() {
  var sel = $("chatProviderSelect");
  if (!sel) return;
  sel.innerHTML = "";

  var auto = document.createElement("option");
  auto.value = "";
  auto.textContent = t("defaultProvider");
  sel.appendChild(auto);

  for (var i = 0; i < state.providers.length; i++) {
    var opt = document.createElement("option");
    opt.value = state.providers[i].name;
    opt.textContent = state.providers[i].name;
    sel.appendChild(opt);
  }

  var before = state.chatProvider;
  var target = state.chatProvider || state.defaultProvider || "";
  if (target) sel.value = target;
  state.chatProvider = sel.value || "";
  if (before !== state.chatProvider) renderModelSelect();
}

/* ========== Tool Sessions ========== */
function isVisibleToolSession(item) {
  if (!item) return false;
  var st = String(item.state || "").toLowerCase();
  return st !== "archived";
}

function getToolSessionRawByID(id) {
  return state.toolSessionsRaw.find(function(item) { return item.id === id; }) || null;
}

function getToolSessionByID(id) {
  return state.toolSessions.find(function(item) { return item.id === id; }) || null;
}

function getToolTitle(item) {
  if (!item) return "-";
  return String(item.title || item.tool || item.id || "-").trim() || "-";
}

function getToolAccessRecord(id) {
  if (!id) return null;
  var rec = state.toolAccessRecords[id];
  if (!rec || typeof rec !== "object") return null;
  if (!rec.url || !rec.password) return null;
  return rec;
}

function setToolAccessRecord(id, mode, url, password) {
  if (!id || !url || !password) return;
  state.toolAccessRecords[id] = {
    mode: mode || "",
    url: url,
    password: password,
    updated_at: Date.now()
  };
  saveToolAccessRecords();
}

function ensureToolBuffer(id) {
  if (!id) return null;
  if (!state.toolBuffers[id]) {
    state.toolBuffers[id] = {
      offset: 0,
      chunks: [],
      running: false,
      exitCode: 0,
      missing: false,
      error: "",
      wsConnected: false
    };
  }
  return state.toolBuffers[id];
}

function toolPanelByPrefix(prefix) {
  return state.toolTerminalPanels[prefix] || null;
}

function buildToolTerminalTheme() {
  var css = getComputedStyle(document.documentElement);
  return {
    background: css.getPropertyValue("--input-bg").trim(),
    foreground: css.getPropertyValue("--text").trim(),
    cursor: css.getPropertyValue("--accent").trim(),
    cursorAccent: css.getPropertyValue("--panel").trim(),
    selectionBackground: css.getPropertyValue("--accent-bg").trim()
  };
}

function applyToolTerminalTheme() {
  if (!state.toolTerminalEnabled) return;
  var theme = buildToolTerminalTheme();
  ["Primary", "Split"].forEach(function(prefix) {
    var panel = toolPanelByPrefix(prefix);
    if (!panel || !panel.term) return;
    panel.term.options.theme = theme;
    if (panel.fitAddon) {
      try { panel.fitAddon.fit(); } catch (_) {}
    }
  });
}

function initToolTerminals() {
  if (!window.Terminal) return;
  state.toolTerminalEnabled = true;
  $("tab-tools").classList.add("tool-xterm-mode");
  ["Primary", "Split"].forEach(function(prefix) {
    var outputEl = prefix === "Primary" ? $("toolOutputPrimary") : $("toolOutputSplit");
    if (!outputEl) return;
    outputEl.innerHTML = "";
    var term = new window.Terminal({
      convertEol: true,
      cursorBlink: true,
      fontFamily: "'SF Mono', 'Fira Code', Consolas, monospace",
      fontSize: 12,
      scrollback: 10000
    });
    var fitAddon = null;
    if (window.FitAddon && window.FitAddon.FitAddon) {
      fitAddon = new window.FitAddon.FitAddon();
      term.loadAddon(fitAddon);
    }
    if (window.WebLinksAddon && window.WebLinksAddon.WebLinksAddon) {
      try {
        term.loadAddon(new window.WebLinksAddon.WebLinksAddon());
      } catch (_) {}
    }
    if (window.Unicode11Addon && window.Unicode11Addon.Unicode11Addon) {
      try {
        term.loadAddon(new window.Unicode11Addon.Unicode11Addon());
        if (term.unicode) term.unicode.activeVersion = "11";
      } catch (_) {}
    }
    term.open(outputEl);
    state.toolTerminalPanels[prefix] = {
      term: term,
      fitAddon: fitAddon,
      sessionID: "",
      renderedChunks: 0
    };

    term.attachCustomKeyEventHandler(function(ev) {
      var isMac = navigator.platform.toUpperCase().indexOf("MAC") >= 0;
      var ctrlOrMeta = isMac ? ev.metaKey : ev.ctrlKey;
      if (!ctrlOrMeta || !ev.shiftKey) return true;
      var key = String(ev.key || "").toLowerCase();
      if (key === "c") {
        var selected = term.getSelection();
        if (!selected) return true;
        copyTextToClipboard(selected);
        return false;
      }
      if (key === "v") {
        if (navigator.clipboard && navigator.clipboard.readText) {
          navigator.clipboard.readText().then(function(text) {
            if (text) queueToolInput(state.toolTerminalPanels[prefix].sessionID, text);
          }).catch(function() {});
        }
        return false;
      }
      return true;
    });

    term.onData(function(data) {
      var panel = toolPanelByPrefix(prefix);
      if (!panel || !panel.sessionID) return;
      queueToolInput(panel.sessionID, data);
    });

    term.onResize(function(size) {
      var panel = toolPanelByPrefix(prefix);
      if (!panel || !panel.sessionID) return;
      sendToolResize(panel.sessionID, size.cols, size.rows);
    });

    outputEl.addEventListener("click", function() {
      var panel = toolPanelByPrefix(prefix);
      if (!panel || !panel.term) return;
      panel.term.focus();
      if (panel.sessionID && prefix === "Split") {
        state.activeToolTab = panel.sessionID;
        renderToolTabs();
      }
    });
  });
  applyToolTerminalTheme();
  window.addEventListener("resize", function() {
    ["Primary", "Split"].forEach(function(prefix) {
      var panel = toolPanelByPrefix(prefix);
      if (!panel || !panel.fitAddon) return;
      try { panel.fitAddon.fit(); } catch (_) {}
    });
  });
}

function applyToolLayoutState() {
  var layout = $("toolsLayout");
  if (!layout) return;
  layout.classList.remove("tools-layout-maximized", "tools-layout-max-primary", "tools-layout-max-split");
  if (!state.toolMaximized) return;
  layout.classList.add("tools-layout-maximized");
  if (state.toolMaximized === "Split") layout.classList.add("tools-layout-max-split");
  else layout.classList.add("tools-layout-max-primary");
}

function renderToolMaximizeButtons() {
  var max = state.toolMaximized;
  var primaryBtn = $("toolPrimaryMaxBtn");
  var splitBtn = $("toolSplitMaxBtn");
  if (primaryBtn) primaryBtn.textContent = t(max === "Primary" ? "exitFullScreen" : "maximize");
  if (splitBtn) splitBtn.textContent = t(max === "Split" ? "exitFullScreen" : "maximize");
}

function toggleToolMaximize(prefix) {
  if (!prefix) return;
  if (prefix === "Split" && (!state.splitToolTab || state.splitToolTab === state.activeToolTab)) return;
  state.toolMaximized = state.toolMaximized === prefix ? "" : prefix;
  renderToolMaximizeButtons();
  renderToolPanels();
}

function refreshVisibleToolSessions() {
  state.terminatedToolSessions = state.toolSessionsRaw.filter(function(item) {
    return String(item.state || "").toLowerCase() === "terminated";
  }).length;
  state.toolSessions = state.toolSessionsRaw.filter(isVisibleToolSession);
}

async function loadToolSessions() {
  try {
    var data = await api("/api/tool-sessions?limit=200");
    state.toolSessionsRaw = Array.isArray(data) ? data : [];
  } catch (_) {
    state.toolSessionsRaw = [];
    state.toolSessions = [];
  }
  refreshVisibleToolSessions();
  var knownIDs = new Set(state.toolSessionsRaw.map(function(item) { return item.id; }));
  Object.keys(state.toolAccessRecords || {}).forEach(function(id) {
    if (!knownIDs.has(id)) delete state.toolAccessRecords[id];
  });
  saveToolAccessRecords();

  state.toolTabs = state.toolTabs.filter(function(id) {
    return !!getToolSessionRawByID(id);
  });
  closeUnusedToolSockets();
  if (state.activeToolTab && !getToolSessionRawByID(state.activeToolTab)) state.activeToolTab = "";
  if (state.splitToolTab && !getToolSessionRawByID(state.splitToolTab)) state.splitToolTab = "";
  if (!state.activeToolTab && state.toolSessions.length > 0) {
    state.activeToolTab = state.toolSessions[0].id;
    if (state.toolTabs.indexOf(state.activeToolTab) === -1) state.toolTabs.push(state.activeToolTab);
  }
  if (state.pendingToolSessionID && getToolSessionRawByID(state.pendingToolSessionID)) {
    openToolTab(state.pendingToolSessionID, false);
    state.pendingToolSessionID = "";
    if ((new URLSearchParams(location.search || "")).get("tab") === "tools") {
      document.querySelectorAll(".tab").forEach(function(t) { t.classList.remove("active"); });
      var toolsTabBtn = document.querySelector('.tab[data-tab="tools"]');
      if (toolsTabBtn) toolsTabBtn.classList.add("active");
      ["chat", "tools", "providers", "channels", "config", "status"].forEach(function(name) {
        $("tab-" + name).classList.toggle("hidden", name !== "tools");
      });
    }
  }
  renderToolSessionsList();
  renderToolTabs();
  renderToolPanels();
}

function renderToolSessionsList() {
  var list = $("toolSessionsList");
  if (!list) return;
  list.innerHTML = "";
  var cleanupBtn = $("cleanupToolSessionsBtn");
  if (cleanupBtn) {
    cleanupBtn.disabled = state.terminatedToolSessions === 0;
    cleanupBtn.textContent = t("cleanupTerminatedCount", state.terminatedToolSessions);
  }

  if (!state.toolSessions.length) {
    var empty = document.createElement("div");
    empty.className = "msg-text";
    empty.textContent = t("noToolSessions");
    list.appendChild(empty);
    return;
  }

  for (var i = 0; i < state.toolSessions.length; i++) {
    var item = state.toolSessions[i];
    var row = document.createElement("div");
    row.className = "tool-session-item" + (item.id === state.activeToolTab ? " active" : "");
    row.dataset.toolSelect = item.id;
    var title = getToolTitle(item);
    var stateText = item.state || "-";
    var running = stateText === "running";
    var accessEnabled = String(item.access_mode || "").trim().toLowerCase() !== "none";
    var accessRecord = getToolAccessRecord(item.id);
    var accessButtonHTML = accessEnabled
      ? ('<button class="btn btn-sm" data-tool-access="' + item.id + '">' + t(accessRecord ? "copyAccess" : "refreshAccess") + '</button>')
      : "";
    row.innerHTML =
      '<div class="tool-session-item-head">' +
        '<span class="tool-session-title">' + title + '</span>' +
        '<span class="badge ' + (running ? "badge-on" : "badge-off") + '">' + stateText + '</span>' +
      '</div>' +
      '<div class="tool-session-meta">' + (item.tool || "-") + " · " + (item.command || "-") + '</div>' +
      '<div class="tool-session-actions">' +
        accessButtonHTML +
        '<button class="btn btn-sm" data-tool-edit="' + item.id + '">' + t("modify") + '</button>' +
        (running ? ('<button class="btn btn-sm btn-danger" data-tool-kill="' + item.id + '">' + t("kill") + '</button>') : "") +
      '</div>';
    list.appendChild(row);
  }
}

function buildToolSocketURL(sessionID) {
  var proto = location.protocol === "https:" ? "wss" : "ws";
  return proto + "://" + location.host + "/api/tool-sessions/ws?token=" +
    encodeURIComponent(state.token) + "&session_id=" + encodeURIComponent(sessionID);
}

function closeToolSocket(sessionID) {
  var ws = state.toolSockets[sessionID];
  if (!ws) return;
  delete state.toolSockets[sessionID];
  try { ws.close(); } catch (_) {}
  var buf = ensureToolBuffer(sessionID);
  if (buf) buf.wsConnected = false;
}

function closeUnusedToolSockets() {
  var keep = new Set(state.toolTabs);
  if (state.activeToolTab) keep.add(state.activeToolTab);
  if (state.splitToolTab) keep.add(state.splitToolTab);
  Object.keys(state.toolSockets).forEach(function(id) {
    if (!keep.has(id)) closeToolSocket(id);
  });
}

function sendToolResize(sessionID, cols, rows) {
  if (!sessionID || cols <= 0 || rows <= 0) return;
  var ws = state.toolSockets[sessionID];
  if (!ws || ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type: "resize", cols: cols, rows: rows }));
}

function openToolSocket(sessionID) {
  if (!sessionID || !state.token) return;
  if (state.toolSockets[sessionID]) return;
  var ws;
  try { ws = new WebSocket(buildToolSocketURL(sessionID)); } catch (_) { return; }
  state.toolSockets[sessionID] = ws;
  ws.onopen = function() {
    var buf = ensureToolBuffer(sessionID);
    if (!buf) return;
    buf.wsConnected = true;
    buf.error = "";
    ["Primary", "Split"].forEach(function(prefix) {
      var panel = toolPanelByPrefix(prefix);
      if (!panel || panel.sessionID !== sessionID || !panel.term) return;
      sendToolResize(sessionID, panel.term.cols || 0, panel.term.rows || 0);
    });
    renderToolPanels();
  };
  ws.onmessage = function(ev) {
    var msg = {};
    try { msg = JSON.parse(ev.data || "{}"); } catch (_) { return; }
    var buf = ensureToolBuffer(sessionID);
    if (!buf) return;
    if (msg.type === "output") {
      var data = String(msg.data || "");
      if (data) {
        buf.chunks.push(data);
        if (buf.chunks.length > 4000) buf.chunks = buf.chunks.slice(buf.chunks.length - 4000);
      }
      if (typeof msg.total === "number") buf.offset = msg.total;
      renderToolPanels();
      return;
    }
    if (msg.type === "status") {
      buf.running = !!msg.running;
      buf.missing = !!msg.missing;
      if (typeof msg.exit_code === "number") buf.exitCode = msg.exit_code;
      if (!buf.running && !buf.missing) {
        var item = getToolSessionRawByID(sessionID);
        if (item) item.state = "terminated";
        refreshVisibleToolSessions();
      }
      renderToolSessionsList();
      renderToolPanels();
      return;
    }
    if (msg.type === "error") {
      buf.error = msg.message || "error";
      renderToolPanels();
    }
  };
  ws.onerror = function() {
    var buf = ensureToolBuffer(sessionID);
    if (buf) {
      buf.wsConnected = false;
      if (!buf.error) buf.error = t("toolSocketError");
    }
    renderToolPanels();
  };
  ws.onclose = function() {
    delete state.toolSockets[sessionID];
    var buf = ensureToolBuffer(sessionID);
    if (buf) buf.wsConnected = false;
    renderToolPanels();
  };
}

function openToolTab(sessionID, asSplit) {
  if (!sessionID) return;
  if (state.toolTabs.indexOf(sessionID) === -1) state.toolTabs.push(sessionID);
  openToolSocket(sessionID);
  if (asSplit) {
    if (!state.activeToolTab) state.activeToolTab = sessionID;
    state.splitToolTab = sessionID;
    if (state.activeToolTab === sessionID) {
      var fallback = "";
      for (var i = 0; i < state.toolTabs.length; i++) {
        if (state.toolTabs[i] !== sessionID) { fallback = state.toolTabs[i]; break; }
      }
      if (!fallback) {
        for (var j = 0; j < state.toolSessions.length; j++) {
          if (state.toolSessions[j].id !== sessionID) { fallback = state.toolSessions[j].id; break; }
        }
      }
      if (fallback) {
        state.activeToolTab = fallback;
        if (state.toolTabs.indexOf(fallback) === -1) state.toolTabs.push(fallback);
        openToolSocket(fallback);
      } else {
        state.splitToolTab = "";
        showToast(t("splitRequiresAnotherSession"), "warning");
      }
    }
  } else {
    state.activeToolTab = sessionID;
  }
  if (!state.activeToolTab) state.activeToolTab = sessionID;
  renderToolSessionsList();
  renderToolTabs();
  renderToolPanels();
}

function closeToolTab(sessionID) {
  state.toolTabs = state.toolTabs.filter(function(id) { return id !== sessionID; });
  if (state.activeToolTab === sessionID) state.activeToolTab = state.toolTabs[0] || "";
  if (state.splitToolTab === sessionID) state.splitToolTab = "";
  if (state.splitToolTab && state.splitToolTab === state.activeToolTab) state.splitToolTab = "";
  closeUnusedToolSockets();
  renderToolSessionsList();
  renderToolTabs();
  renderToolPanels();
}

function renderToolTabs() {
  var wrap = $("toolTabs");
  if (!wrap) return;
  wrap.innerHTML = "";
  for (var i = 0; i < state.toolTabs.length; i++) {
    var id = state.toolTabs[i];
    var item = getToolSessionRawByID(id);
    var tab = document.createElement("div");
    tab.className = "tool-tab" + (id === state.activeToolTab ? " active" : "");
    tab.dataset.toolTab = id;
    tab.innerHTML = '<span>' + getToolTitle(item) + '</span><button class="tool-tab-close" data-tool-close="' + id + '">×</button>';
    wrap.appendChild(tab);
  }
}

function renderToolPanel(prefix, sessionID) {
  var titleEl = prefix === "Primary" ? $("toolPrimaryTitle") : $("toolSplitTitle");
  var statusEl = prefix === "Primary" ? $("toolPrimaryStatus") : $("toolSplitStatus");
  var outputEl = prefix === "Primary" ? $("toolOutputPrimary") : $("toolOutputSplit");
  var inputEl = prefix === "Primary" ? $("toolInputPrimary") : $("toolInputSplit");
  var sendBtn = prefix === "Primary" ? $("toolSendPrimaryBtn") : $("toolSendSplitBtn");
  if (!titleEl || !statusEl || !outputEl || !inputEl || !sendBtn) return;

  if (!sessionID) {
    titleEl.textContent = t("noSessionOpened");
    statusEl.textContent = "-";
    outputEl.textContent = "";
    var emptyPanel = toolPanelByPrefix(prefix);
    if (emptyPanel && emptyPanel.term) {
      emptyPanel.term.reset();
      emptyPanel.sessionID = "";
      emptyPanel.renderedChunks = 0;
    }
    inputEl.value = "";
    inputEl.disabled = true;
    sendBtn.disabled = true;
    return;
  }

  var item = getToolSessionRawByID(sessionID);
  var buf = ensureToolBuffer(sessionID);
  openToolSocket(sessionID);
  titleEl.textContent = getToolTitle(item) + " · " + sessionID.slice(0, 8);
  if (buf.error) statusEl.textContent = buf.error;
  else if (buf.missing) statusEl.textContent = t("processMissing");
  else if (buf.running) statusEl.textContent = buf.wsConnected ? t("runningWebsocket") : t("running");
  else statusEl.textContent = t("stoppedWithCode", buf.exitCode);

  var panel = toolPanelByPrefix(prefix);
  if (state.toolTerminalEnabled && panel && panel.term) {
    if (panel.sessionID !== sessionID || panel.renderedChunks > buf.chunks.length) {
      panel.term.reset();
      panel.sessionID = sessionID;
      panel.renderedChunks = 0;
    }
    if (buf.chunks.length > panel.renderedChunks) {
      var chunk = buf.chunks.slice(panel.renderedChunks).join("");
      if (chunk) panel.term.write(chunk);
      panel.renderedChunks = buf.chunks.length;
      panel.term.scrollToBottom();
    }
    if (panel.fitAddon) {
      try { panel.fitAddon.fit(); } catch (_) {}
    }
    sendToolResize(sessionID, panel.term.cols || 0, panel.term.rows || 0);
  } else {
    outputEl.textContent = buf.chunks.join("");
    outputEl.scrollTop = outputEl.scrollHeight;
  }
  var fallbackInput = !state.toolTerminalEnabled;
  inputEl.disabled = !fallbackInput;
  sendBtn.disabled = !fallbackInput;
}

function renderToolPanels() {
  var splitPanel = $("toolPanelSplit");
  var splitWrap = document.querySelector(".tools-split");
  var clearSplitBtn = $("clearSplitBtn");
  var hasSplit = !!state.splitToolTab && state.splitToolTab !== state.activeToolTab;
  if (state.toolMaximized === "Split" && !hasSplit) state.toolMaximized = "";
  applyToolLayoutState();
  renderToolMaximizeButtons();
  if (splitWrap) splitWrap.classList.toggle("has-split", hasSplit);
  if (clearSplitBtn) clearSplitBtn.classList.toggle("hidden", !hasSplit);
  if (splitPanel) splitPanel.classList.toggle("hidden", !hasSplit);
  renderToolPanel("Primary", state.activeToolTab);
  renderToolPanel("Split", hasSplit ? state.splitToolTab : "");
  closeUnusedToolSockets();
}

async function pollToolSessionOutput(sessionID) {
  if (!sessionID) return;
  var buf = ensureToolBuffer(sessionID);
  if (buf.wsConnected) return;
  try {
    var result = await api("/api/tool-sessions/" + encodeURIComponent(sessionID) + "/process/output?offset=" + buf.offset + "&limit=400");
    var chunks = Array.isArray(result.lines) ? result.lines : [];
    if (chunks.length > 0) {
      buf.chunks = buf.chunks.concat(chunks);
      if (buf.chunks.length > 4000) buf.chunks = buf.chunks.slice(buf.chunks.length - 4000);
    }
    if (typeof result.total === "number") buf.offset = result.total;
    buf.running = !!result.running;
    buf.missing = !!result.missing;
    if (typeof result.exit_code === "number") buf.exitCode = result.exit_code;
    buf.error = "";
  } catch (err) {
    buf.error = err.message || "error";
  }
}

async function pollActiveToolOutputs() {
  var ids = [];
  if (state.activeToolTab) ids.push(state.activeToolTab);
  if (state.splitToolTab && state.splitToolTab !== state.activeToolTab) ids.push(state.splitToolTab);
  if (!ids.length) return;
  await Promise.all(ids.map(function(id) { return pollToolSessionOutput(id); }));
  renderToolPanels();
}

function startToolPoller() {
  stopToolPoller();
  state.toolPollTimer = setInterval(async function() {
    if (!state.token) return;
    if (state.toolPollBusy) return;
    state.toolPollBusy = true;
    try {
      await pollActiveToolOutputs();
      state.toolReloadTicker++;
      if (state.toolReloadTicker % 8 === 0) {
        await loadToolSessions();
      }
    } finally {
      state.toolPollBusy = false;
    }
  }, 1200);
  pollActiveToolOutputs();
}

function stopToolPoller() {
  if (!state.toolPollTimer) return;
  clearInterval(state.toolPollTimer);
  state.toolPollTimer = null;
  state.toolPollBusy = false;
}

async function sendToolRawInput(sessionID, data) {
  if (!sessionID || !data) return;
  var ws = state.toolSockets[sessionID];
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: "input", data: data }));
    return;
  }
  await api("/api/tool-sessions/" + encodeURIComponent(sessionID) + "/process/input", {
    method: "POST",
    body: JSON.stringify({ data: data })
  });
}

function queueToolInput(sessionID, data) {
  if (!sessionID || !data) return;
  state.toolInputQueue[sessionID] = (state.toolInputQueue[sessionID] || "") + data;
  flushToolInput(sessionID);
}

async function flushToolInput(sessionID) {
  if (!sessionID) return;
  if (state.toolInputSending[sessionID]) return;
  var payload = state.toolInputQueue[sessionID];
  if (!payload) return;
  state.toolInputQueue[sessionID] = "";
  state.toolInputSending[sessionID] = true;
  try {
    await sendToolRawInput(sessionID, payload);
    var buf = ensureToolBuffer(sessionID);
    if (buf) buf.error = "";
  } catch (err) {
    var failed = ensureToolBuffer(sessionID);
    if (failed) failed.error = err.message || "error";
    renderToolPanels();
  } finally {
    state.toolInputSending[sessionID] = false;
    if (state.toolInputQueue[sessionID]) flushToolInput(sessionID);
  }
}

async function sendToolInput(isSplit) {
  var sessionID = isSplit ? state.splitToolTab : state.activeToolTab;
  if (!sessionID) return;
  var inputEl = isSplit ? $("toolInputSplit") : $("toolInputPrimary");
  if (!inputEl) return;
  var text = inputEl.value;
  if (!text.trim()) return;
  try {
    await sendToolRawInput(sessionID, text + "\n");
    inputEl.value = "";
    await pollToolSessionOutput(sessionID);
    renderToolPanels();
  } catch (err) {
    var buf = ensureToolBuffer(sessionID);
    buf.error = err.message || "error";
    renderToolPanels();
  }
}

async function killToolSession(sessionID) {
  if (!sessionID) return;
  if (!confirm(t("killConfirm"))) return;
  try {
    await api("/api/tool-sessions/" + encodeURIComponent(sessionID) + "/process/kill", { method: "POST" });
    closeToolSocket(sessionID);
    await loadToolSessions();
    await pollActiveToolOutputs();
  } catch (_) {}
}

async function startToolSession(sessionID) {
  if (!sessionID) return;
  try {
    var created = await api("/api/tool-sessions/" + encodeURIComponent(sessionID) + "/restart", {
      method: "POST",
      body: JSON.stringify({})
    });
    var session = created && created.session ? created.session : null;
    if (session && session.id) {
      state.toolSessionsRaw = [session].concat(state.toolSessionsRaw.filter(function(item) {
        return item && item.id !== session.id;
      }));
      refreshVisibleToolSessions();
    }
    openToolTab(sessionID, false);
    await pollToolSessionOutput(sessionID);
    await loadToolSessions();
    renderToolPanels();
  } catch (err) {
    showToast((err && err.message) ? err.message : t("restartSessionFailed"), "error");
  }
}

function openToolSessionDialog() {
  state.toolSessionDialogMode = "new";
  state.editingToolSessionID = "";
  var draft = state.toolSessionDraft || {};
  var tool = String(draft.tool || "codex").trim();
  var toolSelect = $("toolSessionTool");
  var hasPreset = false;
  for (var i = 0; i < toolSelect.options.length; i++) {
    if (toolSelect.options[i].value === tool) {
      hasPreset = true;
      break;
    }
  }
  if (hasPreset) {
    toolSelect.value = tool;
    $("toolSessionToolCustom").value = "";
  } else {
    toolSelect.value = "__custom__";
    $("toolSessionToolCustom").value = tool;
  }
  $("toolSessionTitle").value = "";
  $("toolSessionCommand").value = String(draft.command_args || "").trim();
  $("toolSessionWorkdir").value = getDefaultToolSessionWorkdir();
  $("toolSessionProxyMode").value = String(draft.proxy_mode || "inherit").trim() || "inherit";
  $("toolSessionProxyUrl").value = String(draft.proxy_url || "").trim();
  $("toolSessionAccessMode").value = String(draft.access_mode || "none").trim() || "none";
  $("toolSessionAccessPassword").value = "";
  $("toolSessionPublicBaseUrl").value = String(draft.public_base_url || "").trim();
  $("toolSessionDialogTitle").textContent = t("newToolSessionTitle");
  $("toolSessionDialogCreateBtn").textContent = t("createSession");
  updateToolSessionToolMode();
  updateToolSessionProxyMode();
  $("toolSessionDialog").classList.remove("hidden");
  setTimeout(function() {
    if ($("toolSessionTitle").value.trim()) $("toolSessionTitle").select();
    $("toolSessionTitle").focus();
  }, 0);
}

function openToolSessionDialogForEdit(sessionID) {
  var item = getToolSessionRawByID(sessionID);
  if (!item) return;
  state.toolSessionDialogMode = "edit";
  state.editingToolSessionID = sessionID;
  var tool = String(item.tool || "").trim();
  $("toolSessionTool").value = "codex";
  $("toolSessionToolCustom").value = "";
  $("toolSessionPublicBaseUrl").value = "";
  var toolSelect = $("toolSessionTool");
  var hasPreset = false;
  for (var i = 0; i < toolSelect.options.length; i++) {
    if (toolSelect.options[i].value === tool) {
      hasPreset = true;
      break;
    }
  }
  if (hasPreset) {
    toolSelect.value = tool;
    $("toolSessionToolCustom").value = "";
  } else {
    toolSelect.value = "__custom__";
    $("toolSessionToolCustom").value = tool;
  }
  $("toolSessionTitle").value = String(item.title || "").trim();
  var metadata = item.metadata && typeof item.metadata === "object" ? item.metadata : {};
  var args = String(metadata.user_args || "").trim();
  if (!args) {
    var command = String(item.command || "").trim();
    if (!command) command = String(metadata.user_command || "").trim();
    args = inferCommandArgs(tool, command);
  }
  $("toolSessionCommand").value = args;
  $("toolSessionWorkdir").value = String(item.workdir || "").trim();
  $("toolSessionProxyMode").value = String(metadata.proxy_mode || "inherit").trim() || "inherit";
  $("toolSessionProxyUrl").value = String(metadata.proxy_url || "").trim();
  $("toolSessionAccessMode").value = String(item.access_mode || "none").trim() || "none";
  $("toolSessionAccessPassword").value = "";
  $("toolSessionDialogTitle").textContent = t("editToolSessionTitle");
  $("toolSessionDialogCreateBtn").textContent = t("save");
  updateToolSessionToolMode();
  updateToolSessionProxyMode();
  $("toolSessionDialog").classList.remove("hidden");
  setTimeout(function() { $("toolSessionTitle").focus(); }, 0);
}

function closeToolSessionDialog() {
  $("toolSessionDialog").classList.add("hidden");
  state.toolSessionDialogMode = "new";
  state.editingToolSessionID = "";
}

function openToolAccessDialog(sessionID, accessURL, accessPassword) {
  state.toolAccessDialogSessionID = sessionID || "";
  var item = getToolSessionRawByID(state.toolAccessDialogSessionID);
  var hint = $("toolAccessSessionHint");
  if (hint) hint.textContent = item ? (getToolTitle(item) + " · " + state.toolAccessDialogSessionID.slice(0, 8)) : "";
  $("toolAccessUrlInput").value = accessURL || "";
  $("toolAccessPasswordInput").value = accessPassword || "";
  $("toolAccessOtpInput").value = "";
  renderOTPCircle(0, "-");
  $("toolAccessDialog").classList.remove("hidden");
  if (sessionID) refreshToolSessionOTP(sessionID);
}

function closeToolAccessDialog() {
  stopToolOTPTimer();
  state.toolAccessOTP = null;
  state.toolAccessDialogSessionID = "";
  $("toolAccessDialog").classList.add("hidden");
}

async function showToolSessionAccess(sessionID, forceRefresh) {
  if (!sessionID) return;
  var item = getToolSessionRawByID(sessionID);
  if (!item) return;
  var record = getToolAccessRecord(sessionID);
  if (record && !forceRefresh) {
    openToolAccessDialog(sessionID, record.url, record.password);
    return;
  }
  var mode = String(item.access_mode || "").trim();
  if (!mode || mode === "none") {
    showToast(t("externalAccessDisabled"), "warning");
    return;
  }
  try {
    var payload = await api("/api/tool-sessions/" + encodeURIComponent(sessionID) + "/access", {
      method: "POST",
      body: JSON.stringify({ mode: mode, password: "" })
    });
    var accessURL = payload.access_url || "";
    var accessPassword = payload.access_password || "";
    var nextMode = payload.access_mode || mode;
    item.access_mode = nextMode;
    if (!accessURL || !accessPassword) throw new Error(t("accessNotAvailable"));
    setToolAccessRecord(sessionID, nextMode, accessURL, accessPassword);
    openToolAccessDialog(sessionID, accessURL, accessPassword);
    renderToolSessionsList();
  } catch (err) {
    showToast((err && err.message) ? err.message : t("accessNotAvailable"), "error");
  }
}

function stopToolOTPTimer() {
  if (!state.toolAccessOTPTimer) return;
  clearInterval(state.toolAccessOTPTimer);
  state.toolAccessOTPTimer = null;
}

function renderOTPCircle(progress, text) {
  var ring = $("toolAccessOtpRing");
  var label = $("toolAccessOtpCountdown");
  if (!ring || !label) return;
  var deg = Math.max(0, Math.min(360, Math.round(progress * 360)));
  ring.style.setProperty("--otp-progress", deg + "deg");
  label.textContent = text || "-";
}

function startToolOTPTimer(expiresAtMs, ttlMs) {
  stopToolOTPTimer();
  var totalWindow = Math.max(1000, ttlMs || 180000);
  function tick() {
    var leftMs = Math.max(0, expiresAtMs - Date.now());
    if (leftMs <= 0) {
      renderOTPCircle(0, t("expired"));
      stopToolOTPTimer();
      return;
    }
    var progress = leftMs / totalWindow;
    var seconds = Math.ceil(leftMs / 1000);
    renderOTPCircle(progress, seconds + "s");
  }
  tick();
  state.toolAccessOTPTimer = setInterval(tick, 250);
}

async function refreshToolSessionOTP(sessionID) {
  if (!sessionID) return;
  try {
    var payload = await api("/api/tool-sessions/" + encodeURIComponent(sessionID) + "/otp", {
      method: "POST",
      body: JSON.stringify({})
    });
    var otp = String(payload.otp_code || "").trim();
    var expiresAt = Number(payload.expires_at || 0) * 1000;
    var ttlMs = Math.max(1000, Number(payload.ttl_seconds || 180) * 1000);
    if (!otp || !expiresAt) throw new Error(t("otpUnavailable"));
    state.toolAccessOTP = { code: otp, expiresAt: expiresAt };
    $("toolAccessOtpInput").value = otp;
    startToolOTPTimer(expiresAt, ttlMs);
  } catch (err) {
    state.toolAccessOTP = null;
    $("toolAccessOtpInput").value = "";
    renderOTPCircle(0, t("expired"));
    showToast((err && err.message) ? err.message : t("otpUnavailable"), "error");
  }
}

async function copyTextToClipboard(text) {
  var value = String(text || "");
  if (!value) return false;
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch (_) {}
  var temp = document.createElement("textarea");
  temp.value = value;
  temp.setAttribute("readonly", "");
  temp.style.position = "fixed";
  temp.style.opacity = "0";
  document.body.appendChild(temp);
  temp.select();
  var ok = false;
  try { ok = document.execCommand("copy"); } catch (_) {}
  document.body.removeChild(temp);
  return ok;
}

function updateToolSessionToolMode() {
  var custom = $("toolSessionTool").value === "__custom__";
  $("toolSessionCustomToolField").classList.toggle("hidden", !custom);
  if (custom) setTimeout(function() { $("toolSessionToolCustom").focus(); }, 0);
}

function updateToolSessionProxyMode() {
  var mode = String($("toolSessionProxyMode").value || "inherit").trim().toLowerCase();
  var isCustom = mode === "custom";
  $("toolSessionProxyUrlField").classList.toggle("hidden", !isCustom);
  if (!isCustom) return;
  setTimeout(function() { $("toolSessionProxyUrl").focus(); }, 0);
}

function inferCommandArgs(toolValue, commandValue) {
  var tool = String(toolValue || "").trim();
  var command = String(commandValue || "").trim();
  if (!command) return "";
  if (!tool) return command;
  if (command === tool) return "";
  var prefix = tool + " ";
  if (command.indexOf(prefix) === 0) return command.slice(prefix.length).trim();
  return command;
}

function getToolSessionToolValue() {
  var selected = $("toolSessionTool").value.trim();
  if (selected === "__custom__") return $("toolSessionToolCustom").value.trim();
  return selected;
}

function getDefaultToolSessionWorkdir() {
  var draft = state.toolSessionDraft || {};
  var byDraft = String(draft.workdir || "").trim();
  if (byDraft) return byDraft;
  var byConfig = String((state.loadedConfig && state.loadedConfig.agents && state.loadedConfig.agents.defaults && state.loadedConfig.agents.defaults.workspace) || "").trim();
  if (byConfig) return byConfig;
  for (var i = 0; i < state.toolSessionsRaw.length; i++) {
    var wd = String(state.toolSessionsRaw[i].workdir || "").trim();
    if (wd) return wd;
  }
  return "";
}

async function createToolSession() {
  var tool = getToolSessionToolValue();
  if (!tool) return;
  var editingID = state.toolSessionDialogMode === "edit" ? String(state.editingToolSessionID || "").trim() : "";
  var proxyMode = String($("toolSessionProxyMode").value || "inherit").trim().toLowerCase();
  if (proxyMode !== "clear" && proxyMode !== "custom") proxyMode = "inherit";
  var proxyURL = $("toolSessionProxyUrl").value.trim();
  if (proxyMode !== "custom") proxyURL = "";
  var payload = {
    tool: tool,
    title: $("toolSessionTitle").value.trim(),
    command_args: $("toolSessionCommand").value.trim(),
    workdir: $("toolSessionWorkdir").value.trim(),
    proxy_mode: proxyMode,
    proxy_url: proxyURL,
    access_mode: $("toolSessionAccessMode").value.trim(),
    access_password: $("toolSessionAccessPassword").value.trim(),
    public_base_url: $("toolSessionPublicBaseUrl").value.trim()
  };
  if (proxyMode === "custom" && !proxyURL) {
    showToast(t("proxyUrlRequired"), "warning");
    return;
  }
  state.toolSessionDraft = {
    tool: tool,
    command_args: payload.command_args,
    workdir: payload.workdir,
    proxy_mode: payload.proxy_mode,
    proxy_url: payload.proxy_url,
    access_mode: payload.access_mode,
    public_base_url: payload.public_base_url
  };
  saveToolSessionDraft(state.toolSessionDraft);
  try {
    var endpoint = editingID
      ? ("/api/tool-sessions/" + encodeURIComponent(editingID))
      : "/api/tool-sessions/spawn";
    var method = editingID ? "PUT" : "POST";
    var created = await api(endpoint, { method: method, body: JSON.stringify(payload) });
    var session = created && created.session ? created.session : null;
    if (!session || !session.id) throw new Error(editingID ? "session update failed" : "session create failed");
    closeToolSessionDialog();
    state.toolSessionsRaw = [session].concat(state.toolSessionsRaw.filter(function(item) {
      return item && item.id !== session.id;
    }));
    refreshVisibleToolSessions();
    renderToolSessionsList();
    if (!editingID) {
      openToolTab(session.id, false);
      await pollToolSessionOutput(session.id);
    }
    await loadToolSessions();
    renderToolPanels();
    if (created.access_url && created.access_password) {
      setToolAccessRecord(session.id, created.access_mode || payload.access_mode || "", created.access_url, created.access_password);
      openToolAccessDialog(session.id, created.access_url, created.access_password);
    }
    if (editingID) showToast(t("saved"), "success");
  } catch (err) {
    showToast((err && err.message) ? err.message : t(editingID ? "saveSessionFailed" : "createSessionFailed"), "error");
  }
}

function normalizeModelList(list) {
  var out = [];
  var seen = new Set();
  if (!Array.isArray(list)) return out;
  for (var i = 0; i < list.length; i++) {
    var model = String(list[i] || "").trim();
    if (!model || seen.has(model)) continue;
    seen.add(model);
    out.push(model);
  }
  return out;
}

function getProviderModelCatalogKey(payload) {
  var name = payload && payload.name ? String(payload.name).trim() : "";
  if (!name) name = String(state.editingProviderName || "").trim();
  return name || "__new__";
}

function updateProviderModelPicker(payload) {
  var key = getProviderModelCatalogKey(payload);
  var catalog = normalizeModelList(state.providerModelCatalog[key]);
  var selected = normalizeModelList(payload && payload.models);
  var options = normalizeModelList(catalog.concat(selected));
  var defaultModel = String((payload && payload.default_model) || "").trim();
  if (!selected.length && defaultModel && options.includes(defaultModel)) selected = [defaultModel];
  state.providerModelOptions = options;
  state.providerModelSelected = selected;
  state.providerModelFilter = "";
  var filterInput = $("providerModelFilter");
  if (filterInput) filterInput.value = "";
  renderProviderModelPicker();
}

function renderProviderModelPicker() {
  var picker = $("providerModelPicker");
  var manualRow = $("providerManualModelRow");
  if (!picker || !manualRow) return;

  var hasOptions = state.providerModelOptions.length > 0;
  picker.classList.toggle("hidden", !hasOptions);
  manualRow.classList.toggle("hidden", hasOptions);

  var counter = $("providerModelCounter");
  if (!hasOptions) {
    if (counter) counter.textContent = "";
    return;
  }

  var selectedSet = new Set(state.providerModelSelected);
  var select = $("providerModelSelect");
  if (!select) return;
  select.innerHTML = "";

  var filter = String(state.providerModelFilter || "").trim().toLowerCase();
  var visibleCount = 0;
  for (var i = 0; i < state.providerModelOptions.length; i++) {
    var model = state.providerModelOptions[i];
    if (filter && model.toLowerCase().indexOf(filter) === -1) continue;
    var opt = document.createElement("option");
    opt.value = model;
    opt.textContent = model;
    opt.selected = selectedSet.has(model);
    select.appendChild(opt);
    visibleCount++;
  }

  if (counter) {
    counter.textContent = t("providerModelsSelected", selectedSet.size, state.providerModelOptions.length, visibleCount);
  }
}

async function cleanupTerminatedToolSessions() {
  if (state.terminatedToolSessions === 0) return;
  try {
    await api("/api/tool-sessions/cleanup-terminated", { method: "POST" });
    await loadToolSessions();
  } catch (_) {}
}

function syncProviderModelSelectionFromVisible() {
  var select = $("providerModelSelect");
  if (!select) return;
  var selectedSet = new Set(state.providerModelSelected);
  for (var i = 0; i < select.options.length; i++) {
    var option = select.options[i];
    if (option.selected) selectedSet.add(option.value);
    else selectedSet.delete(option.value);
  }
  state.providerModelSelected = state.providerModelOptions.filter(function(model) {
    return selectedSet.has(model);
  });
}

function selectFilteredProviderModels(selectAll) {
  var select = $("providerModelSelect");
  if (!select) return;
  var selectedSet = new Set(state.providerModelSelected);
  for (var i = 0; i < select.options.length; i++) {
    var value = select.options[i].value;
    if (selectAll) selectedSet.add(value);
    else selectedSet.delete(value);
  }
  state.providerModelSelected = state.providerModelOptions.filter(function(model) {
    return selectedSet.has(model);
  });
  renderProviderModelPicker();
}

function applyProviderModelSelection() {
  var payload;
  try { payload = readProviderEditor(); } catch (err) {
    setProviderMessage(t("invalidJson", err.message), true); return;
  }
  syncProviderModelSelectionFromVisible();
  payload.models = state.providerModelSelected.slice();
  var currentDefault = String(payload.default_model || "").trim();
  if (!payload.models.length) payload.default_model = "";
  else if (!currentDefault || !payload.models.includes(currentDefault)) payload.default_model = payload.models[0];
  writeProviderEditor(payload);
  setProviderMessage(t("modelsApplied", payload.models.length));
  updateProviderModelPicker(payload);
}

/* ========== Providers ========== */
function defaultProviderTemplate() {
  return { name: "new-provider", provider_kind: "openai", api_base: "", api_key: "", proxy: "", models: [], default_model: "", timeout: 60 };
}

function setProviderMessage(text, isError) {
  var el = $("providerMsg");
  el.className = "msg-text" + (isError ? " err" : "");
  el.textContent = text || "";
}

function renderProviderSelect() {
  var sel = $("providerSelect");
  sel.innerHTML = "";
  for (var i = 0; i < state.providers.length; i++) {
    var opt = document.createElement("option");
    opt.value = state.providers[i].name;
    opt.textContent = state.providers[i].name;
    sel.appendChild(opt);
  }
  if (!state.providers.length) {
    var opt = document.createElement("option");
    opt.value = "";
    opt.textContent = t("noProviders");
    sel.appendChild(opt);
  }
  renderProviderCardList();
}

function renderProviderCardList() {
  var list = $("providerCardList");
  if (!list) return;
  list.innerHTML = "";
  var emptyState = $("providerEmptyState");
  var formFields = $("providerFormFields");
  var formHeader = document.querySelector(".provider-form-header");

  if (!state.providers.length) {
    if (emptyState) emptyState.classList.remove("hidden");
    if (formFields) formFields.classList.add("hidden");
    if (formHeader) formHeader.classList.add("hidden");
    return;
  }
  if (emptyState) emptyState.classList.add("hidden");
  if (formFields) formFields.classList.remove("hidden");
  if (formHeader) formHeader.classList.remove("hidden");

  for (var i = 0; i < state.providers.length; i++) {
    var p = state.providers[i];
    var card = document.createElement("div");
    card.className = "provider-card" + (p.name === state.editingProviderName ? " active" : "");
    card.dataset.providerCard = p.name;
    var kindText = String(p.provider_kind || "openai").trim();
    var modelCount = Array.isArray(p.models) ? p.models.length : 0;
    card.innerHTML =
      '<div class="provider-card-name">' + p.name + '</div>' +
      '<div class="provider-card-meta">' + kindText + (modelCount > 0 ? (' · ' + modelCount + ' model' + (modelCount > 1 ? 's' : '')) : '') + '</div>';
    list.appendChild(card);
  }
}

function openProvider(name) {
  var provider = state.providers.find(function(p) { return p.name === name; });
  if (!provider) {
    state.editingProviderName = "";
    var emptyPayload = defaultProviderTemplate();
    $("providerEditor").value = JSON.stringify(emptyPayload, null, 2);
    $("providerModelInput").value = "";
    populateProviderForm(emptyPayload);
    updateProviderModelPicker(emptyPayload);
    updateProviderFormTitle();
    renderProviderCardList();
    return;
  }
  state.editingProviderName = provider.name;
  $("providerEditor").value = JSON.stringify(provider, null, 2);
  $("providerSelect").value = provider.name;
  $("providerModelInput").value = "";
  populateProviderForm(provider);
  updateProviderModelPicker(provider);
  updateProviderFormTitle();
  renderProviderCardList();
}

function populateProviderForm(payload) {
  var nameEl = $("providerFormName");
  var kindEl = $("providerFormKind");
  var apiBaseEl = $("providerFormApiBase");
  var apiKeyEl = $("providerFormApiKey");
  var proxyEl = $("providerFormProxy");
  var timeoutEl = $("providerFormTimeout");
  if (nameEl) nameEl.value = String(payload.name || "").trim();
  if (kindEl) kindEl.value = String(payload.provider_kind || "openai").trim() || "openai";
  if (apiBaseEl) apiBaseEl.value = String(payload.api_base || "").trim();
  if (apiKeyEl) apiKeyEl.value = String(payload.api_key || "").trim();
  if (proxyEl) proxyEl.value = String(payload.proxy || "").trim();
  if (timeoutEl) timeoutEl.value = typeof payload.timeout === "number" ? payload.timeout : 60;
}

function readProviderForm() {
  return {
    name: ($("providerFormName") || {}).value || "",
    provider_kind: ($("providerFormKind") || {}).value || "openai",
    api_base: ($("providerFormApiBase") || {}).value || "",
    api_key: ($("providerFormApiKey") || {}).value || "",
    proxy: ($("providerFormProxy") || {}).value || "",
    timeout: parseInt(($("providerFormTimeout") || {}).value, 10) || 60
  };
}

function syncFormToEditor() {
  var payload;
  try { payload = readProviderEditor(); } catch (_) { payload = defaultProviderTemplate(); }
  var form = readProviderForm();
  payload.name = form.name.trim();
  payload.provider_kind = form.provider_kind.trim() || "openai";
  payload.api_base = form.api_base.trim();
  payload.api_key = form.api_key.trim();
  payload.proxy = form.proxy.trim();
  payload.timeout = form.timeout;
  writeProviderEditor(payload);
}

function updateProviderFormTitle() {
  var titleEl = $("providerFormTitle");
  if (!titleEl) return;
  if (state.editingProviderName) {
    titleEl.textContent = state.editingProviderName;
  } else if (state.providers.length > 0) {
    titleEl.textContent = t("noProviderSelected");
  } else {
    titleEl.textContent = t("noProviders");
  }
}

function readProviderEditor() { return JSON.parse($("providerEditor").value || "{}"); }
function writeProviderEditor(payload) { $("providerEditor").value = JSON.stringify(payload || {}, null, 2); }

async function loadProviders() {
  try {
    var data = await api("/api/providers");
    state.providers = Array.isArray(data) ? data : [];
    renderProviderSelect();
    renderChatProviderSelect();
    if (state.providers.length > 0) {
      // If editingProviderName is set but not found (e.g. just created), keep editor as-is
      if (state.editingProviderName && !state.providers.find(function(p) { return p.name === state.editingProviderName; })) {
        // Provider not yet in list (race condition) — don't clear the editor
      } else {
        openProvider(state.editingProviderName || state.providers[0].name);
      }
    } else {
      state.editingProviderName = "";
      var emptyPayload = defaultProviderTemplate();
      $("providerEditor").value = JSON.stringify(emptyPayload, null, 2);
      updateProviderModelPicker(emptyPayload);
    }
    setProviderMessage("");
  } catch (err) {
    setProviderMessage(err.message, true);
  }
}

async function saveProvider() {
  syncFormToEditor();
  var payload;
  try { payload = readProviderEditor(); } catch (err) {
    setProviderMessage(t("invalidJson", err.message), true); return;
  }
  payload.models = normalizeModelList(payload.models);
  if (payload.default_model && !payload.models.includes(payload.default_model)) payload.default_model = "";
  if (!payload.default_model && payload.models.length > 0) payload.default_model = payload.models[0];
  if (!payload.name || !String(payload.name).trim()) {
    setProviderMessage(t("providerNameRequired"), true); return;
  }
  try {
    if (state.editingProviderName) {
      await api("/api/providers/" + encodeURIComponent(state.editingProviderName), { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await api("/api/providers", { method: "POST", body: JSON.stringify(payload) });
    }
    state.editingProviderName = payload.name;
    setProviderMessage(t("saved"));
    await loadProviders();
    await loadModels();
  } catch (err) {
    setProviderMessage(err.message, true);
  }
}

async function deleteProvider() {
  var name = state.editingProviderName || $("providerSelect").value;
  if (!name) { setProviderMessage(t("noProviderSelected"), true); return; }
  if (!confirm(t("deleteConfirm"))) return;
  try {
    await api("/api/providers/" + encodeURIComponent(name), { method: "DELETE" });
    state.editingProviderName = "";
    setProviderMessage(t("deleted"));
    await loadProviders();
    await loadModels();
  } catch (err) {
    setProviderMessage(err.message, true);
  }
}

function newProvider() {
  state.providerDialogMode = "new";
  openProviderDialog();
}

function editProvider() {
  if (!state.editingProviderName && !$("providerSelect").value) {
    setProviderMessage(t("noProviderSelected"), true);
    return;
  }
  state.providerDialogMode = "edit";
  openProviderDialog();
}

function openProviderDialog() {
  var payload = defaultProviderTemplate();
  if (state.providerDialogMode === "edit") {
    // Read from server state to avoid stale editor content
    var serverProvider = state.providers.find(function(p) { return p.name === state.editingProviderName; });
    if (serverProvider) {
      payload = JSON.parse(JSON.stringify(serverProvider));
    } else {
      try { payload = readProviderEditor(); } catch (_) {}
    }
  } else {
    payload.name = "";
    payload.api_key = "";
    payload.api_base = "";
    payload.proxy = "";
  }
  $("providerDialogKind").value = String(payload.provider_kind || "openai").trim() || "openai";
  $("providerDialogName").value = String(payload.name || "").trim();
  $("providerDialogApiBase").value = String(payload.api_base || "").trim();
  $("providerDialogProxy").value = String(payload.proxy || "").trim();
  $("providerDialogApiKey").value = String(payload.api_key || "").trim();
  $("providerDialogTitle").textContent = t(state.providerDialogMode === "edit" ? "editProviderDialogTitle" : "newProviderDialogTitle");
  $("providerDialog").classList.remove("hidden");
  setTimeout(function() {
    $("providerDialogName").focus();
    $("providerDialogName").select();
  }, 0);
}

function closeProviderDialog() {
  $("providerDialog").classList.add("hidden");
}

async function applyProviderDialog() {
  var name = $("providerDialogName").value.trim();
  if (!name) {
    setProviderMessage(t("providerNameRequired"), true);
    return;
  }

  var payload;
  if (state.providerDialogMode === "edit") {
    // Read from server state (not editor textarea) to avoid stale content
    var serverProvider = state.providers.find(function(p) { return p.name === state.editingProviderName; });
    payload = serverProvider ? JSON.parse(JSON.stringify(serverProvider)) : defaultProviderTemplate();
  } else {
    payload = defaultProviderTemplate();
    state.editingProviderName = "";
  }

  payload.provider_kind = $("providerDialogKind").value.trim() || "openai";
  payload.name = name;
  payload.api_base = $("providerDialogApiBase").value.trim();
  payload.proxy = $("providerDialogProxy").value.trim();
  payload.api_key = $("providerDialogApiKey").value.trim();
  if (!Array.isArray(payload.models)) payload.models = [];
  payload.models = normalizeModelList(payload.models);
  if (typeof payload.timeout !== "number") payload.timeout = 60;
  if (payload.default_model && !payload.models.includes(payload.default_model)) payload.default_model = "";
  if (!payload.default_model && payload.models.length > 0) payload.default_model = payload.models[0];

  writeProviderEditor(payload);
  populateProviderForm(payload);
  updateProviderModelPicker(payload);
  updateProviderFormTitle();

  // Ensure form area is visible (it may be hidden by empty state)
  var emptyState = $("providerEmptyState");
  var formFields = $("providerFormFields");
  var formHeader = document.querySelector(".provider-form-header");
  if (emptyState) emptyState.classList.add("hidden");
  if (formFields) formFields.classList.remove("hidden");
  if (formHeader) formHeader.classList.remove("hidden");

  closeProviderDialog();
  setProviderMessage(t("providerDraftUpdated"));
}

async function fetchProviderModels() {
  var payload;
  try { payload = readProviderEditor(); } catch (err) {
    setProviderMessage(t("invalidJson", err.message), true); return;
  }
  setProviderMessage(t("discoveringModels"));
  try {
    var result = await api("/api/providers/discover-models", { method: "POST", body: JSON.stringify(payload) });
    var models = normalizeModelList(result.models);
    payload.models = normalizeModelList(payload.models);
    state.providerModelCatalog[getProviderModelCatalogKey(payload)] = models;
    if (payload.default_model && !models.includes(payload.default_model) && !normalizeModelList(payload.models).includes(payload.default_model)) {
      payload.default_model = "";
    }
    writeProviderEditor(payload);
    updateProviderModelPicker(payload);
    setProviderMessage(t("discoveredModels", models.length));
  } catch (err) {
    setProviderMessage(t("discoveryFailed", err.message), true);
  }
}

function addProviderModel() {
  var model = $("providerModelInput").value.trim();
  if (!model) { setProviderMessage(t("enterModelName"), true); return; }
  var payload;
  try { payload = readProviderEditor(); } catch (err) {
    setProviderMessage(t("invalidJson", err.message), true); return;
  }
  var existing = normalizeModelList(payload.models);
  if (!existing.includes(model)) existing.push(model);
  payload.models = existing;
  if (!payload.default_model) payload.default_model = model;
  writeProviderEditor(payload);
  updateProviderModelPicker(payload);
  $("providerModelInput").value = "";
  setProviderMessage(t("modelAdded"));
}

/* ========== Channels ========== */
async function loadChannels() {
  try {
    state.channels = await api("/api/channels");
  } catch (err) {
    state.channels = {};
  }
  renderChannelsTable();
}

function renderChannelsTable() {
  var body = $("channelsBody");
  body.innerHTML = "";
  var names = Object.keys(state.channels).sort();
  for (var i = 0; i < names.length; i++) {
    var name = names[i];
    var cfg = state.channels[name] || {};
    if (typeof cfg !== "object" || cfg === null || !Object.prototype.hasOwnProperty.call(cfg, "enabled")) continue;
    var tr = document.createElement("tr");
    var statusCell = document.createElement("td");
    statusCell.textContent = "-";
    var isOn = cfg.enabled;
    tr.innerHTML =
      "<td>" + name + "</td>" +
      '<td><span class="badge ' + (isOn ? "badge-on" : "badge-off") + '">' + t(isOn ? "on" : "off") + "</span></td>" +
      "<td></td>" +
      '<td><button class="btn btn-sm" data-test="' + name + '">' + t("test") + "</button> " +
      '<button class="btn btn-sm" data-edit="' + name + '">' + t("edit") + "</button></td>";
    tr.children[2].replaceWith(statusCell);
    body.appendChild(tr);
  }
}

async function testChannel(name, statusCell) {
  statusCell.textContent = t("testing");
  try {
    var r = await api("/api/channels/" + encodeURIComponent(name) + "/test", { method: "POST" });
    statusCell.textContent = (r.status || "ok") + (r.reachable ? " (reachable)" : "");
  } catch (err) {
    statusCell.textContent = "error: " + err.message;
  }
}

function openChannelEditor(name) {
  state.editingChannel = name;
  var cfg = state.channels[name] || {};
  $("channelEditorTitle").textContent = t("editChannelTitle", name);
  $("channelEditorInput").value = JSON.stringify(cfg, null, 2);
  $("channelEditorMsg").textContent = "";
  $("channelEditor").classList.remove("hidden");
}

function closeChannelEditor() {
  state.editingChannel = "";
  $("channelEditor").classList.add("hidden");
  $("channelEditorMsg").textContent = "";
}

async function saveChannelConfig() {
  if (!state.editingChannel) return;
  var msgEl = $("channelEditorMsg");
  var payload;
  try { payload = JSON.parse($("channelEditorInput").value || "{}"); } catch (err) {
    msgEl.className = "msg-text err";
    msgEl.textContent = t("invalidJson", err.message);
    return;
  }
  msgEl.className = "msg-text";
  msgEl.textContent = t("saving");
  try {
    await api("/api/channels/" + encodeURIComponent(state.editingChannel), { method: "PUT", body: JSON.stringify(payload) });
    msgEl.className = "msg-text ok";
    msgEl.textContent = t("channelSaved");
    await loadChannels();
  } catch (err) {
    msgEl.className = "msg-text err";
    msgEl.textContent = err.message;
  }
}

/* ========== Config ========== */
var CONFIG_SECTIONS = ["agents", "gateway", "tools", "heartbeat", "approval", "logger", "webui"];

function normalizeConfigSection(section) {
  var key = String(section || "").trim().toLowerCase();
  if (!CONFIG_SECTIONS.includes(key)) return "agents";
  return key;
}

function configSectionLabel(section) {
  switch (normalizeConfigSection(section)) {
    case "agents": return t("configSectionAgents");
    case "gateway": return t("configSectionGateway");
    case "tools": return t("configSectionTools");
    case "heartbeat": return t("configSectionHeartbeat");
    case "approval": return t("configSectionApproval");
    case "logger": return t("configSectionLogger");
    case "webui": return t("configSectionWebUI");
    default: return section;
  }
}

function renderConfigSectionSelect() {
  var sel = $("configSectionSelect");
  if (!sel) return;
  var current = normalizeConfigSection(state.configSection);
  sel.innerHTML = "";
  for (var i = 0; i < CONFIG_SECTIONS.length; i++) {
    var section = CONFIG_SECTIONS[i];
    var opt = document.createElement("option");
    opt.value = section;
    opt.textContent = configSectionLabel(section);
    sel.appendChild(opt);
  }
  sel.value = current;
}

function renderConfigEditorForSection() {
  var section = normalizeConfigSection(state.configSection);
  var payload = {};
  if (state.loadedConfig && Object.prototype.hasOwnProperty.call(state.loadedConfig, section)) {
    payload = state.loadedConfig[section];
  }
  $("configEditor").value = JSON.stringify(payload || {}, null, 2);
  var hint = $("configSectionHint");
  if (hint) hint.textContent = t("configSectionHint", configSectionLabel(section));
}

function setConfigMessage(text, isError) {
  var el = $("configMsg");
  el.className = "msg-text" + (isError ? " err" : "");
  el.textContent = text || "";
}

async function loadConfig() {
  try {
    var cfg = await api("/api/config");
    state.loadedConfig = cfg || null;
    renderConfigSectionSelect();
    renderConfigEditorForSection();
    setConfigMessage("");
  } catch (err) {
    state.loadedConfig = null;
    $("configEditor").value = "{}";
    setConfigMessage(err.message, true);
  }
}

async function saveConfig() {
  var section = normalizeConfigSection(state.configSection);
  var payload;
  try { payload = JSON.parse($("configEditor").value || "{}"); } catch (err) {
    setConfigMessage(t("invalidJson", err.message), true); return;
  }
  setConfigMessage(t("saving"));
  var body = {};
  body[section] = payload;
  try {
    await api("/api/config", { method: "PUT", body: JSON.stringify(body) });
    setConfigMessage(t("configSectionSaved", configSectionLabel(section)));
    await Promise.all([loadStatus(), loadModels()]);
  } catch (err) {
    setConfigMessage(err.message, true);
  }
}

function resetConfigEditor() {
  renderConfigEditorForSection();
  setConfigMessage("");
}

async function exportConfig() {
  setConfigMessage(t("exporting"));
  try {
    var data = await api("/api/config/export");
    var blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    var url = URL.createObjectURL(blob);
    var a = document.createElement("a");
    a.href = url;
    a.download = "nekobot-config-export.json";
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    setConfigMessage(t("exported"));
  } catch (err) {
    setConfigMessage(err.message, true);
  }
}

async function importConfig(file) {
  if (!file) return;
  setConfigMessage(t("importing"));
  try {
    var text = await file.text();
    var payload = JSON.parse(text);
    var result = await api("/api/config/import", { method: "POST", body: JSON.stringify(payload) });
    setConfigMessage(t("imported", result.sections_saved || 0, result.providers_imported || 0));
    await Promise.all([loadConfig(), loadProviders(), loadModels(), loadStatus()]);
  } catch (err) {
    setConfigMessage(err.message || t("importFailed"), true);
  }
}

/* ========== Status ========== */
function updateVersionBadge(versionValue) {
  var badge = $("versionBadge");
  if (!badge) return;
  var versionText = String(versionValue || "").trim();
  if (!versionText) {
    badge.classList.add("hidden");
    return;
  }
  badge.textContent = versionText[0] === "v" ? versionText : ("v" + versionText);
  badge.classList.remove("hidden");
}

async function loadStatus() {
  try {
    var status = await api("/api/status");
    $("statusPre").textContent = JSON.stringify(status, null, 2);
    updateVersionBadge(status.version);
  } catch (err) {
    $("statusPre").textContent = "Error: " + err.message;
    updateVersionBadge("");
  }
}

/* ========== Chat / WebSocket ========== */
function connectWS() {
  if (!state.token) return;
  if (state.ws) { try { state.ws.close(); } catch (_) {} }
  var proto = location.protocol === "https:" ? "wss" : "ws";
  var ws = new WebSocket(proto + "://" + location.host + "/api/chat/ws?token=" + encodeURIComponent(state.token));
  state.ws = ws;
  ws.onopen = function() { setWSState(true, "wsConnected"); };
  ws.onclose = function() { setWSState(false, "wsDisconnected"); };
  ws.onerror = function() { setWSState(false, "wsError"); };
  ws.onmessage = function(ev) {
    var msg = {};
    try { msg = JSON.parse(ev.data); } catch (_) { return; }
    if (msg.type === "message") pushChat("bot", msg.content || "");
    else if (msg.type === "error") pushChat("error", msg.content || "Unknown error");
    else pushChat("system", msg.content || msg.type || "event");
  };
}

function sendChat() {
  var text = $("chatInput").value.trim();
  if (!text || !state.ws || state.ws.readyState !== WebSocket.OPEN) return;
  var customModel = $("modelInput").value.trim();
  var model = customModel || $("modelSelect").value;
  var provider = $("chatProviderSelect").value.trim();
  var fallback = $("fallbackInput").value.split(",").map(function(name) { return name.trim(); }).filter(Boolean);
  state.chatProvider = provider;
  state.ws.send(JSON.stringify({ type: "message", content: text, model: model, provider: provider, fallback: fallback }));
  pushChat("user", text);
  $("chatInput").value = "";
}

/* ========== Events ========== */
function setupEvents() {
  $("themeToggle").addEventListener("click", toggleTheme);
  $("langSelect").addEventListener("change", function(e) { switchLang(e.target.value); });

  $("initBtn").addEventListener("click", async function() {
    setAuthError("");
    try {
      var payload = await fetch("/api/auth/init", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: $("initUsername").value.trim() || "admin", password: $("initPassword").value })
      }).then(function(r) { return r.json(); });
      if (!payload.token) throw new Error(payload.error || "Initialization failed");
      state.token = payload.token;
      localStorage.setItem("nekobot_webui_token", state.token);
      showMain();
      await initMain();
    } catch (err) { setAuthError(err.message); }
  });

  $("loginBtn").addEventListener("click", async function() {
    setAuthError("");
    try {
      var payload = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: $("loginUsername").value.trim(), password: $("loginPassword").value })
      }).then(function(r) { return r.json(); });
      if (!payload.token) throw new Error(payload.error || "Login failed");
      state.token = payload.token;
      localStorage.setItem("nekobot_webui_token", state.token);
      showMain();
      await initMain();
    } catch (err) { setAuthError(err.message); }
  });

  $("initPassword").addEventListener("keydown", function(e) { if (e.key === "Enter") $("initBtn").click(); });
  $("loginPassword").addEventListener("keydown", function(e) { if (e.key === "Enter") $("loginBtn").click(); });

  $("logoutBtn").addEventListener("click", function() {
    state.token = "";
    localStorage.removeItem("nekobot_webui_token");
    if (state.ws) try { state.ws.close(); } catch (_) {}
    Object.keys(state.toolSockets).forEach(closeToolSocket);
    stopToolPoller();
    showAuth();
  });

  document.querySelectorAll(".tab").forEach(function(btn) {
    btn.addEventListener("click", function() {
      document.querySelectorAll(".tab").forEach(function(t) { t.classList.remove("active"); });
      btn.classList.add("active");
      var tab = btn.dataset.tab;
      ["chat", "tools", "providers", "channels", "config", "status"].forEach(function(name) {
        $("tab-" + name).classList.toggle("hidden", name !== tab);
      });
      if (tab === "tools") {
        setTimeout(renderToolPanels, 0);
      }
    });
  });

  $("sendBtn").addEventListener("click", sendChat);
  $("modelSelect").addEventListener("change", function() {
    var selected = $("modelSelect").value;
    $("modelInput").value = selected || "";
    var option = $("modelSelect").selectedOptions && $("modelSelect").selectedOptions[0];
    if (option && option.dataset && option.dataset.provider && option.dataset.provider !== "default") {
      $("chatProviderSelect").value = option.dataset.provider;
      state.chatProvider = option.dataset.provider;
    }
  });
  $("chatProviderSelect").addEventListener("change", function(e) {
    state.chatProvider = e.target.value || "";
    renderModelSelect();
    var selectedModel = $("modelSelect").value;
    if (selectedModel) $("modelInput").value = selectedModel;
  });
  $("chatInput").addEventListener("keydown", function(e) {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendChat(); }
  });
  $("connectWsBtn").addEventListener("click", connectWS);
  $("clearChatBtn").addEventListener("click", function() {
    $("chatLog").innerHTML = '<div class="chat-empty">' + t("chatEmptyHint") + '</div>';
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify({ type: "clear" }));
    }
  });

  $("refreshChannelsBtn").addEventListener("click", loadChannels);
  $("channelsBody").addEventListener("click", function(e) {
    var testBtn = e.target.closest("button[data-test]");
    if (testBtn) {
      var name = testBtn.dataset.test;
      var row = testBtn.closest("tr");
      testChannel(name, row.children[2]);
      return;
    }
    var editBtn = e.target.closest("button[data-edit]");
    if (editBtn) openChannelEditor(editBtn.dataset.edit);
  });
  $("saveChannelBtn").addEventListener("click", saveChannelConfig);
  $("cancelChannelBtn").addEventListener("click", closeChannelEditor);

  $("refreshProvidersBtn").addEventListener("click", loadProviders);
  $("providerSelect").addEventListener("change", function(e) { openProvider(e.target.value); });
  $("providerCardList").addEventListener("click", function(e) {
    var card = e.target.closest(".provider-card[data-provider-card]");
    if (card) openProvider(card.dataset.providerCard);
  });
  $("newProviderBtn").addEventListener("click", newProvider);
  $("editProviderBtn").addEventListener("click", editProvider);
  $("fetchProviderModelsBtn").addEventListener("click", fetchProviderModels);
  $("toggleProviderJsonBtn").addEventListener("click", function() {
    var wrap = $("providerJsonWrap");
    var isHidden = wrap.classList.contains("hidden");
    if (isHidden) {
      syncFormToEditor();
    }
    wrap.classList.toggle("hidden");
    $("toggleProviderJsonBtn").textContent = isHidden ? t("hideAdvancedJson") : t("advancedJson");
  });
  ["providerFormName", "providerFormKind", "providerFormApiBase", "providerFormApiKey", "providerFormProxy", "providerFormTimeout"].forEach(function(id) {
    var el = $(id);
    if (el) el.addEventListener("input", syncFormToEditor);
  });
  $("providerModelFilter").addEventListener("input", function(e) {
    state.providerModelFilter = e.target.value || "";
    renderProviderModelPicker();
  });
  $("providerModelSelect").addEventListener("change", function() {
    syncProviderModelSelectionFromVisible();
    renderProviderModelPicker();
  });
  $("selectAllProviderModelsBtn").addEventListener("click", function() {
    selectFilteredProviderModels(true);
  });
  $("clearProviderModelsBtn").addEventListener("click", function() {
    state.providerModelSelected = [];
    renderProviderModelPicker();
  });
  $("applyProviderModelsBtn").addEventListener("click", applyProviderModelSelection);
  $("addProviderModelBtn").addEventListener("click", addProviderModel);
  $("providerDialogCancelBtn").addEventListener("click", closeProviderDialog);
  $("providerDialogApplyBtn").addEventListener("click", applyProviderDialog);
  $("providerDialog").addEventListener("click", function(e) {
    if (e.target === $("providerDialog")) closeProviderDialog();
  });
  document.addEventListener("keydown", function(e) {
    if (e.key !== "Escape") return;
    if (state.toolMaximized) {
      state.toolMaximized = "";
      renderToolPanels();
    }
    if (!$("providerDialog").classList.contains("hidden")) closeProviderDialog();
    if (!$("toolSessionDialog").classList.contains("hidden")) closeToolSessionDialog();
    if (!$("toolAccessDialog").classList.contains("hidden")) closeToolAccessDialog();
  });
  ["providerDialogName", "providerDialogApiBase", "providerDialogProxy", "providerDialogApiKey"].forEach(function(id) {
    $(id).addEventListener("keydown", function(e) {
      if (e.key === "Enter") {
        e.preventDefault();
        applyProviderDialog();
      }
    });
  });
  $("saveProviderBtn").addEventListener("click", saveProvider);
  $("deleteProviderBtn").addEventListener("click", deleteProvider);

  $("newToolSessionBtn").addEventListener("click", openToolSessionDialog);
  $("refreshToolSessionsBtn").addEventListener("click", loadToolSessions);
  $("cleanupToolSessionsBtn").addEventListener("click", cleanupTerminatedToolSessions);
  $("clearSplitBtn").addEventListener("click", function() {
    state.splitToolTab = "";
    closeUnusedToolSockets();
    renderToolPanels();
  });
  $("toolSendPrimaryBtn").addEventListener("click", function() { sendToolInput(false); });
  $("toolSendSplitBtn").addEventListener("click", function() { sendToolInput(true); });
  $("toolInputPrimary").addEventListener("keydown", function(e) {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendToolInput(false); }
  });
  $("toolInputSplit").addEventListener("keydown", function(e) {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendToolInput(true); }
  });
  $("toolSessionsList").addEventListener("click", function(e) {
    var accessBtn = e.target.closest("button[data-tool-access]");
    if (accessBtn) { showToolSessionAccess(accessBtn.dataset.toolAccess, false); return; }
    var editBtn = e.target.closest("button[data-tool-edit]");
    if (editBtn) { openToolSessionDialogForEdit(editBtn.dataset.toolEdit); return; }
    var killBtn = e.target.closest("button[data-tool-kill]");
    if (killBtn) { killToolSession(killBtn.dataset.toolKill); return; }
    var row = e.target.closest(".tool-session-item[data-tool-select]");
    if (row && row.dataset.toolSelect) { openToolTab(row.dataset.toolSelect, false); return; }
  });
  $("toolTabs").addEventListener("click", function(e) {
    var closeBtn = e.target.closest("button[data-tool-close]");
    if (closeBtn) { closeToolTab(closeBtn.dataset.toolClose); return; }
    var tab = e.target.closest(".tool-tab[data-tool-tab]");
    if (tab) { state.activeToolTab = tab.dataset.toolTab; renderToolSessionsList(); renderToolTabs(); renderToolPanels(); }
  });
  $("toolSessionDialogCancelBtn").addEventListener("click", closeToolSessionDialog);
  $("toolSessionDialogCreateBtn").addEventListener("click", createToolSession);
  $("toolSessionTool").addEventListener("change", updateToolSessionToolMode);
  $("toolSessionProxyMode").addEventListener("change", updateToolSessionProxyMode);
  $("toolSessionDialog").addEventListener("click", function(e) {
    if (e.target === $("toolSessionDialog")) closeToolSessionDialog();
  });
  $("toolAccessDialogCloseBtn").addEventListener("click", closeToolAccessDialog);
  $("toolAccessDialog").addEventListener("click", function(e) {
    if (e.target === $("toolAccessDialog")) closeToolAccessDialog();
  });
  $("toolAccessCopyUrlBtn").addEventListener("click", async function() {
    var ok = await copyTextToClipboard($("toolAccessUrlInput").value);
    if (ok) showToast(t("copied"), "success");
  });
  $("toolAccessCopyPasswordBtn").addEventListener("click", async function() {
    var ok = await copyTextToClipboard($("toolAccessPasswordInput").value);
    if (ok) showToast(t("copied"), "success");
  });
  $("toolAccessRefreshBtn").addEventListener("click", function() {
    if (!state.toolAccessDialogSessionID) return;
    showToolSessionAccess(state.toolAccessDialogSessionID, true);
  });
  $("toolAccessOtpRefreshBtn").addEventListener("click", function() {
    if (!state.toolAccessDialogSessionID) return;
    refreshToolSessionOTP(state.toolAccessDialogSessionID);
  });
  $("toolAccessCopyOtpBtn").addEventListener("click", async function() {
    var ok = await copyTextToClipboard($("toolAccessOtpInput").value);
    if (ok) showToast(t("copied"), "success");
  });
  $("toolPrimaryMaxBtn").addEventListener("click", function() { toggleToolMaximize("Primary"); });
  $("toolSplitMaxBtn").addEventListener("click", function() { toggleToolMaximize("Split"); });
  ["toolSessionTitle", "toolSessionToolCustom", "toolSessionCommand", "toolSessionWorkdir", "toolSessionProxyUrl", "toolSessionAccessPassword", "toolSessionPublicBaseUrl"].forEach(function(id) {
    $(id).addEventListener("keydown", function(e) {
      if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        createToolSession();
      }
    });
  });

  $("configSectionSelect").addEventListener("change", function(e) {
    state.configSection = normalizeConfigSection(e.target.value);
    renderConfigEditorForSection();
    setConfigMessage("");
  });
  $("refreshConfigBtn").addEventListener("click", loadConfig);
  $("resetConfigBtn").addEventListener("click", resetConfigEditor);
  $("saveConfigBtn").addEventListener("click", saveConfig);
  $("exportConfigBtn").addEventListener("click", exportConfig);
  $("importConfigBtn").addEventListener("click", function() { $("importConfigFile").click(); });
  $("importConfigFile").addEventListener("change", function(e) {
    var file = e.target.files && e.target.files[0];
    if (file) importConfig(file);
    e.target.value = "";
  });
  $("refreshStatusBtn").addEventListener("click", loadStatus);

  initToolTerminals();
}

/* ========== Init ========== */
(async function() {
  applyTheme(getTheme());
  setupEvents();
  await loadI18n();
  $("langSelect").value = currentLang;
  renderI18n();
  await bootstrapToolAccessFromURL();
  checkInitAndAuth();
})();
