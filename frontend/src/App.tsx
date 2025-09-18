import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, Route, Routes, useNavigate, useParams } from "react-router-dom";
import PixelCanvas, { Pixel } from "./components/PixelCanvas";

type PixelResponse = {
  width: number;
  height: number;
  pixels: Pixel[];
};

function usePixels() {
  const [data, setData] = useState<PixelResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let ignore = false;
    const fetchPixels = async () => {
      setLoading(true);
      try {
        const response = await fetch("/api/pixels");
        if (!response.ok) {
          throw new Error(`Błąd API: ${response.status}`);
        }
        const json = (await response.json()) as PixelResponse;
        if (!ignore) {
          setData(json);
          setError(null);
        }
      } catch (err) {
        console.error(err);
        if (!ignore) {
          setError("Nie udało się pobrać stanu pikseli.");
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
  }, []);

  return { data, loading, error } as const;
}

function LandingPage() {
  const navigate = useNavigate();
  const { data, loading, error } = usePixels();

  const handlePixelClick = useCallback(
    (pixel: Pixel) => {
      if (pixel.status === "taken" && pixel.url) {
        window.open(pixel.url, "_blank");
        return;
      }
      navigate(`/buy/${pixel.id}`);
    },
    [navigate]
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
  const id = pixelId ? Number(pixelId) : null;

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-6 px-6 text-center">
      <h2 className="text-3xl font-semibold text-blue-400">Kup ten piksel</h2>
      <p className="max-w-xl text-slate-300">
        Wybrałeś piksel o ID <span className="font-mono text-white">{id}</span>. To tylko placeholder –
        w docelowej wersji podepniesz tu płatności, grafikę oraz dane kontaktowe.
      </p>
      <Link
        to="/"
        className="rounded-full bg-blue-500 px-6 py-2 font-semibold text-white transition hover:bg-blue-400"
      >
        Wróć na tablicę
      </Link>
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<LandingPage />} />
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
  );
}
