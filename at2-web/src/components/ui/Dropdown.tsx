import { useState, useRef, useEffect, useCallback } from "preact/hooks";
import type { FunctionalComponent, ComponentChildren } from "preact";
import { createPortal } from "preact/compat";

/**
 * Where to place the dropdown panel relative to the trigger.
 */
export type DropdownPlacement =
  | "bottom-start"
  | "bottom-end"
  | "top-start"
  | "top-end";

export interface DropdownProps {
  /**
   * Controlled open state. If provided, component becomes controlled.
   */
  isOpen?: boolean;
  /**
   * Uncontrolled initial state (ignored if isOpen provided).
   */
  defaultOpen?: boolean;
  /**
   * Called whenever open state changes.
   */
  onOpenChange?: (open: boolean) => void;
  /**
   * Trigger element or render function receiving helpers.
   */
  trigger:
    | ComponentChildren
    | ((ctx: {
        open: boolean;
        toggle: () => void;
        ref: (el: HTMLElement | null) => void;
      }) => ComponentChildren);
  /**
   * Dropdown panel content.
   */
  children: ComponentChildren;
  /**
   * Panel placement.
   */
  placement?: DropdownPlacement;
  /**
   * Additional classes for the panel surface.
   */
  panelClassName?: string;
  /**
   * Additional classes for the root wrapper.
   */
  className?: string;
  /**
   * Use a portal (positioned by viewport absolute coords).
   */
  portal?: boolean;
  /**
   * Close when clicking outside.
   */
  closeOnOutsideClick?: boolean;
  /**
   * Close when pressing Escape.
   */
  closeOnEsc?: boolean;
  /**
   * Arrow pointer (small triangle) visibility.
   */
  showArrow?: boolean;
  /**
   * Optional id for panel (aria-controls).
   */
  panelId?: string;
  /**
   * Accessible label for icon-only triggers.
   */
  triggerAriaLabel?: string;
  /**
   * Auto focus first focusable element in panel on open.
   */
  autoFocus?: boolean;
  /**
   * Disable panel closing when clicking inside (default false).
   */
  keepOpenOnPanelClick?: boolean;
}

/**
 * Utility: join class names without external dependencies.
 */
function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

/**
 * Find and focus the first focusable element inside a container.
 */
function focusFirst(container: HTMLElement | null) {
  if (!container) return;
  const el = container.querySelector<HTMLElement>(
    [
      "button:not([disabled])",
      "a[href]",
      "input:not([disabled])",
      "select:not([disabled])",
      "textarea:not([disabled])",
      "[tabindex]:not([tabindex='-1'])",
    ].join(","),
  );
  el?.focus();
}

