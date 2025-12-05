import { createContext } from "react";
import { useContext } from "react";
import type { LocalizedName } from "./schema";

/**
 * Value stored in the LocaleContext.
 */
export interface LocaleContextValue {
  /**
   * Current BCP-47 / custom locale code.
   * Hardcoded to "pl" for now.
   */
  locale: string;
  /**
   * Resolve a localized name; falls back to provided fallbackId if missing.
   */
  getName: (data: LocalizedName | null | undefined, fallbackId?: string) => string;
}

/**
 * Internal helper to resolve a localized name object into a display string.
 */
function resolveLocalizedName(
  locale: string,
  data: LocalizedName | null | undefined,
  fallbackId?: string
): string {
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
 * Context with default implementation (locale "pl").
 */
const LocaleContext = createContext<LocaleContextValue>({
  locale: "pl",
  getName: (data, fallbackId) => resolveLocalizedName("pl", data, fallbackId),
});

/**
 * Provider component. For now locale is hardcoded to "pl" unless overridden.
 */
export function LocaleProvider({
  children,
  locale = "pl",
}: {
  children: React.ReactNode;
  locale?: string;
}) {
  const value: LocaleContextValue = {
    locale,
    getName: (data, fallbackId) => resolveLocalizedName(locale, data, fallbackId),
  };
  return (
    <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>
  );
}

/**
 * Hook to consume the locale context.
 */
export function useLocale(): LocaleContextValue {
  return useContext(LocaleContext);
}

/**
 * Convenience function if you want a pure function without hook usage.
 */
export function getLocalizedName(
  locale: string,
  data: LocalizedName | null | undefined,
  fallbackId?: string
): string {
  return resolveLocalizedName(locale, data, fallbackId);
}

export default LocaleContext;
