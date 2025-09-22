import { type ChangeEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Link, Route, Routes, useLocation, useNavigate, useParams } from "react-router-dom";
import PixelCanvas, { Pixel } from "./components/PixelCanvas";
import LoginModal from "./components/LoginModal";
import RegisterModal from "./components/RegisterModal";
import VerifyAccountPage from "./components/VerifyAccountPage";
import AccountPage from "./components/AccountPage";
import ActivationCodeModal from "./components/ActivationCodeModal";
import { useAuth } from "./useAuth";

type PixelResponse = {
  width: number;
  height: number;
  pixels: Pixel[];
};

function usePixels() {
  const { user, openLoginModal } = useAuth();
  const [data, setData] = useState<PixelResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let ignore = false;
    const fetchPixels = async () => {
      setLoading(true);
      try {
        const response = await fetch("/api/pixels", {
          credentials: "include",
        });
        if (!response.ok) {
          if (response.status === 401) {
            if (!ignore) {
              setData(null);
              setError("Aby zobaczyć tablicę pikseli, zaloguj się.");
            }
            void openLoginModal({
              message: "Twoja sesja wygasła. Zaloguj się, aby ponownie zobaczyć tablicę.",
            });
            return;
          }
          const message = await response.text().catch(() => "");
          throw new Error(message || `Błąd API: ${response.status}`);
        }
        const json = (await response.json()) as PixelResponse;
        if (!ignore) {
          setData(json);
          setError(null);
        }
      } catch (err) {
        console.error(err);
        if (!ignore) {
          const message = err instanceof Error ? err.message : "Nie udało się pobrać stanu pikseli.";
          setError(message);
        }
      } finally {
        if (!ignore) {
          setLoading(false);
        }
      }
    };

    fetchPixels();
    return () => {
      ignore = true;
    };
  }, [openLoginModal, user]);

  return { data, loading, error } as const;
}

type LandingPageProps = {
  onOpenRegister: () => void;
  onOpenActivationCode: () => void;
};

