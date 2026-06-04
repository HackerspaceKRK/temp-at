import { useEffect, useRef, type FC } from "react";
import {
  isKioskAvailable,
  kioskScreenOff,
  kioskScreenOn,
  kioskSetBrightness,
  onKioskEvent,
} from "../../lib/kioskApi";

const OFF_AFTER_MS = 90_000; // turn the backlight off after this much inactivity
const DIM_BEFORE_OFF_MS = 15_000; // start dimming this long before turning off
const DIM_AT_MS = OFF_AFTER_MS - DIM_BEFORE_OFF_MS; // = 75s
const DIM_BRIGHTNESS = 0.1;
const FULL_BRIGHTNESS = 1.0;
const PROXIMITY_WAKE_THRESHOLD = 700; // raw proximity value that wakes the screen
const TICK_MS = 500;

const ACTIVITY_EVENTS = [
  "pointerdown",
  "mousemove",
  "keydown",
  "touchstart",
  "wheel",
] as const;

type ScreenState = "awake" | "dimmed" | "off";

/**
 * ScreenSleepManager dims and then turns the kiosk backlight off after a period
 * of inactivity, and wakes it on user interaction or a near proximity reading.
 *
 * - After {@link DIM_AT_MS} (75s) idle the backlight dims to
 *   {@link DIM_BRIGHTNESS} (10%).
 * - After {@link OFF_AFTER_MS} (90s) idle the backlight is turned off.
 * - Any interaction, a `kiosk_screen_woken` event, or a proximity sample above
 *   {@link PROXIMITY_WAKE_THRESHOLD} wakes the screen back to full brightness.
 *
 * Mounted once inside the tablet layout. Outside the kiosk the bridge calls are
 * no-ops, so this is harmless in a normal browser.
 */
export const ScreenSleepManager: FC = () => {
  const lastActivity = useRef(Date.now());
  const screenState = useRef<ScreenState>("awake");

  useEffect(() => {
    if (!isKioskAvailable()) return;

    const wake = () => {
      lastActivity.current = Date.now();
      if (screenState.current !== "awake") {
        kioskScreenOn();
        kioskSetBrightness(FULL_BRIGHTNESS);
        screenState.current = "awake";
      }
    };

    // Plain user interaction just refreshes the activity timestamp (and wakes
    // if we were dimmed/off).
    ACTIVITY_EVENTS.forEach((e) =>
      window.addEventListener(e, wake, { passive: true }),
    );

    // The native overlay already turns the backlight on when tapped; mirror our
    // state and restore full brightness.
    const offWoken = onKioskEvent("kiosk_screen_woken", wake);

    // A near object (value over the threshold) wakes the screen.
    const offProximity = onKioskEvent("kiosk_proximity", (detail) => {
      if (detail.value > PROXIMITY_WAKE_THRESHOLD) wake();
    });

    const id = window.setInterval(() => {
      const idle = Date.now() - lastActivity.current;
      if (idle >= OFF_AFTER_MS) {
        if (screenState.current !== "off") {
          kioskScreenOff();
          screenState.current = "off";
        }
      } else if (idle >= DIM_AT_MS) {
        if (screenState.current === "awake") {
          kioskSetBrightness(DIM_BRIGHTNESS);
          screenState.current = "dimmed";
        }
      }
    }, TICK_MS);

    return () => {
      ACTIVITY_EVENTS.forEach((e) => window.removeEventListener(e, wake));
      offWoken();
      offProximity();
      window.clearInterval(id);
    };
  }, []);

  return null;
};
