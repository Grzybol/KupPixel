import { FormEvent, useCallback, useState } from "react";
import { useAuth } from "../useAuth";

type RegisterModalProps = {
  isOpen: boolean;
  onClose: () => void;
  onOpenLogin?: () => void;
};

export default function RegisterModal({ isOpen, onClose, onOpenLogin }: RegisterModalProps) {
  const { register } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const resetState = useCallback(() => {
    setEmail("");
    setPassword("");
    setError(null);
    setIsSubmitting(false);
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
      setIsSubmitting(true);
      try {
        await register({ email, password });
        resetState();
        onClose();
      } catch (err) {
        console.error("register error", err);
        const message = err instanceof Error ? err.message : "Nie udało się utworzyć konta.";
        setError(message);
      } finally {
        setIsSubmitting(false);
      }
    },
    [email, password, register, resetState, onClose]
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
            <h2 className="text-2xl font-semibold text-slate-100">Załóż konto</h2>
            <p className="mt-1 text-sm text-slate-400">
              Podaj adres e-mail i hasło, aby utworzyć konto i rozpocząć zabawę z tablicą pikseli.
            </p>
          </div>
          <button
            type="button"
            onClick={handleClose}
            className="rounded-full bg-slate-800/80 p-2 text-slate-400 transition hover:text-slate-200"
            aria-label="Zamknij"
          >
            ×
          </button>
        </div>

        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <label className="block text-sm font-medium text-slate-200">
            Adres e-mail
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
            Hasło
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
                Masz już konto? Zaloguj się
              </button>
            )}
            <div className="flex items-center justify-end gap-3 sm:justify-end">
              <button
                type="button"
                onClick={handleClose}
                className="rounded-full px-4 py-2 text-sm font-semibold text-slate-300 transition hover:text-slate-100"
                disabled={isSubmitting}
              >
                Anuluj
              </button>
              <button
                type="submit"
                className="inline-flex items-center justify-center rounded-full bg-blue-500 px-5 py-2 text-sm font-semibold text-white shadow-lg transition hover:bg-blue-400 disabled:cursor-not-allowed disabled:opacity-70"
                disabled={isSubmitting}
              >
                {isSubmitting ? "Tworzenie..." : "Zarejestruj się"}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
