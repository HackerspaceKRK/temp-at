// Web Push helpers: registers the service worker, requests notification
// permission, and subscribes the browser to print notifications for a printer.

import { apiPath } from "./config";

export function pushSupported(): boolean {
  return (
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

// VAPID public keys are base64url; PushManager wants a Uint8Array.
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = window.atob(base64);
  const output = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) output[i] = raw.charCodeAt(i);
  return output;
}

let swRegistration: ServiceWorkerRegistration | null = null;

export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!("serviceWorker" in navigator)) return null;
  if (swRegistration) return swRegistration;
  try {
    swRegistration = await navigator.serviceWorker.register("/sw.js");
    return swRegistration;
  } catch (err) {
    console.error("[push] service worker registration failed", err);
    return null;
  }
}

/**
 * Subscribe the current browser to notifications about the print currently
 * running on `printerId`. Throws on failure (e.g. permission denied).
 */
export async function subscribeToPrint(printerId: string): Promise<void> {
  if (!pushSupported()) {
    throw new Error("Push notifications are not supported in this browser");
  }

  const registration =
    (await navigator.serviceWorker.ready) || (await registerServiceWorker());
  if (!registration) throw new Error("Service worker unavailable");

  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    throw new Error("Notification permission denied");
  }

  // Fetch the server's VAPID public key.
  const keyResp = await fetch(apiPath("api/v1/push/vapid-public-key"));
  if (!keyResp.ok) throw new Error("Failed to fetch VAPID key");
  const { key } = await keyResp.json();

  let subscription = await registration.pushManager.getSubscription();
  if (!subscription) {
    subscription = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(key) as BufferSource,
    });
  }

  const resp = await fetch(apiPath("api/v1/push/subscribe"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      printer_id: printerId,
      subscription: subscription.toJSON(),
    }),
  });
  if (!resp.ok) {
    throw new Error(`Subscription failed: ${resp.status}`);
  }
}
