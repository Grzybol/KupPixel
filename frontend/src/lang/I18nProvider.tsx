import {
  ReactNode,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { LANGUAGE_STORAGE_KEY, LanguageCode, LanguageDefinition, TranslationDictionary, defaultLanguage, languages } from "./index";

type TranslationParams = Record<string, string | number>;

type I18nContextValue = {
  language: LanguageCode;
  setLanguage: (code: LanguageCode) => void;
  availableLanguages: LanguageDefinition[];
  t: (key: string, params?: TranslationParams) => string;
  dictionary: TranslationDictionary;
};

type TranslationValue = string | TranslationDictionary | TranslationValue[];

const I18nContext = createContext<I18nContextValue | undefined>(undefined);

const placeholderPattern = /\{\{\s*(\w+)\s*\}\}/g;

function resolveTranslation(dictionary: TranslationDictionary, key: string): string | undefined {
  const segments = key.split(".");
  let current: TranslationDictionary | TranslationValue | undefined = dictionary;
  for (const segment of segments) {
    if (!current || typeof current !== "object") {
      return undefined;
    }
    const next = (current as TranslationDictionary)[segment];
    if (next === undefined) {
      return undefined;
    }
    current = next;
  }
  if (typeof current === "string") {
    return current;
  }
  return undefined;
}

function formatTranslation(template: string, params?: TranslationParams): string {
  if (!params) {
    return template;
  }
  return template.replace(placeholderPattern, (match, token: string) => {
    if (Object.prototype.hasOwnProperty.call(params, token)) {
      return String(params[token]);
    }
    return match;
  });
}

function isLanguageCode(value: string | null): value is LanguageCode {
  return value === "pl" || value === "en";
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, setLanguage] = useState<LanguageCode>(() => {
    if (typeof window === "undefined") {
      return defaultLanguage;
    }
    const stored = window.localStorage.getItem(LANGUAGE_STORAGE_KEY);
    if (isLanguageCode(stored)) {
      return stored;
    }
    return defaultLanguage;
  });

  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(LANGUAGE_STORAGE_KEY, language);
    }
  }, [language]);

  const availableLanguages = useMemo(() => Object.values(languages), []);

  const dictionary = useMemo(() => languages[language]?.dictionary ?? languages[defaultLanguage].dictionary, [language]);
  const fallbackDictionary = useMemo(
    () => (language === defaultLanguage ? dictionary : languages[defaultLanguage].dictionary),
    [dictionary, language]
  );

  const translate = useCallback(
    (key: string, params?: TranslationParams) => {
      const primary = resolveTranslation(dictionary, key);
      if (primary) {
        return formatTranslation(primary, params);
      }
      const fallback = resolveTranslation(fallbackDictionary, key);
      if (fallback) {
        return formatTranslation(fallback, params);
      }
      return key;
    },
    [dictionary, fallbackDictionary]
  );

  const value = useMemo<I18nContextValue>(
    () => ({
      language,
      setLanguage,
      availableLanguages,
      t: translate,
      dictionary,
    }),
    [availableLanguages, dictionary, language, translate]
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useI18n must be used within an I18nProvider");
  }
  return context;
}
