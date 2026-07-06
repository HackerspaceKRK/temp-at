import { useEffect, useRef, useState, type RefObject } from "react";
import { printerStreamUrl } from "../config";

// Wire protocol of /api/v1/printer-stream (see stream_manager.go):
// binary frames with a 1-byte type prefix.
const MSG_STATUS = 0x00; // JSON {"codec": "...", "error": "..."}
const MSG_INIT = 0x01; // fMP4 init segment
const MSG_MEDIA = 0x02; // fMP4 media segment

export type MsePlayerStatus = "connecting" | "playing" | "error" | "unsupported";

declare global {
  interface Window {
    ManagedMediaSource?: typeof MediaSource;
  }
}

function mediaSourceClass(): typeof MediaSource | undefined {
  // ManagedMediaSource is the only MSE flavour available on iPhone (iOS 17+).
  return window.ManagedMediaSource ?? window.MediaSource;
}

export function msePlaybackSupported(): boolean {
  return mediaSourceClass() !== undefined;
}

/**
 * useMsePlayer streams a printer camera over the authenticated WebSocket and
 * feeds it into a <video> element through Media Source Extensions.
 */
export function useMsePlayer(
  printerId: string,
  videoRef: RefObject<HTMLVideoElement | null>,
): { status: MsePlayerStatus; error: string | null } {
  const [status, setStatus] = useState<MsePlayerStatus>("connecting");
  const [error, setError] = useState<string | null>(null);
  // Everything lives in a ref: the effect tears the whole pipeline down itself.
  const stateRef = useRef<{
    ws: WebSocket | null;
    ms: MediaSource | null;
    sb: SourceBuffer | null;
    objectUrl: string | null;
    codec: string | null;
    initSegment: Uint8Array | null;
    queue: Uint8Array[];
    sourceOpen: boolean;
    closed: boolean;
    reconnectDelay: number;
    reconnectTimer: ReturnType<typeof setTimeout> | null;
  } | null>(null);

  useEffect(() => {
    const video = videoRef.current;
    const MS = mediaSourceClass();
    if (!video) return;
    if (!MS) {
      setStatus("unsupported");
      return;
    }

    const s = {
      ws: null as WebSocket | null,
      ms: null as MediaSource | null,
      sb: null as SourceBuffer | null,
      objectUrl: null as string | null,
      codec: null as string | null,
      initSegment: null as Uint8Array | null,
      queue: [] as Uint8Array[],
      sourceOpen: false,
      closed: false,
      reconnectDelay: 2000,
      reconnectTimer: null as ReturnType<typeof setTimeout> | null,
    };
    stateRef.current = s;

    const ms = new MS();
    s.ms = ms;
    if (window.ManagedMediaSource && MS === window.ManagedMediaSource) {
      // Required for ManagedMediaSource playback on iOS.
      video.disableRemotePlayback = true;
    }
    s.objectUrl = URL.createObjectURL(ms);
    video.src = s.objectUrl;

    const pump = () => {
      if (!s.sb || s.sb.updating || s.queue.length === 0) return;
      const chunk = s.queue.shift()!;
      try {
        s.sb.appendBuffer(chunk as BufferSource);
      } catch (err) {
        // QuotaExceeded or detached buffer: drop buffered data and resync.
        console.error("appendBuffer failed", err);
        s.queue = [];
      }
    };

    const ensureSourceBuffer = () => {
      if (!s.sourceOpen || !s.codec || !s.ms || !s.initSegment) return;
      if (s.sb) return;
      try {
        const sb = s.ms.addSourceBuffer(`video/mp4; codecs="${s.codec}"`);
        // Sequence mode: segments are appended back-to-back regardless of
        // their timestamps, which absorbs server-side reconnects/drops.
        sb.mode = "sequence";
        sb.addEventListener("updateend", () => {
          trimBuffer();
          pump();
        });
        s.sb = sb;
        s.queue.unshift(s.initSegment);
        s.initSegment = null;
        pump();
      } catch (err) {
        console.error("addSourceBuffer failed", err);
        setStatus("error");
        setError(String(err));
      }
    };

    const resetSourceBuffer = () => {
      if (s.sb && s.ms) {
        try {
          s.sb.abort();
          s.ms.removeSourceBuffer(s.sb);
        } catch {
          // SourceBuffer may already be detached.
        }
      }
      s.sb = null;
      s.queue = [];
    };

    const trimBuffer = () => {
      const sb = s.sb;
      if (!sb || sb.updating) return;
      try {
        const buffered = sb.buffered;
        if (buffered.length === 0) return;
        const start = buffered.start(0);
        const end = buffered.end(buffered.length - 1);
        // Keep the live buffer bounded (~30 s).
        if (end - start > 30) {
          sb.remove(start, end - 10);
        }
        // Chase the live edge if we fell behind (tab was hidden etc.).
        if (video.currentTime > 0 && end - video.currentTime > 3) {
          video.currentTime = end - 0.5;
        }
      } catch {
        // buffered can throw while the SourceBuffer is being removed.
      }
    };

    ms.addEventListener("sourceopen", () => {
      s.sourceOpen = true;
      ensureSourceBuffer();
    });

    const connect = () => {
      if (s.closed) return;
      const ws = new WebSocket(printerStreamUrl(printerId));
      ws.binaryType = "arraybuffer";
      s.ws = ws;

      ws.onmessage = (ev: MessageEvent) => {
        if (!(ev.data instanceof ArrayBuffer) || ev.data.byteLength === 0) return;
        const data = new Uint8Array(ev.data);
        const payload = data.subarray(1);
        switch (data[0]) {
          case MSG_STATUS: {
            try {
              const info = JSON.parse(new TextDecoder().decode(payload));
              if (info.codec && info.codec !== s.codec) {
                s.codec = info.codec;
              }
              if (info.error) {
                setError(info.error);
              } else {
                setError(null);
              }
            } catch {
              // Malformed status frame: ignore.
            }
            break;
          }
          case MSG_INIT: {
            // New parameter sets: rebuild the SourceBuffer.
            resetSourceBuffer();
            s.initSegment = new Uint8Array(ev.data, 1).slice();
            ensureSourceBuffer();
            break;
          }
          case MSG_MEDIA: {
            if (!s.sb) return;
            s.queue.push(new Uint8Array(ev.data, 1).slice());
            pump();
            setStatus("playing");
            s.reconnectDelay = 2000;
            if (video.paused) {
              video.play().catch(() => {
                // Autoplay may require a user gesture; the play button provides one.
              });
            }
            break;
          }
        }
      };

      ws.onclose = () => {
        if (s.closed) return;
        setStatus("connecting");
        s.reconnectTimer = setTimeout(connect, s.reconnectDelay);
        s.reconnectDelay = Math.min(s.reconnectDelay * 2, 15000);
      };
      ws.onerror = () => {
        ws.close();
      };
    };

    setStatus("connecting");
    connect();

    return () => {
      s.closed = true;
      if (s.reconnectTimer) clearTimeout(s.reconnectTimer);
      s.ws?.close();
      resetSourceBuffer();
      if (s.ms && s.ms.readyState === "open") {
        try {
          s.ms.endOfStream();
        } catch {
          // Best effort.
        }
      }
      if (s.objectUrl) URL.revokeObjectURL(s.objectUrl);
      video.removeAttribute("src");
      video.load();
      stateRef.current = null;
    };
  }, [printerId, videoRef]);

  return { status, error };
}
