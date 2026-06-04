/**
 * Typed wrapper around the HSKRKKiosk native bridge (`window.kiosk_*`).
 *
 * The kiosk runs our site inside a full-screen Android WebView and installs a
 * set of `kiosk_*` globals at document start, plus dispatches `kiosk_*`
 * CustomEvents on `window`. Outside the kiosk these globals are absent, so
 * every wrapper is guarded and `isKioskAvailable()` can be used to branch the
 * UI.
 *
 * See HSKRKKiosk/docs/kiosk-js-api.md for the authoritative reference.
 */

export interface KioskAudioDevice {
  index: number;
  name: string;
  inputCount: number;
  outputCount: number;
}

export interface KioskSipRegistrationDetail {
  registered: boolean;
  code: number;
  text: string;
}

export interface KioskCallDetail {
  remoteUri: string;
}

export interface KioskProximityDetail {
  value: number;
  near: boolean;
}

interface KioskBridge {
  kiosk_make_call?: (dest: string) => void;
  kiosk_answer_call?: () => void;
  kiosk_reject_call?: () => void;
  kiosk_hangup_call?: () => void;
  kiosk_list_audio_devices?: () => KioskAudioDevice[];
  kiosk_screen_on?: () => void;
  kiosk_screen_off?: () => void;
  kiosk_set_brightness?: (fraction: number) => void;
  kiosk_proximity_start?: () => void;
  kiosk_proximity_stop?: () => void;
  kiosk_get_proximity?: () => number;
  kiosk_watchdog_enable?: () => void;
  kiosk_watchdog_feed?: () => void;
}

/** The set of CustomEvents the native side dispatches on `window`. */
export interface KioskEventMap {
  kiosk_sip_registration: KioskSipRegistrationDetail;
  kiosk_incoming_call: KioskCallDetail;
  kiosk_call_answered: KioskCallDetail;
  kiosk_call_hangup: KioskCallDetail;
  kiosk_proximity: KioskProximityDetail;
}

export const KIOSK_EVENT_NAMES: (keyof KioskEventMap)[] = [
  "kiosk_sip_registration",
  "kiosk_incoming_call",
  "kiosk_call_answered",
  "kiosk_call_hangup",
  "kiosk_proximity",
];

function bridge(): KioskBridge {
  return window as unknown as KioskBridge;
}

/** True when running inside the kiosk WebView (native bridge present). */
export function isKioskAvailable(): boolean {
  return typeof bridge().kiosk_set_brightness === "function";
}

/* --- SIP --- */
export function kioskMakeCall(dest: string): void {
  bridge().kiosk_make_call?.(dest);
}
export function kioskAnswerCall(): void {
  bridge().kiosk_answer_call?.();
}
export function kioskRejectCall(): void {
  bridge().kiosk_reject_call?.();
}
export function kioskHangupCall(): void {
  bridge().kiosk_hangup_call?.();
}

/* --- Audio --- */
export function kioskListAudioDevices(): KioskAudioDevice[] {
  return bridge().kiosk_list_audio_devices?.() ?? [];
}

/* --- Screen & brightness --- */
export function kioskScreenOn(): void {
  bridge().kiosk_screen_on?.();
}
export function kioskScreenOff(): void {
  bridge().kiosk_screen_off?.();
}
export function kioskSetBrightness(fraction: number): void {
  bridge().kiosk_set_brightness?.(fraction);
}

/* --- Proximity --- */
export function kioskProximityStart(): void {
  bridge().kiosk_proximity_start?.();
}
export function kioskProximityStop(): void {
  bridge().kiosk_proximity_stop?.();
}
export function kioskGetProximity(): number {
  return bridge().kiosk_get_proximity?.() ?? 0;
}

/* --- Watchdog --- */
export function kioskWatchdogEnable(): void {
  bridge().kiosk_watchdog_enable?.();
}
export function kioskWatchdogFeed(): void {
  bridge().kiosk_watchdog_feed?.();
}

/**
 * Subscribe to a kiosk CustomEvent with a typed `detail`. Returns an
 * unsubscribe function.
 */
export function onKioskEvent<K extends keyof KioskEventMap>(
  name: K,
  handler: (detail: KioskEventMap[K]) => void,
): () => void {
  const listener = (event: Event) => {
    handler((event as CustomEvent<KioskEventMap[K]>).detail);
  };
  window.addEventListener(name, listener);
  return () => window.removeEventListener(name, listener);
}
