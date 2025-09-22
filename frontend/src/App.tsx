import { type ChangeEvent, type ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import { Link, Route, Routes, useLocation, useNavigate, useParams } from "react-router-dom";
import PixelCanvas, { Pixel } from "./components/PixelCanvas";
import LoginModal from "./components/LoginModal";
import RegisterModal from "./components/RegisterModal";
import VerifyAccountPage from "./components/VerifyAccountPage";
import AccountPage from "./components/AccountPage";
import ActivationCodeModal from "./components/ActivationCodeModal";
import TermsFooter from "./components/TermsFooter";
import NavigationBar from "./components/NavigationBar";
import TermsPage from "./components/TermsPage";
import ForgotPasswordPage from "./components/ForgotPasswordPage";
import ResetPasswordPage from "./components/ResetPasswordPage";
import { useAuth } from "./useAuth";
import { useI18n } from "./lang/I18nProvider";

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
  const { t } = useI18n();

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
              setError(t("auth.errors.loginRequired"));
            }
            void openLoginModal({
              message: t("auth.errors.sessionExpired"),
            });
            return;
          }
          const message = await response.text().catch(() => "");
          throw new Error(message || t("auth.errors.api", { status: response.status }));
        }
        const json = (await response.json()) as PixelResponse;
        if (!ignore) {
          setData(json);
          setError(null);
        }
      } catch (err) {
        console.error(err);
        if (!ignore) {
          const message = err instanceof Error ? err.message : t("auth.errors.fetchPixels");
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
  }, [openLoginModal, t, user]);

  return { data, loading, error } as const;
}

