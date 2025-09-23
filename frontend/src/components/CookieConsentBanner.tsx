import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useI18n } from "../lang/I18nProvider";

const STORAGE_KEY = "kup_pixel_cookie_consent_v1";

export default function CookieConsentBanner() {
  const { t } = useI18n();
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      if (!stored) {
        setVisible(true);
      }
    } catch (error) {
      console.error("cookie consent storage", error);
      setVisible(true);
    }
  }, []);

  const handleAccept = useCallback(() => {
    if (typeof window !== "undefined") {
      try {
        window.localStorage.setItem(STORAGE_KEY, "accepted");
      } catch (error) {
        console.error("cookie consent accept", error);
      }
    }
    setVisible(false);
  }, []);

  if (!visible) {
    return null;
  }

  return (
    <div className="fixed inset-x-0 bottom-0 z-40 px-4 pb-6">
      <div className="mx-auto max-w-4xl rounded-3xl bg-slate-950/95 p-4 shadow-2xl ring-1 ring-white/10 sm:p-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-slate-200">{t("cookies.banner.message")}</p>
          <div className="flex items-center gap-3 sm:shrink-0">
            <Link
              to="/terms"
              className="rounded-full border border-slate-700 px-4 py-2 text-xs font-semibold text-slate-200 transition hover:bg-slate-800/60"
            >
              {t("cookies.banner.moreInfo")}
            </Link>
            <button
              type="button"
              onClick={handleAccept}
              className="rounded-full bg-blue-500 px-5 py-2 text-xs font-semibold text-white shadow-lg transition hover:bg-blue-400"
            >
              {t("cookies.banner.accept")}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
