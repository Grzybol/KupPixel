import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth, type AuthUser } from "../useAuth";

type AccountPixel = {
  id: number;
  status: string;
  color: string;
  url: string;
  updated_at?: string;
};

const GRID_WIDTH = 1000;

function parseAccountPixels(data: unknown): AccountPixel[] {
  if (!Array.isArray(data)) {
    return [];
  }

  const pixels: AccountPixel[] = [];
  for (const item of data) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const record = item as Record<string, unknown>;
    if (typeof record.id !== "number") {
      continue;
    }
    pixels.push({
      id: record.id,
      status: typeof record.status === "string" ? record.status : "free",
      color: typeof record.color === "string" ? record.color : "",
      url: typeof record.url === "string" ? record.url : "",
      updated_at: typeof record.updated_at === "string" ? record.updated_at : undefined,
    });
  }
  return pixels;
}

function parseAccountUser(data: unknown): AuthUser | null {
  if (!data || typeof data !== "object") {
    return null;
  }
  const record = data as Record<string, unknown>;
  if (typeof record.email === "string") {
    return record as AuthUser;
  }
  return null;
}

function formatPosition(id: number) {
  const x = id % GRID_WIDTH;
  const y = Math.floor(id / GRID_WIDTH);
  return { x, y };
}

function formatDate(value?: string) {
  if (!value) {
    return "";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat("pl-PL", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(parsed);
}

export default function AccountPage() {
  const navigate = useNavigate();
  const { user, isLoading: isAuthLoading, openLoginModal, refresh } = useAuth();
  const [accountUser, setAccountUser] = useState<AuthUser | null>(null);
  const [pixels, setPixels] = useState<AccountPixel[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editValue, setEditValue] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);

  useEffect(() => {
    if (user) {
      setAccountUser(user);
    }
  }, [user]);

  useEffect(() => {
    if (isAuthLoading) {
      return;
    }
    if (!user) {
      void openLoginModal({ message: "Zaloguj się, aby zobaczyć dane swojego konta." });
      navigate("/");
    }
  }, [isAuthLoading, navigate, openLoginModal, user]);

  const loadAccount = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    setActionError(null);
    setActionMessage(null);
    try {
      const response = await fetch("/api/account", {
        credentials: "include",
      });
      if (response.status === 401) {
        void openLoginModal({ message: "Zaloguj się ponownie, aby zobaczyć swoje piksele." });
        navigate("/");
        return;
      }
      if (!response.ok) {
        const message = await response.text().catch(() => "");
        throw new Error(message || `Nie udało się pobrać danych konta (${response.status}).`);
      }
      const payload = (await response.json().catch(() => null)) as Record<string, unknown> | null;
      if (payload && "user" in payload) {
        const parsedUser = parseAccountUser((payload as Record<string, unknown>).user);
        if (parsedUser) {
          setAccountUser(parsedUser);
          if (!user) {
            await refresh().catch(() => undefined);
          }
        }
      }
      const parsedPixels = payload ? parseAccountPixels((payload as Record<string, unknown>).pixels) : [];
      setPixels(parsedPixels);
    } catch (accountError) {
      console.error(accountError);
      const message =
        accountError instanceof Error ? accountError.message : "Wystąpił błąd podczas pobierania danych konta.";
      setError(message);
    } finally {
      setIsLoading(false);
    }
  }, [navigate, openLoginModal, refresh, user]);

  useEffect(() => {
    if (!user) {
      return;
    }
    void loadAccount();
  }, [loadAccount, user]);

  const currentUserEmail = useMemo(() => accountUser?.email ?? user?.email ?? "", [accountUser, user]);

  const handleStartEdit = useCallback(
    (pixel: AccountPixel) => {
      setEditingId(pixel.id);
      setEditValue(pixel.url);
      setActionError(null);
      setActionMessage(null);
    },
    []
  );

  const handleCancelEdit = useCallback(() => {
    setEditingId(null);
    setEditValue("");
    setIsSaving(false);
  }, []);

  const handleSaveEdit = useCallback(async () => {
    if (editingId === null) {
      return;
    }
    const pixel = pixels.find((item) => item.id === editingId);
    if (!pixel) {
      return;
    }
    const trimmed = editValue.trim();
    if (!trimmed) {
      setActionError("Podaj poprawny adres URL.");
      return;
    }
    let normalizedUrl = trimmed;
    try {
      const parsed = new URL(trimmed);
      normalizedUrl = parsed.toString();
    } catch (urlError) {
      console.error(urlError);
      setActionError("Adres URL musi zawierać poprawny schemat (np. https://).");
      return;
    }
    if (!pixel.color) {
      setActionError("Ten piksel nie ma przypisanego koloru i nie może zostać zaktualizowany.");
      return;
    }
    setIsSaving(true);
    setActionError(null);
    setActionMessage(null);
    try {
      const response = await fetch("/api/pixels", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          id: pixel.id,
          status: "taken",
          color: pixel.color,
          url: normalizedUrl,
        }),
      });
      if (response.status === 401) {
        void openLoginModal({ message: "Zaloguj się, aby zaktualizować adres reklamy." });
        navigate("/");
        return;
      }
      if (!response.ok) {
        const message = await response.text().catch(() => "");
        throw new Error(message || `Nie udało się zaktualizować piksela (${response.status}).`);
      }
      const payload = (await response.json().catch(() => null)) as Record<string, unknown> | null;
      const updatedPixel: AccountPixel = {
        id: pixel.id,
        status: typeof payload?.status === "string" ? (payload.status as string) : pixel.status,
        color: typeof payload?.color === "string" ? (payload.color as string) : pixel.color,
        url: typeof payload?.url === "string" ? (payload.url as string) : normalizedUrl,
        updated_at: typeof payload?.updated_at === "string" ? (payload.updated_at as string) : payload?.updated_at ? String(payload.updated_at) : pixel.updated_at,
      };
      setPixels((prev) => prev.map((item) => (item.id === pixel.id ? updatedPixel : item)));
      setEditingId(null);
      setEditValue("");
      setActionMessage("Adres reklamy został zaktualizowany.");
    } catch (saveError) {
      console.error(saveError);
      const message = saveError instanceof Error ? saveError.message : "Nie udało się zapisać zmian.";
      setActionError(message);
    } finally {
      setIsSaving(false);
    }
  }, [editValue, editingId, navigate, openLoginModal, pixels]);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-200">
      <div className="mx-auto flex max-w-5xl flex-col gap-8 px-4 py-12">
        <header className="space-y-2 text-center">
          <h1 className="text-3xl font-semibold text-blue-400">Twoje konto</h1>
          <p className="text-sm text-slate-400">Zarządzaj zakupionymi pikselami i aktualizuj adresy reklam.</p>
          {currentUserEmail && (
            <p className="text-sm text-slate-300">
              Zalogowano jako <span className="font-semibold text-white">{currentUserEmail}</span>
            </p>
          )}
        </header>

        <div className="rounded-3xl border border-slate-800 bg-slate-900/70 p-6 shadow-xl">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <h2 className="text-xl font-semibold text-slate-100">Wykupione piksele</h2>
            <Link to="/" className="text-sm font-semibold text-blue-400 hover:text-blue-300">
              ← Wróć na tablicę
            </Link>
          </div>

          {isLoading && <p className="mt-6 text-sm text-slate-400">Ładuję dane konta...</p>}
          {error && (
            <p className="mt-6 rounded-xl border border-rose-500/40 bg-rose-500/10 p-4 text-sm text-rose-300" role="alert">
              {error}
            </p>
          )}

          {!isLoading && !error && (
            <div className="mt-6 space-y-4">
              {actionMessage && (
                <p className="rounded-xl border border-emerald-500/40 bg-emerald-500/10 p-4 text-sm text-emerald-200" role="status">
                  {actionMessage}
                </p>
              )}
              {actionError && (
                <p className="rounded-xl border border-rose-500/40 bg-rose-500/10 p-4 text-sm text-rose-300" role="alert">
                  {actionError}
                </p>
              )}

              {pixels.length === 0 ? (
                <p className="text-sm text-slate-400">Nie masz jeszcze żadnych wykupionych pikseli.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-slate-800 text-left text-sm">
                    <thead className="bg-slate-900/60 text-xs uppercase tracking-wide text-slate-400">
                      <tr>
                        <th className="px-4 py-3 font-semibold">ID</th>
                        <th className="px-4 py-3 font-semibold">Pozycja</th>
                        <th className="px-4 py-3 font-semibold">Kolor</th>
                        <th className="px-4 py-3 font-semibold">Adres URL</th>
                        <th className="px-4 py-3 font-semibold">Ostatnia aktualizacja</th>
                        <th className="px-4 py-3 font-semibold text-right">Akcje</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800 text-slate-200">
                      {pixels.map((pixel) => {
                        const position = formatPosition(pixel.id);
                        const isEditing = editingId === pixel.id;
                        return (
                          <tr key={pixel.id} className="bg-slate-900/40">
                            <td className="whitespace-nowrap px-4 py-3 font-mono text-sm">#{pixel.id}</td>
                            <td className="whitespace-nowrap px-4 py-3 text-xs text-slate-400">
                              x: {position.x}, y: {position.y}
                            </td>
                            <td className="px-4 py-3">
                              <div className="flex items-center gap-3">
                                <span
                                  aria-hidden
                                  className="inline-block h-5 w-5 rounded border border-slate-700"
                                  style={{ backgroundColor: pixel.color }}
                                />
                                <span className="font-mono text-xs text-slate-300">{pixel.color.toUpperCase()}</span>
                              </div>
                            </td>
                            <td className="px-4 py-3">
                              {isEditing ? (
                                <input
                                  type="url"
                                  value={editValue}
                                  onChange={(event) => setEditValue(event.target.value)}
                                  className="w-full rounded-lg border border-slate-700 bg-slate-950/70 px-3 py-2 text-xs text-slate-200 focus:border-blue-500 focus:outline-none"
                                  placeholder="https://twoja-domena.pl"
                                />
                              ) : (
                                <a
                                  href={pixel.url || "#"}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  className="break-all text-xs text-blue-400 hover:text-blue-300"
                                >
                                  {pixel.url || "Brak"}
                                </a>
                              )}
                            </td>
                            <td className="whitespace-nowrap px-4 py-3 text-xs text-slate-400">
                              {formatDate(pixel.updated_at)}
                            </td>
                            <td className="px-4 py-3 text-right">
                              {isEditing ? (
                                <div className="flex items-center justify-end gap-3">
                                  <button
                                    type="button"
                                    onClick={() => void handleSaveEdit()}
                                    disabled={isSaving}
                                    className="rounded-full bg-emerald-500/80 px-4 py-2 text-xs font-semibold text-emerald-950 transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-60"
                                  >
                                    Zapisz
                                  </button>
                                  <button
                                    type="button"
                                    onClick={handleCancelEdit}
                                    disabled={isSaving}
                                    className="rounded-full bg-slate-800/70 px-4 py-2 text-xs font-semibold text-slate-200 transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-60"
                                  >
                                    Anuluj
                                  </button>
                                </div>
                              ) : (
                                <button
                                  type="button"
                                  onClick={() => handleStartEdit(pixel)}
                                  className="rounded-full bg-slate-800/70 px-4 py-2 text-xs font-semibold text-slate-200 transition hover:bg-slate-700"
                                >
                                  Zmień URL
                                </button>
                              )}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
