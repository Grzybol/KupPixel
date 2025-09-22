import en from "./en.json";
import pl from "./pl.json";

export type TranslationValue = string | TranslationDictionary | TranslationValue[];
export type TranslationDictionary = { [key: string]: TranslationValue };

export type LanguageCode = "pl" | "en";

export type LanguageDefinition = {
  code: LanguageCode;
  label: string;
  dictionary: TranslationDictionary;
};

const castDictionary = (value: unknown): TranslationDictionary => value as TranslationDictionary;

export const languages: Record<LanguageCode, LanguageDefinition> = {
  pl: { code: "pl", label: "Polski", dictionary: castDictionary(pl) },
  en: { code: "en", label: "English", dictionary: castDictionary(en) },
};

export const defaultLanguage: LanguageCode = "pl";

export const LANGUAGE_STORAGE_KEY = "kuppixel.language";
