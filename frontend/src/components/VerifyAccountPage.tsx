import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useAuth } from "../useAuth";
import ResendVerificationForm from "./ResendVerificationForm";

type VerifyStatus = "idle" | "loading" | "success" | "error";

export default function VerifyAccountPage() {
  const { openLoginModal } = useAuth();
  const [searchParams] = useSearchParams();
  const token = useMemo(() => searchParams.get("token"), [searchParams]);
  const [status, setStatus] = useState<VerifyStatus>(token ? "loading" : "error");
  const [message, setMessage] = useState<string>("");
  const [error, setError] = useState<string>(token ? "" : "Brakuje tokenu weryfikacyjnego w adresie URL.");

  useEffect(() => {
    let ignore = false;
    if (!token) {
      setStatus("error");
      setError("Brakuje tokenu weryfikacyjnego w adresie URL.");
      return () => {
        ignore = true;
      };
    }

    const verify = async () => {
      setStatus("loading");
      setError("");
      setMessage("");
      try {
        const response = await fetch(`/api/verify?token=${encodeURIComponent(token)}`, {
          credentials: "include",
        });
        const payload = await response.json().catch(() => null);
        if (!response.ok) {
          const errMsg =
            (payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
              ? ((payload as Record<string, unknown>).error as string)
              : null) || "Nie udało się potwierdzić adresu e-mail.";
          if (!ignore) {
            setError(errMsg);
            setStatus("error");
          }
          return;
        }
        const successMsg =
          payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).message === "string"
            ? ((payload as Record<string, unknown>).message as string)
            : "Adres e-mail został potwierdzony. Możesz się teraz zalogować.";
        if (!ignore) {
          setMessage(successMsg);
          setStatus("success");
        }
      } catch (err) {
        console.error("verify account", err);
        if (!ignore) {
          setError("Wystąpił nieoczekiwany błąd. Spróbuj ponownie później.");
          setStatus("error");
        }
      }
    };

    verify().catch((err) => console.error(err));
    return () => {
      ignore = true;
    };
  }, [token]);

  const handleOpenLogin = useCallback(() => {
    void openLoginModal({ message: "Zaloguj się, aby rozpocząć." });
  }, [openLoginModal]);

  return (
    <div className="flex min-h-full flex-col items-center justify-center px-4 py-16 text-center text-slate-200">
      <div className="w-full max-w-xl rounded-3xl bg-slate-900/80 p-10 shadow-2xl ring-1 ring-white/10">
            <h1 className="text-3xl font-semibold text-blue-400">Potwierdzenie adresu e-mail</h1>
            {status === "loading" && (
              <div className="mt-6 space-y-3 text-sm text-slate-300">
                <p>Trwa potwierdzanie tokenu weryfikacyjnego...</p>
                <div className="mx-auto h-2 w-40 overflow-hidden rounded-full bg-slate-800">
                  <div className="h-full w-1/2 animate-pulse rounded-full bg-blue-500" />
                </div>
              </div>
            )}

            {status === "success" && (
              <div className="mt-8 space-y-6">
                <div className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 p-6 text-left text-emerald-100">
                  <p className="text-lg font-semibold text-emerald-300">Gotowe!</p>
                  <p className="mt-2 text-sm text-emerald-100">{message}</p>
                </div>
                <div className="flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
                  <button
                    type="button"
                    onClick={handleOpenLogin}
                    className="inline-flex items-center justify-center rounded-full bg-blue-500 px-6 py-2 text-sm font-semibold text-white shadow-lg transition hover:bg-blue-400"
                  >
                    Przejdź do logowania
                  </button>
                  <Link
                    to="/"
                    className="inline-flex items-center justify-center rounded-full bg-slate-800/70 px-6 py-2 text-sm font-semibold text-slate-200 transition hover:bg-slate-700"
                  >
                    Wróć na stronę główną
                  </Link>
                </div>
              </div>
            )}

            {status === "error" && (
              <div className="mt-8 space-y-6 text-left">
                <div className="rounded-2xl border border-rose-500/40 bg-rose-500/10 p-6 text-rose-100">
                  <p className="text-lg font-semibold text-rose-200">Nie udało się potwierdzić konta</p>
                  <p className="mt-2 text-sm text-rose-100">{error}</p>
                </div>
                <div className="rounded-2xl border border-blue-500/30 bg-blue-500/10 p-6 text-sm text-blue-100">
                  <p className="text-base font-semibold text-blue-200">Wyślij link ponownie</p>
                  <p className="mt-2 text-blue-100/80">Wpisz swój adres e-mail, aby otrzymać nowy token weryfikacyjny.</p>
                  <div className="mt-4">
                    <ResendVerificationForm />
                  </div>
                </div>
                <div className="flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
                  <Link
                    to="/"
                    className="inline-flex items-center justify-center rounded-full bg-slate-800/70 px-6 py-2 text-sm font-semibold text-slate-200 transition hover:bg-slate-700"
                  >
                    Wróć na stronę główną
                  </Link>
                </div>
              </div>
            )}
      </div>
    </div>
  );
}
