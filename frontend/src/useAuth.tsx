import {
  PropsWithChildren,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useI18n } from "./lang/I18nProvider.js";

export type AuthUser = {
  id?: number;
  email: string;
  is_verified?: boolean;
  verified_at?: string | null;
  points?: number;
  [key: string]: unknown;
};

type LoginCredentials = {
  email: string;
  password: string;
  turnstileToken: string;
};

type RegisterCredentials = {
  email: string;
  password: string;
  confirmPassword: string;
  turnstileToken: string;
};

type RegisterResult = {
  message: string;
};

type PasswordResetRequestPayload = {
  email: string;
  turnstileToken: string;
};

type PasswordResetConfirmPayload = {
  token: string;
  password: string;
  confirmPassword: string;
  turnstileToken: string;
};

type OpenOptions = {
  message?: string;
};

type AuthContextValue = {
  user: AuthUser | null;
  isLoading: boolean;
  login: (credentials: LoginCredentials) => Promise<void>;
  register: (credentials: RegisterCredentials) => Promise<RegisterResult>;
  requestPasswordReset: (payload: PasswordResetRequestPayload) => Promise<string>;
  confirmPasswordReset: (payload: PasswordResetConfirmPayload) => Promise<string>;
  logout: () => Promise<void>;
  refresh: () => Promise<AuthUser | null>;
  ensureAuthenticated: (options?: OpenOptions) => Promise<boolean>;
  openLoginModal: (options?: OpenOptions) => Promise<boolean>;
  closeLoginModal: () => void;
  isLoginModalOpen: boolean;
  loginPrompt: string | null;
  pixelCostPoints: number | null;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function parseUser(data: unknown): AuthUser | null {
  if (!data || typeof data !== "object") {
    return null;
  }
  if ("email" in data && typeof (data as Record<string, unknown>).email === "string") {
    return data as AuthUser;
  }
  if ("user" in data) {
    const nested = (data as Record<string, unknown>).user;
    if (!nested || typeof nested !== "object") {
      return null;
    }
    const record = nested as Record<string, unknown>;
    if ("email" in record && typeof record.email === "string") {
      return record as AuthUser;
    }
    return null;
  }
  return null;
}

function extractPixelCostPoints(data: unknown): number | null {
  if (!data || typeof data !== "object") {
    return null;
  }
  const record = data as Record<string, unknown>;
  if (typeof record.pixel_cost_points === "number" && Number.isFinite(record.pixel_cost_points)) {
    return record.pixel_cost_points;
  }
  return null;
}

export function AuthProvider({ children }: PropsWithChildren) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoginModalOpen, setIsLoginModalOpen] = useState(false);
  const [loginPrompt, setLoginPrompt] = useState<string | null>(null);
  const [pixelCostPoints, setPixelCostPoints] = useState<number | null>(null);
  const loginResolverRef = useRef<((result: boolean) => void) | null>(null);
  const { t } = useI18n();

  const closeModal = useCallback(() => {
    setIsLoginModalOpen(false);
    setLoginPrompt(null);
    if (loginResolverRef.current) {
      loginResolverRef.current(false);
      loginResolverRef.current = null;
    }
  }, []);

  const refresh = useCallback(async (): Promise<AuthUser | null> => {
    try {
      const response = await fetch("/api/session", {
        credentials: "include",
      });
      if (!response.ok) {
        if (response.status === 401) {
          setUser(null);
          setPixelCostPoints(null);
          return null;
        }
        const message = await response.text().catch(() => "");
        throw new Error(message || t("auth.errors.refresh", { status: response.status }));
      }
      const data = await response.json().catch(() => null);
      const parsed = parseUser(data);
      setUser(parsed);
      const cost = extractPixelCostPoints(data);
      setPixelCostPoints(cost);
      return parsed;
    } catch (error) {
      console.error("refresh session", error);
      setUser(null);
      setPixelCostPoints(null);
      if (error instanceof Error && error.name === "TypeError") {
        return null;
      }
      throw error;
    } finally {
      setIsLoading(false);
    }
  }, [t]);

  const openLoginModal = useCallback(
    (options?: OpenOptions) => {
      if (loginResolverRef.current) {
        loginResolverRef.current(false);
      }
      if (options?.message) {
        setLoginPrompt(options.message);
      } else {
        setLoginPrompt(null);
      }
      setIsLoginModalOpen(true);
      return new Promise<boolean>((resolve) => {
        loginResolverRef.current = resolve;
      });
    },
    []
  );

  const ensureAuthenticated = useCallback(
    async (options?: OpenOptions) => {
      if (user) {
        return true;
      }
      try {
        const result = await openLoginModal(options);
        return result;
      } catch (error) {
        console.error("ensureAuthenticated", error);
        return false;
      }
    },
    [openLoginModal, user]
  );

  const login = useCallback(
    async (credentials: LoginCredentials) => {
      const response = await fetch("/api/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          email: credentials.email,
          password: credentials.password,
          turnstile_token: credentials.turnstileToken,
        }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        const message =
          (payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
            ? ((payload as Record<string, unknown>).error as string)
            : null) || t("auth.errors.login");
        throw new Error(message);
      }
      const parsed = parseUser(payload);
      if (!parsed) {
        throw new Error(t("auth.errors.parseUser"));
      }
      setUser(parsed);
      const cost = extractPixelCostPoints(payload);
      if (cost !== null) {
        setPixelCostPoints(cost);
      }
      setIsLoginModalOpen(false);
      setLoginPrompt(null);
      if (loginResolverRef.current) {
        loginResolverRef.current(true);
        loginResolverRef.current = null;
      }
    },
    [t]
  );

  const register = useCallback(
    async (credentials: RegisterCredentials): Promise<RegisterResult> => {
      if (credentials.password !== credentials.confirmPassword) {
        throw new Error(t("auth.errors.passwordMismatch"));
      }
      const response = await fetch("/api/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          email: credentials.email,
          password: credentials.password,
          confirm_password: credentials.confirmPassword,
          turnstile_token: credentials.turnstileToken,
        }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        const message =
          (payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
            ? ((payload as Record<string, unknown>).error as string)
            : null) || t("auth.errors.register");
        throw new Error(message);
      }

      const message =
        payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).message === "string"
          ? ((payload as Record<string, unknown>).message as string)
          : t("auth.messages.registerSuccess");
      if (loginResolverRef.current) {
        loginResolverRef.current(false);
        loginResolverRef.current = null;
      }
      return { message };
    },
    [t]
  );

  const requestPasswordReset = useCallback(
    async ({ email, turnstileToken }: PasswordResetRequestPayload): Promise<string> => {
      const response = await fetch("/api/password-reset/request", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ email, turnstile_token: turnstileToken }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        const message =
          (payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
            ? ((payload as Record<string, unknown>).error as string)
            : null) || t("auth.passwordReset.errors.request");
        throw new Error(message);
      }
      const message =
        payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).message === "string"
          ? ((payload as Record<string, unknown>).message as string)
          : t("auth.passwordReset.success");
      return message;
    },
    [t]
  );

  const confirmPasswordReset = useCallback(
    async ({ token, password, confirmPassword, turnstileToken }: PasswordResetConfirmPayload): Promise<string> => {
      const response = await fetch("/api/password-reset/confirm", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          token,
          password,
          confirm_password: confirmPassword,
          turnstile_token: turnstileToken,
        }),
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) {
        const message =
          (payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).error === "string"
            ? ((payload as Record<string, unknown>).error as string)
            : null) || t("auth.passwordReset.errors.confirm");
        throw new Error(message);
      }
      const message =
        payload && typeof payload === "object" && typeof (payload as Record<string, unknown>).message === "string"
          ? ((payload as Record<string, unknown>).message as string)
          : t("auth.passwordReset.confirmSuccess");
      return message;
    },
    [t]
  );

  const logout = useCallback(async () => {
    try {
      await fetch("/api/logout", {
        method: "POST",
        credentials: "include",
      });
    } catch (error) {
      console.error("logout", error);
    } finally {
      setUser(null);
      setPixelCostPoints(null);
      closeModal();
    }
  }, [closeModal]);

  useEffect(() => {
    void refresh().catch((error) => {
      console.error("initial session load", error);
    });
  }, [refresh]);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      isLoading,
      login,
      register,
      requestPasswordReset,
      confirmPasswordReset,
      logout,
      refresh,
      ensureAuthenticated,
      openLoginModal,
      closeLoginModal: closeModal,
      isLoginModalOpen,
      loginPrompt,
      pixelCostPoints,
    }),
    [
      closeModal,
      ensureAuthenticated,
      isLoading,
      isLoginModalOpen,
      login,
      loginPrompt,
      logout,
      requestPasswordReset,
      confirmPasswordReset,
      openLoginModal,
      refresh,
      user,
      pixelCostPoints,
    ]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