function LandingPage() {
  const navigate = useNavigate();
  const { data, loading, error } = usePixels();
  const { user, ensureAuthenticated, pixelCostPoints } = useAuth();
  const [selectedPixels, setSelectedPixels] = useState<Pixel[]>([]);
  const { t, dictionary } = useI18n();

  const handlePixelClick = useCallback(
    async (pixel: Pixel) => {
      if (pixel.status === "taken" && pixel.url) {
        window.open(pixel.url, "_blank");
        return;
      }
      const authenticated = await ensureAuthenticated({
        message: t("auth.errors.loginToBuy"),
      });
      if (!authenticated) {
        return;
      }
      navigate(`/buy/${pixel.id}`);
    },
    [ensureAuthenticated, navigate, t]
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
        message: t("auth.errors.loginToBuyMany"),
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
    [ensureAuthenticated, handlePixelClick, navigate, t]
  );

  const heroStats = useMemo(() => {
    if (!data) return { taken: 0, free: 0 };
    let taken = 0;
    for (const pixel of data.pixels) {
      if (pixel.status === "taken") taken++;
    }
    return { taken, free: data.pixels.length - taken };
  }, [data]);

  const pointsShort = t("common.units.pointsShort");
  const formatPoints = useCallback((value: number) => t("common.units.points", { count: value }), [t]);

  const instructionItems = useMemo<ReactNode[]>(() => {
    const landingSection = (dictionary.landing as Record<string, unknown>) ?? {};
    const instructions = landingSection.instructions as Record<string, unknown> | undefined;
    const getInstruction = (key: string) => {
      const fallbackKey = `landing.instructions.${key}`;
      if (instructions && typeof instructions[key] === "string") {
        return instructions[key] as string;
      }
      return t(fallbackKey);
    };
    const panText = getInstruction("pan");
    const panParts = panText.split(/(\{\{ctrl\}\}|\{\{shift\}\})/g).filter((part) => part !== "");
    const panNodes = panParts.map((part, index) => {
      if (part === "{{ctrl}}") {
        return (
          <kbd key={`ctrl-${index}`} className="rounded bg-slate-800 px-1.5 py-0.5 text-xs font-semibold text-slate-100">
            Ctrl
          </kbd>
        );
      }
      if (part === "{{shift}}") {
        return (
          <kbd key={`shift-${index}`} className="ml-1 rounded bg-slate-800 px-1.5 py-0.5 text-xs font-semibold text-slate-100">
            Shift
          </kbd>
        );
      }
      return (
        <span key={`text-${index}`}>{part}</span>
      );
    });
    return [
      getInstruction("single"),
      getInstruction("multi"),
      <>{panNodes}</>,
      getInstruction("zoom"),
    ];
  }, [dictionary, t]);

  return (
    <div className="min-h-screen">
      <header className="py-10 text-center">
        <h1 className="text-4xl font-bold text-blue-400">{t("landing.title")}</h1>
        <p className="mt-2 text-slate-300">{t("landing.subtitle")}</p>
        <div className="mt-4 flex items-center justify-center gap-6 text-sm text-slate-400">
          <span className="font-semibold text-slate-200">{t("landing.stats.taken", { count: heroStats.taken })}</span>
          <span className="font-semibold text-slate-200">{t("landing.stats.free", { count: heroStats.free })}</span>
        </div>
        {user && (
          <div className="mt-3 text-sm text-slate-300">
            <span className="mr-4 inline-flex items-center rounded-full bg-slate-800/70 px-4 py-1 text-slate-200">
              {t("common.labels.balance")}: {" "}
              <span className="ml-1 font-semibold">
                {`${typeof user.points === "number" ? user.points : 0} ${pointsShort}`}
              </span>
            </span>
            {typeof pixelCostPoints === "number" && pixelCostPoints > 0 && (
              <span className="inline-flex items-center rounded-full bg-slate-800/70 px-4 py-1 text-slate-200">
                {t("common.labels.pixelCost")}: {" "}
                <span className="ml-1 font-semibold">{`${pixelCostPoints} ${pointsShort}`}</span>
              </span>
            )}
          </div>
        )}
      </header>
      <main className="mx-auto flex max-w-5xl flex-col items-center gap-6 px-4 pb-16">
        {loading && <div className="text-slate-300">{t("landing.loading")}</div>}
        {error && <div className="text-rose-400">{error}</div>}
        {data && (
          <>
            <div className="w-full max-w-3xl rounded-xl border border-slate-600/70 bg-slate-900/70 px-6 py-4 text-sm text-slate-200 shadow-lg">
              <h2 className="text-base font-semibold text-slate-100">{t("landing.instructionsTitle")}</h2>
              <ul className="mt-2 list-disc space-y-1 pl-5 text-slate-300">
                {instructionItems.map((item, index) => (
                  <li key={`instruction-${index}`}>{item}</li>
                ))}
              </ul>
            </div>
            <PixelCanvas
              width={data.width}
              height={data.height}
              pixels={data.pixels}
              onPixelClick={handlePixelClick}
              onSelectionComplete={handleSelectionComplete}
            />
          </>
        )}
        {selectedPixels.length > 1 && (
          <div className="w-full max-w-2xl rounded-xl border border-blue-500/20 bg-blue-500/5 px-4 py-3 text-center text-sm text-blue-100">
            {t("landing.selection", { count: selectedPixels.length })}
          </div>
        )}
        <p className="max-w-2xl text-center text-sm text-slate-400">{t("landing.tips")}</p>
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
  const { t } = useI18n();
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
  const pointsShort = t("common.units.pointsShort");
  const selectedIdsLabel = selectedIds.join(", ");
  const firstSelectedId = selectedIds[0];
  const pageTitle = selectedCount > 1 ? t("buy.titleMultiple") : t("buy.titleSingle");
  const description =
    selectedCount > 1
      ? t("buy.descriptionMultiple", { count: selectedCount, ids: selectedIdsLabel })
      : t("buy.descriptionSingle", {
          id: typeof firstSelectedId === "number" ? firstSelectedId : t("buy.unknownPixel"),
        });
  const costDisplay =
    typeof pixelCostPoints === "number" && pixelCostPoints > 0
      ? selectedCount > 1 && totalCost !== null
        ? t("buy.costBreakdown", {
            count: selectedCount,
            price: pixelCostPoints,
            unit: pointsShort,
            total: totalCost,
          })
        : t("buy.singlePrice", { price: pixelCostPoints })
      : t("buy.configure");
  const balanceDisplay =
    typeof user?.points === "number"
      ? t("common.units.points", { count: user.points })
      : t("buy.balanceLogin");

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
      setPurchaseError(t("auth.errors.selectionRequired"));
      return;
    }

    setIsProcessing(true);
    setPurchaseStatus(null);
    setPurchaseError(null);
    setPixelResults([]);

    try {
      const authenticated = await ensureAuthenticated({
        message: t("auth.errors.loginToBuyMany"),
      });
      if (!authenticated) {
        throw new Error(t("auth.errors.requiresLogin"));
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
          void openLoginModal({ message: t("auth.errors.loginToFinish") });
          throw new Error(t("auth.errors.sessionExpired"));
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
            apiMessage || t("auth.errors.insufficientPoints")
          );
        }
        throw new Error(apiMessage || t("auth.errors.api", { status: response.status }));
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
        setPurchaseError(t("auth.messages.simulationPartial"));
      } else {
        const errorMessage =
          (payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
            ? ((payload as Record<string, unknown>).error as string)
            : null) ||
          t("auth.errors.purchase");
        setPurchaseError(errorMessage);
      }
    } catch (error) {
      console.error(error);
      const message = error instanceof Error ? error.message : t("auth.errors.purchase");
      setPurchaseError(message);
    } finally {
      setIsProcessing(false);
    }
  }, [ensureAuthenticated, openLoginModal, refresh, selectedColor, selectedIds, t, url]);

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-6 py-12 text-center">
      <div className="w-full max-w-2xl rounded-3xl bg-slate-900/60 p-10 shadow-xl">
        <h2 className="text-3xl font-semibold text-blue-400">{pageTitle}</h2>
        <p className="mt-3 text-slate-300">{description}</p>

        <div className="mt-6 grid gap-3 text-left text-sm text-slate-300 sm:grid-cols-2">
          <div className="rounded-xl border border-slate-800/70 bg-slate-900/70 p-4">
            <p className="font-semibold text-slate-100">{t("buy.costTitle")}</p>
            <p className="mt-1 text-slate-300">{costDisplay}</p>
          </div>
          <div className="rounded-xl border border-slate-800/70 bg-slate-900/70 p-4">
            <p className="font-semibold text-slate-100">{t("buy.balanceTitle")}</p>
            <p className="mt-1 text-slate-300">{balanceDisplay}</p>
          </div>
        </div>

        <div className="mt-8 rounded-2xl bg-slate-800/70 p-6 text-left">
          <h3 className="text-lg font-semibold text-slate-100">{t("buy.matchTitle")}</h3>
          <p className="mt-1 text-sm text-slate-400">{t("buy.matchDescription")}</p>

          <div className="mt-6 flex flex-col gap-6 sm:flex-row sm:items-center">
            <label className="flex flex-col gap-3 text-sm font-medium text-slate-200" htmlFor="pixel-color">
              {t("common.labels.pixelColor")}
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
            {t("common.labels.pixelUrl")}
            <input
              id="pixel-url"
              type="url"
              value={url}
              onChange={handleUrlChange}
              disabled={isProcessing}
              placeholder={t("common.placeholders.pixelUrl")}
              className="w-full rounded-lg border border-slate-700 bg-slate-900/80 px-4 py-3 text-sm text-slate-100 shadow-inner placeholder:text-slate-500"
            />
            <span className="text-xs font-normal text-slate-400">{t("buy.urlHint")}</span>
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
                {t("common.status.processing")}
              </span>
            ) : (
              selectedCount > 1 ? t("buy.simulateMultiple", { count: selectedCount }) : t("buy.simulateSingle")
            )}
          </button>

          <p aria-live="polite" className="mt-4 text-sm text-slate-400">
            {isProcessing
              ? t("auth.messages.simulationProcessing")
              : t("auth.messages.simulationInfo")}
          </p>

          {purchaseError && (
            <p role="alert" className="mt-4 text-sm text-rose-400">
              {purchaseError}
            </p>
          )}
          {pixelResults.length > 0 && (
            <div className="mt-6 rounded-xl border border-slate-700 bg-slate-900/60 p-4">
              <h4 className="text-sm font-semibold text-slate-200">{t("buy.resultsTitle")}</h4>
              <ul className="mt-3 space-y-2 text-sm text-slate-300">
                {pixelResults.map((result) => (
                  <li key={result.id} className="flex items-start justify-between gap-4">
                    <span className="font-mono text-slate-200">#{result.id}</span>
                    {result.error ? (
                      <span className="text-rose-400">{result.error}</span>
                    ) : (
                      <span className="text-emerald-300">
                        {result.status && result.status !== "taken"
                          ? result.status
                          : t("buy.resultStatus.taken")}
                      </span>
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
            <h3 className="text-xl font-semibold text-emerald-300">{t("buy.successTitle")}</h3>
            <p className="mt-2 text-sm">
              {selectedCount > 1
                ? t("buy.successDescriptionMultiple", {
                    count: selectedCount,
                    color: selectedColor.toUpperCase(),
                  })
                : t("buy.successDescriptionSingle", {
                    id: selectedIds[0] ?? t("buy.unknownPixel"),
                    color: selectedColor.toUpperCase(),
                  })}
            </p>
            <div className="mt-4 flex items-center gap-3">
              <span className="text-sm text-emerald-200">{t("buy.successPreview")}</span>
              <div
                aria-hidden
                className="h-8 w-8 rounded border border-emerald-300 shadow-inner"
                style={{ backgroundColor: selectedColor }}
              />
            </div>
            <p className="mt-4 text-xs text-emerald-200/80">{t("buy.successMockInfo")}</p>
          </div>
        )}
        {purchaseStatus === "partial" && (
          <div
            role="status"
            className="mt-8 rounded-2xl border border-amber-500/40 bg-amber-500/10 p-6 text-left text-amber-100"
          >
            <h3 className="text-xl font-semibold text-amber-200">{t("buy.partialTitle")}</h3>
            <p className="mt-2 text-sm">{t("buy.partialDescription")}</p>
          </div>
        )}

        <div className="mt-10">
          <Link
            to="/"
            className="inline-flex items-center justify-center rounded-full bg-blue-500 px-6 py-2 font-semibold text-white transition hover:bg-blue-400"
          >
            {t("buy.return")}
          </Link>
        </div>
      </div>
    </div>
  );
}

type PageLayoutProps = {
  children: ReactNode;
  onOpenRegister?: () => void;
  onOpenActivationCode?: () => void;
};

function PageLayout({ children, onOpenRegister, onOpenActivationCode }: PageLayoutProps) {
  return (
    <div className="flex min-h-screen flex-col bg-slate-950 text-slate-100">
      <NavigationBar onOpenRegister={onOpenRegister} onOpenActivationCode={onOpenActivationCode} />
      <main className="flex-1">{children}</main>
      <TermsFooter />
    </div>
  );
}

export default function App() {
  const { openLoginModal, refresh } = useAuth();
  const [isRegisterOpen, setIsRegisterOpen] = useState(false);
  const [isActivationModalOpen, setIsActivationModalOpen] = useState(false);
  const { t } = useI18n();

  const handleOpenRegister = useCallback(() => {
    setIsRegisterOpen(true);
  }, []);

  const handleCloseRegister = useCallback(() => {
    setIsRegisterOpen(false);
  }, []);

  const handleOpenLoginFromRegister = useCallback(() => {
    void openLoginModal({ message: t("auth.errors.loginToStart") });
  }, [openLoginModal, t]);

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
          element={
            <PageLayout onOpenRegister={handleOpenRegister} onOpenActivationCode={handleOpenActivationCode}>
              <LandingPage />
            </PageLayout>
          }
        />
        <Route
          path="/account"
          element={
            <PageLayout onOpenActivationCode={handleOpenActivationCode}>
              <AccountPage onOpenActivationCode={handleOpenActivationCode} />
            </PageLayout>
          }
        />
        <Route
          path="/verify"
          element={
            <PageLayout onOpenRegister={handleOpenRegister}>
              <VerifyAccountPage />
            </PageLayout>
          }
        />
        <Route
          path="/forgot-password"
          element={
            <PageLayout onOpenRegister={handleOpenRegister}>
              <ForgotPasswordPage />
            </PageLayout>
          }
        />
        <Route
          path="/reset-password"
          element={
            <PageLayout onOpenRegister={handleOpenRegister}>
              <ResetPasswordPage />
            </PageLayout>
          }
        />
        <Route
          path="/buy"
          element={
            <PageLayout onOpenRegister={handleOpenRegister} onOpenActivationCode={handleOpenActivationCode}>
              <BuyPixelPage />
            </PageLayout>
          }
        />
        <Route
          path="/buy/:pixelId"
          element={
            <PageLayout onOpenRegister={handleOpenRegister} onOpenActivationCode={handleOpenActivationCode}>
              <BuyPixelPage />
            </PageLayout>
          }
        />
        <Route
          path="/terms"
          element={
            <PageLayout onOpenRegister={handleOpenRegister}>
              <TermsPage />
            </PageLayout>
          }
        />
        <Route
          path="*"
          element={
            <PageLayout onOpenRegister={handleOpenRegister}>
              <div className="flex h-full flex-col items-center justify-center gap-3 text-center text-slate-300">
                <h2 className="text-2xl font-semibold text-white">{t("notFound.title")}</h2>
                <Link to="/" className="text-blue-400 underline">
                  {t("notFound.cta")}
                </Link>
              </div>
            </PageLayout>
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
