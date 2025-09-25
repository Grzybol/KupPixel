/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_TURNSTILE_SITE_KEY: string;
  readonly VITE_TURNSTILE_DEBUG?: string;
}
