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
}

function switchLang(lang) {
  currentLang = lang;
  localStorage.setItem("nekobot_lang", lang);
  renderI18n();
  renderChannelsTable();
  renderModelSelect();
  renderProviderSelect();
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
}

function toggleTheme() {
  applyTheme(getTheme() === "light" ? "dark" : "light");
}

/* ========== State & API ========== */
const state = {
  token: localStorage.getItem("nekobot_webui_token") || "",
  ws: null,
  models: [],
  channels: {},
  editingChannel: "",
  providers: [],
  editingProviderName: ""
};

const $ = (id) => document.getElementById(id);

function setAuthError(msg) { $("authError").textContent = msg || ""; }

async function api(path, options) {
  options = options || {};
  const headers = Object.assign({ "Content-Type": "application/json" }, options.headers || {});
  if (state.token) headers["Authorization"] = "Bearer " + state.token;
  const resp = await fetch(path, Object.assign({}, options, { headers: headers }));
  const text = await resp.text();
  var payload = {};
  try { payload = text ? JSON.parse(text) : {}; } catch (_) { payload = { raw: text }; }
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
    $("authHint").textContent = t("loginHint");
    $("authLogin").classList.remove("hidden");
    $("authInit").classList.add("hidden");
  } else {
    $("authHint").textContent = t("firstRunHint");
    $("authInit").classList.remove("hidden");
    $("authLogin").classList.add("hidden");
  }
}

async function initMain() {
  await Promise.all([loadModels(), loadProviders(), loadChannels(), loadConfig(), loadStatus()]);
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
    var currentProvider = (cfg && cfg.agents && cfg.agents.defaults && cfg.agents.defaults.provider) || "default";
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
  if (state.models.length > 0) {
    $("modelInput").value = state.models[0].model;
  }
}

function renderModelSelect() {
  var sel = $("modelSelect");
  sel.innerHTML = "";
  var auto = document.createElement("option");
  auto.value = "";
  auto.textContent = t("defaultModel");
  sel.appendChild(auto);
  for (var i = 0; i < state.models.length; i++) {
    var item = state.models[i];
    var opt = document.createElement("option");
    opt.value = item.model;
    opt.textContent = item.model + " (" + item.provider + ")";
    sel.appendChild(opt);
  }
}

/* ========== Providers ========== */
function defaultProviderTemplate() {
  return { name: "new-provider", provider_kind: "openai", api_base: "", api_key: "", models: [], default_model: "", timeout: 60 };
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
}

function openProvider(name) {
  var provider = state.providers.find(function(p) { return p.name === name; });
  if (!provider) {
    state.editingProviderName = "";
    $("providerEditor").value = JSON.stringify(defaultProviderTemplate(), null, 2);
    $("providerModelInput").value = "";
    return;
  }
  state.editingProviderName = provider.name;
  $("providerEditor").value = JSON.stringify(provider, null, 2);
  $("providerSelect").value = provider.name;
  $("providerModelInput").value = "";
}

function readProviderEditor() { return JSON.parse($("providerEditor").value || "{}"); }
function writeProviderEditor(payload) { $("providerEditor").value = JSON.stringify(payload || {}, null, 2); }

async function loadProviders() {
  try {
    var data = await api("/api/providers");
    state.providers = Array.isArray(data) ? data : [];
    renderProviderSelect();
    if (state.providers.length > 0) {
      openProvider(state.editingProviderName || state.providers[0].name);
    } else {
      state.editingProviderName = "";
      $("providerEditor").value = JSON.stringify(defaultProviderTemplate(), null, 2);
    }
    setProviderMessage("");
  } catch (err) {
    setProviderMessage(err.message, true);
  }
}

async function saveProvider() {
  var payload;
  try { payload = readProviderEditor(); } catch (err) {
    setProviderMessage(t("invalidJson", err.message), true); return;
  }
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
  state.editingProviderName = "";
  $("providerEditor").value = JSON.stringify(defaultProviderTemplate(), null, 2);
  setProviderMessage(t("creatingNew"));
}

async function fetchProviderModels() {
  var payload;
  try { payload = readProviderEditor(); } catch (err) {
    setProviderMessage(t("invalidJson", err.message), true); return;
  }
  setProviderMessage(t("discoveringModels"));
  try {
    var result = await api("/api/providers/discover-models", { method: "POST", body: JSON.stringify(payload) });
    var models = Array.isArray(result.models) ? result.models : [];
    payload.models = models;
    if (!payload.default_model && models.length > 0) payload.default_model = models[0];
    writeProviderEditor(payload);
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
  var existing = Array.isArray(payload.models) ? payload.models : [];
  if (!existing.includes(model)) existing.push(model);
  payload.models = existing;
  if (!payload.default_model) payload.default_model = model;
  writeProviderEditor(payload);
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
function setConfigMessage(text, isError) {
  var el = $("configMsg");
  el.className = "msg-text" + (isError ? " err" : "");
  el.textContent = text || "";
}

async function loadConfig() {
  try {
    var cfg = await api("/api/config");
    $("configEditor").value = JSON.stringify(cfg, null, 2);
    setConfigMessage("");
  } catch (err) {
    setConfigMessage(err.message, true);
  }
}

async function saveConfig() {
  var payload;
  try { payload = JSON.parse($("configEditor").value || "{}"); } catch (err) {
    setConfigMessage(t("invalidJson", err.message), true); return;
  }
  setConfigMessage(t("saving"));
  try {
    await api("/api/config", { method: "PUT", body: JSON.stringify(payload) });
    setConfigMessage(t("configSaved"));
    await Promise.all([loadStatus(), loadModels()]);
  } catch (err) {
    setConfigMessage(err.message, true);
  }
}

/* ========== Status ========== */
async function loadStatus() {
  try {
    var status = await api("/api/status");
    $("statusPre").textContent = JSON.stringify(status, null, 2);
  } catch (err) {
    $("statusPre").textContent = "Error: " + err.message;
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
  state.ws.send(JSON.stringify({ type: "message", content: text, model: model }));
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
    showAuth();
  });

  document.querySelectorAll(".tab").forEach(function(btn) {
    btn.addEventListener("click", function() {
      document.querySelectorAll(".tab").forEach(function(t) { t.classList.remove("active"); });
      btn.classList.add("active");
      var tab = btn.dataset.tab;
      ["chat", "providers", "channels", "config", "status"].forEach(function(name) {
        $("tab-" + name).classList.toggle("hidden", name !== tab);
      });
    });
  });

  $("sendBtn").addEventListener("click", sendChat);
  $("modelSelect").addEventListener("change", function() {
    var selected = $("modelSelect").value;
    if (selected) $("modelInput").value = selected;
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
  $("newProviderBtn").addEventListener("click", newProvider);
  $("fetchProviderModelsBtn").addEventListener("click", fetchProviderModels);
  $("addProviderModelBtn").addEventListener("click", addProviderModel);
  $("saveProviderBtn").addEventListener("click", saveProvider);
  $("deleteProviderBtn").addEventListener("click", deleteProvider);

  $("refreshConfigBtn").addEventListener("click", loadConfig);
  $("saveConfigBtn").addEventListener("click", saveConfig);
  $("refreshStatusBtn").addEventListener("click", loadStatus);
}

/* ========== Init ========== */
(async function() {
  applyTheme(getTheme());
  setupEvents();
  await loadI18n();
  $("langSelect").value = currentLang;
  renderI18n();
  checkInitAndAuth();
})();
