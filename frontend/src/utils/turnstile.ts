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

declare global {
interface Window {
turnstile?: TurnstileInstance;
}
}

const SCRIPT_ID = "cloudflare-turnstile";
const SCRIPT_SRC = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";

let loaderPromise: Promise<void> | null = null;

export function loadTurnstile(): Promise<void> {
if (typeof window === "undefined") {
return Promise.reject(new Error("Turnstile is not available in this environment"));
}

if (window.turnstile) {
return Promise.resolve();
}

if (loaderPromise) {
return loaderPromise;
}

loaderPromise = new Promise((resolve, reject) => {
const existing = document.getElementById(SCRIPT_ID) as HTMLScriptElement | null;
if (existing) {
if (window.turnstile) {
resolve();
return;
}
existing.addEventListener("load", () => resolve(), { once: true });
existing.addEventListener(
"error",
() => reject(new Error("Failed to load Cloudflare Turnstile")),
{ once: true }
);
return;
}

const script = document.createElement("script");
script.id = SCRIPT_ID;
script.src = SCRIPT_SRC;
script.async = true;
script.defer = true;
script.onload = () => resolve();
script.onerror = () => reject(new Error("Failed to load Cloudflare Turnstile"));
document.head.appendChild(script);
});

return loaderPromise;
}
