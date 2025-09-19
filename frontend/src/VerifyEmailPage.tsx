import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useAuth } from "./useAuth";

type VerifyError = Error & {
  tokenExpired?: boolean;
};

type VerifyStatus = "idle" | "pending" | "success";

export default function VerifyEmailPage() {
  const { refresh } = useAuth();
  const [searchParams] = useSearchParams();
  const queryToken = useMemo(() => searchParams.get("token") ?? "", [searchParams]);
  const [token, setToken] = useState<string>(queryToken);
  const [status, setStatus] = useState<VerifyStatus>("idle");
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [tokenExpired, setTokenExpired] = useState(false);
  const [hasAutoSubmitted, setHasAutoSubmitted] = useState(false);

  useEffect(() => {
    setToken(queryToken);
  }, [queryToken]);

  const verifyToken = useCallback(
    async (value: string) => {
      const trimmed = value.trim();
      if (trimmed === "") {
        setError("Podaj token otrzymany w wiadomości e-mail.");
        setTokenExpired(false);
        return;
      }

      setStatus("pending");
      setError(null);
      setInfo(null);
      setTokenExpired(false);

      try {
        const response = await fetch("/api/verify-email", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
          body: JSON.stringify({ token: trimmed }),
        });
        if (!response.ok) {
          let message = "Nie udało się potwierdzić adresu e-mail.";
          let expired = false;
          try {
            const data = (await response.json()) as Record<string, unknown>;
            if (data && typeof data.error === "string" && data.error.trim() !== "") {
              message = data.error;
            }
            if (typeof data?.token_expired === "boolean") {
              expired = data.token_expired;
            }
          } catch (parseError) {
            const fallback = await response.text().catch(() => "");
            if (fallback.trim() !== "") {
              message = fallback;
            }
          }
          const verifyError = new Error(message) as VerifyError;
          if (expired) {
            verifyError.tokenExpired = true;
          }
          throw verifyError;
        }

        const data = (await response.json().catch(() => null)) as Record<string, unknown> | null;
        const message = data && typeof data.message === "string" && data.message.trim() !== ""
          ? data.message
          : "Adres e-mail został potwierdzony.";

        setStatus("success");
        setInfo(message);
        setError(null);
        setTokenExpired(false);
        await refresh().catch((refreshError) => {
          console.error("refresh after verification", refreshError);
        });
      } catch (err) {
        console.error("verify email", err);
        const message = err instanceof Error ? err.message : "Nie udało się potwierdzić adresu e-mail.";
        setError(message);
        setStatus("idle");
        if (err && typeof err === "object" && "tokenExpired" in err) {
          setTokenExpired(Boolean((err as VerifyError).tokenExpired));
        }
      }
    },
    [refresh]
  );

  useEffect(() => {
    if (!hasAutoSubmitted && queryToken.trim() !== "") {
      setHasAutoSubmitted(true);
      void verifyToken(queryToken);
    }
  }, [hasAutoSubmitted, queryToken, verifyToken]);

  const handleSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      void verifyToken(token);
    },
    [token, verifyToken]
  );

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-6 py-12 text-center">
      <div className="w-full max-w-lg rounded-3xl bg-slate-900/70 p-10 shadow-2xl ring-1 ring-white/10">
        <h1 className="text-3xl font-semibold text-blue-400">Potwierdź adres e-mail</h1>
        <p className="mt-2 text-sm text-slate-300">
          Wpisz token przesłany w wiadomości e-mail. Po pomyślnej weryfikacji konto zostanie aktywowane automatycznie.
        </p>

        {status !== "success" && (
          <form onSubmit={handleSubmit} className="mt-8 space-y-5">
            <label className="block text-left text-sm font-medium text-slate-200">
              Token weryfikacyjny
              <input
                type="text"
                value={token}
                onChange={(event) => setToken(event.target.value)}
                className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900/70 px-4 py-3 font-mono text-slate-100 shadow-inner focus:border-blue-400 focus:outline-none"
                placeholder="wklej lub wpisz token"
                required
                disabled={status === "pending"}
              />
            </label>

            <button
              type="submit"
              className="inline-flex w-full items-center justify-center rounded-full bg-blue-500 px-6 py-3 font-semibold text-white shadow-lg transition hover:bg-blue-400 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={status === "pending"}
            >
              {status === "pending" ? "Potwierdzanie..." : "Potwierdź adres"}
            </button>
          </form>
        )}

        {status === "success" && (
          <div className="mt-8 rounded-2xl border border-emerald-500/40 bg-emerald-500/10 p-6 text-left text-emerald-100">
            <h2 className="text-xl font-semibold text-emerald-300">Gotowe!</h2>
            <p className="mt-2 text-sm">{info ?? "Adres e-mail został potwierdzony."}</p>
            <p className="mt-4 text-xs text-emerald-200/80">
              Możesz już zamknąć tę stronę lub wrócić do tablicy pikseli, aby zalogować się na swoje konto.
            </p>
          </div>
        )}

        {error && (
          <p role="alert" className="mt-6 text-sm text-rose-400">
            {error}
          </p>
        )}

        {tokenExpired && (
          <p className="mt-3 text-sm text-amber-300">
            Token wygasł. Zarejestruj się ponownie, aby otrzymać nowy link aktywacyjny.
          </p>
        )}

        <div className="mt-8 space-y-2 text-sm text-slate-400">
          <p>
            Nie widzisz wiadomości? Sprawdź folder spam lub poczekaj kilka minut – wysyłka może chwilę potrwać.
          </p>
          <p>
            Wróć na
            {" "}
            <Link to="/" className="text-blue-300 underline">
              stronę główną Kup Piksel
            </Link>
            .
          </p>
        </div>
      </div>
    </div>
  );
}
