// Central configuration for API base URLs and resource helpers.

// Base HTTP(S) API URL (no trailing slash). Customize for deployment.
// Can be overridden at runtime by calling setApiUrl or defining global __AT2_API_URL__ prior to bundle execution.
export const API_URL =
  (globalThis as any).__AT2_API_URL__ || "";

/**
 * Build full API path for REST endpoints.
 * Strips any duplicate slashes between segments.
 */
export function apiPath(...segments: string[]): string {
  const cleaned = segments
    .filter(Boolean)
    .map((s) => s.replace(/^\/+|\/+$/g, ""));
  return `${API_URL}/${cleaned.join("/")}`;
}

/**
 * Live websocket URL for room state stream.
 * Converts http/https base to ws/wss automatically.
 */
export function liveWebsocketUrl(): string {
  const wsBase = API_URL.replace(/^http/, "ws");
  return `${wsBase}/api/v1/live-ws`;
}

/**
 * Resolve image URL that may be absolute or relative.
 * If it already starts with http(s) we return it unchanged; otherwise join with API_URL.
 */
export function resolveImageUrl(url: string): string {
  if (!url) return url;
  if (/^https?:\/\//i.test(url)) return url;
  return apiPath(url);
}

/**
 * Given a snapshot entity image object (with a url field), produce the usable URL.
 */
export function snapshotImageUrl(image: { url: string }): string {
  return resolveImageUrl(image.url);
}

/**
 * Allow overriding config at runtime (e.g. before hydration) without bundler replace.
 */
export function setApiUrl(newUrl: string): void {
  (globalThis as any).__AT2_API_URL__ = newUrl.replace(/\/+$/, "");
}

export type Config = {
  API_URL: string;
  liveWebsocketUrl: () => string;
  apiPath: (...segments: string[]) => string;
  resolveImageUrl: (url: string) => string;
  snapshotImageUrl: (image: { url: string }) => string;
  setApiUrl: (newUrl: string) => void;
};

export const config: Config = {
  API_URL,
  liveWebsocketUrl,
  apiPath,
  resolveImageUrl,
  snapshotImageUrl,
  setApiUrl,
};
