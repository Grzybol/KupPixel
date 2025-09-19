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

export type AuthUser = {
  email: string;
  [key: string]: unknown;
};

type LoginCredentials = {
  email: string;
  password: string;
};

type RegisterCredentials = {
  email: string;
  password: string;
};

type OpenOptions = {
  message?: string;
};

type AuthContextValue = {
  user: AuthUser | null;
  isLoading: boolean;
  login: (credentials: LoginCredentials) => Promise<void>;
  register: (credentials: RegisterCredentials) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<AuthUser | null>;
  ensureAuthenticated: (options?: OpenOptions) => Promise<boolean>;
  openLoginModal: (options?: OpenOptions) => Promise<boolean>;
  closeLoginModal: () => void;
  isLoginModalOpen: boolean;
  loginPrompt: string | null;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

function parseUser(data: unknown): AuthUser | null {
  if (!data || typeof data !== "object") {
    return null;
  }
  if ("email" in data && typeof (data as Record<string, unknown>).email === "string") {
    return data as AuthUser;
  }
  if ("user" in data && typeof (data as Record<string, unknown>).user === "object") {
    const nested = (data as Record<string, unknown>).user as Record<string, unknown>;
    if (typeof nested.email === "string") {
      return nested as AuthUser;
    }
  }
  return null;
}

export function AuthProvider({ children }: PropsWithChildren) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoginModalOpen, setIsLoginModalOpen] = useState(false);
  const [loginPrompt, setLoginPrompt] = useState<string | null>(null);
  const loginResolverRef = useRef<((result: boolean) => void) | null>(null);

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
          return null;
        }
        const message = await response.text().catch(() => "");
        throw new Error(message || `Nie udało się odświeżyć sesji (${response.status})`);
      }
      const data = await response.json().catch(() => null);
      const parsed = parseUser(data);
      setUser(parsed);
      return parsed;
    } catch (error) {
      console.error("refresh session", error);
      setUser(null);
      throw error;
    } finally {
      setIsLoading(false);
    }
  }, []);

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
        }),
      });
      if (!response.ok) {
        const message = await response.text().catch(() => "");
        throw new Error(message || "Nie udało się zalogować. Spróbuj ponownie.");
      }
      await refresh().catch(() => {
        // refresh already logs error and resets user; surface a more helpful message
        throw new Error("Nie udało się odczytać informacji o sesji.");
      });
      setIsLoginModalOpen(false);
      setLoginPrompt(null);
      if (loginResolverRef.current) {
        loginResolverRef.current(true);
        loginResolverRef.current = null;
      }
    },
    [refresh]
  );

  const register = useCallback(
    async (credentials: RegisterCredentials) => {
      const response = await fetch("/api/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(credentials),
      });
      if (!response.ok) {
        const message = await response.text().catch(() => "");
        throw new Error(message || "Nie udało się utworzyć konta. Spróbuj ponownie.");
      }
      await refresh().catch(() => {
        throw new Error("Nie udało się odczytać informacji o sesji.");
      });
      setIsLoginModalOpen(false);
      setLoginPrompt(null);
      if (loginResolverRef.current) {
        loginResolverRef.current(true);
        loginResolverRef.current = null;
      }
    },
    [refresh]
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
      logout,
      refresh,
      ensureAuthenticated,
      openLoginModal,
      closeLoginModal: closeModal,
      isLoginModalOpen,
      loginPrompt,
    }),
    [closeModal, ensureAuthenticated, isLoading, isLoginModalOpen, login, loginPrompt, logout, openLoginModal, refresh, user]
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
