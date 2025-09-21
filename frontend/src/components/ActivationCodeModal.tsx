import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "../useAuth";
import { isActivationCodeValid, normalizeActivationCode } from "../utils/activationCode";

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

  const resetState = useCallback(() => {
    setCode("");
    setError(null);
    setSuccessMessage(null);
    setIsSubmitting(false);
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
        setError("Kod musi mieć format XXXX-XXXX-XXXX-XXXX i składać się z cyfr lub liter.");
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
          body: JSON.stringify({ code: normalized }),
        });

        if (!response.ok) {
          if (response.status === 401) {
            throw new Error("Twoja sesja wygasła. Zaloguj się ponownie i spróbuj jeszcze raz.");
          }
          const payload = (await response.json().catch(() => null)) as RedeemResponse | null;
          const message =
            payload && typeof payload.error === "string"
              ? (payload.error as string)
              : "Nie udało się aktywować kodu. Upewnij się, że nie został już użyty.";
          throw new Error(message);
        }

        const payload = (await response.json().catch(() => null)) as RedeemResponse | null;
        const addedPoints = payload && typeof payload.added_points === "number" ? payload.added_points : undefined;
        const totalPoints =
          payload && payload.user && typeof payload.user.points === "number" ? payload.user.points : undefined;

        setCode("");
        if (addedPoints && addedPoints > 0) {
          setSuccessMessage(`Kod aktywowany! Dodano ${addedPoints} punktów${
            typeof totalPoints === "number" ? ` (razem ${totalPoints} pkt).` : "."
          }`);
        } else {
          setSuccessMessage("Kod został aktywowany.");
        }
        setError(null);
        if (onSuccess) {
          onSuccess(addedPoints);
        }
      } catch (err) {
        console.error("redeem activation code", err);
        const message = err instanceof Error ? err.message : "Nie udało się aktywować kodu.";
        setError(message);
      } finally {
        setIsSubmitting(false);
      }
    },
    [code, isSubmitting, onSuccess]
  );

  const infoText = useMemo(() => {
    if (typeof pixelCostPoints === "number" && Number.isFinite(pixelCostPoints)) {
      return `Koszt jednego piksela to ${pixelCostPoints} punktów.`;
    }
    return "Aktywuj kod, aby otrzymać punkty i kupować piksele.";
  }, [pixelCostPoints]);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/70 backdrop-blur">
      <div className="w-full max-w-lg rounded-3xl bg-slate-950/95 p-8 shadow-2xl ring-1 ring-white/10" role="dialog" aria-modal="true">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-2xl font-semibold text-slate-100">Aktywuj kod punktowy</h2>
            <p className="mt-1 text-sm text-slate-400">{infoText}</p>
            {typeof user?.points === "number" && (
              <p className="mt-1 text-xs text-slate-400">Aktualne saldo: {user.points} pkt</p>
            )}
          </div>
          <button
            type="button"
            onClick={handleClose}
            className="rounded-full bg-slate-800/80 px-2 py-1 text-lg leading-none text-slate-300 transition hover:text-slate-100"
            aria-label="Zamknij"
          >
            ×
          </button>
        </div>

        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <label className="block text-sm font-medium text-slate-200" htmlFor="activation-code">
            Kod aktywacyjny
            <input
              id="activation-code"
              type="text"
              value={code}
              onChange={(event) => {
                setCode(event.target.value.toUpperCase());
                setError(null);
              }}
              className="mt-2 w-full rounded-xl border border-slate-800 bg-slate-950/70 px-4 py-3 font-mono text-sm uppercase tracking-[0.3em] text-slate-100 shadow-inner focus:border-emerald-400 focus:outline-none"
              placeholder="XXXX-XXXX-XXXX-XXXX"
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

          <div className="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={handleClose}
              className="rounded-full px-4 py-2 text-sm font-semibold text-slate-300 transition hover:text-slate-100"
              disabled={isSubmitting}
            >
              Zamknij
            </button>
            <button
              type="submit"
              className="inline-flex items-center justify-center rounded-full bg-emerald-500 px-5 py-2 text-sm font-semibold text-emerald-950 shadow-lg transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={isSubmitting}
            >
              {isSubmitting ? "Aktywuję..." : "Aktywuj"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
