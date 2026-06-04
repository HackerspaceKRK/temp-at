import { useEffect, useRef, useState, type FC } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useTabletSession } from "./TabletSessionContext";

const COUNTDOWN_START_MS = 35_000; // show overlay after this much inactivity
const REDIRECT_AT_MS = 45_000; // navigate home at this much inactivity
const TICK_MS = 250;
const ACTIVITY_EVENTS = [
  "pointerdown",
  "mousemove",
  "keydown",
  "touchstart",
  "wheel",
] as const;

/**
 * InactivityRedirect returns the kiosk to its initial page after a period of
 * inactivity. While the user is away from the initial page and idle, an overlay
 * with a progress bar appears for the final 10 seconds; any interaction cancels
 * it. Mounted once inside the tablet layout.
 */
export const InactivityRedirect: FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { initialPath } = useTabletSession();

  const enabled = location.pathname !== initialPath;
  const lastActivity = useRef(Date.now());
  const [progress, setProgress] = useState<number | null>(null);

  // Reset the timer whenever the page changes (so landing somewhere new starts fresh).
  useEffect(() => {
    lastActivity.current = Date.now();
    setProgress(null);
  }, [location.pathname]);

  useEffect(() => {
    if (!enabled) {
      setProgress(null);
      return;
    }

    const onActivity = () => {
      lastActivity.current = Date.now();
    };
    ACTIVITY_EVENTS.forEach((e) =>
      window.addEventListener(e, onActivity, { passive: true }),
    );

    const id = window.setInterval(() => {
      const idle = Date.now() - lastActivity.current;
      if (idle >= REDIRECT_AT_MS) {
        setProgress(null);
        navigate(initialPath);
        return;
      }
      if (idle >= COUNTDOWN_START_MS) {
        const p =
          (idle - COUNTDOWN_START_MS) / (REDIRECT_AT_MS - COUNTDOWN_START_MS);
        setProgress(Math.min(1, Math.max(0, p)));
      } else {
        setProgress((prev) => (prev === null ? prev : null));
      }
    }, TICK_MS);

    return () => {
      ACTIVITY_EVENTS.forEach((e) => window.removeEventListener(e, onActivity));
      window.clearInterval(id);
    };
  }, [enabled, initialPath, navigate]);

  if (progress === null) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-6 bg-background/90 backdrop-blur-sm"
      onClick={() => {
        lastActivity.current = Date.now();
        setProgress(null);
      }}
    >
      <div className="text-2xl font-semibold text-foreground">
        {t("Redirecting back to home...")}
      </div>
      <div className="h-3 w-80 max-w-[80vw] overflow-hidden rounded-full bg-muted">
        <div
          className="h-full bg-primary transition-[width] duration-200 ease-linear"
          style={{ width: `${Math.round(progress * 100)}%` }}
        />
      </div>
      <div className="text-sm text-muted-foreground">{t("Tap to cancel")}</div>
    </div>
  );
};
