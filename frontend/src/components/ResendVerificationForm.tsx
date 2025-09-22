import { FormEvent, useCallback, useEffect, useId, useState } from "react";
import { useI18n } from "../lang/I18nProvider";

type ResendVerificationFormProps = {
  initialEmail?: string;
  className?: string;
};

export default function ResendVerificationForm({ initialEmail = "", className }: ResendVerificationFormProps) {
  const [email, setEmail] = useState(initialEmail);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const inputId = useId();
  const { t } = useI18n();

  useEffect(() => {
    setEmail(initialEmail);
  }, [initialEmail]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setError(null);
      setSuccess(null);
      setIsSubmitting(true);
      try {
        const response = await fetch("/api/resend-verification", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
          body: JSON.stringify({ email }),
        });
        const payload = await response.json().catch(() => null);
        if (!response.ok) {
        const message =
          (payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
            ? ((payload as Record<string, unknown>).error as string)
            : null) || t("auth.errors.resend");
          throw new Error(message);
        }
        const message =
          payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).message === "string"
            ? ((payload as Record<string, unknown>).message as string)
            : t("auth.messages.resendSuccess");
        setSuccess(message);
      } catch (err) {
        console.error("resend verification", err);
        const message = err instanceof Error ? err.message : t("auth.errors.resendGeneric");
        setError(message);
      } finally {
        setIsSubmitting(false);
      }
    },
    [email, t]
  );

  return (
    <form onSubmit={handleSubmit} className={className ?? "space-y-3"}>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-slate-200" htmlFor={inputId}>
          {t("common.labels.email")}
        </label>
        <input
          id={inputId}
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          className="w-full rounded-lg border border-slate-700 bg-slate-900/70 px-4 py-3 text-slate-100 shadow-inner focus:border-blue-400 focus:outline-none"
          placeholder={t("common.placeholders.email")}
          required
          disabled={isSubmitting}
          autoComplete="email"
        />
        <p className="text-xs text-slate-400">{t("auth.messages.verificationEmailInfo")}</p>
      </div>
      {error && (
        <p role="alert" className="text-sm text-rose-400">
          {error}
        </p>
      )}
      {success && (
        <p role="status" className="text-sm text-emerald-300">
          {success}
        </p>
      )}
      <div className="flex justify-end">
        <button
          type="submit"
          className="inline-flex items-center justify-center rounded-full bg-blue-500 px-5 py-2 text-sm font-semibold text-white shadow-lg transition hover:bg-blue-400 disabled:cursor-not-allowed disabled:opacity-70"
          disabled={isSubmitting}
        >
          {isSubmitting ? t("resend.submitting") : t("resend.submit")}
        </button>
      </div>
    </form>
  );
}
