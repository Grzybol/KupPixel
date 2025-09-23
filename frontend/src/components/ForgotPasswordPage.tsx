import { FormEvent, useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../useAuth";
import { useI18n } from "../lang/I18nProvider";
import TurnstileWidget from "./TurnstileWidget";
import { TURNSTILE_SITE_KEY } from "../config";

export default function ForgotPasswordPage() {
  const { requestPasswordReset } = useAuth();
  const { t } = useI18n();
  const [email, setEmail] = useState("");
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [captchaToken, setCaptchaToken] = useState("");
  const [captchaResetKey, setCaptchaResetKey] = useState(0);

  const resetCaptcha = useCallback(() => {
    setCaptchaToken("");
    setCaptchaResetKey((key) => key + 1);
  }, []);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setStatus("idle");
    setError("");
    setMessage("");
    if (!captchaToken) {
      setError(t("auth.captcha.required"));
      setStatus("error");
      return;
    }
    setIsSubmitting(true);
    try {
      const result = await requestPasswordReset({ email: email.trim(), turnstileToken: captchaToken });
      setMessage(result);
      setStatus("success");
    } catch (err) {
      console.error("password reset request", err);
      const fallback = err instanceof Error ? err.message : t("auth.passwordReset.errors.request");
      setError(fallback);
      setStatus("error");
    } finally {
      setIsSubmitting(false);
      resetCaptcha();
    }
  };

  return (
    <div className="flex min-h-full flex-col items-center justify-center px-4 py-16 text-center text-slate-200">
      <div className="w-full max-w-xl rounded-3xl bg-slate-900/80 p-10 shadow-2xl ring-1 ring-white/10">
        <h1 className="text-3xl font-semibold text-blue-400">{t("auth.passwordReset.title")}</h1>
        <p className="mt-2 text-sm text-slate-300">{t("auth.passwordReset.description")}</p>

        <form onSubmit={handleSubmit} className="mt-8 space-y-4 text-left">
          <label className="block text-sm font-medium text-slate-200">
            {t("common.labels.email")}
            <input
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900/70 px-4 py-3 text-slate-100 shadow-inner focus:border-blue-400 focus:outline-none"
              required
              autoComplete="email"
              disabled={isSubmitting}
            />
          </label>

          {status === "error" && error && (
            <p role="alert" className="text-sm text-rose-400">
              {error}
            </p>
          )}

          <div className="space-y-2">
            <p className="text-xs text-slate-400">{t("auth.captcha.label")}</p>
            <TurnstileWidget
              siteKey={TURNSTILE_SITE_KEY}
              onTokenChange={setCaptchaToken}
              onExpire={() => {
                setCaptchaToken("");
                setError(t("auth.captcha.required"));
                setStatus("error");
              }}
              onError={(message) => {
                setError(message);
                setStatus("error");
              }}
              resetKey={captchaResetKey}
              className="mt-1"
            />
          </div>

          {status === "success" && message && (
            <div className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 p-4 text-sm text-emerald-100">
              {message}
            </div>
          )}

          <div className="flex items-center justify-end gap-3 pt-4">
            <Link
              to="/"
              className="rounded-full px-4 py-2 text-sm font-semibold text-slate-300 transition hover:text-slate-100"
            >
              {t("common.actions.goHome")}
            </Link>
            <button
              type="submit"
              className="inline-flex items-center justify-center rounded-full bg-blue-500 px-5 py-2 text-sm font-semibold text-white shadow-lg transition hover:bg-blue-400 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={isSubmitting}
            >
              {isSubmitting ? t("common.status.sending") : t("auth.passwordReset.submit")}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
