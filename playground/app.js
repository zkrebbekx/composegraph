"use strict";

const EXAMPLES = {
  "compose-simple": `services:
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8123/ping"]

  fake-gcs-server:
    image: fsouza/fake-gcs-server:latest
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:4443/storage/v1/b"]

  app:
    image: ghcr.io/example/app
    depends_on:
      clickhouse:
        condition: service_healthy
      fake-gcs-server:
        condition: service_healthy
`,
  "compose-network": `services:
  postgres:
    image: postgres:16-alpine
    healthcheck:
      test: ["CMD", "pg_isready"]
    networks: [backend]

  api:
    image: example/api
    depends_on:
      postgres:
        condition: service_healthy
    networks: [backend, frontend]

  web:
    image: example/web
    depends_on: [api]
    networks: [frontend]

networks:
  backend:
  frontend:
`,
  "k8s-app": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: shop
spec:
  template:
    metadata:
      labels: {app: api}
    spec:
      containers:
        - name: api
          image: api:latest
          readinessProbe:
            httpGet: {path: /health, port: 8080}
          envFrom:
            - configMapRef: {name: api-config}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: api-config, namespace: shop}
---
apiVersion: v1
kind: Service
metadata: {name: api-svc, namespace: shop}
spec:
  selector: {app: api}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: {name: api-ingress, namespace: shop}
spec:
  rules:
    - http:
        paths:
          - path: /api
            backend: {service: {name: api-svc}}
`,
  "k8s-owner": `# Shaped like a \`kubectl get all -o yaml\` dump: ownerReferences
# carry the Deployment → ReplicaSet → Pod chain.
apiVersion: apps/v1
kind: Deployment
metadata: {name: app, namespace: shop}
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: app-7f8c9
  namespace: shop
  ownerReferences: [{kind: Deployment, name: app}]
---
apiVersion: v1
kind: Pod
metadata:
  name: app-7f8c9-x2z9k
  namespace: shop
  ownerReferences: [{kind: ReplicaSet, name: app-7f8c9}]
`,
};
const DICE = Object.keys(EXAMPLES);

const $ = (id) => document.getElementById(id);
const ed = $("yaml");
let ready = false, benchTimer = null;

// --- theme (default dark, persisted) ---
const SUN = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5L19 19M19 5l-1.5 1.5M6.5 17.5L5 19"/></svg>';
const MOON = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/></svg>';
function applyTheme(t) {
  document.documentElement.dataset.theme = t;
  $("theme").innerHTML = t === "dark" ? MOON : SUN;
}
applyTheme(localStorage.getItem("composegraph-theme") || "dark");
$("theme").addEventListener("click", () => {
  const t = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
  localStorage.setItem("composegraph-theme", t);
  applyTheme(t);
});

// --- wasm boot ---
window.onComposegraphReady = () => {
  ready = true;
  $("loader").classList.add("gone");
  run();
};
(function boot() {
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch("composegraph.wasm?v=1", { cache: "no-cache" }), go.importObject)
    .then((r) => go.run(r.instance))
    .catch((e) => { $("loader").innerHTML = "<span>failed to load wasm: " + e + "</span>"; });
})();

// --- toolbar ---
$("examples").addEventListener("click", pick);
$("dice").addEventListener("click", pick);
function pick(e) {
  const b = e.target.closest("button");
  if (!b) return;
  let key = b.dataset.ex;
  if (key === "dice") key = DICE[(Math.random() * DICE.length) | 0];
  ed.value = EXAMPLES[key];
  run();
  ed.focus();
}

// --- tabs ---
document.querySelectorAll(".tab").forEach((t) =>
  t.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach((x) => x.classList.remove("active"));
    document.querySelectorAll(".tabpane").forEach((x) => x.classList.remove("active"));
    t.classList.add("active");
    $("tab-" + t.dataset.tab).classList.add("active");
  })
);

// --- live rendering ---
let deb = null;
ed.addEventListener("input", () => { clearTimeout(deb); deb = setTimeout(run, 150); });

const FORMAT = { compose: { cls: "read", label: "docker-compose" }, k8s: { cls: "txn", label: "kubernetes" } };

function run() {
  if (!ready) return;
  const yaml = ed.value.trim();
  if (!yaml) {
    setVerdict("util", "—");
    $("meta").textContent = "paste a docker-compose.yml or Kubernetes manifest";
    $("speed").textContent = "— µs";
    $("tab-diagram").innerHTML = "";
    $("tab-diagram").classList.add("empty");
    $("tab-sql").textContent = ""; $("tab-tree").innerHTML = ""; $("err").hidden = true;
    return;
  }
  const res = composegraphRender(yaml);
  if (!res.ok) { showError(res); return; }
  $("err").hidden = true;
  $("tab-diagram").classList.remove("empty");

  const fmt = FORMAT[res.format] || { cls: "util", label: res.format };
  setVerdict(fmt.cls, fmt.label);
  $("meta").textContent = res.nodeCount + " node" + (res.nodeCount === 1 ? "" : "s") +
    " · " + res.edgeCount + " edge" + (res.edgeCount === 1 ? "" : "s");

  $("tab-diagram").innerHTML = "";
  const img = document.createElement("img");
  img.src = res.svgDataURL;
  img.alt = "rendered dependency graph";
  $("tab-diagram").appendChild(img);

  $("tab-sql").textContent = res.mermaid;
  renderGraphList(res.nodes, res.edges);
  scheduleBench(yaml);
}

function showError(res) {
  setVerdict("bad", "Invalid YAML");
  $("meta").textContent = ""; $("speed").textContent = "— µs";
  $("tab-diagram").innerHTML = ""; $("tab-diagram").classList.add("empty");
  $("err").textContent = res.error || "parse error";
  $("err").hidden = false;
}

function setVerdict(cls, label) {
  $("verdict").className = "pill " + cls;
  $("vlabel").textContent = label;
}

// --- nodes & edges list ---
function renderGraphList(nodes, edges) {
  const root = document.createElement("div");
  root.appendChild(el("div", "stmt-sep", "nodes (" + nodes.length + ")"));
  nodes.forEach((n) => {
    const row = el("div", "row");
    row.appendChild(el("span", "kind", n.name));
    if (n.shape) row.appendChild(el("span", "fname", n.shape));
    if (n.group) row.appendChild(el("span", "count", n.group));
    const wrap = el("div", "node"); wrap.appendChild(row);
    root.appendChild(wrap);
  });
  root.appendChild(el("div", "stmt-sep", "edges (" + edges.length + ")"));
  edges.forEach((e) => {
    const row = el("div", "row");
    row.appendChild(el("span", "val", e.from + " → " + e.to));
    if (e.label) row.appendChild(el("span", "val str", e.label));
    const wrap = el("div", "node"); wrap.appendChild(row);
    root.appendChild(wrap);
  });
  const host = $("tab-tree"); host.innerHTML = ""; host.appendChild(root);
}

function el(tag, cls, text) {
  const e = document.createElement(tag); e.className = cls;
  if (text != null) e.textContent = text; return e;
}

// --- live render-time badge (real in-wasm benchmark) ---
function scheduleBench(yaml) {
  clearTimeout(benchTimer);
  benchTimer = setTimeout(() => {
    const ns = composegraphBench(yaml, 200);
    if (!ns) return;
    const us = ns / 1000;
    const b = $("speed");
    b.textContent = (us < 10 ? us.toFixed(2) : us.toFixed(1)) + " µs";
    b.classList.add("flash"); setTimeout(() => b.classList.remove("flash"), 150);
  }, 320);
}

ed.value = EXAMPLES["compose-simple"];
