import { useEffect, useRef, useState } from "react";
import { loadTurnstile } from "../utils/turnstile";
import { useI18n } from "../lang/I18nProvider";

type TurnstileWidgetProps = {
siteKey: string;
onTokenChange: (token: string) => void;
onError?: (message: string) => void;
onExpire?: () => void;
action?: string;
className?: string;
resetKey?: number | string;
};

type LoadState = "idle" | "loading" | "ready" | "error";

export default function TurnstileWidget({
  siteKey,
  onTokenChange,
  onError,
  onExpire,
  action,
  className,
  resetKey,
}: TurnstileWidgetProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const widgetIdRef = useRef<string | null>(null);
  const onTokenChangeRef = useRef(onTokenChange);
  const onErrorRef = useRef<TurnstileWidgetProps["onError"] | null>(onError ?? null);
  const onExpireRef = useRef<TurnstileWidgetProps["onExpire"] | null>(onExpire ?? null);
  const { t, language } = useI18n();
  const [state, setState] = useState<LoadState>("idle");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const translateRef = useRef(t);

  useEffect(() => {
    translateRef.current = t;
  }, [t]);

  useEffect(() => {
    onTokenChangeRef.current = onTokenChange;
  }, [onTokenChange]);

  useEffect(() => {
    onErrorRef.current = onError ?? null;
  }, [onError]);

  useEffect(() => {
    onExpireRef.current = onExpire ?? null;
  }, [onExpire]);

  useEffect(() => {
    let isActive = true;

    onTokenChangeRef.current("");

    const translate = translateRef.current;

    if (!siteKey) {
      const message = translate("auth.captcha.missing");
      setState("error");
      setErrorMessage(message);
      onErrorRef.current?.(message);
      return () => {
        onTokenChangeRef.current("");
      };
    }

    setState("loading");
    setErrorMessage(null);

    loadTurnstile()
      .then(() => {
        if (!isActive) {
          return;
        }
        if (!containerRef.current || !window.turnstile) {
          const message = translate("auth.captcha.error");
          setState("error");
          setErrorMessage(message);
          onErrorRef.current?.(message);
          onTokenChangeRef.current("");
          return;
        }

        try {
          widgetIdRef.current = window.turnstile.render(containerRef.current, {
            sitekey: siteKey,
            action,
            theme: "dark",
            callback: (token) => {
              if (!isActive) {
                return;
              }
              onTokenChangeRef.current(token);
              setState("ready");
              setErrorMessage(null);
            },
            "expired-callback": () => {
              onTokenChangeRef.current("");
              onExpireRef.current?.();
            },
            "error-callback": () => {
              const message = translate("auth.captcha.error");
              setState("error");
              setErrorMessage(message);
              onErrorRef.current?.(message);
              onTokenChangeRef.current("");
            },
          });
          setState("ready");
        } catch (error) {
          console.error("turnstile render", error);
          const message = translate("auth.captcha.error");
          setState("error");
          setErrorMessage(message);
          onErrorRef.current?.(message);
          onTokenChangeRef.current("");
        }
      })
      .catch((error) => {
        if (!isActive) {
          return;
        }
        console.error("turnstile load", error);
        const message = translate("auth.captcha.error");
        setState("error");
        setErrorMessage(message);
        onErrorRef.current?.(message);
        onTokenChangeRef.current("");
      });

    return () => {
      isActive = false;
      onTokenChangeRef.current("");
      if (widgetIdRef.current && window.turnstile) {
        try {
          window.turnstile.remove(widgetIdRef.current);
        } catch (error) {
          console.error("turnstile cleanup", error);
        }
      }
      widgetIdRef.current = null;
    };
  }, [siteKey, action, resetKey, language]);

  return (
    <div className={className}>
      <div ref={containerRef} className="min-h-[65px]" />
      {state === "loading" && (
        <p className="mt-2 text-xs text-slate-400">{t("auth.captcha.loading")}</p>
      )}
      {state === "error" && errorMessage && (
        <p role="alert" className="mt-2 text-xs text-rose-400">
          {errorMessage}
        </p>
      )}
    </div>
  );
}
