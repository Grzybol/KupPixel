import { useCallback, useMemo } from "react";
import { Link, useLocation } from "react-router-dom";
import { useAuth } from "../useAuth";

type NavigationBarProps = {
  onOpenRegister?: () => void;
  onOpenActivationCode?: () => void;
};

export default function NavigationBar({ onOpenRegister, onOpenActivationCode }: NavigationBarProps) {
  const { user, openLoginModal, logout } = useAuth();
  const location = useLocation();

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
    void openLoginModal({ message: "Zaloguj się, aby rozpocząć." });
  }, [openLoginModal]);

  const handleLogout = useCallback(() => {
    void logout();
  }, [logout]);

  const isAccountRoute = location.pathname.startsWith("/account");

  return (
    <nav className="border-b border-slate-800/80 bg-slate-950/80 backdrop-blur supports-[backdrop-filter]:bg-slate-950/60">
      <div className="mx-auto flex max-w-5xl flex-col gap-3 px-4 py-4 text-sm text-slate-200 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-6">
          <Link to="/" className="text-base font-semibold text-blue-400 transition hover:text-blue-300">
            Home
          </Link>
        </div>
        {user ? (
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
            <div className="flex flex-col items-start gap-1 text-xs text-slate-300 sm:text-sm">
              <span className="font-semibold text-slate-100">{displayName}</span>
              <span className="inline-flex items-center rounded-full bg-slate-900/80 px-3 py-1 text-xs font-medium text-emerald-300">
                Saldo: {pointsBalance} pkt
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {onOpenActivationCode && (
                <button
                  type="button"
                  onClick={onOpenActivationCode}
                  className="rounded-full bg-emerald-500/90 px-4 py-2 font-semibold text-emerald-950 transition hover:bg-emerald-400"
                >
                  Aktywuj kod
                </button>
              )}
              {!isAccountRoute && (
                <Link
                  to="/account"
                  className="rounded-full bg-slate-800/80 px-4 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
                >
                  Twoje konto
                </Link>
              )}
              <button
                type="button"
                onClick={handleLogout}
                className="rounded-full bg-slate-800/80 px-4 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
              >
                Wyloguj
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
                Załóż konto
              </button>
            )}
            <button
              type="button"
              onClick={handleOpenLogin}
              className="rounded-full bg-slate-800/80 px-5 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
            >
              Zaloguj się
            </button>
          </div>
        )}
      </div>
    </nav>
  );
}