export const Dropdown: FunctionalComponent<DropdownProps> = ({
  isOpen,
  defaultOpen = false,
  onOpenChange,
  trigger,
  children,
  placement = "bottom-start",
  panelClassName,
  className,
  portal = false,
  closeOnOutsideClick = true,
  closeOnEsc = true,
  showArrow = true,
  panelId,
  triggerAriaLabel,
  autoFocus = true,
  keepOpenOnPanelClick = false,
}) => {
  const [internalOpen, setInternalOpen] = useState(defaultOpen);
  const open = isOpen !== undefined ? isOpen : internalOpen;

  const triggerRef = useRef<HTMLElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const coordsRef = useRef<{
    top: number;
    left: number;
    width: number;
    height: number;
  } | null>(null);

  const setOpen = useCallback(
    (next: boolean) => {
      if (isOpen === undefined) {
        setInternalOpen(next);
      }
      onOpenChange?.(next);
    },
    [isOpen, onOpenChange],
  );

  const toggle = useCallback(() => {
    const next = !open;
    // Pre-measure immediately before first portal render to avoid initial flash at (0,0)
    if (next && portal && triggerRef.current) {
      const r = triggerRef.current.getBoundingClientRect();
      coordsRef.current = {
        top: r.top + window.scrollY,
        left: r.left + window.scrollX,
        width: r.width,
        height: r.height,
      };
    }
    setOpen(next);
  }, [open, portal, setOpen]);

  // Measure trigger for portal positioning
  const measure = useCallback(() => {
    if (!triggerRef.current) return;
    const r = triggerRef.current.getBoundingClientRect();
    coordsRef.current = {
      top: r.top + window.scrollY,
      left: r.left + window.scrollX,
      width: r.width,
      height: r.height,
    };
  }, []);

  useEffect(() => {
    if (open && portal) {
      measure();
      const handle = () => measure();
      window.addEventListener("resize", handle);
      window.addEventListener("scroll", handle, true);
      return () => {
        window.removeEventListener("resize", handle);
        window.removeEventListener("scroll", handle, true);
      };
    }
  }, [open, portal, measure]);

  // Outside click & ESC
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!closeOnOutsideClick) return;
      const target = e.target as Node;
      if (
        panelRef.current &&
        !panelRef.current.contains(target) &&
        triggerRef.current &&
        !triggerRef.current.contains(target)
      ) {
        setOpen(false);
      }
      if (
        !keepOpenOnPanelClick &&
        panelRef.current &&
        panelRef.current.contains(target) &&
        target instanceof HTMLElement &&
        target.dataset.dismiss === "dropdown"
      ) {
        setOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (!closeOnEsc) return;
      if (e.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open, closeOnOutsideClick, closeOnEsc, setOpen, keepOpenOnPanelClick]);

  // Focus management
  useEffect(() => {
    if (open && autoFocus) {
      focusFirst(panelRef.current);
    }
  }, [open, autoFocus]);

  // Post-mount re-measure for portal to correct any layout shifts after first render
  useEffect(() => {
    if (open && portal) {
      requestAnimationFrame(() => {
        if (triggerRef.current) {
          const r = triggerRef.current.getBoundingClientRect();
          coordsRef.current = {
            top: r.top + window.scrollY,
            left: r.left + window.scrollX,
            width: r.width,
            height: r.height,
          };
          // Force a re-render (only affects uncontrolled mode) so style recalculates
          if (isOpen === undefined) {
            setInternalOpen((o) => o);
          }
        }
      });
    }
  }, [open, portal, isOpen]);

  const panelBase =
    "min-w-[10rem] rounded-lg bg-white dark:bg-neutral-800 border border-neutral-300 dark:border-neutral-700 shadow-lg text-sm";
  const arrowSize = 8;

  // Pre-computed base positioning style for portal mode (used for overflow correction)
  let baseStyle: Record<string, string | number> | undefined;
  if (portal && open && coordsRef.current) {
    const { top, left, width, height } = coordsRef.current;
    switch (placement) {
      case "bottom-start":
        baseStyle = {
          position: "absolute",
          top: top + height + arrowSize,
          left,
        };
        break;
      case "bottom-end":
        baseStyle = {
          position: "absolute",
          top: top + height + arrowSize,
          left: left + width,
          transform: "translateX(-100%)",
        };
        break;
      case "top-start":
        baseStyle = { position: "absolute", top: top - arrowSize, left };
        break;
      case "top-end":
        baseStyle = {
          position: "absolute",
          top: top - arrowSize,
          left: left + width,
          transform: "translateX(-100%)",
        };
        break;
    }
  }

  // Dynamic style for panel (portal or local) including right-edge overflow correction.
  const [panelDynamicStyle, setPanelDynamicStyle] = useState<
    Record<string, string | number> | undefined
  >(undefined);

  // Adjust panel position if it would overflow the right viewport edge.
  useEffect(() => {
    if (!open) {
      setPanelDynamicStyle(undefined);
      return;
    }
    if (!panelRef.current) return;

    // Base positioning style computed earlier (portal only)
    let nextStyle: Record<string, string | number> | undefined = baseStyle
      ? { ...baseStyle }
      : undefined;

    const rect = panelRef.current.getBoundingClientRect();
    const vw = window.innerWidth;
    const overflowRight = rect.right - vw;

    if (overflowRight > 0) {
      // Provide an 8px margin from the right edge.
      const correction = overflowRight + 8;

      if (portal) {
        // Shift absolute left coordinate for portal panels.
        if (nextStyle && typeof nextStyle.left !== "undefined") {
          const leftNum =
            typeof nextStyle.left === "number"
              ? nextStyle.left
              : parseFloat(String(nextStyle.left));
          nextStyle.left = leftNum - correction;
        } else {
          // Fallback: apply transform.
          nextStyle = {
            ...(nextStyle || {}),
            transform: `translateX(-${correction}px)`,
          };
        }
      } else {
        // Local (non-portal): use transform to nudge left.
        nextStyle = {
          ...(nextStyle || {}),
          transform: `translateX(-${correction}px)`,
        };
      }
    }

    setPanelDynamicStyle(nextStyle);
  }, [open, portal, baseStyle]);

  // Recalculate on resize while open.
  useEffect(() => {
    if (!open) return;
    const onResize = () => {
      if (!panelRef.current) return;
      // Force effect re-run by copying style ref (style may be stable).
      setPanelDynamicStyle((prev) => ({ ...(prev || {}) }));
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [open]);

  {
    /* Moved base positioning style computation above dynamic style effect */
  }

  // Local (non-portal) offset classes
  const localOffsetClasses = (() => {
    switch (placement) {
      case "bottom-start":
        return "top-full left-0 mt-2";
      case "bottom-end":
        return "top-full right-0 mt-2";
      case "top-start":
        return "bottom-full left-0 mb-2";
      case "top-end":
        return "bottom-full right-0 mb-2";
      default:
        return "top-full left-0 mt-2";
    }
  })();

  const arrow = showArrow ? (
    <span
      className={cx(
        "absolute w-3 h-3 bg-white dark:bg-neutral-800 border rotate-45",
        // Base border colors
        "border-neutral-300 dark:border-neutral-700",
        // Remove interior borders depending on orientation to prevent leaking into panel.
        placement.startsWith("bottom") &&
          "border-b-transparent border-r-transparent dark:border-b-transparent dark:border-r-transparent",
        placement.startsWith("top") &&
          "border-t-transparent border-l-transparent dark:border-t-transparent dark:border-l-transparent",
      )}
      style={{
        ...(placement.startsWith("bottom") && { top: -6 }),
        ...(placement.startsWith("top") && { bottom: -6 }),
        ...(placement.endsWith("start") && { left: 12 }),
        ...(placement.endsWith("end") && { right: 12 }),
      }}
    />
  ) : null;

  const panel = open ? (
    <div
      ref={panelRef}
      id={panelId}
      role="menu"
      aria-hidden={!open}
      className={cx(panelBase, "relative isolate p-2", panelClassName)}
      style={panelDynamicStyle}
    >
      {arrow}
      {children}
    </div>
  ) : null;

  // Trigger node creation
  const triggerNode =
    typeof trigger === "function" ? (
      trigger({
        open,
        toggle,
        ref: (el) => {
          triggerRef.current = el;
        },
      })
    ) : (
      <button
        ref={(el) => {
          triggerRef.current = el;
        }}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={panelId}
        aria-label={triggerAriaLabel}
        onClick={toggle}
        className="inline-flex items-center gap-1 px-3 py-2 rounded border border-neutral-300 dark:border-neutral-600 bg-white dark:bg-neutral-700 text-neutral-900 dark:text-neutral-100 text-sm hover:bg-neutral-200 dark:hover:bg-neutral-600 transition-colors"
      >
        {trigger}
        <span className="text-xs opacity-70">{open ? "▲" : "▼"}</span>
      </button>
    );

  const wrapper = (
    <div className={cx("relative inline-block", className)}>
      {triggerNode}
      {!portal && (
        <div className={cx("absolute z-50", localOffsetClasses)}>{panel}</div>
      )}
    </div>
  );

  if (portal) {
    return (
      <>
        {wrapper}
        {panel && createPortal(panel, document.body)}
      </>
    );
  }

  return wrapper;
};

export default Dropdown;