function LandingPage({ onOpenRegister, onOpenActivationCode }: LandingPageProps) {
  const navigate = useNavigate();
  const { data, loading, error } = usePixels();
  const { user, ensureAuthenticated, openLoginModal, logout, pixelCostPoints } = useAuth();
  const [selectedPixels, setSelectedPixels] = useState<Pixel[]>([]);

  const handlePixelClick = useCallback(
    async (pixel: Pixel) => {
      if (pixel.status === "taken" && pixel.url) {
        window.open(pixel.url, "_blank");
        return;
      }
      const authenticated = await ensureAuthenticated({
        message: "Zaloguj się, aby kupić wybrany piksel.",
      });
      if (!authenticated) {
        return;
      }
      navigate(`/buy/${pixel.id}`);
    },
    [ensureAuthenticated, navigate]
  );

  const handleSelectionComplete = useCallback(
    async (freePixels: Pixel[]) => {
      setSelectedPixels(freePixels);
      if (freePixels.length <= 1) {
        if (freePixels.length === 1) {
          await handlePixelClick(freePixels[0]);
        }
        return;
      }

      const authenticated = await ensureAuthenticated({
        message: "Zaloguj się, aby kupić zaznaczone piksele.",
      });
      if (!authenticated) {
        return;
      }

      const ids = freePixels.map((pixel) => pixel.id);
      navigate(
        {
          pathname: "/buy",
          search: `?ids=${ids.join(",")}`,
        },
        { state: { pixelIds: ids } }
      );
    },
    [ensureAuthenticated, handlePixelClick, navigate]
  );

  const heroStats = useMemo(() => {
    if (!data) return { taken: 0, free: 0 };
    let taken = 0;
    for (const pixel of data.pixels) {
      if (pixel.status === "taken") taken++;
    }
    return { taken, free: data.pixels.length - taken };
  }, [data]);

  return (
    <div className="min-h-screen">
      <header className="py-10 text-center">
        <h1 className="text-4xl font-bold text-blue-400">Kup Piksel</h1>
        <p className="mt-2 text-slate-300">
          Wybierz swój piksel na cyfrowej tablicy 1000×1000 i zostaw po sobie ślad.
        </p>
        <div className="mt-4 flex items-center justify-center gap-6 text-sm text-slate-400">
          <span className="font-semibold text-slate-200">Zajęte: {heroStats.taken}</span>
          <span className="font-semibold text-slate-200">Wolne: {heroStats.free}</span>
        </div>
        {user && (
          <div className="mt-3 text-sm text-slate-300">
            <span className="mr-4 inline-flex items-center rounded-full bg-slate-800/70 px-4 py-1 text-slate-200">
              Saldo: <span className="ml-1 font-semibold">{typeof user.points === "number" ? user.points : 0} pkt</span>
            </span>
            {typeof pixelCostPoints === "number" && pixelCostPoints > 0 && (
              <span className="inline-flex items-center rounded-full bg-slate-800/70 px-4 py-1 text-slate-200">
                Koszt piksela: <span className="ml-1 font-semibold">{pixelCostPoints} pkt</span>
              </span>
            )}
          </div>
        )}
        <div className="mt-6 flex items-center justify-center gap-4 text-sm text-slate-300">
          {user ? (
            <>
              <span className="rounded-full bg-slate-800/80 px-4 py-2 text-slate-200">
                Zalogowano jako <span className="font-semibold">{user.email}</span>
              </span>
              <button
                type="button"
                onClick={onOpenActivationCode}
                className="rounded-full bg-emerald-500/90 px-4 py-2 font-semibold text-emerald-950 shadow-lg transition hover:bg-emerald-400"
              >
                Aktywuj kod
              </button>
              <Link
                to="/account"
                className="rounded-full bg-slate-800/70 px-4 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
              >
                Twoje konto
              </Link>
              <button
                type="button"
                onClick={() => void logout()}
                className="rounded-full bg-slate-800/70 px-4 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
              >
                Wyloguj
              </button>
            </>
          ) : (
            <div className="flex flex-col items-stretch gap-3 sm:flex-row">
              <button
                type="button"
                onClick={onOpenRegister}
                className="rounded-full bg-blue-500 px-6 py-2 font-semibold text-white shadow-lg transition hover:bg-blue-400"
              >
                Załóż konto
              </button>
              <button
                type="button"
                onClick={() => {
                  void openLoginModal({ message: "Zaloguj się, aby rozpocząć." });
                }}
                className="rounded-full bg-slate-800/70 px-6 py-2 font-semibold text-slate-200 transition hover:bg-slate-700"
              >
                Zaloguj się
              </button>
            </div>
          )}
        </div>
      </header>
      <main className="mx-auto flex max-w-5xl flex-col items-center gap-6 px-4 pb-16">
        {loading && <div className="text-slate-300">Ładuję siatkę pikseli...</div>}
        {error && <div className="text-rose-400">{error}</div>}
        {data && (
          <PixelCanvas
            width={data.width}
            height={data.height}
            pixels={data.pixels}
            onPixelClick={handlePixelClick}
            onSelectionComplete={handleSelectionComplete}
          />
        )}
        {selectedPixels.length > 1 && (
          <div className="w-full max-w-2xl rounded-xl border border-blue-500/20 bg-blue-500/5 px-4 py-3 text-center text-sm text-blue-100">
            Zaznaczono {selectedPixels.length} wolnych pikseli. Dokończ zakup na stronie formularza, aby zarezerwować wszystkie.
          </div>
        )}
        <p className="max-w-2xl text-center text-sm text-slate-400">
          Kliknij pojedynczy piksel, aby przejść do strony zakupu lub obejrzeć reklamę. Możesz też przeciągnąć myszą, aby
          zaznaczyć blok wolnych pikseli i kupić kilka naraz.
        </p>
      </main>
    </div>
  );
}

type PixelPurchaseResult = {
  id: number;
  status?: string;
  error?: string;
};

