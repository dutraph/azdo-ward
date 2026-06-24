/* azdo-ward frontend — vanilla JS, no build step.
   Talks only to the local Go server; the PAT stays server-side. */
(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const el = (tag, cls, txt) => {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (txt != null) e.textContent = txt;
    return e;
  };

  let ALL = []; // last loaded entries

  // ---- theme -----------------------------------------------------------
  // Dark by default; respect a stored choice, else the OS preference.
  const THEME_KEY = "azdo-ward-theme";
  function initTheme() {
    let t = null;
    try { t = localStorage.getItem(THEME_KEY); } catch (_) {}
    if (t !== "light" && t !== "dark") {
      const prefersLight = window.matchMedia && window.matchMedia("(prefers-color-scheme: light)").matches;
      t = prefersLight ? "light" : "dark";
    }
    applyTheme(t);
  }
  function applyTheme(t) {
    document.documentElement.setAttribute("data-theme", t);
    const btn = $("themeToggle");
    if (btn) {
      btn.textContent = t === "light" ? "☀️" : "🌙";
      btn.title = t === "light" ? "Switch to dark theme" : "Switch to light theme";
    }
  }
  function toggleTheme() {
    const cur = document.documentElement.getAttribute("data-theme") === "light" ? "light" : "dark";
    const next = cur === "light" ? "dark" : "light";
    applyTheme(next);
    try { localStorage.setItem(THEME_KEY, next); } catch (_) {}
  }

  // ---- categories ------------------------------------------------------
  const CATS = ["create", "modify", "remove", "access", "execute"];
  const catClass = (c) => (CATS.includes((c || "").toLowerCase()) ? (c || "").toLowerCase() : "unknown");

  let STATE = null; // last /api/state response
  const ADD = "__add__"; // sentinel option value in the switcher

  // ---- boot ------------------------------------------------------------
  async function boot() {
    try {
      const st = await api("/api/state");
      $("version").textContent = st.version || "";
      STATE = st;
      if (st.configured) showDashboard(st);
      else showConnect();
    } catch (e) {
      showConnect();
    }
  }

  function showConnect() {
    $("connectView").classList.remove("hidden");
    $("dashView").classList.add("hidden");
    $("orgSwitcher").classList.add("hidden");
    $("orgPill").classList.remove("hidden");
    $("orgPill").textContent = "not connected";
    // Offer "Cancel" only when there's already a configured org to return to.
    const hasOrgs = STATE && STATE.orgs && STATE.orgs.length > 0;
    $("connectCancel").classList.toggle("hidden", !hasOrgs);
  }

  function showDashboard(st) {
    STATE = st;
    $("connectView").classList.add("hidden");
    $("dashView").classList.remove("hidden");
    $("orgPill").classList.add("hidden");
    $("orgSwitcher").classList.remove("hidden");
    populateSwitcher(st);
    setRange(7);
  }

  function populateSwitcher(st) {
    const sel = $("orgSelect");
    sel.innerHTML = "";
    (st.orgs || []).forEach((o) => {
      const opt = el("option", null, o.name + (o.auth ? `  (${o.auth})` : ""));
      opt.value = o.name;
      sel.appendChild(opt);
    });
    const add = el("option", null, "+ Add organization…");
    add.value = ADD;
    sel.appendChild(add);
    sel.value = st.active || "";
  }

  // ---- org switching / removal ----------------------------------------
  $("orgSelect").addEventListener("change", async () => {
    const val = $("orgSelect").value;
    if (val === ADD) {
      $("orgSelect").value = (STATE && STATE.active) || "";
      resetConnectForm();
      showConnect();
      return;
    }
    try {
      const st = await api("/api/switch", { method: "POST", body: JSON.stringify({ org: val }) });
      showDashboard(st);
      load(); // auto-reload the same time window for the new org
    } catch (e) {
      showLoadError(e.message);
    }
  });

  $("removeOrgBtn").addEventListener("click", async () => {
    const cur = $("orgSelect").value;
    if (!cur || cur === ADD) return;
    if (!confirm(`Remove "${cur}" from saved organizations? This deletes its stored token from the local config.`)) return;
    try {
      const st = await api("/api/remove", { method: "POST", body: JSON.stringify({ org: cur }) });
      if (st.configured) {
        showDashboard(st);
        load();
      } else {
        ALL = [];
        STATE = st;
        resetConnectForm();
        showConnect();
      }
    } catch (e) {
      showLoadError(e.message);
    }
  });

  $("addOrg").addEventListener("click", () => {
    resetConnectForm();
    showConnect();
  });

  // ---- connect ---------------------------------------------------------
  // Toggle the PAT field vs the Entra hint based on the chosen auth mode.
  $("authMode").addEventListener("change", () => {
    const entra = $("authMode").value === "entra";
    $("patField").classList.toggle("hidden", entra);
    $("entraHint").classList.toggle("hidden", !entra);
  });

  function resetConnectForm() {
    $("org").value = "";
    $("pat").value = "";
    $("authMode").value = "pat";
    $("patField").classList.remove("hidden");
    $("entraHint").classList.add("hidden");
    $("connectError").classList.add("hidden");
  }

  $("connectCancel").addEventListener("click", () => {
    if (STATE && STATE.configured) showDashboard(STATE);
  });

  $("connectBtn").addEventListener("click", async () => {
    const org = $("org").value.trim();
    const auth = $("authMode").value;
    const pat = $("pat").value.trim();
    const errBox = $("connectError");
    errBox.classList.add("hidden");
    if (!org) {
      errBox.textContent = "Organization is required.";
      errBox.classList.remove("hidden");
      return;
    }
    if (auth === "pat" && !pat) {
      errBox.textContent = "A PAT is required for PAT authentication (or switch to Entra).";
      errBox.classList.remove("hidden");
      return;
    }
    try {
      const st = await api("/api/connect", { method: "POST", body: JSON.stringify({ org, pat, auth }) });
      $("pat").value = "";
      showDashboard(st);
      load();
    } catch (e) {
      errBox.textContent = e.message;
      errBox.classList.remove("hidden");
    }
  });

  function showLoadError(msg) {
    const errorBox = $("errorBox");
    errorBox.textContent = msg;
    errorBox.classList.remove("hidden");
  }

  // ---- date range ------------------------------------------------------
  function setRange(days) {
    const end = new Date();
    const start = new Date(end.getTime() - days * 86400000);
    $("start").value = toLocalInput(start);
    $("end").value = toLocalInput(end);
  }
  document.querySelectorAll("[data-range]").forEach((b) =>
    b.addEventListener("click", () => setRange(parseInt(b.dataset.range, 10)))
  );

  // ---- load ------------------------------------------------------------
  $("loadBtn").addEventListener("click", load);
  async function load() {
    const status = $("status");
    const errorBox = $("errorBox");
    errorBox.classList.add("hidden");
    status.innerHTML = '<span class="spinner"></span> Querying audit log…';
    const params = new URLSearchParams();
    if ($("start").value) params.set("start", new Date($("start").value).toISOString());
    if ($("end").value) params.set("end", new Date($("end").value).toISOString());
    try {
      const res = await api("/api/audit?" + params.toString());
      ALL = res.entries || [];
      status.textContent = `${res.count} events from ${res.org}`;
      populateFilters(ALL);
      render();
    } catch (e) {
      status.textContent = "";
      errorBox.textContent = e.message;
      errorBox.classList.remove("hidden");
    }
  }

  // ---- filters ---------------------------------------------------------
  ["search", "fCategory", "fArea", "fActor", "groupBy"].forEach((id) =>
    $(id).addEventListener("input", render)
  );

  function populateFilters(entries) {
    fill($("fCategory"), uniq(entries.map((e) => e.categoryDisplayName || e.category)));
    fill($("fArea"), uniq(entries.map((e) => e.area)));
    fill($("fActor"), uniq(entries.map((e) => e.actorDisplayName)));
  }
  function fill(sel, values) {
    const cur = sel.value;
    sel.innerHTML = '<option value="">All</option>';
    values.sort((a, b) => a.localeCompare(b)).forEach((v) => {
      const o = el("option", null, v);
      o.value = v;
      sel.appendChild(o);
    });
    if (values.includes(cur)) sel.value = cur;
  }

  function filtered() {
    const q = $("search").value.trim().toLowerCase();
    const cat = $("fCategory").value;
    const area = $("fArea").value;
    const actor = $("fActor").value;
    return ALL.filter((e) => {
      if (cat && (e.categoryDisplayName || e.category) !== cat) return false;
      if (area && e.area !== area) return false;
      if (actor && e.actorDisplayName !== actor) return false;
      if (q) {
        const hay = [e.actionId, e.details, e.actorDisplayName, e.area, e.scopeDisplayName, e.ipAddress, JSON.stringify(e.data)]
          .join(" ").toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }

  // ---- render ----------------------------------------------------------
  function render() {
    const items = filtered().slice().sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
    renderStats(items);

    const timeline = $("timeline");
    timeline.innerHTML = "";
    if (items.length === 0) {
      timeline.appendChild(el("div", "empty", ALL.length ? "No events match the current filters." : "Load the audit log to begin."));
      return;
    }

    const groupBy = $("groupBy").value;
    if (groupBy === "none") {
      items.forEach((e) => timeline.appendChild(entryCard(e)));
      return;
    }
    const groups = groupEntries(items, groupBy);
    for (const [key, list] of groups) {
      const head = el("div", "group-head");
      head.appendChild(el("span", null, key));
      head.appendChild(el("span", "count", `${list.length} event${list.length === 1 ? "" : "s"}`));
      timeline.appendChild(head);
      list.forEach((e) => timeline.appendChild(entryCard(e)));
    }
  }

  function renderStats(items) {
    const stats = $("stats");
    stats.innerHTML = "";
    const counts = { total: items.length, create: 0, modify: 0, remove: 0 };
    items.forEach((e) => {
      const c = (e.category || "").toLowerCase();
      if (c in counts) counts[c]++;
    });
    const defs = [
      ["total", "Total", ""],
      ["create", "Created", "create"],
      ["modify", "Modified", "modify"],
      ["remove", "Removed", "remove"],
    ];
    defs.forEach(([k, label, cls]) => {
      const s = el("div", "stat " + cls);
      s.appendChild(el("div", "n", String(counts[k])));
      s.appendChild(el("div", "l", label));
      stats.appendChild(s);
    });
  }

  function groupEntries(items, by) {
    const m = new Map();
    const keyFn = {
      day: (e) => new Date(e.timestamp).toISOString().slice(0, 10),
      actor: (e) => e.actorDisplayName || "(unknown)",
      area: (e) => e.area || "(none)",
      category: (e) => e.categoryDisplayName || e.category || "(none)",
      action: (e) => e.actionId || "(none)",
    }[by];
    items.forEach((e) => {
      const k = keyFn(e);
      if (!m.has(k)) m.set(k, []);
      m.get(k).push(e);
    });
    // keep map insertion order (already sorted by time for day; otherwise alpha)
    if (by !== "day") {
      return new Map([...m.entries()].sort((a, b) => b[1].length - a[1].length));
    }
    return m;
  }

  // ---- one entry -------------------------------------------------------
  function entryCard(e) {
    const cls = catClass(e.category);
    const card = el("div", "entry cat-" + cls);

    const head = el("div", "entry-head");
    head.appendChild(badge(e));
    head.appendChild(el("span", "action", e.actionId || "—"));
    head.appendChild(el("span", "details", e.details || ""));
    head.appendChild(el("span", "actor", e.actorDisplayName || ""));
    head.appendChild(el("span", "meta", fmtTime(e.timestamp)));
    const chev = el("span", "chevron", "›");
    head.appendChild(chev);
    head.addEventListener("click", () => card.classList.toggle("open"));
    card.appendChild(head);

    const body = el("div", "entry-body");
    // before/after diff
    const diffs = detectDiffs(e.data);
    if (diffs.length) body.appendChild(renderDiff(diffs));
    // metadata grid
    body.appendChild(metaGrid(e));
    // raw data
    if (e.data && Object.keys(e.data).length) {
      const pre = el("pre", "raw", JSON.stringify(e.data, null, 2));
      body.appendChild(pre);
    }
    card.appendChild(body);
    return card;
  }

  function badge(e) {
    const cls = catClass(e.category);
    return el("span", "badge " + cls, (e.categoryDisplayName || e.category || "event"));
  }

  function metaGrid(e) {
    const kv = el("div", "kv");
    const add = (k, v) => {
      if (v == null || v === "") return;
      kv.appendChild(el("span", "k", k));
      kv.appendChild(el("span", "v", String(v)));
    };
    add("timestamp", e.timestamp);
    add("actor", e.actorDisplayName);
    add("actorUPN", e.actorUPN);
    add("area", e.area);
    add("scope", e.scopeDisplayName);
    add("project", e.projectName);
    add("ip", e.ipAddress);
    add("auth", e.authenticationMechanism);
    add("correlationId", e.correlationId);
    return kv;
  }

  // ---- before/after detection -----------------------------------------
  // The Audit API's `data` object is action-specific and has no single
  // before/after schema, so we heuristically pair keys: Old/New,
  // Previous/Current, From/To, and a generic <X>Old / <X>New.
  function detectDiffs(data) {
    if (!data || typeof data !== "object") return [];
    const out = [];
    const keys = Object.keys(data);
    const lower = {};
    keys.forEach((k) => (lower[k.toLowerCase()] = k));
    const used = new Set();

    const pairs = [
      [/^old(.*)$/i, (m) => ["new" + m, m || "value"]],
      [/^previous(.*)$/i, (m) => ["current" + m, m || "value", "new" + m]],
      [/^from(.+)$/i, (m) => ["to" + m, m]],
      [/^(.*)old$/i, (m) => [m + "new", m || "value"]],
      [/^(.*)before$/i, (m) => [m + "after", m || "value"]],
    ];

    keys.forEach((k) => {
      if (used.has(k)) return;
      for (const [re, fn] of pairs) {
        const mm = k.match(re);
        if (!mm) continue;
        const spec = fn(mm[1] || "");
        const label = spec[1];
        // find the matching "new" key among candidates
        const candidates = spec.filter((_, i) => i !== 1);
        for (const cand of candidates) {
          const nk = lower[cand.toLowerCase()];
          if (nk && !used.has(nk) && nk !== k) {
            out.push({ field: prettify(label), old: data[k], neu: data[nk] });
            used.add(k);
            used.add(nk);
            break;
          }
        }
        if (used.has(k)) break;
      }
    });
    return out;
  }

  function renderDiff(diffs) {
    const wrap = el("div", "diff");
    wrap.appendChild(el("div", "diff-title", "Changes"));
    diffs.forEach((d) => {
      const row = el("div", "diff-row");
      row.appendChild(el("div", "field-name", d.field));
      const vals = el("div", "diff-vals");
      vals.appendChild(el("div", "diff-old", valStr(d.old)));
      vals.appendChild(el("div", "diff-new", valStr(d.neu)));
      row.appendChild(vals);
      wrap.appendChild(row);
    });
    return wrap;
  }

  // ---- export ----------------------------------------------------------
  $("exportJson").addEventListener("click", () => {
    download("azdo-ward-audit.json", JSON.stringify(filtered(), null, 2), "application/json");
  });
  $("exportCsv").addEventListener("click", () => {
    const rows = filtered();
    const cols = ["timestamp", "category", "actionId", "area", "actorDisplayName", "actorUPN", "scopeDisplayName", "projectName", "ipAddress", "details"];
    const lines = [cols.join(",")];
    rows.forEach((e) => lines.push(cols.map((c) => csv(e[c])).join(",")));
    download("azdo-ward-audit.csv", lines.join("\n"), "text/csv");
  });

  // ---- helpers ---------------------------------------------------------
  async function api(path, opts) {
    const r = await fetch(path, Object.assign({ headers: { "Content-Type": "application/json" } }, opts));
    let data = {};
    try { data = await r.json(); } catch (_) {}
    if (!r.ok) throw new Error(data.error || `${r.status} ${r.statusText}`);
    return data;
  }
  const uniq = (arr) => [...new Set(arr.filter(Boolean))];
  function fmtTime(ts) {
    if (!ts) return "";
    const d = new Date(ts);
    return d.toLocaleString(undefined, { month: "short", day: "2-digit", hour: "2-digit", minute: "2-digit" });
  }
  function toLocalInput(d) {
    const p = (n) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
  }
  function valStr(v) {
    if (v == null) return "∅";
    if (typeof v === "object") return JSON.stringify(v);
    return String(v);
  }
  function prettify(s) {
    return s.replace(/([a-z])([A-Z])/g, "$1 $2").replace(/[_.]/g, " ").trim() || "value";
  }
  function csv(v) {
    if (v == null) return "";
    const s = String(v).replace(/"/g, '""');
    return /[",\n]/.test(s) ? `"${s}"` : s;
  }
  function download(name, content, type) {
    const blob = new Blob([content], { type });
    const a = el("a");
    a.href = URL.createObjectURL(blob);
    a.download = name;
    a.click();
    URL.revokeObjectURL(a.href);
  }
  function escapeHtml(s) {
    return String(s || "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  }

  $("themeToggle").addEventListener("click", toggleTheme);

  initTheme();
  boot();
})();
