import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useAuth } from "../useAuth";
import ResendVerificationForm from "./ResendVerificationForm";
import { useI18n } from "../lang/I18nProvider";

type VerifyStatus = "idle" | "loading" | "success" | "error";

export default function VerifyAccountPage() {
  const { openLoginModal } = useAuth();
  const [searchParams] = useSearchParams();
  const token = useMemo(() => searchParams.get("token"), [searchParams]);
  const { t } = useI18n();
  const [status, setStatus] = useState<VerifyStatus>(token ? "loading" : "error");
  const [message, setMessage] = useState<string>("");
  const [error, setError] = useState<string>(() => (token ? "" : t("auth.errors.verifyMissingToken")));

  useEffect(() => {
    let ignore = false;
    if (!token) {
      setStatus("error");
      setError(t("auth.errors.verifyMissingToken"));
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
              : null) || t("auth.errors.verify");
          if (!ignore) {
            setError(errMsg);
            setStatus("error");
          }
          return;
        }
        const successMsg =
          payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).message === "string"
            ? ((payload as Record<string, unknown>).message as string)
            : t("auth.messages.verifySuccess");
        if (!ignore) {
          setMessage(successMsg);
          setStatus("success");
        }
      } catch (err) {
        console.error("verify account", err);
        if (!ignore) {
          setError(t("auth.errors.verifyUnexpected"));
          setStatus("error");
        }
      }
    };

    verify().catch((err) => console.error(err));
    return () => {
      ignore = true;
    };
  }, [t, token]);

  const handleOpenLogin = useCallback(() => {
    void openLoginModal({ message: t("auth.errors.loginToStart") });
  }, [openLoginModal, t]);

  return (
    <div className="flex min-h-full flex-col items-center justify-center px-4 py-16 text-center text-slate-200">
      <div className="w-full max-w-xl rounded-3xl bg-slate-900/80 p-10 shadow-2xl ring-1 ring-white/10">
            <h1 className="text-3xl font-semibold text-blue-400">{t("verify.title")}</h1>
            {status === "loading" && (
              <div className="mt-6 space-y-3 text-sm text-slate-300">
                <p>{t("auth.messages.verifyLoading")}</p>
                <div className="mx-auto h-2 w-40 overflow-hidden rounded-full bg-slate-800">
                  <div className="h-full w-1/2 animate-pulse rounded-full bg-blue-500" />
                </div>
              </div>
            )}

            {status === "success" && (
              <div className="mt-8 space-y-6">
                <div className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 p-6 text-left text-emerald-100">
                  <p className="text-lg font-semibold text-emerald-300">{t("auth.messages.verifyReady")}</p>
                  <p className="mt-2 text-sm text-emerald-100">{message}</p>
                </div>
                <div className="flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
                  <button
                    type="button"
                    onClick={handleOpenLogin}
                    className="inline-flex items-center justify-center rounded-full bg-blue-500 px-6 py-2 text-sm font-semibold text-white shadow-lg transition hover:bg-blue-400"
                  >
                    {t("common.actions.openLogin")}
                  </button>
                  <Link
                    to="/"
                    className="inline-flex items-center justify-center rounded-full bg-slate-800/70 px-6 py-2 text-sm font-semibold text-slate-200 transition hover:bg-slate-700"
                  >
                    {t("common.actions.goHome")}
                  </Link>
                </div>
              </div>
            )}

            {status === "error" && (
              <div className="mt-8 space-y-6 text-left">
                <div className="rounded-2xl border border-rose-500/40 bg-rose-500/10 p-6 text-rose-100">
                  <p className="text-lg font-semibold text-rose-200">{t("auth.messages.verifyErrorTitle")}</p>
                  <p className="mt-2 text-sm text-rose-100">{error}</p>
                </div>
                <div className="rounded-2xl border border-blue-500/30 bg-blue-500/10 p-6 text-sm text-blue-100">
                  <p className="text-base font-semibold text-blue-200">{t("auth.messages.verifyResendTitle")}</p>
                  <p className="mt-2 text-blue-100/80">{t("auth.messages.verifyResendDescription")}</p>
                  <div className="mt-4">
                    <ResendVerificationForm />
                  </div>
                </div>
                <div className="flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
                  <Link
                    to="/"
                    className="inline-flex items-center justify-center rounded-full bg-slate-800/70 px-6 py-2 text-sm font-semibold text-slate-200 transition hover:bg-slate-700"
                  >
                    {t("common.actions.goHome")}
                  </Link>
                </div>
              </div>
            )}
      </div>
    </div>
  );
}
