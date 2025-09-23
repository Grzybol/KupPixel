import { ChangeEvent, useCallback, useMemo } from "react";
import { Link, useLocation } from "react-router-dom";
import { useAuth } from "../useAuth";
import { useI18n } from "../lang/I18nProvider";
import { LanguageCode } from "../lang";

type NavigationBarProps = {
  onOpenRegister?: () => void;
  onOpenActivationCode?: () => void;
};

export default function NavigationBar({ onOpenRegister, onOpenActivationCode }: NavigationBarProps) {
  const { user, openLoginModal, logout } = useAuth();
  const location = useLocation();
  const { t, availableLanguages, language, setLanguage } = useI18n();

  const displayName = useMemo(() => {
    if (!user) {
      return "";
    }
    const username = typeof user.username === "string" ? user.username : null;
    return username && username.trim().length > 0 ? username : user.email;
  }, [user]);

  const pointsBalance = useMemo(() => {
    if (typeof user?.points === "number" && Number.isFinite(user.points)) {
      return user.points;
    }
    return 0;
  }, [user]);

  const handleOpenLogin = useCallback(() => {
    void openLoginModal({ message: t("navigation.loginPrompt") });
  }, [openLoginModal, t]);

  const handleLogout = useCallback(() => {
    void logout();
  }, [logout]);

  const handleLanguageChange = useCallback(
    (event: ChangeEvent<HTMLSelectElement>) => {
      setLanguage(event.target.value as LanguageCode);
    },
    [setLanguage]
  );

  const isAccountRoute = location.pathname.startsWith("/account");

  return (
    <nav className="border-b border-slate-800/80 bg-slate-950/80 backdrop-blur supports-[backdrop-filter]:bg-slate-950/60">
      <div className="mx-auto flex max-w-5xl flex-col gap-3 px-4 py-4 text-sm text-slate-200 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-6">
          <Link to="/" className="text-base font-semibold text-blue-400 transition hover:text-blue-300">
            {t("navigation.home")}
          </Link>
          <label className="flex items-center gap-2 text-xs text-slate-400">
            <span>{t("common.languageLabel")}:</span>
            <select
              value={language}
              onChange={handleLanguageChange}
              className="rounded-full border border-slate-700 bg-slate-900/70 px-2 py-1 text-xs text-slate-200"
              aria-label={t("common.languageLabel")}
            >
              {availableLanguages.map((item) => (
                <option key={item.code} value={item.code} className="text-black">
                  {item.label}
                </option>
              ))}
            </select>
          </label>
        </div>
        {user ? (
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
            <div className="flex flex-col items-start gap-1 text-xs text-slate-300 sm:text-sm">
              <span className="font-semibold text-slate-100">{displayName}</span>
              <span className="inline-flex items-center rounded-full bg-slate-900/80 px-3 py-1 text-xs font-medium text-emerald-300">
                {t("navigation.balance", { points: pointsBalance })}
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {onOpenActivationCode && (
                <button
                  type="button"
                  onClick={onOpenActivationCode}
                  className="rounded-full bg-emerald-500/90 px-4 py-2 font-semibold text-emerald-950 transition hover:bg-emerald-400"
                >
                  {t("common.actions.activateCode")}
                </button>
              )}
              {!isAccountRoute && (
                <Link
                  to="/account"
                  className="rounded-full bg-slate-800/80 px-4 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
                >
                  {t("navigation.account")}
                </Link>
              )}
              <button
                type="button"
                onClick={handleLogout}
                className="rounded-full bg-slate-800/80 px-4 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
              >
                {t("common.actions.logout")}
              </button>
            </div>
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            {onOpenRegister && (
              <button
                type="button"
                onClick={onOpenRegister}
                className="rounded-full bg-blue-500 px-5 py-2 font-semibold text-white transition hover:bg-blue-400"
              >
                {t("common.actions.openRegister")}
              </button>
            )}
            <button
              type="button"
              onClick={handleOpenLogin}
              className="rounded-full bg-slate-800/80 px-5 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
            >
              {t("common.actions.loginCta")}
            </button>
          </div>
        )}
      </div>
    </nav>
  );
}
