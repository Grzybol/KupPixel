import { FormEvent, useCallback, useEffect, useId, useState } from "react";

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
              : null) || "Nie udało się wysłać wiadomości. Spróbuj ponownie.";
          throw new Error(message);
        }
        const message =
          payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).message === "string"
            ? ((payload as Record<string, unknown>).message as string)
            : "Wysłaliśmy nowy link weryfikacyjny.";
        setSuccess(message);
      } catch (err) {
        console.error("resend verification", err);
        const message = err instanceof Error ? err.message : "Nie udało się wysłać wiadomości.";
        setError(message);
      } finally {
        setIsSubmitting(false);
      }
    },
    [email]
  );

  return (
    <form onSubmit={handleSubmit} className={className ?? "space-y-3"}>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-slate-200" htmlFor={inputId}>
          Adres e-mail
        </label>
        <input
          id={inputId}
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          className="w-full rounded-lg border border-slate-700 bg-slate-900/70 px-4 py-3 text-slate-100 shadow-inner focus:border-blue-400 focus:outline-none"
          placeholder="adres@email.pl"
          required
          disabled={isSubmitting}
          autoComplete="email"
        />
        <p className="text-xs text-slate-400">Wpisz swój adres, aby otrzymać nowy link weryfikacyjny.</p>
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
          {isSubmitting ? "Wysyłanie..." : "Wyślij ponownie"}
        </button>
      </div>
    </form>
  );
}
