// PortPilot desktop UI: port scanner, live listening-port sidebar with search,
// and a kill flow with confirmation.

export { };

type PortStatus = "AVAILABLE" | "OCCUPIED" | "ERROR" | "INVALID";

interface PortCheckResult {
  port: number;
  service?: string;
  status: PortStatus;
  message: string;
}

interface ActivePort {
  port: number;
  protocol: string;
  service?: string;
  pid: number;
  process?: string;
  verified: boolean;
}

interface ListPortsResult {
  ports: ActivePort[];
  error?: string;
}

interface PortOwner {
  port: number;
  service?: string;
  pid: number;
  process: string;
  verified: boolean;
  status: string;
  message: string;
}

interface KillResult {
  port: number;
  pid: number;
  status: string;
  message: string;
}

interface WailsApp {
  CheckPort(port: number, timeoutMS: number): Promise<PortCheckResult>;
  ListPorts(): Promise<ListPortsResult>;
  GetPortOwner(port: number): Promise<PortOwner>;
  KillPort(port: number, pid: number): Promise<KillResult>;
}

interface WailsRuntime {
  EventsOn(event: string, callback: (...data: any[]) => void): () => void;
}

declare global {
  interface Window {
    go?: { desktop?: { App?: WailsApp } };
    runtime?: WailsRuntime;
  }
}

const MIN_PORT = 1;
const MAX_PORT = 65535;
const MIN_TIMEOUT_MS = 100;
const MAX_TIMEOUT_MS = 10000;
const DEFAULT_PORT = 3000;
const DEFAULT_TIMEOUT_MS = 2000;

const EM_DASH = "\u2014";

const STATUS_LABELS: Record<PortStatus, string> = {
  AVAILABLE: "Available",
  OCCUPIED: "Occupied",
  ERROR: "Error",
  INVALID: "Invalid input",
};

function byId<T extends HTMLElement>(id: string): T {
  const node = document.getElementById(id);
  if (!node) throw new Error(`Missing element #${id}`);
  return node as T;
}

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function getApp(): WailsApp | null {
  return window.go?.desktop?.App ?? null;
}

function parseIntInRange(raw: string, min: number, max: number): number | null {
  const trimmed = raw.trim();
  if (!/^-?\d+$/.test(trimmed)) return null;
  const value = Number(trimmed);
  return value < min || value > max ? null : value;
}

function showToast(kind: "success" | "error" | "info", text: string): void {
  const region = byId<HTMLElement>("toast-region");
  region.appendChild(el("div", `toast toast-${kind}`, text));
  window.setTimeout(() => region.firstElementChild?.remove(), 5000);
}

function matchesPortQuery(p: ActivePort, query: string): boolean {
  if (!query) return true;
  const haystack = [
    String(p.port),
    p.protocol,
    p.service ?? "",
    p.pid > 0 ? String(p.pid) : "",
    p.process ?? "",
  ]
    .join(" ")
    .toLowerCase();
  return query.split(/\s+/).every((term) => haystack.includes(term));
}

