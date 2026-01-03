import { useState, useRef, useEffect, type FunctionComponent, type JSX } from "react";
import { useTranslation } from "react-i18next";
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
  placeholder?: JSX.Element;
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
 * - Choose largest image.
 */
function pickDefaultSrc(images: SnapshotImage[]): string | null {
  if (images.length === 0) return null;
  const sorted = [...images]
    .filter((i) => i.width)
    .sort((a, b) => a.width - b.width);
  if (sorted.length === 0) return null;
  const mid = sorted[sorted.length - 1];
  return resolveImageUrl(mid.url);
}

export const CameraSnapshot: FunctionComponent<CameraSnapshotProps> = ({
  images,
  alt,
  className,
  fit = "cover",
  priority = false,
  loading,
  sizes = "100vw",
  placeholder,
  lockAspectRatio = true,
}) => {
  const { t } = useTranslation();
  const displayAlt = alt || t("Camera snapshot");
  const [isEnlarged, setIsEnlarged] = useState(false);
  // isVisible triggers the CSS transitions (opacity/transform)
  const [isVisible, setIsVisible] = useState(false);

  // Transform state for zoom/pan
  const [transform, setTransform] = useState({ x: 0, y: 0, scale: 1 });
  const [isDragging, setIsDragging] = useState(false);

  const imgRef = useRef<HTMLImageElement>(null);
  const placeholderRef = useRef<HTMLDivElement>(null);
  const enlargedImgRef = useRef<HTMLImageElement>(null);
  const dragStartRef = useRef({ x: 0, y: 0, initialX: 0, initialY: 0 });
  const pinchStartRef = useRef<{
    distance: number;
    initialScale: number;
    initialX: number;
    initialY: number;
    initialGx: number; // Initial centroid X (client coords)
    initialGy: number; // Initial centroid Y (client coords)
  } | null>(null);
  const lastTapRef = useRef<number>(0);
  const clickTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const validImages = Array.isArray(images)
    ? images.filter((i) => !!i?.url)
    : [];
  const hasImages = validImages.length > 0;

  const src = hasImages ? pickDefaultSrc(validImages) : null;
  const srcSet = hasImages ? buildSrcSet(validImages) : undefined;

  // Aspect ratio lock using padding-top trick
  let aspectWrapperStyle: React.CSSProperties = {};
  if (lockAspectRatio && hasImages) {
    const first = validImages[0];
    if (first?.width && first?.height) {
      const ratio = (first.height / first.width) * 100;
      aspectWrapperStyle = { position: "relative", paddingTop: `${ratio}%` };
    }
  }

  // Calculate target dimensions for the enlarged image
  const getTargetRect = () => {
    const targetWidth = window.innerWidth * 0.9;
    const first = validImages[0];
    const aspectRatio = first?.height && first?.width ? first.height / first.width : 1;
    const targetHeight = targetWidth * aspectRatio;

    return {
      width: targetWidth,
      height: targetHeight,
      left: (window.innerWidth - targetWidth) / 2,
      top: (window.innerHeight - targetHeight) / 2,
    };
  };

  const handleImageClick = () => {
    if (!hasImages || !imgRef.current) return;

    // Reset transform
    setTransform({ x: 0, y: 0, scale: 1 });

    const rect = imgRef.current.getBoundingClientRect();

    // 1. Mount the portal
    setIsEnlarged(true);

    // 2. Start animation frame loop
    requestAnimationFrame(() => {
      if (!enlargedImgRef.current) return;

      // Set initial position to match original image exactly
      const el = enlargedImgRef.current;
      el.style.left = `${rect.left}px`;
      el.style.top = `${rect.top}px`;
      el.style.width = `${rect.width}px`;
      el.style.height = `${rect.height}px`;
      el.style.transition = 'none'; // No transition for initial set

      // Force layout
      el.getBoundingClientRect();

      // 3. Trigger transition to target
      requestAnimationFrame(() => {
        const target = getTargetRect();
        el.style.transition = 'all 300ms cubic-bezier(0.4, 0, 0.2, 1)';
        el.style.left = `${target.left}px`;
        el.style.top = `${target.top}px`;
        el.style.width = `${target.width}px`;
        el.style.height = `${target.height}px`;

        // Build-in the backdrop fade
        setIsVisible(true);

        // 4. Remove transition after animation so dragging is 1:1 (not rubbery)
        setTimeout(() => {
          if (enlargedImgRef.current) {
            enlargedImgRef.current.style.transition = '';
          }
        }, 300);
      });
    });
  };

  const closeViewer = () => {
    if (!enlargedImgRef.current || !placeholderRef.current) {
      setIsEnlarged(false);
      setIsVisible(false);
      return;
    }

    const rect = placeholderRef.current.getBoundingClientRect();
    const el = enlargedImgRef.current;

    // Reset transform to animate back cleanly
    setTransform({ x: 0, y: 0, scale: 1 });
    setIsVisible(false); // Start fading out backdrop

    // Animate back to original position
    el.style.transition = 'all 300ms cubic-bezier(0.4, 0, 0.2, 1)';
    el.style.left = `${rect.left}px`;
    el.style.top = `${rect.top}px`;
    el.style.width = `${rect.width}px`;
    el.style.height = `${rect.height}px`;

    // Wait for animation to finish
    setTimeout(() => {
      setIsEnlarged(false);
    }, 300);
  };

  // Window resize handling
  useEffect(() => {
    if (!isEnlarged) return;

    const handleResize = () => {
      if (!enlargedImgRef.current) return;
      const target = getTargetRect();
      // Only update if not currently being dragged/transitioned
      if (!enlargedImgRef.current.style.transition) {
        enlargedImgRef.current.style.left = `${target.left}px`;
        enlargedImgRef.current.style.top = `${target.top}px`;
        enlargedImgRef.current.style.width = `${target.width}px`;
        enlargedImgRef.current.style.height = `${target.height}px`;
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [isEnlarged, validImages]);

  // Handle escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isEnlarged) {
        closeViewer();
      }
    };

    if (isEnlarged) {
      document.addEventListener("keydown", handleEscape);
      return () => document.removeEventListener("keydown", handleEscape);
    }
  }, [isEnlarged]);

  // --- Interaction Handlers ---

  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    e.stopPropagation();

    if (!enlargedImgRef.current) return;

    const scaleSensitivity = 0.001;
    const delta = -e.deltaY * scaleSensitivity;

    setTransform(prev => {
      const newScale = Math.max(1, Math.min(prev.scale + delta, 5));

      // If simply resetting to 1, clear translation
      if (newScale === 1 && prev.scale === 1) return prev;
      if (newScale === 1) return { x: 0, y: 0, scale: 1 };

      // Calculate zoom-to-cursor
      // The standard formula for zoom-towards-point:
      // x' = x * (newScale / oldScale) + (cx - cx * (newScale / oldScale))
      // where cx is cursor pos relative to origin... 
      // My transform origin is center.

      // Correct logic for Translate(x,y) Scale(s) Origin(Center):
      // x_new = x_old + (Mx - Cx) * (1 - newScale / oldScale) ? No.

      // Let's use the proven formula derived earlier:
      // x_new = x_prev * ratio + (Mx - Cx) * (1 - ratio)
      // where ratio = newScale / oldScale

      const rect = enlargedImgRef.current!.getBoundingClientRect();
      // Since we are using fixed positioning, getBoundingClientRect returns true screen coords.
      // However, our transform x,y applies to the element's base position.
      // Base center Cx, Cy is calculated from getTargetRect (or styled left/top + width/height / 2)

      // Let's rely on the fact that visually center is at rect.left + rect.width/2
      // Cx (base center) = visually_center - prev.x

      const visualCx = rect.left + rect.width / 2;
      const visualCy = rect.top + rect.height / 2;

      // Base center (where image would be if x=0, y=0) - APPROXIMATELY?
      // Actually, rect is affected by scale. 
      // visualCx = BaseCx + prev.x.

      const baseCx = visualCx - prev.x;
      const baseCy = visualCy - prev.y;

      const mx = e.clientX;
      const my = e.clientY;

      const ratio = newScale / prev.scale;

      // x_new = x_prev * ratio + (mx - baseCx) * (1 - ratio)
      // Wait, (mx - baseCx) is the offset of mouse from base center.
      // Before zoom: VisualPos = Base + x_prev. Mouse is at Mx.
      // After zoom: VisualPos = Base + x_new.
      // Relative mouse pos on image (unscaled) = (Mx - VisualPos_old) / oldScale
      // We want (Mx - VisualPos_new) / newScale = same.
      // (Mx - (Base + x_new)) / newScale = (Mx - (Base + x_prev)) / oldScale
      // Mx - Base - x_new = (Mx - Base - x_prev) * ratio
      // x_new = Mx - Base - (Mx - Base - x_prev) * ratio
      // x_new = (Mx - Base) * (1 - ratio) + x_prev * ratio

      const newX = (mx - baseCx) * (1 - ratio) + prev.x * ratio;
      const newY = (my - baseCy) * (1 - ratio) + prev.y * ratio;

      return { x: newX, y: newY, scale: newScale };
    });
  };

  // Helper to get distance between two touch points
  const getTouchDistance = (t1: React.Touch, t2: React.Touch) => {
    return Math.sqrt(
      Math.pow(t2.clientX - t1.clientX, 2) + Math.pow(t2.clientY - t1.clientY, 2)
    );
  };

  // Helper to get centroid of two touch points
  const getTouchCentroid = (t1: React.Touch, t2: React.Touch) => {
    return {
      x: (t1.clientX + t2.clientX) / 2,
      y: (t1.clientY + t2.clientY) / 2
    };
  };

  // Helper for toggle zoom
  const toggleZoom = () => {
    setTransform(prev => ({
      ...prev,
      scale: prev.scale > 1 ? 1 : 2.5,
      x: 0,
      y: 0
    }));
  };

  const handleTouchStart = (e: React.TouchEvent) => {
    e.preventDefault();

    if (e.touches.length === 2) {
      // Pinch start
      const dist = getTouchDistance(e.touches[0], e.touches[1]);
      const centroid = getTouchCentroid(e.touches[0], e.touches[1]);

      pinchStartRef.current = {
        distance: dist,
        initialScale: transform.scale,
        initialX: transform.x,
        initialY: transform.y,
        initialGx: centroid.x,
        initialGy: centroid.y
      };
      setIsDragging(true);
    } else if (e.touches.length === 1) {
      // Pan start
      const t = e.touches[0];
      dragStartRef.current = {
        x: t.clientX,
        y: t.clientY,
        initialX: transform.x,
        initialY: transform.y
      };
      if (transform.scale > 1) {
        setIsDragging(true);
      }
    }
  };

  const handleTouchMove = (e: React.TouchEvent) => {
    e.preventDefault();

    if (e.touches.length === 2 && pinchStartRef.current) {
      // Pinch move
      const dist = getTouchDistance(e.touches[0], e.touches[1]);
      const centroid = getTouchCentroid(e.touches[0], e.touches[1]);
      const start = pinchStartRef.current;

      const scaleFactor = dist / start.distance;
      const newScale = Math.max(1, Math.min(start.initialScale * scaleFactor, 5));

      // Calculate new translation to support zoom-to-centroid + pan-with-pinch
      // Math:
      // Point P on image (relative to Base Center, unscaled) was under Initial Centroid (Gx0, Gy0)
      // P_unscaled = (Gx0 - BaseCx - startX) / startScale
      // We want P to now be under Current Centroid (Gx, Gy)
      // Gx = BaseCx + newX + P_unscaled * newScale
      // newX = Gx - BaseCx - P_unscaled * newScale
      //      = Gx - BaseCx - ((Gx0 - BaseCx - startX) / startScale) * newScale
      //      = Gx - BaseCx - (Gx0 - BaseCx - startX) * (newScale / startScale)

      if (!enlargedImgRef.current) return;
      // visualCx = rect.left + rect.width / 2 ... but rect changes during render loop? 
      // We need BaseCx. Since x shifts visual center, visualCx - x = BaseCx.
      // We can use current transform.x to get BaseCx, OR recalculate from window logic.
      // Safer to recalculate from logic if possible, OR just trust state if sync.
      // Inside event handler, visual state might lag or lead?
      // Actually 'rect' is reliable for 'current visual state' DOM. 'transform' state is React state.
      // Using 'start' values is safer.
      // But we need BaseCx.
      // BaseCx is constant-ish (unless window resizes).
      // Let's compute BaseCx from knowns at start?
      // At Start: visualCx_start = BaseCx + startX.
      // Actually BaseCx is not stored.
      // But we can approximate BaseCx using the fact we know how getTargetRect works (centered in window).
      // BaseCx = window.innerWidth / 2  (Assuming images are centered by getTargetRect)
      // Yes! getTargetRect: left = (winW - w)/2 -> center = left + w/2 = winW/2.
      // Unless window resized during pinch (unlikely).

      const baseCx = window.innerWidth / 2;
      const baseCy = window.innerHeight / 2;

      const newX = centroid.x - baseCx - (start.initialGx - baseCx - start.initialX) * (newScale / start.initialScale);
      const newY = centroid.y - baseCy - (start.initialGy - baseCy - start.initialY) * (newScale / start.initialScale);

      setTransform(() => ({
        scale: newScale,
        x: newX,
        y: newY
      }));

    } else if (e.touches.length === 1 && isDragging && transform.scale > 1) {
      // Pan move (only if zoomed in)
      const t = e.touches[0];
      const dx = t.clientX - dragStartRef.current.x;
      const dy = t.clientY - dragStartRef.current.y;

      setTransform(prev => ({
        ...prev,
        x: dragStartRef.current.initialX + dx,
        y: dragStartRef.current.initialY + dy
      }));
    }
  };

  const handleTouchEnd = (e: React.TouchEvent) => {
    if (e.touches.length === 0) {
      setIsDragging(false);
      pinchStartRef.current = null;

      // Custom Tap / Double Tap Logic for Touch
      // Only if we weren't just actively dragging/pinching significantly?
      // For simplicity: check if this touch was short and didn't move much
      // But since we are tracking 'isDragging' for movement state, let's use a simpler heuristic:
      // If we processed movement, we probably shouldn't click.
      // However, 'isDragging' is true immediately on touch start. We need to check if movement actually happened?

      // Actually, let's just use standard Double Tap detection logic based on time
      if (isDragging && transform.scale > 1) {
        // If we were dragging/zoomed in, we might still want to register a tap if movement was minimal?
        // Let's assume touchEnd without active pinch/movement is a tap.
      }

      // Simple double tap detection
      const now = Date.now();
      const DOUBLE_TAP_DELAY = 300;

      if (now - lastTapRef.current < DOUBLE_TAP_DELAY) {
        // Double tap!
        if (clickTimeoutRef.current) {
          clearTimeout(clickTimeoutRef.current);
          clickTimeoutRef.current = null;
        }
        toggleZoom();
        lastTapRef.current = 0; // Prevent triple tap triggering
      } else {
        // Single tap (maybe)
        lastTapRef.current = now;

        // We only want to trigger 'close' if it wasn't a drag/pinch
        // To be safe, we can check if dragStart is close to touchEnd position (requires tracking end pos or move)
        // Alternatively, use a "hasMoved" flag.

        // For now, let's delay the single click action
        // Note: This logic assumes 'e.preventDefault' prevented native click.
        // So we MUST handle single tap "Click" here manually.

        // We schedule the close, but we need to cancel it if a double tap comes
        // We also shouldn't close if we just panned. 
        // Let's assume if isDragging was true and scale > 1, we panned, so don't close.
        // BUT isDragging is set true on start.

        // Refined logic: only close if transform.scale is 1 (can't pan anyway) 
        // OR if we track movement.

        // Let's trust that if the user lifts their finger and scale is 1, they likely tapped.
        if (transform.scale === 1) {
          clickTimeoutRef.current = setTimeout(() => {
            closeViewer();
            clickTimeoutRef.current = null;
          }, DOUBLE_TAP_DELAY);
        }
      }
    }
  };

  // Mouse handlers (Desktop)
  const handlePointerDown = (e: React.PointerEvent) => {
    // Only handle Mouse here, Touch is handled by onTouchX
    if (e.pointerType === 'touch') return;

    e.preventDefault();
    if (transform.scale <= 1) return;

    setIsDragging(true);
    (e.target as Element).setPointerCapture(e.pointerId);

    dragStartRef.current = {
      x: e.clientX,
      y: e.clientY,
      initialX: transform.x,
      initialY: transform.y
    };
  };

  const handlePointerMove = (e: React.PointerEvent) => {
    if (e.pointerType === 'touch') return;
    if (!isDragging) return;
    e.preventDefault();

    const dx = e.clientX - dragStartRef.current.x;
    const dy = e.clientY - dragStartRef.current.y;

    setTransform(prev => ({
      ...prev,
      x: dragStartRef.current.initialX + dx,
      y: dragStartRef.current.initialY + dy
    }));
  };

  const handlePointerUp = (e: React.PointerEvent) => {
    if (e.pointerType === 'touch') return;
    setIsDragging(false);
    if (e.target instanceof Element && e.target.hasPointerCapture(e.pointerId)) {
      e.target.releasePointerCapture(e.pointerId);
    }
  };

  // Mouse click handlers
  const handleEnlargedClick = (e: React.MouseEvent) => {
    // Only care about mouse clicks, touch taps handled in onTouchEnd
    // Pointer event type check not available on MouseEvent easily, 
    // but we preventedDefault on touch, so this shouldn't fire for touch?
    // Actually React synthesizes clicks.

    // To be safe, if we trust preventDefault on touchStart stops synthetic clicks:
    if ((e.nativeEvent as any).pointerType === 'touch') return;

    if (isDragging) return;

    // Mouse drift check
    if (Math.abs(e.clientX - dragStartRef.current.x) > 5 ||
      Math.abs(e.clientY - dragStartRef.current.y) > 5) {
      if (transform.scale > 1) return;
    }

    if (clickTimeoutRef.current) {
      clearTimeout(clickTimeoutRef.current);
      clickTimeoutRef.current = null;
    }

    clickTimeoutRef.current = setTimeout(() => {
      closeViewer();
      clickTimeoutRef.current = null;
    }, 250);
  };

  const handleEnlargedDoubleClick = (e: React.MouseEvent) => {
    if ((e.nativeEvent as any).pointerType === 'touch') return;
    if (clickTimeoutRef.current) {
      clearTimeout(clickTimeoutRef.current);
      clickTimeoutRef.current = null;
    }
    toggleZoom();
  };

  const imgNode =
    hasImages && src ? (
      <img
        ref={imgRef}
        src={src}
        srcSet={srcSet}
        sizes={sizes}
        alt={displayAlt}
        loading={loading ?? (priority ? "eager" : "lazy")}
        className={
          "absolute inset-0 w-full h-full " +
          (fit === "cover" ? "object-cover" : "object-contain") +
          (isEnlarged ? " invisible" : " cursor-pointer")
        }
        decoding="async"
        onClick={handleImageClick}
      />
    ) : (
      <div
        className={
          "flex items-center justify-center text-neutral-400 text-xs bg-neutral-900 " +
          (lockAspectRatio ? "absolute inset-0" : "w-full h-full")
        }
      >
        {placeholder ?? <span>{t("No image")}</span>}
      </div>
    );

  const containerClass = (lockAspectRatio && !hasImages)
    ? "relative overflow-hidden bg-neutral-900 aspect-video"
    : ((lockAspectRatio && hasImages)
      ? "overflow-hidden bg-neutral-900"
      : "relative overflow-hidden bg-neutral-900 w-full");

  return (
    <>
      <div
        className={`${containerClass} ${className || ''}`}
        style={aspectWrapperStyle}
      >
        {isEnlarged && (
          // Transparent placeholder as requested, showing card bg behind
          <div
            ref={placeholderRef}
            className="absolute inset-0 opacity-0"
          />
        )}
        {imgNode}
      </div>

      {/* Enlarged view portal */}
      {isEnlarged && hasImages && src && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          {/* Backdrop */}
          <div
            className={
              "fixed inset-0 bg-black/80 transition-opacity duration-300 " +
              (isVisible ? "opacity-100" : "opacity-0")
            }
            onClick={closeViewer}
          />

          {/* Enlarged image */}
          <img
            ref={enlargedImgRef}
            src={src}
            srcSet={srcSet}
            alt={displayAlt}
            className="fixed object-contain origin-center select-none"
            style={{
              transform: `translate(${transform.x}px, ${transform.y}px) scale(${transform.scale})`,
              cursor: transform.scale > 1 ? (isDragging ? "grabbing" : "grab") : "zoom-in",
              touchAction: "none" // Critical for handling touch events manually
            }}
            // Touch Handlers (Mobile)
            onTouchStart={handleTouchStart}
            onTouchMove={handleTouchMove}
            onTouchEnd={handleTouchEnd}

            // Mouse Handlers (Desktop)
            onWheel={handleWheel}
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerUp}
            onClick={handleEnlargedClick}
            onDoubleClick={handleEnlargedDoubleClick}
          />
        </div>
      )}
    </>
  );
};

export default CameraSnapshot;
