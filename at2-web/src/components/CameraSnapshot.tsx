import type { FunctionalComponent } from "preact";
import type { SnapshotImage } from "../schema";
import { resolveImageUrl } from "../config";

/**
 * CameraSnapshot component
 *
 * Renders a responsive <img> with a generated srcset from provided snapshot images.
 * Falls back gracefully when no images are available.
 *
 * Usage:
 * <CameraSnapshot images={entity.state?.images ?? []} alt="Room camera" />
 */
export interface CameraSnapshotProps {
  /**
   * Array of snapshot image variants (different resolutions/media types).
   */
  images: SnapshotImage[] | null | undefined;
  /**
   * Accessible alternative text.
   */
  alt?: string;
  /**
   * Optional extra class names for the outer wrapper.
   */
  className?: string;
  /**
   * Tailwind object-fit choice. Defaults to cover.
   */
  fit?: "cover" | "contain";
  /**
   * If true, loading is set to eager (high priority).
   */
  priority?: boolean;
  /**
   * HTML loading attribute (overrides priority if provided).
   */
  loading?: "eager" | "lazy";
  /**
   * Sizes attribute for responsive selection. Provide if container layout differs.
   * Defaults to "100vw" so the browser picks the closest width.
   */
  sizes?: string;
  /**
   * Optional placeholder node rendered when there are no images.
   */
  placeholder?: preact.ComponentChildren;
  /**
   * Whether to constrain aspect ratio using the first image dimensions.
   * If true and first image exists, creates a padded box to prevent layout shift.
   */
  lockAspectRatio?: boolean;
}

/**
 * Build a srcset string from the provided images.
 * - Deduplicates by width.
 * - Prefers http(s) fully qualified or resolves relative URLs via resolveImageUrl.
 */
function buildSrcSet(images: SnapshotImage[]): string {
  const byWidth = new Map<number, string>();
  for (const img of images) {
    if (!img?.url || !img?.width) continue;
    if (!byWidth.has(img.width)) {
      byWidth.set(img.width, resolveImageUrl(img.url));
    }
  }
  const parts = [...byWidth.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([w, url]) => `${url} ${w}w`);
  return parts.join(", ");
}

/**
 * Pick a default src:
 * - Prefer the median (middle) width for a balanced choice.
 * - Fallback to first item if only one.
 */
function pickDefaultSrc(images: SnapshotImage[]): string | null {
  if (images.length === 0) return null;
  const sorted = [...images]
    .filter((i) => i.width)
    .sort((a, b) => a.width - b.width);
  if (sorted.length === 0) return null;
  const mid = sorted[Math.floor(sorted.length / 2)];
  return resolveImageUrl(mid.url);
}

export const CameraSnapshot: FunctionalComponent<CameraSnapshotProps> = ({
  images,
  alt = "Camera snapshot",
  className,
  fit = "cover",
  priority = false,
  loading,
  sizes = "100vw",
  placeholder,
  lockAspectRatio = true,
}) => {
  const validImages = Array.isArray(images)
    ? images.filter((i) => !!i?.url)
    : [];
  const hasImages = validImages.length > 0;

  const src = hasImages ? pickDefaultSrc(validImages) : null;
  const srcSet = hasImages ? buildSrcSet(validImages) : undefined;

  // Aspect ratio lock using padding-top trick
  let aspectWrapperStyle: preact.JSX.CSSProperties | undefined;
  if (lockAspectRatio && hasImages) {
    const first = validImages[0];
    if (first?.width && first?.height) {
      const ratio = (first.height / first.width) * 100;
      aspectWrapperStyle = { position: "relative", paddingTop: `${ratio}%` };
    }
  }

  const imgNode =
    hasImages && src ? (
      <img
        src={src}
        srcSet={srcSet}
        sizes={sizes}
        alt={alt}
        loading={loading ?? (priority ? "eager" : "lazy")}
        className={
          "absolute inset-0 w-full h-full " +
          (fit === "cover" ? "object-cover" : "object-contain")
        }
        decoding="async"
      />
    ) : (
      <div
        className={
          "flex items-center justify-center text-neutral-400 text-xs bg-neutral-900 " +
          (lockAspectRatio ? "absolute inset-0" : "w-full h-full")
        }
      >
        {placeholder ?? <span>Brak obrazu</span>}
      </div>
    );

  return lockAspectRatio && hasImages ? (
    <div
      className={
        "overflow-hidden bg-neutral-900 " + (className ? className : "")
      }
      style={aspectWrapperStyle}
    >
      {imgNode}
    </div>
  ) : (
    <div
      className={
        "relative overflow-hidden bg-neutral-900 " +
        (lockAspectRatio && !hasImages ? "aspect-video" : "w-full") +
        (className ? " " + className : "")
      }
    >
      {imgNode}
    </div>
  );
};

export default CameraSnapshot;
