import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "../useAuth";
import { isActivationCodeValid, normalizeActivationCode } from "../utils/activationCode";
import { useI18n } from "../lang/I18nProvider";
import TurnstileWidget from "./TurnstileWidget";
import { TURNSTILE_SITE_KEY } from "../config";

type ActivationCodeModalProps = {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: (addedPoints?: number) => void;
};

type RedeemResponse = {
  added_points?: number;
  error?: string;
  user?: {
    points?: number;
  };
  [key: string]: unknown;
};

export default function ActivationCodeModal({ isOpen, onClose, onSuccess }: ActivationCodeModalProps) {
  const { user, pixelCostPoints } = useAuth();
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [captchaToken, setCaptchaToken] = useState("");
  const [captchaResetKey, setCaptchaResetKey] = useState(0);
  const { t } = useI18n();

  const resetState = useCallback(() => {
    setCode("");
    setError(null);
    setSuccessMessage(null);
    setIsSubmitting(false);
    setCaptchaToken("");
    setCaptchaResetKey((key) => key + 1);
  }, []);

  useEffect(() => {
    if (!isOpen) {
      resetState();
    }
  }, [isOpen, resetState]);

  const handleClose = useCallback(() => {
    if (isSubmitting) return;
    resetState();
    onClose();
  }, [isSubmitting, onClose, resetState]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (isSubmitting) return;
      setError(null);
      setSuccessMessage(null);

      const normalized = normalizeActivationCode(code);
      if (!isActivationCodeValid(normalized)) {
        setError(t("auth.errors.activationFormat"));
        return;
      }
      if (!captchaToken) {
        setError(t("auth.captcha.required"));
        return;
      }

      setIsSubmitting(true);
      try {
        const response = await fetch("/api/activation-codes/redeem", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
          body: JSON.stringify({ code: normalized, turnstile_token: captchaToken }),
        });

        if (!response.ok) {
          if (response.status === 401) {
            throw new Error(t("auth.errors.sessionExpiredLong"));
          }
          const payload = (await response.json().catch(() => null)) as RedeemResponse | null;
          const message =
            payload && typeof payload.error === "string"
              ? (payload.error as string)
              : t("auth.errors.activateCode");
          throw new Error(message);
        }

        const payload = (await response.json().catch(() => null)) as RedeemResponse | null;
        const addedPoints = payload && typeof payload.added_points === "number" ? payload.added_points : undefined;
        const totalPoints =
          payload && payload.user && typeof payload.user.points === "number" ? payload.user.points : undefined;

        setCode("");
        if (addedPoints && addedPoints > 0) {
          const suffix = typeof totalPoints === "number" ? t("auth.messages.activationSuffix", { total: totalPoints }) : "";
          setSuccessMessage(t("auth.messages.activationSuccessWithPoints", { added: addedPoints, suffix }));
        } else {
          setSuccessMessage(t("auth.messages.activationSuccess"));
        }
        setError(null);
        if (onSuccess) {
          onSuccess(addedPoints);
        }
      } catch (err) {
        console.error("redeem activation code", err);
        const message = err instanceof Error ? err.message : t("auth.errors.activateCode");
        setError(message);
      } finally {
        setIsSubmitting(false);
        setCaptchaToken("");
        setCaptchaResetKey((key) => key + 1);
      }
    },
    [captchaToken, code, isSubmitting, onSuccess, t]
  );

  const infoText = useMemo(() => {
    if (typeof pixelCostPoints === "number" && Number.isFinite(pixelCostPoints)) {
      return t("auth.messages.activationCost", { points: pixelCostPoints });
    }
    return t("auth.messages.activationInfo");
  }, [pixelCostPoints, t]);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/70 backdrop-blur">
      <div className="w-full max-w-lg rounded-3xl bg-slate-950/95 p-8 shadow-2xl ring-1 ring-white/10" role="dialog" aria-modal="true">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-2xl font-semibold text-slate-100">{t("activationModal.title")}</h2>
            <p className="mt-1 text-sm text-slate-400">{infoText}</p>
            {typeof user?.points === "number" && (
              <p className="mt-1 text-xs text-slate-400">{t("activationModal.currentBalance", { points: user.points })}</p>
            )}
          </div>
          <button
            type="button"
            onClick={handleClose}
            className="rounded-full bg-slate-800/80 px-2 py-1 text-lg leading-none text-slate-300 transition hover:text-slate-100"
            aria-label={t("common.actions.close")}
          >
            ×
          </button>
        </div>

        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <label className="block text-sm font-medium text-slate-200" htmlFor="activation-code">
            {t("common.labels.activationCode")}
            <input
              id="activation-code"
              type="text"
              value={code}
              onChange={(event) => {
                setCode(event.target.value.toUpperCase());
                setError(null);
              }}
              className="mt-2 w-full rounded-xl border border-slate-800 bg-slate-950/70 px-4 py-3 font-mono text-sm uppercase tracking-[0.3em] text-slate-100 shadow-inner focus:border-emerald-400 focus:outline-none"
              placeholder={t("common.placeholders.activationCode")}
              maxLength={19}
              disabled={isSubmitting}
              autoFocus
            />
          </label>

          {error && (
            <p role="alert" className="text-sm text-rose-400">
              {error}
            </p>
          )}
          {successMessage && (
            <p role="status" className="rounded-xl border border-emerald-500/40 bg-emerald-500/10 p-4 text-sm text-emerald-200">
              {successMessage}
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

          <div className="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={handleClose}
              className="rounded-full px-4 py-2 text-sm font-semibold text-slate-300 transition hover:text-slate-100"
              disabled={isSubmitting}
            >
              {t("common.actions.close")}
            </button>
            <button
              type="submit"
              className="inline-flex items-center justify-center rounded-full bg-emerald-500 px-5 py-2 text-sm font-semibold text-emerald-950 shadow-lg transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={isSubmitting}
            >
              {isSubmitting ? t("activationModal.submitting") : t("activationModal.submit")}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
