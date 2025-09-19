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
  id: number;
  email: string;
  emailVerified: boolean;
  [key: string]: unknown;
};

type LoginCredentials = {
  email: string;
  password: string;
};

type AuthError = Error & {
  requiresVerification?: boolean;
};

type OpenOptions = {
  message?: string;
};

type AuthContextValue = {
  user: AuthUser | null;
  isLoading: boolean;
  login: (credentials: LoginCredentials) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<AuthUser | null>;
  ensureAuthenticated: (options?: OpenOptions) => Promise<boolean>;
  openLoginModal: (options?: OpenOptions) => Promise<boolean>;
  closeLoginModal: () => void;
  isLoginModalOpen: boolean;
  loginPrompt: string | null;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

function toAuthUser(raw: unknown): AuthUser | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const record = raw as Record<string, unknown>;
  const emailValue = record.email;
  if (typeof emailValue !== "string" || emailValue.trim() === "") {
    return null;
  }

  const idValue = record.id;
  let id: number | null = null;
  if (typeof idValue === "number" && Number.isFinite(idValue)) {
    id = idValue;
  } else if (typeof idValue === "string" && idValue.trim() !== "") {
    const parsed = Number(idValue);
    if (Number.isFinite(parsed)) {
      id = parsed;
    }
  }
  if (id === null) {
    return null;
  }

  const verifiedRaw = record.email_verified ?? record.emailVerified;
  const emailVerified = typeof verifiedRaw === "boolean" ? verifiedRaw : false;

  const user: AuthUser = {
    id,
    email: emailValue,
    emailVerified,
  };
  return user;
}

function parseUser(data: unknown): AuthUser | null {
  const direct = toAuthUser(data);
  if (direct) {
    return direct;
  }
  if (data && typeof data === "object" && "user" in data) {
    return toAuthUser((data as Record<string, unknown>).user);
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
      const payload = {
        email: credentials.email.trim().toLowerCase(),
        password: credentials.password,
      };
      const response = await fetch("/api/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        let message = "Nie udało się zalogować. Spróbuj ponownie.";
        let requiresVerification = false;
        try {
          const data = (await response.json()) as Record<string, unknown>;
          if (data && typeof data.error === "string" && data.error.trim() !== "") {
            message = data.error;
          }
          if (typeof data?.requires_verification === "boolean") {
            requiresVerification = data.requires_verification;
          }
        } catch (error) {
          const fallback = await response.text().catch(() => "");
          if (fallback.trim() !== "") {
            message = fallback;
          }
        }
        const authError = new Error(message) as AuthError;
        if (requiresVerification) {
          authError.requiresVerification = true;
        }
        throw authError;
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
