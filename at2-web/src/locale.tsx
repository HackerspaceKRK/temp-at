import { useTranslation } from "react-i18next";
import i18n from "./i18n";
import type { LocalizedName } from "./schema";

/**
 * Resolve a localized name; falls back to provided fallbackId if missing.
 * Uses the current language from i18next.
 */
export function getName(
  data: LocalizedName | null | undefined,
  fallbackId?: string
): string {
  const locale = i18n.language || "en";
  if (!data) return fallbackId ?? "";
  const direct = data[locale];
  if (direct && direct.trim().length > 0) return direct;
  // Try simple language subtag fallback (e.g. pl-PL -> pl)
  const short = locale.split("-")[0];
  if (short !== locale && data[short]) return data[short];
  // Otherwise pick first non-empty entry
  for (const key of Object.keys(data)) {
    const val = data[key];
    if (val && val.trim().length > 0) return val;
  }
  return fallbackId ?? "";
}

/**
 * Hook to get the getName function. 
 * Using a hook ensures the component re-renders when the language changes.
 */
export function useLocale() {
  const { i18n } = useTranslation();
  return {
    locale: i18n.language,
    getName,
  };
}

export default getName;
