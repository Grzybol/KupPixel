import { type ChangeEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Link, Route, Routes, useNavigate, useParams } from "react-router-dom";
import PixelCanvas, { Pixel } from "./components/PixelCanvas";
import LoginModal from "./components/LoginModal";
import RegisterModal from "./components/RegisterModal";
import VerifyAccountPage from "./components/VerifyAccountPage";
import AccountPage from "./components/AccountPage";
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
};

function LandingPage({ onOpenRegister }: LandingPageProps) {
  const navigate = useNavigate();
  const { data, loading, error } = usePixels();
  const { user, ensureAuthenticated, openLoginModal, logout } = useAuth();

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
        <div className="mt-6 flex items-center justify-center gap-4 text-sm text-slate-300">
          {user ? (
            <>
              <span className="rounded-full bg-slate-800/80 px-4 py-2 text-slate-200">
                Zalogowano jako <span className="font-semibold">{user.email}</span>
              </span>
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
          />
        )}
        <p className="max-w-2xl text-center text-sm text-slate-400">
          Kliknij dowolny piksel, aby przejść do strony zakupu lub obejrzeć reklamę.
        </p>
      </main>
    </div>
  );
}

function BuyPixelPage() {
  const { pixelId } = useParams<{ pixelId: string }>();
  const { ensureAuthenticated, openLoginModal } = useAuth();
  const id = useMemo(() => {
    if (!pixelId) return null;
    const parsed = Number(pixelId);
    return Number.isFinite(parsed) ? parsed : null;
  }, [pixelId]);
  const [selectedColor, setSelectedColor] = useState("#ff4d4f");
  const [isProcessing, setIsProcessing] = useState(false);
  const [purchaseStatus, setPurchaseStatus] = useState<null | "success">(null);
  const [purchaseError, setPurchaseError] = useState<string | null>(null);
  const [url, setUrl] = useState("https://example.com");

  useEffect(() => {
    setSelectedColor("#ff4d4f");
    setIsProcessing(false);
    setPurchaseStatus(null);
    setPurchaseError(null);
    setUrl("https://example.com");
  }, [id]);

  const handleColorChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setSelectedColor(event.target.value);
  }, []);

  const handleUrlChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setUrl(event.target.value);
  }, []);

  const handleSimulatePurchase = useCallback(async () => {
    if (id === null) return;

    setIsProcessing(true);
    setPurchaseStatus(null);
    setPurchaseError(null);

    try {
      const authenticated = await ensureAuthenticated({
        message: "Zaloguj się, aby kupić ten piksel.",
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
        body: JSON.stringify({ id, status: "taken", color: selectedColor, url }),
      });

      if (!response.ok) {
        if (response.status === 401) {
          void openLoginModal({ message: "Zaloguj się ponownie, aby sfinalizować zakup." });
          throw new Error("Twoja sesja wygasła. Zaloguj się ponownie.");
        }
        const message = await response.text().catch(() => null);
        throw new Error(message || `Błąd API: ${response.status}`);
      }

      setPurchaseStatus("success");
    } catch (error) {
      console.error(error);
      const message = error instanceof Error ? error.message : "Nie udało się zarezerwować piksela. Spróbuj ponownie.";
      setPurchaseError(message);
    } finally {
      setIsProcessing(false);
    }
  }, [ensureAuthenticated, id, openLoginModal, selectedColor, url]);

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-6 py-12 text-center">
      <div className="w-full max-w-2xl rounded-3xl bg-slate-900/60 p-10 shadow-xl">
        <h2 className="text-3xl font-semibold text-blue-400">Kup ten piksel</h2>
        <p className="mt-3 text-slate-300">
          Wybrałeś piksel o ID {" "}
          <span className="font-mono text-white">{id ?? "nieznany"}</span>. Tutaj możesz dobrać kolor i
          przejść przez fikcyjny proces płatności, aby zobaczyć, jak będzie działał prawdziwy checkout.
        </p>

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
            disabled={isProcessing || id === null}
            className="mt-8 inline-flex items-center justify-center rounded-full bg-blue-500 px-6 py-3 font-semibold text-white shadow-lg transition hover:bg-blue-400 disabled:cursor-not-allowed disabled:opacity-70"
          >
            {isProcessing ? (
              <span className="flex items-center gap-3">
                <span className="h-2 w-2 animate-ping rounded-full bg-white" />
                Przetwarzanie płatności...
              </span>
            ) : (
              "Zasymuluj zakup"
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
        </div>

        {purchaseStatus === "success" && (
          <div
            role="status"
            className="mt-8 rounded-2xl border border-emerald-500/40 bg-emerald-500/10 p-6 text-left text-emerald-100"
          >
            <h3 className="text-xl font-semibold text-emerald-300">Udało się!</h3>
            <p className="mt-2 text-sm">
              Piksel <span className="font-mono">#{id}</span> został zarezerwowany z kolorem
              {" "}
              <span className="font-mono">{selectedColor.toUpperCase()}</span>.
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
  const { openLoginModal } = useAuth();
  const [isRegisterOpen, setIsRegisterOpen] = useState(false);

  const handleOpenRegister = useCallback(() => {
    setIsRegisterOpen(true);
  }, []);

  const handleCloseRegister = useCallback(() => {
    setIsRegisterOpen(false);
  }, []);

  const handleOpenLoginFromRegister = useCallback(() => {
    void openLoginModal({ message: "Zaloguj się, aby rozpocząć." });
  }, [openLoginModal]);

  return (
    <>
      <Routes>
        <Route path="/" element={<LandingPage onOpenRegister={handleOpenRegister} />} />
        <Route path="/account" element={<AccountPage />} />
        <Route path="/verify" element={<VerifyAccountPage />} />
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
    </>
  );
}
