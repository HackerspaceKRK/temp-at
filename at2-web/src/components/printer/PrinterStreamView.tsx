import { useRef, useState, type FC } from "react";
import { Loader2, Play, Square, VideoOff } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { PrinterEntity } from "../../schema";
import { useAuth } from "../../AuthContext";
import { Button } from "../ui/button";
import { useMsePlayer, msePlaybackSupported } from "@/lib/useMsePlayer";
import CameraSnapshot from "../CameraSnapshot";

// PrinterLiveStream plays the live fMP4 camera stream via MSE.
const PrinterLiveStream: FC<{ printerId: string; onStop: () => void }> = ({
  printerId,
  onStop,
}) => {
  const { t } = useTranslation();
  const videoRef = useRef<HTMLVideoElement>(null);
  const { status, error } = useMsePlayer(printerId, videoRef);

  const enlargeStream = () => {
    const video = videoRef.current;
    if (!video) return;
    if (typeof video.requestFullscreen === "function") {
      void video.requestFullscreen().catch(() => {
        // Ignore browser fullscreen rejections (gesture/policy).
      });
      return;
    }
    const webkitVideo = video as HTMLVideoElement & {
      webkitEnterFullscreen?: () => void;
    };
    if (typeof webkitVideo.webkitEnterFullscreen === "function") {
      webkitVideo.webkitEnterFullscreen();
    }
  };

  return (
    <div className="group relative aspect-video overflow-hidden rounded-md bg-neutral-950">
      <video
        ref={videoRef}
        muted
        playsInline
        autoPlay
        onClick={enlargeStream}
        className="h-full w-full cursor-zoom-in object-contain"
      />
      {status !== "playing" && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 text-white">
          {status === "unsupported" ? (
            <p className="px-4 text-center text-sm">
              {t("Live streaming is not supported by this browser.")}
            </p>
          ) : (
            <>
              <Loader2 className="h-8 w-8 animate-spin" />
              <p className="text-sm">{error ? error : t("Loading...")}</p>
            </>
          )}
        </div>
      )}
      <div className="pointer-events-none absolute right-2 top-2 flex items-center gap-2 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
        <Button
          size="sm"
          variant="secondary"
          onClick={onStop}
          className="pointer-events-auto shadow-lg"
        >
          <Square className="h-4 w-4" /> {t("Stop")}
        </Button>
      </div>
    </div>
  );
};

// PrinterStreamView shows the printer camera: a minute-refreshed snapshot by
// default, swapped for the live MSE stream after pressing play (logged-in
// users only; logged-out users get a blurred preview + login prompt).
export const PrinterStreamView: FC<{ entity: PrinterEntity; autoPlay?: boolean }> = ({
  entity,
  autoPlay = false,
}) => {
  const { t } = useTranslation();
  const { user, login } = useAuth();
  const [streaming, setStreaming] = useState(autoPlay);

  const state = entity.state;
  const images = state?.snapshot_images ?? [];
  const lowRes = state?.low_res_preview;
  const canStream = !!state?.has_stream && !!state?.online && msePlaybackSupported();

  if (!state?.has_stream && images.length === 0 && !lowRes) {
    return null;
  }

  // Logged-out users only ever see the tiny blurred preview.
  if (!user) {
    return (
      <div className="group relative aspect-video overflow-hidden rounded-md bg-neutral-900">
        {lowRes && (
          <img
            src={lowRes}
            alt=""
            className="absolute inset-0 h-full w-full scale-125 object-cover blur-xl"
          />
        )}
        <div className="absolute inset-0 flex flex-col items-center justify-center bg-black/40 backdrop-blur-[2px]">
          <p className="mb-3 text-sm font-semibold text-white drop-shadow-md">
            {t("Log in to watch the printer camera")}
          </p>
          <Button size="sm" variant="outline" onClick={login} className="shadow-lg">
            {t("Log In")}
          </Button>
        </div>
      </div>
    );
  }

  if (streaming && canStream) {
    return <PrinterLiveStream printerId={entity.id} onStop={() => setStreaming(false)} />;
  }

  return (
    <div className="group relative aspect-video overflow-hidden rounded-md bg-neutral-950">
      {images.length > 0 ? (
        <CameraSnapshot
          images={images}
          alt={t("Printer camera snapshot")}
          fit="contain"
          className="h-full w-full rounded-md"
          sizes="(min-width: 1024px) 33vw, (min-width: 768px) 50vw, 100vw"
        />
      ) : (
        <div className="flex h-full w-full flex-col items-center justify-center text-muted-foreground">
          <VideoOff className="mb-2 h-10 w-10 opacity-20" />
          <p className="text-sm opacity-50">{t("No snapshot yet")}</p>
        </div>
      )}
      {canStream && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <Button
            size="lg"
            variant="secondary"
            onClick={() => setStreaming(true)}
            className="pointer-events-auto gap-2 shadow-lg transition-transform hover:scale-105"
          >
            <Play className="h-5 w-5" /> {t("Play live stream")}
          </Button>
        </div>
      )}
    </div>
  );
};
