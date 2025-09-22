import { FormEvent, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useAuth } from "../useAuth";
import { useI18n } from "../lang/I18nProvider";

export default function ResetPasswordPage() {
  const { confirmPasswordReset, openLoginModal } = useAuth();
  const { t } = useI18n();
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token") ?? "";
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [status, setStatus] = useState<"idle" | "success" | "error">(token ? "idle" : "error");
  const [message, setMessage] = useState("");
  const [error, setError] = useState(() => (token ? "" : t("auth.passwordReset.errors.missingToken")));
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!token) {
      setError(t("auth.passwordReset.errors.missingToken"));
      setStatus("error");
      return;
    }
    if (!password.trim()) {
      setError(t("auth.passwordReset.errors.passwordRequired"));
      setStatus("error");
      return;
    }
    if (!confirmPassword.trim()) {
      setError(t("auth.passwordReset.errors.passwordConfirmationRequired"));
      setStatus("error");
      return;
    }
    if (password.trim() !== confirmPassword.trim()) {
      setError(t("auth.passwordReset.errors.passwordMismatch"));
      setStatus("error");
      return;
    }

    setIsSubmitting(true);
    setStatus("idle");
    setError("");
    setMessage("");
    try {
      const normalizedPassword = password.trim();
      const normalizedConfirm = confirmPassword.trim();
      const result = await confirmPasswordReset(token, normalizedPassword, normalizedConfirm);
      setMessage(result);
      setStatus("success");
    } catch (err) {
      console.error("password reset confirm", err);
      const fallback = err instanceof Error ? err.message : t("auth.passwordReset.errors.confirm");
      setError(fallback);
      setStatus("error");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleOpenLogin = () => {
    void openLoginModal({ message: t("auth.errors.loginToStart") });
  };

  return (
    <div className="flex min-h-full flex-col items-center justify-center px-4 py-16 text-center text-slate-200">
      <div className="w-full max-w-xl rounded-3xl bg-slate-900/80 p-10 shadow-2xl ring-1 ring-white/10">
        <h1 className="text-3xl font-semibold text-blue-400">{t("auth.passwordReset.confirmTitle")}</h1>
        <p className="mt-2 text-sm text-slate-300">{t("auth.passwordReset.confirmDescription")}</p>

        <form onSubmit={handleSubmit} className="mt-8 space-y-4 text-left">
          <label className="block text-sm font-medium text-slate-200">
            {t("common.labels.password")}
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900/70 px-4 py-3 text-slate-100 shadow-inner focus:border-blue-400 focus:outline-none"
              required
              autoComplete="new-password"
              disabled={isSubmitting}
            />
          </label>

          <label className="block text-sm font-medium text-slate-200">
            {t("common.labels.confirmPassword")}
            <input
              type="password"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900/70 px-4 py-3 text-slate-100 shadow-inner focus:border-blue-400 focus:outline-none"
              required
              autoComplete="new-password"
              disabled={isSubmitting}
            />
          </label>

          {status === "error" && error && (
            <p role="alert" className="text-sm text-rose-400">
              {error}
            </p>
          )}

          {status === "success" && message && (
            <div className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 p-4 text-sm text-emerald-100">
              {message}
            </div>
          )}

          <div className="flex flex-wrap items-center justify-end gap-3 pt-4">
            <button
              type="button"
              onClick={handleOpenLogin}
              className="rounded-full px-4 py-2 text-sm font-semibold text-slate-300 transition hover:text-slate-100"
            >
              {t("common.actions.openLogin")}
            </button>
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
              {isSubmitting ? t("common.status.sending") : t("auth.passwordReset.confirmSubmit")}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