function init(): void {
  const portInput = byId<HTMLInputElement>("port");
  const timeoutInput = byId<HTMLInputElement>("timeout");
  const portError = byId<HTMLElement>("port-error");
  const timeoutError = byId<HTMLElement>("timeout-error");
  const checkButton = byId<HTMLButtonElement>("check");
  const checkLabel = byId<HTMLElement>("check-label");
  const badge = byId<HTMLElement>("status-badge");
  const message = byId<HTMLElement>("message");
  const resultPort = byId<HTMLElement>("result-port");
  const resultService = byId<HTMLElement>("result-service");
  const killButton = byId<HTMLButtonElement>("kill-btn");
  const refreshButton = byId<HTMLButtonElement>("refresh-ports");
  const sidebarStatus = byId<HTMLElement>("sidebar-status");
  const portList = byId<HTMLUListElement>("port-list");
  const portSearch = byId<HTMLInputElement>("port-search");
  const dialog = byId<HTMLDialogElement>("confirm-dialog");
  const confirmBody = byId<HTMLElement>("confirm-body");
  const confirmCancel = byId<HTMLButtonElement>("confirm-cancel");
  const confirmOk = byId<HTMLButtonElement>("confirm-ok");

  portInput.min = String(MIN_PORT);
  portInput.max = String(MAX_PORT);
  portInput.value = String(DEFAULT_PORT);
  timeoutInput.min = String(MIN_TIMEOUT_MS);
  timeoutInput.max = String(MAX_TIMEOUT_MS);
  timeoutInput.value = String(DEFAULT_TIMEOUT_MS);

  let currentPort: number | null = null;
  let pendingKill: { port: number; pid: number } | null = null;
  let portSnapshot: ActivePort[] = [];
  let portListError: string | undefined;

  function renderStatus(status: PortStatus): void {
    badge.dataset.status = status;
    badge.textContent = STATUS_LABELS[status];
  }

  function setChecking(busy: boolean): void {
    checkButton.disabled = busy;
    checkButton.setAttribute("aria-busy", String(busy));
    checkLabel.textContent = busy ? "Checking..." : "Check port";
  }

  function validate(): { port: number; timeoutMS: number } | null {
    const port = parseIntInRange(portInput.value, MIN_PORT, MAX_PORT);
    const timeoutMS = parseIntInRange(
      timeoutInput.value,
      MIN_TIMEOUT_MS,
      MAX_TIMEOUT_MS,
    );

    portError.textContent =
      port === null
        ? `Enter a whole number from ${MIN_PORT} to ${MAX_PORT}.`
        : "";
    timeoutError.textContent =
      timeoutMS === null
        ? `Enter a whole number from ${MIN_TIMEOUT_MS} to ${MAX_TIMEOUT_MS} ms.`
        : "";
    portInput.setAttribute("aria-invalid", String(port === null));
    timeoutInput.setAttribute("aria-invalid", String(timeoutMS === null));

    if (port === null || timeoutMS === null) return null;
    return { port, timeoutMS };
  }

  async function runCheck(): Promise<void> {
    const input = validate();
    killButton.hidden = true;
    currentPort = null;

    if (!input) {
      renderStatus("INVALID");
      resultService.textContent = EM_DASH;
      message.textContent = "Fix the highlighted fields and try again.";
      return;
    }

    const app = getApp();
    if (!app) {
      renderStatus("ERROR");
      message.textContent =
        "The Go backend is not available. Run this inside Wails.";
      return;
    }

    setChecking(true);
    renderStatus("ERROR");
    message.textContent = "Checking...";
    resultPort.textContent = String(input.port);
    resultService.textContent = EM_DASH;

    try {
      const result = await app.CheckPort(input.port, input.timeoutMS);
      renderStatus(result.status);
      message.textContent = result.message;
      resultPort.textContent = String(result.port);
      resultService.textContent = result.service || EM_DASH;
      if (result.status === "OCCUPIED") {
        currentPort = result.port;
        killButton.hidden = false;
      }
    } catch (err) {
      renderStatus("ERROR");
      message.textContent = `Request failed: ${String(err)}`;
    } finally {
      setChecking(false);
    }
  }

  function buildPortItem(p: ActivePort): HTMLLIElement {
    const item = el("li");
    const button = el("button", "port-item");
    button.type = "button";

    // Service name first, then process name, then a placeholder.
    const title = p.service || p.process || "Unknown process";

    // When a service is shown, the process name moves to the detail line.
    const processDetail = p.service && p.process ? p.process : "";

    const label = `Check port ${p.port}, ${title}`;
    button.setAttribute("aria-label", label);
    button.title = label;

    const top = el("div", "port-item-top");
    top.append(
      el("span", "port-number", String(p.port)),
      el("span", "port-proto", p.protocol.toUpperCase()),
    );

    const meta = [
      processDetail,
      p.pid > 0 ? `PID ${p.pid}` : "",
      p.verified ? "" : "unverified",
    ]
      .filter(Boolean)
      .join(" · ");

    button.append(
      top,
      el("div", "port-item-sub", title),
      el("div", "port-item-meta", meta || EM_DASH),
    );

    button.addEventListener("click", () => {
      portInput.value = String(p.port);
      void runCheck();
    });

    item.appendChild(button);
    return item;
  }

  function applyPortFilter(): void {
    if (portListError) {
      portList.replaceChildren();
      sidebarStatus.textContent = `Could not list ports: ${portListError}`;
      return;
    }

    const rawQuery = portSearch.value.trim();
    const query = rawQuery.toLowerCase();
    const matches = portSnapshot.filter((p) => matchesPortQuery(p, query));

    portList.replaceChildren(...matches.map((p) => buildPortItem(p)));

    if (portSnapshot.length === 0) {
      sidebarStatus.textContent = "No listening TCP ports found.";
    } else if (matches.length === 0) {
      sidebarStatus.textContent = `No ports match "${rawQuery}".`;
    } else if (query) {
      sidebarStatus.textContent = `${matches.length} of ${portSnapshot.length} port(s) · live`;
    } else {
      sidebarStatus.textContent = `${portSnapshot.length} listening port(s) · live`;
    }
  }

  function renderPorts(result: ListPortsResult): void {
    portListError = result.error || undefined;
    portSnapshot = result.ports;
    applyPortFilter();
  }

  async function refreshPorts(): Promise<void> {
    const app = getApp();
    if (!app) {
      sidebarStatus.textContent = "Backend not available.";
      return;
    }

    refreshButton.disabled = true;
    try {
      renderPorts(await app.ListPorts());
    } catch (err) {
      sidebarStatus.textContent = `Request failed: ${String(err)}`;
    } finally {
      refreshButton.disabled = false;
    }
  }

  async function openKillDialog(port: number): Promise<void> {
    const app = getApp();
    if (!app) {
      showToast("error", "The Go backend is not available.");
      return;
    }

    killButton.disabled = true;
    try {
      const owner = await app.GetPortOwner(port);
      if (owner.status !== "FOUND") {
        showToast("error", owner.message);
        return;
      }

      pendingKill = { port: owner.port, pid: owner.pid };
      const serviceNote = owner.service ? ` (${owner.service})` : "";
      confirmBody.textContent =
        `Terminate "${owner.process}" (PID ${owner.pid}) listening on port ${owner.port}${serviceNote}? ` +
        "Unsaved work in that process may be lost.";
      dialog.showModal();
      confirmCancel.focus();
    } catch (err) {
      showToast("error", `Could not identify the owner: ${String(err)}`);
    } finally {
      killButton.disabled = false;
    }
  }

  async function confirmKill(): Promise<void> {
    const target = pendingKill;
    pendingKill = null;
    dialog.close();
    if (!target) return;

    const app = getApp();
    if (!app) return;

    confirmOk.disabled = true;
    try {
      const result = await app.KillPort(target.port, target.pid);
      showToast(
        result.status === "KILLED" ? "success" : "error",
        result.message,
      );

      if (result.status === "KILLED") {
        portInput.value = String(target.port);
        await runCheck();
        await refreshPorts();
      }
    } catch (err) {
      showToast("error", `Termination request failed: ${String(err)}`);
    } finally {
      confirmOk.disabled = false;
    }
  }

  checkButton.addEventListener("click", () => {
    void runCheck();
  });
  portInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") void runCheck();
  });
  timeoutInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") void runCheck();
  });

  killButton.addEventListener("click", () => {
    if (currentPort !== null) void openKillDialog(currentPort);
  });
  confirmCancel.addEventListener("click", () => {
    pendingKill = null;
    dialog.close();
  });
  confirmOk.addEventListener("click", () => {
    void confirmKill();
  });
  dialog.addEventListener("close", () => {
    pendingKill = null;
  });

  refreshButton.addEventListener("click", () => {
    void refreshPorts();
  });
  portSearch.addEventListener("input", () => applyPortFilter());
  portSearch.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && portSearch.value) {
      portSearch.value = "";
      applyPortFilter();
    }
  });

  // Disable the default right-click context menu across the app.
  document.addEventListener("contextmenu", (e) => e.preventDefault());

  // Block Ctrl+A (Cmd+A on macOS) outside text fields, so search and port
  // inputs still allow select-all.
  document.addEventListener("keydown", (e) => {
    const inText =
      e.target instanceof HTMLInputElement ||
      e.target instanceof HTMLTextAreaElement;
    if (!inText && (e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "a") {
      e.preventDefault();
    }
  });

  window.runtime?.EventsOn("ports:updated", (data: ListPortsResult) => {
    renderPorts(data);
  });

  void refreshPorts();
}

document.addEventListener("DOMContentLoaded", init);
