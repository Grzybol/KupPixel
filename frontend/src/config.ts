const rawSiteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY;

if (typeof rawSiteKey !== "string") {
  throw new Error("VITE_TURNSTILE_SITE_KEY environment variable is not defined");
}

const normalizedSiteKey = rawSiteKey.trim();

if (normalizedSiteKey.length === 0) {
  throw new Error("VITE_TURNSTILE_SITE_KEY environment variable cannot be empty");
}

export const TURNSTILE_SITE_KEY = normalizedSiteKey;
