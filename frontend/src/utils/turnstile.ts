export type TurnstileWidgetId = string;

export type TurnstileRenderOptions = {
  sitekey: string;
  callback?: (token: string) => void;
  "expired-callback"?: () => void;
  "error-callback"?: () => void;
  action?: string;
  theme?: "light" | "dark" | "auto";
};

export interface TurnstileInstance {
  render: (container: HTMLElement, options: TurnstileRenderOptions) => TurnstileWidgetId;
  reset: (widgetId?: TurnstileWidgetId) => void;
  remove: (widgetId?: TurnstileWidgetId) => void;
}

export type TurnstileDebugOptions = {
  status?: string;
  detail?: unknown;
  error?: string;
  meta?: Record<string, unknown>;
};

declare global {
  interface Window {
    turnstile?: TurnstileInstance;
  }
}

const SCRIPT_ID = "cloudflare-turnstile";
const SCRIPT_SRC = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
const DEBUG_ENDPOINT = "/api/debug/turnstile";
const TURNSTILE_DEBUG_ENABLED = import.meta.env.VITE_TURNSTILE_DEBUG === "true";

function normalizeDetail(detail: unknown): unknown {
  if (detail === undefined) {
    return undefined;
  }
  if (detail === null) {
    return null;
  }
  if (detail instanceof Error) {
    return {
      name: detail.name,
      message: detail.message,
      stack: detail.stack,
    };
  }
  if (typeof detail === "string" || typeof detail === "number" || typeof detail === "boolean") {
    return detail;
  }
  try {
    return JSON.parse(JSON.stringify(detail));
  } catch (error) {
    return String(detail);
  }
}

async function postTurnstileDebug(payload: Record<string, unknown>): Promise<void> {
  if (typeof fetch !== "function") {
    return;
  }
  try {
    await fetch(DEBUG_ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(payload),
      keepalive: true,
    });
  } catch (error) {
    if (import.meta.env.DEV) {
      console.warn("Failed to report Turnstile debug", error);
    }
  }
}

export function reportTurnstileDebug(stage: string, options: TurnstileDebugOptions = {}): void {
  if (!TURNSTILE_DEBUG_ENABLED || typeof window === "undefined") {
    return;
  }
  const payload: Record<string, unknown> = { stage };
  if (options.status) {
    payload.status = options.status;
  }
  const normalizedDetail = normalizeDetail(options.detail);
  if (normalizedDetail !== undefined) {
    payload.detail = normalizedDetail;
  }
  if (options.error) {
    payload.error = options.error;
  }
  if (options.meta && Object.keys(options.meta).length > 0) {
    payload.meta = options.meta;
  }
  void postTurnstileDebug(payload);
}

let loaderPromise: Promise<void> | null = null;

export function loadTurnstile(): Promise<void> {
  if (typeof window === "undefined") {
    return Promise.reject(new Error("Turnstile is not available in this environment"));
  }

  if (window.turnstile) {
    reportTurnstileDebug("loader:existing-instance", { status: "ready" });
    return Promise.resolve();
  }

  if (loaderPromise) {
    reportTurnstileDebug("loader:pending", { status: "pending" });
    return loaderPromise;
  }

  reportTurnstileDebug("loader:start", { status: "pending" });
  loaderPromise = new Promise((resolve, reject) => {
    const existing = document.getElementById(SCRIPT_ID) as HTMLScriptElement | null;
    if (existing) {
      reportTurnstileDebug("loader:reuse-script", { status: "pending" });
      if (window.turnstile) {
        reportTurnstileDebug("loader:reuse-script-ready", { status: "ready" });
        resolve();
        return;
      }
      existing.addEventListener(
        "load",
        () => {
          reportTurnstileDebug("loader:script-loaded", { status: "success" });
          resolve();
        },
        { once: true }
      );
      existing.addEventListener(
        "error",
        () => {
          reportTurnstileDebug("loader:script-error", { status: "error", error: "script load error" });
          reject(new Error("Failed to load Cloudflare Turnstile"));
        },
        { once: true }
      );
      return;
    }

    reportTurnstileDebug("loader:inject", { status: "pending" });
    const script = document.createElement("script");
    script.id = SCRIPT_ID;
    script.src = SCRIPT_SRC;
    script.async = true;
    script.defer = true;
    script.onload = () => {
      reportTurnstileDebug("loader:script-loaded", { status: "success" });
      resolve();
    };
    script.onerror = () => {
      reportTurnstileDebug("loader:script-error", { status: "error", error: "script load error" });
      reject(new Error("Failed to load Cloudflare Turnstile"));
    };
    document.head.appendChild(script);
  });

  return loaderPromise;
}
