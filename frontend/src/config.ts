const rawSiteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY;

export const TURNSTILE_SITE_KEY = typeof rawSiteKey === "string" ? rawSiteKey.trim() : "";
