(function () {
  const PAGE_SIZE = 8;
  const storage = {
    baseUrl: "broker_console_base_url",
    apiKey: "broker_console_api_key",
    selectedBroker: "broker_console_selected_broker",
    theme: "broker_console_theme",
  };

  const els = {
    baseUrl: document.getElementById("baseUrl"),
    apiKey: document.getElementById("apiKey"),
    selectedBroker: document.getElementById("selectedBroker"),
    applySelectedBroker: document.getElementById("applySelectedBroker"),
    refreshAll: document.getElementById("refreshAll"),
    saveSettings: document.getElementById("saveSettings"),
    settingsStatus: document.getElementById("settingsStatus"),
    themeToggle: document.getElementById("themeToggle"),

    tabButtons: Array.from(document.querySelectorAll(".tabbtn")),
    tabPanels: Array.from(document.querySelectorAll(".tabpanel")),
    headerTitle: document.getElementById("headerTitle"),
    selectedBrokerPill: document.getElementById("selectedBrokerPill"),

    kpiPublished: document.getElementById("kpiPublished"),
    kpiAcks: document.getElementById("kpiAcks"),
    kpiTopics: document.getElementById("kpiTopics"),
    kpiConsumers: document.getElementById("kpiConsumers"),

    newBrokerName: document.getElementById("newBrokerName"),
    createBroker: document.getElementById("createBroker"),
    reloadBrokers: document.getElementById("reloadBrokers"),
    brokerFilter: document.getElementById("brokerFilter"),
    brokerSort: document.getElementById("brokerSort"),
    brokerTableBody: document.getElementById("brokerTableBody"),
    brokerPrevPage: document.getElementById("brokerPrevPage"),
    brokerNextPage: document.getElementById("brokerNextPage"),
    brokerPageInfo: document.getElementById("brokerPageInfo"),

    newTopicName: document.getElementById("newTopicName"),
    newTopicDescription: document.getElementById("newTopicDescription"),
    newTopicRetention: document.getElementById("newTopicRetention"),
    createTopic: document.getElementById("createTopic"),
    reloadTopics: document.getElementById("reloadTopics"),
    topicFilter: document.getElementById("topicFilter"),
    topicSort: document.getElementById("topicSort"),
    topicTableBody: document.getElementById("topicTableBody"),
    topicPrevPage: document.getElementById("topicPrevPage"),
    topicNextPage: document.getElementById("topicNextPage"),
    topicPageInfo: document.getElementById("topicPageInfo"),
    topicsStatus: document.getElementById("topicsStatus"),

    publishStream: document.getElementById("publishStream"),
    requiredConsumers: document.getElementById("requiredConsumers"),
    publishPayload: document.getElementById("publishPayload"),
    publishButton: document.getElementById("publishButton"),
    publishResult: document.getElementById("publishResult"),

    newConsumerId: document.getElementById("newConsumerId"),
    newConsumerTopic: document.getElementById("newConsumerTopic"),
    newConsumerFromId: document.getElementById("newConsumerFromId"),
    addConsumer: document.getElementById("addConsumer"),
    stopAllConsumers: document.getElementById("stopAllConsumers"),
    consumeStatus: document.getElementById("consumeStatus"),
    consumerSessions: document.getElementById("consumerSessions"),

    refreshMetrics: document.getElementById("refreshMetrics"),
    metricsResult: document.getElementById("metricsResult"),

    editDrawer: document.getElementById("editDrawer"),
    drawerBackdrop: document.getElementById("drawerBackdrop"),
    closeDrawer: document.getElementById("closeDrawer"),
    drawerTitle: document.getElementById("drawerTitle"),
    saveDrawer: document.getElementById("saveDrawer"),
    drawerBrokerForm: document.getElementById("drawerBrokerForm"),
    drawerTopicForm: document.getElementById("drawerTopicForm"),
    drawerBrokerName: document.getElementById("drawerBrokerName"),
    drawerBrokerDisplayName: document.getElementById("drawerBrokerDisplayName"),
    drawerBrokerDescription: document.getElementById("drawerBrokerDescription"),
    drawerBrokerDisabled: document.getElementById("drawerBrokerDisabled"),
    drawerTopicName: document.getElementById("drawerTopicName"),
    drawerTopicDescription: document.getElementById("drawerTopicDescription"),
    drawerTopicRetention: document.getElementById("drawerTopicRetention"),
    drawerTopicDisabled: document.getElementById("drawerTopicDisabled"),
  };

  const state = {
    selectedBroker: "default",
    brokers: [],
    topics: [],
    brokerPage: 1,
    topicPage: 1,
    consumers: new Map(),
    metricsTimer: null,
    drawer: { type: "", key: "" },
  };

  const text = {
    titles: {
      brokers: "Broker Inventory",
      topics: "Topic Catalog",
      publish: "Publisher",
      consumers: "Consumer Sessions",
      metrics: "Metrics Explorer",
    },
  };

  function readStorage(key, fallback) { try { return localStorage.getItem(key) || fallback; } catch (_) { return fallback; } }
  function writeStorage(key, value) { try { localStorage.setItem(key, value); } catch (_) {} }
  function baseURL() { return (els.baseUrl.value || window.location.origin).trim().replace(/\/$/, ""); }
  function headers(isWrite) { const h = {}; if (isWrite && (els.apiKey.value || "").trim()) h["X-API-Key"] = els.apiKey.value.trim(); return h; }

  function applyTheme(theme) {
    document.body.setAttribute("data-theme", theme);
    els.themeToggle.textContent = theme === "dark" ? "Light Theme" : "Dark Theme";
    writeStorage(storage.theme, theme);
  }

  async function request(path, options) {
    const response = await fetch(baseURL() + path, options);
    const raw = await response.text();
    let data = raw;
    if (raw) {
      try { data = JSON.parse(raw); } catch (_) {}
    }
    if (!response.ok) throw new Error(data && data.message ? data.message : String(data || response.statusText));
    return data;
  }

  function setSelectedBroker(name) {
    state.selectedBroker = (name || "").trim() || "default";
    els.selectedBroker.value = state.selectedBroker;
    els.selectedBrokerPill.textContent = "Broker: " + state.selectedBroker;
    writeStorage(storage.selectedBroker, state.selectedBroker);
  }

  function activateTab(name) {
    els.tabButtons.forEach((b) => b.classList.toggle("active", b.dataset.tab === name));
    els.tabPanels.forEach((p) => p.classList.toggle("active", p.id === "tab-" + name));
    els.headerTitle.textContent = text.titles[name] || "Goose Broker";
  }

  function statusBadge(disabled) {
    const cls = disabled ? "status-badge status-disabled" : "status-badge status-active";
    const label = disabled ? "Disabled" : "Active";
    return `<span class="${cls}">${label}</span>`;
  }

  function paginate(items, page) {
    const total = Math.max(1, Math.ceil(items.length / PAGE_SIZE));
    const current = Math.min(Math.max(page, 1), total);
    const start = (current - 1) * PAGE_SIZE;
    return { page: current, total, rows: items.slice(start, start + PAGE_SIZE) };
  }

  function sortedBrokers(items, mode) {
    const out = items.slice();
    switch (mode) {
      case "name_desc": out.sort((a, b) => b.name.localeCompare(a.name)); break;
      case "topics_desc": out.sort((a, b) => (b.topics_count || 0) - (a.topics_count || 0)); break;
      case "published_desc": out.sort((a, b) => (b.published_messages || 0) - (a.published_messages || 0)); break;
      default: out.sort((a, b) => a.name.localeCompare(b.name));
    }
    return out;
  }

  function sortedTopics(items, mode) {
    const out = items.slice();
    switch (mode) {
      case "name_desc": out.sort((a, b) => b.name.localeCompare(a.name)); break;
      case "retention_desc": out.sort((a, b) => (b.retention_hours || 0) - (a.retention_hours || 0)); break;
      case "status": out.sort((a, b) => Number(a.disabled) - Number(b.disabled)); break;
      default: out.sort((a, b) => a.name.localeCompare(b.name));
    }
    return out;
  }

  function renderBrokerTable() {
    const q = (els.brokerFilter.value || "").trim().toLowerCase();
    const filtered = state.brokers.filter((b) => `${b.name} ${b.display_name || ""} ${b.description || ""}`.toLowerCase().includes(q));
    const sorted = sortedBrokers(filtered, els.brokerSort.value);
    const page = paginate(sorted, state.brokerPage);
    state.brokerPage = page.page;

    els.brokerTableBody.innerHTML = "";
    page.rows.forEach((b) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td><div class="resource-title">${b.display_name ? `${b.display_name} (${b.name})` : b.name}</div><div class="resource-meta">${b.description || "No description"}</div></td>
        <td>${statusBadge(!!b.disabled)}</td>
        <td>${b.topics_count || 0}</td>
        <td>${b.subscribers || 0}</td>
        <td>${b.published_messages || 0}</td>
        <td><div class="resource-actions"><button class="mini btn-outline" data-broker-use="${b.name}">Use</button><button class="mini btn-outline" data-broker-edit="${b.name}">Edit</button><button class="mini danger" data-broker-del="${b.name}">Delete</button></div></td>`;
      els.brokerTableBody.appendChild(tr);
    });

    els.brokerPageInfo.textContent = `Page ${page.page} / ${page.total}`;
    els.brokerPrevPage.disabled = page.page <= 1;
    els.brokerNextPage.disabled = page.page >= page.total;
  }

  function renderTopicTable() {
    const q = (els.topicFilter.value || "").trim().toLowerCase();
    const filtered = state.topics.filter((t) => `${t.name} ${t.description || ""}`.toLowerCase().includes(q));
    const sorted = sortedTopics(filtered, els.topicSort.value);
    const page = paginate(sorted, state.topicPage);
    state.topicPage = page.page;

    els.topicTableBody.innerHTML = "";
    page.rows.forEach((t) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td><div class="resource-title">${t.name}</div></td>
        <td>${statusBadge(!!t.disabled)}</td>
        <td>${t.retention_hours || 0}h</td>
        <td>${t.description || "No description"}</td>
        <td><div class="resource-actions"><button class="mini btn-outline" data-topic-edit="${t.name}">Edit</button><button class="mini danger" data-topic-del="${t.name}">Delete</button></div></td>`;
      els.topicTableBody.appendChild(tr);
    });

    els.topicPageInfo.textContent = `Page ${page.page} / ${page.total}`;
    els.topicPrevPage.disabled = page.page <= 1;
    els.topicNextPage.disabled = page.page >= page.total;
  }

  function openDrawer(type, key) {
    state.drawer.type = type;
    state.drawer.key = key;
    els.editDrawer.classList.add("open");
    els.drawerBrokerForm.classList.add("hidden");
    els.drawerTopicForm.classList.add("hidden");

    if (type === "broker") {
      const b = state.brokers.find((x) => x.name === key);
      if (!b) return;
      els.drawerTitle.textContent = "Edit Broker";
      els.drawerBrokerForm.classList.remove("hidden");
      els.drawerBrokerName.value = b.name;
      els.drawerBrokerDisplayName.value = b.display_name || "";
      els.drawerBrokerDescription.value = b.description || "";
      els.drawerBrokerDisabled.value = b.disabled ? "true" : "false";
    }
    if (type === "topic") {
      const t = state.topics.find((x) => x.name === key);
      if (!t) return;
      els.drawerTitle.textContent = "Edit Topic";
      els.drawerTopicForm.classList.remove("hidden");
      els.drawerTopicName.value = t.name;
      els.drawerTopicDescription.value = t.description || "";
      els.drawerTopicRetention.value = t.retention_hours || 0;
      els.drawerTopicDisabled.value = t.disabled ? "true" : "false";
    }
  }

  function closeDrawer() {
    els.editDrawer.classList.remove("open");
    state.drawer = { type: "", key: "" };
  }

  async function saveDrawer() {
    if (state.drawer.type === "broker") {
      await request(`/brokers/${encodeURIComponent(state.drawer.key)}/settings`, {
        method: "PATCH",
        headers: { ...headers(true), "Content-Type": "application/json" },
        body: JSON.stringify({
          display_name: (els.drawerBrokerDisplayName.value || "").trim(),
          description: (els.drawerBrokerDescription.value || "").trim(),
          disabled: els.drawerBrokerDisabled.value === "true",
        }),
      });
      await loadBrokers();
    }
    if (state.drawer.type === "topic") {
      await request(`/brokers/${encodeURIComponent(state.selectedBroker)}/topics/${encodeURIComponent(state.drawer.key)}`, {
        method: "PATCH",
        headers: { ...headers(true), "Content-Type": "application/json" },
        body: JSON.stringify({
          description: (els.drawerTopicDescription.value || "").trim(),
          retention_hours: Number(els.drawerTopicRetention.value || 0),
          disabled: els.drawerTopicDisabled.value === "true",
        }),
      });
      await loadTopics();
    }
    closeDrawer();
  }

  async function loadBrokers() {
    state.brokers = await request("/brokers/details", { method: "GET" });
    if (!state.brokers.find((b) => b.name === state.selectedBroker) && state.brokers.length > 0) setSelectedBroker(state.brokers[0].name);
    renderBrokerTable();
  }

  async function loadTopics() {
    state.topics = await request(`/brokers/${encodeURIComponent(state.selectedBroker)}/topics`, { method: "GET" });
    renderTopicTable();
  }

  async function createBroker() {
    const name = (els.newBrokerName.value || "").trim();
    if (!name) return;
    await request("/brokers", { method: "POST", headers: { ...headers(true), "Content-Type": "application/json" }, body: JSON.stringify({ name }) });
    els.newBrokerName.value = "";
    setSelectedBroker(name);
    await refreshAll();
  }

  async function createTopic() {
    const name = (els.newTopicName.value || "").trim();
    if (!name) { els.topicsStatus.textContent = "Topic name is required."; return; }
    await request(`/brokers/${encodeURIComponent(state.selectedBroker)}/topics`, {
      method: "POST",
      headers: { ...headers(true), "Content-Type": "application/json" },
      body: JSON.stringify({ name, description: (els.newTopicDescription.value || "").trim(), retention_hours: Number(els.newTopicRetention.value || 0) }),
    });
    els.newTopicName.value = "";
    els.topicsStatus.textContent = "Topic created.";
    await loadTopics();
  }

  async function deleteBroker(name) {
    if (!window.confirm("Delete broker " + name + "?")) return;
    await request(`/brokers/${encodeURIComponent(name)}`, { method: "DELETE", headers: headers(true) });
    await refreshAll();
  }

  async function deleteTopic(name) {
    if (!window.confirm("Delete topic " + name + "?")) return;
    await request(`/brokers/${encodeURIComponent(state.selectedBroker)}/topics/${encodeURIComponent(name)}`, { method: "DELETE", headers: headers(true) });
    await loadTopics();
  }

  async function publishMessage() {
    let payload;
    try { payload = JSON.parse(els.publishPayload.value || "{}"); } catch (_) { els.publishResult.textContent = "Invalid JSON payload."; return; }
    const stream = (els.publishStream.value || "").trim();
    const required = (els.requiredConsumers.value || "").split(",").map((x) => x.trim()).filter(Boolean);
    const start = performance.now();
    await request(`/brokers/${encodeURIComponent(state.selectedBroker)}/messages`, {
      method: "PUT",
      headers: { ...headers(true), "Content-Type": "application/json" },
      body: JSON.stringify({ stream, payload, required_consumers: required }),
    });
    els.publishResult.textContent = "Committed in " + Math.round(performance.now() - start) + " ms.";
    await refreshMetrics();
  }

  function appendConsumerLine(session, line) {
    session.log.push(line);
    if (session.log.length > 300) session.log = session.log.slice(-300);
    session.logEl.textContent = session.log.join("\n");
  }

  function stopConsumerSession(id) {
    const s = state.consumers.get(id);
    if (!s) return;
    s.controller.abort();
    state.consumers.delete(id);
    s.card.remove();
    els.consumeStatus.textContent = state.consumers.size + " active consumer session(s).";
  }

  async function runConsumerSession(session) {
    const q = new URLSearchParams();
    q.set("id", session.consumerId);
    if (session.topic) q.set("stream", session.topic);
    if (session.fromId) q.set("from_id", session.fromId);
    const url = `${baseURL()}/brokers/${encodeURIComponent(state.selectedBroker)}/subscribe?${q.toString()}`;
    try {
      const response = await fetch(url, { signal: session.controller.signal });
      if (!response.ok || !response.body) throw new Error("connect failed with status " + response.status);
      appendConsumerLine(session, "Connected");
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      while (true) {
        const chunk = await reader.read();
        if (chunk.done) break;
        buf += decoder.decode(chunk.value, { stream: true });
        const lines = buf.split("\n");
        buf = lines.pop() || "";
        lines.forEach((line) => {
          const t = line.trim();
          if (!t) return;
          try {
            const msg = JSON.parse(t);
            appendConsumerLine(session, `[${new Date().toLocaleTimeString()}] ${msg.stream} ${JSON.stringify(msg.payload)}`);
          } catch (_) {
            appendConsumerLine(session, `[raw] ${t}`);
          }
        });
      }
      appendConsumerLine(session, "Disconnected");
    } catch (err) {
      appendConsumerLine(session, session.controller.signal.aborted ? "Stopped by user" : "Error: " + err.message);
    }
  }

  function addConsumerSession() {
    const consumerId = (els.newConsumerId.value || "").trim() || "ui-consumer-" + Date.now();
    const topic = (els.newConsumerTopic.value || "").trim();
    const fromId = (els.newConsumerFromId.value || "").trim();
    const id = consumerId + "::" + Date.now();
    const card = document.createElement("div");
    card.className = "consumer-card";
    card.innerHTML = `<div class="consumer-head"><div class="title">${consumerId}${topic ? " [" + topic + "]" : " [all topics]"}</div><button class="mini danger" data-stop="${id}">Stop</button></div><pre class="console-log">Connecting...</pre>`;
    const logEl = card.querySelector("pre");
    els.consumerSessions.prepend(card);
    const session = { id, consumerId, topic, fromId, card, logEl, log: [], controller: new AbortController() };
    state.consumers.set(id, session);
    card.querySelector("[data-stop]").onclick = () => stopConsumerSession(id);
    els.consumeStatus.textContent = state.consumers.size + " active consumer session(s).";
    runConsumerSession(session);
  }

  async function refreshMetrics() {
    const m = await request(`/brokers/${encodeURIComponent(state.selectedBroker)}/metrics`, { method: "GET" });
    els.metricsResult.textContent = JSON.stringify(m, null, 2);
    els.kpiPublished.textContent = String(m.published_messages || 0);
    els.kpiAcks.textContent = String(m.required_acks || 0);
    els.kpiTopics.textContent = String(m.topics_count || 0);
    els.kpiConsumers.textContent = String(m.active_subscribers || 0);
  }

  async function refreshAll() {
    await loadBrokers();
    await loadTopics();
    await refreshMetrics();
    els.settingsStatus.textContent = "Snapshot refreshed for broker: " + state.selectedBroker;
  }

  function wireEvents() {
    els.tabButtons.forEach((btn) => btn.addEventListener("click", () => activateTab(btn.dataset.tab)));

    els.themeToggle.onclick = () => applyTheme(document.body.getAttribute("data-theme") === "dark" ? "light" : "dark");
    els.saveSettings.onclick = () => { writeStorage(storage.baseUrl, baseURL()); writeStorage(storage.apiKey, els.apiKey.value || ""); writeStorage(storage.selectedBroker, state.selectedBroker); els.settingsStatus.textContent = "Settings saved."; };
    els.applySelectedBroker.onclick = () => { setSelectedBroker(els.selectedBroker.value); refreshAll().catch((e) => els.settingsStatus.textContent = e.message); };
    els.refreshAll.onclick = () => refreshAll().catch((e) => els.settingsStatus.textContent = e.message);

    els.createBroker.onclick = () => createBroker().catch((e) => els.settingsStatus.textContent = e.message);
    els.reloadBrokers.onclick = () => loadBrokers().catch((e) => els.settingsStatus.textContent = e.message);
    els.brokerFilter.oninput = () => { state.brokerPage = 1; renderBrokerTable(); };
    els.brokerSort.onchange = () => { state.brokerPage = 1; renderBrokerTable(); };
    els.brokerPrevPage.onclick = () => { state.brokerPage--; renderBrokerTable(); };
    els.brokerNextPage.onclick = () => { state.brokerPage++; renderBrokerTable(); };
    els.brokerTableBody.onclick = (ev) => {
      const use = ev.target.getAttribute("data-broker-use");
      const edit = ev.target.getAttribute("data-broker-edit");
      const del = ev.target.getAttribute("data-broker-del");
      if (use) { setSelectedBroker(use); refreshAll().catch(() => {}); }
      if (edit) openDrawer("broker", edit);
      if (del) deleteBroker(del).catch((e) => els.settingsStatus.textContent = e.message);
    };

    els.createTopic.onclick = () => createTopic().catch((e) => els.topicsStatus.textContent = e.message);
    els.reloadTopics.onclick = () => loadTopics().catch((e) => els.topicsStatus.textContent = e.message);
    els.topicFilter.oninput = () => { state.topicPage = 1; renderTopicTable(); };
    els.topicSort.onchange = () => { state.topicPage = 1; renderTopicTable(); };
    els.topicPrevPage.onclick = () => { state.topicPage--; renderTopicTable(); };
    els.topicNextPage.onclick = () => { state.topicPage++; renderTopicTable(); };
    els.topicTableBody.onclick = (ev) => {
      const edit = ev.target.getAttribute("data-topic-edit");
      const del = ev.target.getAttribute("data-topic-del");
      if (edit) openDrawer("topic", edit);
      if (del) deleteTopic(del).catch((e) => els.topicsStatus.textContent = e.message);
    };

    els.publishButton.onclick = () => publishMessage().catch((e) => els.publishResult.textContent = "Publish failed: " + e.message);

    els.addConsumer.onclick = addConsumerSession;
    els.stopAllConsumers.onclick = () => Array.from(state.consumers.keys()).forEach((id) => stopConsumerSession(id));

    els.refreshMetrics.onclick = () => refreshMetrics().catch((e) => els.metricsResult.textContent = e.message);

    els.closeDrawer.onclick = closeDrawer;
    els.drawerBackdrop.onclick = closeDrawer;
    els.saveDrawer.onclick = () => saveDrawer().catch((e) => els.settingsStatus.textContent = e.message);
  }

  function init() {
    els.baseUrl.value = readStorage(storage.baseUrl, window.location.origin);
    els.apiKey.value = readStorage(storage.apiKey, "");
    setSelectedBroker(readStorage(storage.selectedBroker, "default"));
    applyTheme(readStorage(storage.theme, "light"));
    els.publishStream.value = "events";
    wireEvents();
    refreshAll().catch((e) => els.settingsStatus.textContent = "Startup refresh failed: " + e.message);
    state.metricsTimer = setInterval(() => refreshMetrics().catch(() => {}), 5000);
  }

  init();
})();
