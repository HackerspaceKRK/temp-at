import { createContext } from "react";
import type { FunctionalComponent, ComponentChildren } from "react";
import { useContext, useEffect, useState, useCallback } from "react";

/**
 * Theme (color scheme) modes supported.
 */
export type ThemeMode = "light" | "dark";

interface ThemeContextValue {
  theme: ThemeMode;
  setTheme: (mode: ThemeMode) => void;
  toggleTheme: () => void;
}

/**
 * ThemeContext providing current theme and mutators.
 */
const ThemeContext = createContext<ThemeContextValue>({
  theme: "light",
  setTheme: () => {},
  toggleTheme: () => {},
});

/**
 * Hook to access theme context.
 */
export const useTheme = () => useContext(ThemeContext);

/**
 * Resolve the initial theme:
 * 1. If localStorage has a saved theme, use it.
 * 2. Else, use system preference (prefers-color-scheme)
 * 3. Fallback to light
 */
function resolveInitialTheme(): ThemeMode {
  if (typeof window === "undefined") return "light";
  const stored = window.localStorage.getItem("theme");
  if (stored === "dark" || stored === "light") return stored;
  if (
    window.matchMedia &&
    window.matchMedia("(prefers-color-scheme: dark)").matches
  ) {
    return "dark";
  }
  return "light";
}

/**
 * Apply or remove the 'dark' class on the root element.
 * Tailwind's dark mode (class strategy) relies on a 'dark' class
 * somewhere in the ancestor tree, commonly on <html>.
 */
function applyDocumentTheme(mode: ThemeMode) {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  if (mode === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
}

/**
 * Provider component to manage theme state and toggling.
 * Simplified to only add/remove Tailwind's 'dark' class
 * on document.documentElement.
 */
export const ThemeProvider: FunctionalComponent<{
  children: ComponentChildren;
}> = ({ children }) => {
  const [theme, setThemeState] = useState<ThemeMode>(resolveInitialTheme());
  // Track whether the user explicitly chose a theme; if not,
  // we can still auto-switch on system changes.
  const [explicitPreference, setExplicitPreference] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    const stored = window.localStorage.getItem("theme");
    return stored === "dark" || stored === "light";
  });

  const setTheme = useCallback((mode: ThemeMode) => {
    setThemeState(mode);
    setExplicitPreference(true);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("theme", mode);
    }
  }, []);

  const toggleTheme = useCallback(() => {
    setTheme(theme === "dark" ? "light" : "dark");
  }, [theme, setTheme]);

  // Apply dark class whenever theme changes.
  useEffect(() => {
    applyDocumentTheme(theme);
  }, [theme]);

  // Listen for prefers-color-scheme changes only if user
  // has not explicitly chosen a theme.
  useEffect(() => {
    if (explicitPreference || typeof window === "undefined") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const listener = () => {
      setThemeState(mq.matches ? "dark" : "light");
      // Do not mark as explicit; allow continued auto-updates.
    };
    if (mq.addEventListener) {
      mq.addEventListener("change", listener);
    } else {
      // Older Safari fallback
      // @ts-ignore
      mq.addListener(listener);
    }
    return () => {
      if (mq.removeEventListener) {
        mq.removeEventListener("change", listener);
      } else {
        // @ts-ignore
        mq.removeListener(listener);
      }
    };
  }, [explicitPreference]);

  return (
    <ThemeContext.Provider value={{ theme, setTheme, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  );
};

export default ThemeProvider;

/* ============================================
 * Usage:
 * Wrap your app root with <ThemeProvider>.
 * Then in any component:
 *
 *   import { useTheme } from './theme';
 *   const { theme, toggleTheme, setTheme } = useTheme();
 *
 * Tailwind example:
 * <div className="bg-neutral-200 dark:bg-neutral-900 text-neutral-900 dark:text-neutral-100">
 *   ...
 * </div>
 *
 * Toggling:
 * <button onClick={toggleTheme}>
 *   Switch to {theme === 'dark' ? 'light' : 'dark'} mode
 * </button>
 *
 * An explicit set (locking user preference):
 * setTheme('dark');
 * ============================================ */