function BuyPixelPage() {
  const { pixelId } = useParams<{ pixelId: string }>();
  const location = useLocation();
  const { ensureAuthenticated, openLoginModal, user, pixelCostPoints, refresh } = useAuth();
  const singleId = useMemo(() => {
    if (!pixelId) return null;
    const parsed = Number(pixelId);
    return Number.isFinite(parsed) ? parsed : null;
  }, [pixelId]);
  const selectedIds = useMemo(() => {
    const ids = new Set<number>();
    if (singleId !== null) {
      ids.add(singleId);
    }

    const state = (location.state as { pixelIds?: unknown } | null) ?? null;
    if (state && Array.isArray(state.pixelIds)) {
      for (const value of state.pixelIds) {
        const parsed = Number(value);
        if (Number.isFinite(parsed)) {
          ids.add(parsed);
        }
      }
    }

    const params = new URLSearchParams(location.search);
    const idsParam = params.get("ids");
    if (idsParam) {
      for (const raw of idsParam.split(",")) {
        const trimmed = raw.trim();
        if (!trimmed) continue;
        const parsed = Number(trimmed);
        if (Number.isFinite(parsed)) {
          ids.add(parsed);
        }
      }
    }

    return Array.from(ids).sort((a, b) => a - b);
  }, [location.search, location.state, singleId]);
  const selectedCount = selectedIds.length;
  const totalCost = useMemo(() => {
    if (typeof pixelCostPoints !== "number" || pixelCostPoints <= 0) {
      return null;
    }
    return pixelCostPoints * selectedCount;
  }, [pixelCostPoints, selectedCount]);
  const [selectedColor, setSelectedColor] = useState("#ff4d4f");
  const [isProcessing, setIsProcessing] = useState(false);
  const [purchaseStatus, setPurchaseStatus] = useState<null | "success" | "partial">(null);
  const [purchaseError, setPurchaseError] = useState<string | null>(null);
  const [url, setUrl] = useState("https://example.com");
  const [pixelResults, setPixelResults] = useState<PixelPurchaseResult[]>([]);

  useEffect(() => {
    setSelectedColor("#ff4d4f");
    setIsProcessing(false);
    setPurchaseStatus(null);
    setPurchaseError(null);
    setUrl("https://example.com");
    setPixelResults([]);
  }, [selectedIds.join(",")]);

  const handleColorChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setSelectedColor(event.target.value);
  }, []);

  const handleUrlChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setUrl(event.target.value);
  }, []);

  const handleSimulatePurchase = useCallback(async () => {
    if (selectedIds.length === 0) {
      setPurchaseError("Wybierz co najmniej jeden piksel, aby kontynuować.");
      return;
    }

    setIsProcessing(true);
    setPurchaseStatus(null);
    setPurchaseError(null);
    setPixelResults([]);

    try {
      const authenticated = await ensureAuthenticated({
        message: "Zaloguj się, aby kupić zaznaczone piksele.",
      });
      if (!authenticated) {
        throw new Error("Aby kontynuować, zaloguj się.");
      }
      const response = await fetch("/api/pixels", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          pixels: selectedIds.map((pixelId) => ({ id: pixelId, status: "taken", color: selectedColor, url })),
        }),
      });

      if (!response.ok) {
        if (response.status === 401) {
          void openLoginModal({ message: "Zaloguj się ponownie, aby sfinalizować zakup." });
          throw new Error("Twoja sesja wygasła. Zaloguj się ponownie.");
        }
        const payload = await response.json().catch(() => null);
        if (payload && Array.isArray((payload as { results?: unknown }).results)) {
          const mapped = (payload as { results: unknown }).results as unknown[];
          setPixelResults(
            mapped
              .map((item) => {
                if (!item || typeof item !== "object") return null;
                const base = item as { id?: unknown; error?: unknown; pixel?: { status?: unknown } };
                const parsedId = Number(base.id);
                if (!Number.isFinite(parsedId)) return null;
                const status = base.pixel && typeof base.pixel === "object" ? (base.pixel as { status?: unknown }).status : undefined;
                return {
                  id: parsedId,
                  status: typeof status === "string" ? status : undefined,
                  error: typeof base.error === "string" ? base.error : undefined,
                } satisfies PixelPurchaseResult;
              })
              .filter((item): item is PixelPurchaseResult => Boolean(item))
          );
        }
        const apiMessage =
          payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
            ? ((payload as Record<string, unknown>).error as string)
            : null;
        if (response.status === 400 || response.status === 403) {
          throw new Error(
            apiMessage || "Brak wystarczającej liczby punktów, aby kupić piksel. Aktywuj kod i spróbuj ponownie."
          );
        }
        throw new Error(apiMessage || `Błąd API: ${response.status}`);
      }

      const payload = (await response.json().catch(() => null)) as
        | null
        | {
            results?: unknown;
            user?: unknown;
            error?: unknown;
          };

      const results: PixelPurchaseResult[] = Array.isArray(payload?.results)
        ? (payload?.results as unknown[])
            .map((item) => {
              if (!item || typeof item !== "object") return null;
              const base = item as { id?: unknown; error?: unknown; pixel?: { status?: unknown } };
              const parsedId = Number(base.id);
              if (!Number.isFinite(parsedId)) return null;
              const status = base.pixel && typeof base.pixel === "object" ? (base.pixel as { status?: unknown }).status : undefined;
              return {
                id: parsedId,
                status: typeof status === "string" ? status : undefined,
                error: typeof base.error === "string" ? base.error : undefined,
              } satisfies PixelPurchaseResult;
            })
            .filter((item): item is PixelPurchaseResult => Boolean(item))
        : [];

      setPixelResults(results);

      const successfulCount = results.filter((result) => !result.error).length;
      if (successfulCount === results.length && successfulCount > 0) {
        setPurchaseStatus("success");
        await refresh().catch(() => undefined);
      } else if (successfulCount > 0) {
        setPurchaseStatus("partial");
        await refresh().catch(() => undefined);
        setPurchaseError("Nie wszystkie piksele udało się kupić. Sprawdź szczegóły poniżej.");
      } else {
        const errorMessage =
          (payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
            ? ((payload as Record<string, unknown>).error as string)
            : null) ||
          "Nie udało się zarezerwować żadnego z zaznaczonych pikseli. Spróbuj ponownie.";
        setPurchaseError(errorMessage);
      }
    } catch (error) {
      console.error(error);
      const message = error instanceof Error ? error.message : "Nie udało się zarezerwować piksela. Spróbuj ponownie.";
      setPurchaseError(message);
    } finally {
      setIsProcessing(false);
    }
  }, [ensureAuthenticated, openLoginModal, refresh, selectedColor, selectedIds, url]);

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-6 py-12 text-center">
      <div className="w-full max-w-2xl rounded-3xl bg-slate-900/60 p-10 shadow-xl">
        <h2 className="text-3xl font-semibold text-blue-400">
          {selectedCount > 1 ? "Kup wybrane piksele" : "Kup ten piksel"}
        </h2>
        <p className="mt-3 text-slate-300">
          {selectedCount > 1 ? (
            <>
              Wybrałeś {selectedCount} wolnych pikseli o ID: {" "}
              <span className="font-mono text-white">{selectedIds.join(", ")}</span>. Tutaj możesz dobrać kolor i przejść przez
              fikcyjny proces płatności, aby zobaczyć, jak będzie działał prawdziwy checkout.
            </>
          ) : (
            <>
              Wybrałeś piksel o ID {" "}
              <span className="font-mono text-white">{selectedIds[0] ?? "nieznany"}</span>. Tutaj możesz dobrać kolor i przejść
              przez fikcyjny proces płatności, aby zobaczyć, jak będzie działał prawdziwy checkout.
            </>
          )}
        </p>

        <div className="mt-6 grid gap-3 text-left text-sm text-slate-300 sm:grid-cols-2">
          <div className="rounded-xl border border-slate-800/70 bg-slate-900/70 p-4">
            <p className="font-semibold text-slate-100">Koszt zakupu</p>
            <p className="mt-1 text-slate-300">
              {typeof pixelCostPoints === "number" && pixelCostPoints > 0 ? (
                selectedCount > 1 && totalCost !== null ? (
                  <>
                    {selectedCount} × {pixelCostPoints} pkt = {totalCost} punktów
                  </>
                ) : (
                  `${pixelCostPoints} punktów`
                )
              ) : (
                "Sprawdź konfigurację"
              )}
            </p>
          </div>
          <div className="rounded-xl border border-slate-800/70 bg-slate-900/70 p-4">
            <p className="font-semibold text-slate-100">Twoje saldo</p>
            <p className="mt-1 text-slate-300">{typeof user?.points === "number" ? `${user.points} punktów` : "Zaloguj się"}</p>
          </div>
        </div>

        <div className="mt-8 rounded-2xl bg-slate-800/70 p-6 text-left">
          <h3 className="text-lg font-semibold text-slate-100">Dopasuj swój piksel</h3>
          <p className="mt-1 text-sm text-slate-400">
            Wybierz kolor, który najlepiej reprezentuje Twoją markę – podgląd i wartość HEX aktualizują się
            automatycznie.
          </p>

          <div className="mt-6 flex flex-col gap-6 sm:flex-row sm:items-center">
            <label className="flex flex-col gap-3 text-sm font-medium text-slate-200" htmlFor="pixel-color">
              Kolor piksela
              <input
                id="pixel-color"
                type="color"
                value={selectedColor}
                onChange={handleColorChange}
                disabled={isProcessing}
                className="h-14 w-20 cursor-pointer rounded-lg border border-slate-700 bg-slate-900/80 p-1 shadow-inner"
                aria-describedby="color-value"
              />
            </label>

            <div className="flex items-center gap-4">
              <div
                aria-hidden
                className="h-12 w-12 rounded-lg border border-slate-700 shadow-inner"
                style={{ backgroundColor: selectedColor }}
              />
              <span id="color-value" className="font-mono text-lg text-slate-200">
                {selectedColor.toUpperCase()}
              </span>
            </div>
          </div>

          <label className="mt-6 flex flex-col gap-2 text-sm font-medium text-slate-200" htmlFor="pixel-url">
            Adres URL reklamy
            <input
              id="pixel-url"
              type="url"
              value={url}
              onChange={handleUrlChange}
              disabled={isProcessing}
              placeholder="https://twoja-domena.pl"
              className="w-full rounded-lg border border-slate-700 bg-slate-900/80 px-4 py-3 text-sm text-slate-100 shadow-inner placeholder:text-slate-500"
            />
            <span className="text-xs font-normal text-slate-400">
              Po kliknięciu w piksel użytkownik zostanie przekierowany pod ten adres.
            </span>
          </label>

          <button
            type="button"
            onClick={handleSimulatePurchase}
            disabled={isProcessing || selectedCount === 0}
            className="mt-8 inline-flex items-center justify-center rounded-full bg-blue-500 px-6 py-3 font-semibold text-white shadow-lg transition hover:bg-blue-400 disabled:cursor-not-allowed disabled:opacity-70"
          >
            {isProcessing ? (
              <span className="flex items-center gap-3">
                <span className="h-2 w-2 animate-ping rounded-full bg-white" />
                Przetwarzanie płatności...
              </span>
            ) : (
              (selectedCount > 1 ? `Zasymuluj zakup ${selectedCount} pikseli` : "Zasymuluj zakup")
            )}
          </button>

          <p aria-live="polite" className="mt-4 text-sm text-slate-400">
            {isProcessing
              ? "Trwa wirtualne potwierdzanie płatności. To potrwa tylko chwilkę..."
              : "Symulacja nie pobiera prawdziwych środków – to jedynie podgląd przyszłego doświadczenia."}
          </p>

          {purchaseError && (
            <p role="alert" className="mt-4 text-sm text-rose-400">
              {purchaseError}
            </p>
          )}
          {pixelResults.length > 0 && (
            <div className="mt-6 rounded-xl border border-slate-700 bg-slate-900/60 p-4">
              <h4 className="text-sm font-semibold text-slate-200">Rezultaty zakupu</h4>
              <ul className="mt-3 space-y-2 text-sm text-slate-300">
                {pixelResults.map((result) => (
                  <li key={result.id} className="flex items-start justify-between gap-4">
                    <span className="font-mono text-slate-200">#{result.id}</span>
                    {result.error ? (
                      <span className="text-rose-400">{result.error}</span>
                    ) : (
                      <span className="text-emerald-300">{result.status === "taken" ? "Kupiono" : result.status}</span>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>

        {purchaseStatus === "success" && (
          <div
            role="status"
            className="mt-8 rounded-2xl border border-emerald-500/40 bg-emerald-500/10 p-6 text-left text-emerald-100"
          >
            <h3 className="text-xl font-semibold text-emerald-300">Udało się!</h3>
            <p className="mt-2 text-sm">
              {selectedCount > 1 ? (
                <>
                  Wszystkie {selectedCount} piksele zostały zarezerwowane z kolorem {" "}
                  <span className="font-mono">{selectedColor.toUpperCase()}</span>.
                </>
              ) : (
                <>
                  Piksel <span className="font-mono">#{selectedIds[0]}</span> został zarezerwowany z kolorem {" "}
                  <span className="font-mono">{selectedColor.toUpperCase()}</span>.
                </>
              )}
            </p>
            <div className="mt-4 flex items-center gap-3">
              <span className="text-sm text-emerald-200">Podgląd:</span>
              <div
                aria-hidden
                className="h-8 w-8 rounded border border-emerald-300 shadow-inner"
                style={{ backgroundColor: selectedColor }}
              />
            </div>
            <p className="mt-4 text-xs text-emerald-200/80">
              To wciąż makieta płatności – w produkcji dodasz prawdziwy checkout oraz potwierdzenia dla klienta.
            </p>
          </div>
        )}
        {purchaseStatus === "partial" && (
          <div
            role="status"
            className="mt-8 rounded-2xl border border-amber-500/40 bg-amber-500/10 p-6 text-left text-amber-100"
          >
            <h3 className="text-xl font-semibold text-amber-200">Częściowy sukces</h3>
            <p className="mt-2 text-sm">
              Część pikseli została zakupiona, ale kilka wymaga ponownej próby. Sprawdź szczegóły powyżej i spróbuj ponownie
              po rozwiązaniu problemów.
            </p>
          </div>
        )}

        <div className="mt-10">
          <Link
            to="/"
            className="inline-flex items-center justify-center rounded-full bg-blue-500 px-6 py-2 font-semibold text-white transition hover:bg-blue-400"
          >
            Wróć na tablicę
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function App() {
  const { openLoginModal, refresh } = useAuth();
  const [isRegisterOpen, setIsRegisterOpen] = useState(false);
  const [isActivationModalOpen, setIsActivationModalOpen] = useState(false);

  const handleOpenRegister = useCallback(() => {
    setIsRegisterOpen(true);
  }, []);

  const handleCloseRegister = useCallback(() => {
    setIsRegisterOpen(false);
  }, []);

  const handleOpenLoginFromRegister = useCallback(() => {
    void openLoginModal({ message: "Zaloguj się, aby rozpocząć." });
  }, [openLoginModal]);

  const handleOpenActivationCode = useCallback(() => {
    setIsActivationModalOpen(true);
  }, []);

  const handleCloseActivationCode = useCallback(() => {
    setIsActivationModalOpen(false);
  }, []);

  const handleActivationSuccess = useCallback(
    async () => {
      await refresh().catch(() => undefined);
      setIsActivationModalOpen(false);
    },
    [refresh]
  );

  return (
    <>
      <Routes>
        <Route
          path="/"
          element={<LandingPage onOpenRegister={handleOpenRegister} onOpenActivationCode={handleOpenActivationCode} />}
        />
        <Route path="/account" element={<AccountPage onOpenActivationCode={handleOpenActivationCode} />} />
        <Route path="/verify" element={<VerifyAccountPage />} />
        <Route path="/buy" element={<BuyPixelPage />} />
        <Route path="/buy/:pixelId" element={<BuyPixelPage />} />
        <Route
          path="*"
          element={
            <div className="flex min-h-screen flex-col items-center justify-center gap-3 text-center text-slate-300">
              <h2 className="text-2xl font-semibold text-white">Ups! Nie znaleziono strony.</h2>
              <Link to="/" className="text-blue-400 underline">
                Wróć na tablicę pikseli
              </Link>
            </div>
          }
        />
      </Routes>
      <LoginModal />
      <RegisterModal
        isOpen={isRegisterOpen}
        onClose={handleCloseRegister}
        onOpenLogin={() => {
          handleOpenLoginFromRegister();
        }}
      />
      <ActivationCodeModal
        isOpen={isActivationModalOpen}
        onClose={handleCloseActivationCode}
        onSuccess={handleActivationSuccess}
      />
    </>
  );
}
