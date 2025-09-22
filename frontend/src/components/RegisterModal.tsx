import { FormEvent, useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../useAuth";
import ResendVerificationForm from "./ResendVerificationForm";
import { useI18n } from "../lang/I18nProvider";

type RegisterModalProps = {
  isOpen: boolean;
  onClose: () => void;
  onOpenLogin?: () => void;
};

export default function RegisterModal({ isOpen, onClose, onOpenLogin }: RegisterModalProps) {
  const { register } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [acceptedTerms, setAcceptedTerms] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const { t } = useI18n();
  const termsTemplate = t("registerModal.terms", { termsLink: "__LINK__" });
  const [termsPrefix, termsSuffix = ""] = termsTemplate.split("__LINK__");

  const resetState = useCallback(() => {
    setEmail("");
    setPassword("");
    setAcceptedTerms(false);
    setError(null);
    setIsSubmitting(false);
    setIsSuccess(false);
    setSuccessMessage(null);
  }, []);

  const handleClose = useCallback(() => {
    if (isSubmitting) return;
    resetState();
    onClose();
  }, [isSubmitting, onClose, resetState]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setError(null);
      if (!acceptedTerms) {
        setError(t("auth.messages.registerTermsError"));
        return;
      }
      setIsSubmitting(true);
      try {
        const result = await register({ email, password });
        setPassword("");
        setIsSuccess(true);
        setSuccessMessage(result.message);
      } catch (err) {
        console.error("register error", err);
        const message = err instanceof Error ? err.message : t("auth.errors.register");
        setError(message);
      } finally {
        setIsSubmitting(false);
      }
    },
    [acceptedTerms, email, password, register, t]
  );

  if (!isOpen) {
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
            <h2 className="text-2xl font-semibold text-slate-100">
              {isSuccess ? t("registerModal.successTitle") : t("registerModal.title")}
            </h2>
            <p className="mt-1 text-sm text-slate-400">
              {isSuccess
                ? successMessage ?? t("auth.messages.registerEmailInfo")
                : t("auth.messages.registerInfo")}
            </p>
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

        {isSuccess ? (
          <div className="mt-6 space-y-6">
            <div className="rounded-xl border border-blue-500/30 bg-blue-500/10 p-4 text-sm text-blue-100">
              <p>{t("auth.messages.registerHelp")}</p>
            </div>
            <ResendVerificationForm initialEmail={email} />
            <div className="flex flex-col gap-3 pt-2 sm:flex-row sm:items-center sm:justify-between">
              {onOpenLogin && (
                <button
                  type="button"
                  onClick={() => {
                    resetState();
                    onClose();
                    onOpenLogin();
                  }}
                  className="text-left text-sm font-semibold text-blue-300 transition hover:text-blue-200"
                >
                  {t("common.actions.openLogin")}
                </button>
              )}
              <button
                type="button"
                onClick={handleClose}
                className="self-end rounded-full px-4 py-2 text-sm font-semibold text-slate-300 transition hover:text-slate-100"
              >
                {t("common.actions.close")}
              </button>
            </div>
          </div>
        ) : (
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
                autoComplete="new-password"
                required
                disabled={isSubmitting}
              />
            </label>

            <label className="flex items-start gap-3 text-xs text-slate-300">
              <input
                type="checkbox"
                checked={acceptedTerms}
                onChange={(event) => {
                  setAcceptedTerms(event.target.checked);
                  if (event.target.checked) {
                    setError(null);
                  }
                }}
                disabled={isSubmitting}
                className="mt-1 h-4 w-4 rounded border-slate-600 bg-slate-900 text-blue-500 focus:ring-blue-400"
              />
              <span>
                {termsPrefix}
                <Link to="/terms" className="font-semibold text-blue-300 underline-offset-2 hover:underline">
                  {t("registerModal.termsLink")}
                </Link>
                {termsSuffix}
              </span>
            </label>

            {error && (
              <p role="alert" className="text-sm text-rose-400">
                {error}
              </p>
            )}

            <div className="flex flex-col gap-3 pt-4 sm:flex-row sm:items-center sm:justify-between">
              {onOpenLogin && (
                <button
                  type="button"
                  onClick={() => {
                    if (isSubmitting) return;
                    resetState();
                    onClose();
                    onOpenLogin();
                  }}
                  className="text-left text-sm font-semibold text-blue-300 transition hover:text-blue-200"
                >
                  {t("registerModal.loginCta")}
                </button>
              )}
              <div className="flex items-center justify-end gap-3 sm:justify-end">
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
                  disabled={isSubmitting || !acceptedTerms}
                >
                  {isSubmitting ? t("registerModal.submitting") : t("registerModal.submit")}
                </button>
              </div>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
