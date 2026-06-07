/** DOM events that count as user activity on the kiosk. */
export const ACTIVITY_EVENTS = [
  "pointerdown",
  "mousemove",
  "keydown",
  "touchstart",
  "wheel",
] as const;

/**
 * Idle duration after which the kiosk returns to its resting state: the
 * InactivityRedirect navigates back to the initial page, and the reservations
 * panel resets to today and closes any open dialog.
 */
export const TABLET_IDLE_MS = 45_000;
