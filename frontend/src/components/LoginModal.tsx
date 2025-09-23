import { FormEvent, useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../useAuth";
import ResendVerificationForm from "./ResendVerificationForm";
import { useI18n } from "../lang/I18nProvider";
import TurnstileWidget from "./TurnstileWidget";
import { TURNSTILE_SITE_KEY } from "../config";

export default function LoginModal() {
  const { isLoginModalOpen, closeLoginModal, login, loginPrompt } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [captchaToken, setCaptchaToken] = useState("");
  const [captchaResetKey, setCaptchaResetKey] = useState(0);
  const { t } = useI18n();

  const resetCaptcha = useCallback(() => {
    setCaptchaToken("");
    setCaptchaResetKey((key) => key + 1);
  }, []);

  const resetState = useCallback(() => {
    setEmail("");
    setPassword("");
    setError(null);
    setIsSubmitting(false);
    resetCaptcha();
  }, [resetCaptcha]);

  const handleClose = useCallback(() => {
    if (isSubmitting) return;
    resetState();
    closeLoginModal();
  }, [closeLoginModal, isSubmitting, resetState]);

  const handleForgotPassword = useCallback(() => {
    if (isSubmitting) return;
    resetState();
    closeLoginModal();
    navigate("/forgot-password");
  }, [closeLoginModal, isSubmitting, navigate, resetState]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setError(null);
      if (!captchaToken) {
        setError(t("auth.captcha.required"));
        return;
      }
      setIsSubmitting(true);
      let shouldReset = false;
      try {
        await login({ email, password, turnstileToken: captchaToken });
        shouldReset = true;
        resetState();
      } catch (err) {
        console.error("login error", err);
        const message = err instanceof Error ? err.message : t("auth.errors.login");
        setError(message);
      } finally {
        setIsSubmitting(false);
        if (!shouldReset) {
          resetCaptcha();
        }
      }
    },
    [captchaToken, email, login, password, resetCaptcha, resetState, t]
  );

  const showResend = useMemo(() => {
    if (!error) return false;
    const lower = error.toLowerCase();
    return lower.includes("potwierdzone") || lower.includes("weryfik");
  }, [error]);

  if (!isLoginModalOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 backdrop-blur">
      <div
        role="dialog"
        aria-modal="true"
        className="w-full max-w-md rounded-2xl bg-slate-900/90 p-8 shadow-2xl ring-1 ring-white/10"
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-2xl font-semibold text-slate-100">{t("loginModal.title")}</h2>
            <p className="mt-1 text-sm text-slate-400">{loginPrompt || t("auth.messages.loginModalDefault")}</p>
          </div>
          <button
            type="button"
            onClick={handleClose}
            className="rounded-full bg-slate-800/80 p-2 text-slate-400 transition hover:text-slate-200"
            aria-label={t("common.actions.close")}
          >
            ×
          </button>
        </div>

        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <label className="block text-sm font-medium text-slate-200">
            {t("common.labels.email")}
            <input
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900/70 px-4 py-3 text-slate-100 shadow-inner focus:border-blue-400 focus:outline-none"
              autoFocus
              autoComplete="email"
              required
              disabled={isSubmitting}
            />
          </label>

          <label className="block text-sm font-medium text-slate-200">
            {t("common.labels.password")}
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900/70 px-4 py-3 text-slate-100 shadow-inner focus:border-blue-400 focus:outline-none"
              autoComplete="current-password"
              required
              disabled={isSubmitting}
            />
          </label>

          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleForgotPassword}
              className="text-sm font-medium text-blue-400 transition hover:text-blue-300"
              disabled={isSubmitting}
            >
              {t("auth.passwordReset.link")}
            </button>
          </div>

        {error && (
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
            }}
            onError={(message) => setError(message)}
            resetKey={captchaResetKey}
            className="mt-1"
          />
        </div>
        {showResend && (
          <div className="rounded-xl border border-blue-500/30 bg-blue-500/10 p-4 text-sm text-blue-100">
            <p className="mb-3">{t("auth.messages.resendPrompt")}</p>
            <ResendVerificationForm initialEmail={email} className="space-y-3" />
          </div>
          )}

          <div className="flex items-center justify-end gap-3 pt-4">
            <button
              type="button"
              onClick={handleClose}
              className="rounded-full px-4 py-2 text-sm font-semibold text-slate-300 transition hover:text-slate-100"
              disabled={isSubmitting}
            >
              {t("common.actions.cancel")}
            </button>
            <button
              type="submit"
              className="inline-flex items-center justify-center rounded-full bg-blue-500 px-5 py-2 text-sm font-semibold text-white shadow-lg transition hover:bg-blue-400 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={isSubmitting}
            >
              {isSubmitting ? t("loginModal.submitting") : t("loginModal.submit")}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
