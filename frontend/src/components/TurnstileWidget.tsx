import { useEffect, useRef, useState } from "react";
import { loadTurnstile, reportTurnstileDebug } from "../utils/turnstile";
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
      reportTurnstileDebug("widget:missing-sitekey", { status: "error" });
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

    reportTurnstileDebug("widget:load:start", {
      status: "pending",
      meta: action ? { action } : undefined,
    });
    loadTurnstile()
      .then(() => {
        if (!isActive) {
          return;
        }
        reportTurnstileDebug("widget:load:resolved", { status: "success" });
        if (!containerRef.current || !window.turnstile) {
          reportTurnstileDebug("widget:render:missing-container", { status: "error" });
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
              reportTurnstileDebug("widget:callback:token", {
                status: "success",
                detail: { tokenLength: token.length },
              });
            },
            "expired-callback": () => {
              onTokenChangeRef.current("");
              onExpireRef.current?.();
              reportTurnstileDebug("widget:callback:expired", { status: "expired" });
            },
            "error-callback": () => {
              const message = translate("auth.captcha.error");
              setState("error");
              setErrorMessage(message);
              onErrorRef.current?.(message);
              onTokenChangeRef.current("");
              reportTurnstileDebug("widget:callback:error", { status: "error" });
            },
          });
          setState("ready");
          reportTurnstileDebug("widget:render:success", { status: "success" });
        } catch (error) {
          console.error("turnstile render", error);
          const message = translate("auth.captcha.error");
          setState("error");
          setErrorMessage(message);
          onErrorRef.current?.(message);
          onTokenChangeRef.current("");
          const errorMessage = error instanceof Error ? error.message : String(error);
          reportTurnstileDebug("widget:render:error", { status: "error", error: errorMessage });
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
        const errorMessage = error instanceof Error ? error.message : String(error);
        reportTurnstileDebug("widget:load:error", { status: "error", error: errorMessage });
      });

    return () => {
      isActive = false;
      onTokenChangeRef.current("");
      reportTurnstileDebug("widget:cleanup", { status: "cleanup" });
      if (widgetIdRef.current && window.turnstile) {
        try {
          window.turnstile.remove(widgetIdRef.current);
        } catch (error) {
          console.error("turnstile cleanup", error);
          const errorMessage = error instanceof Error ? error.message : String(error);
          reportTurnstileDebug("widget:cleanup:error", { status: "error", error: errorMessage });
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
