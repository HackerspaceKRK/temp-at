import type { FunctionalComponent, ComponentChildren } from "preact";
import { forwardRef } from "preact/compat";

/**
 * Button variants define both background + text color for light/dark themes.
 * Extend as needed.
 */
type ButtonVariant =
  | "primary"
  | "secondary"
  | "danger"
  | "neutral"
  | "ghost"
  | "outline";

type ButtonSize = "sm" | "md" | "lg";

interface BaseProps {
  children?: ComponentChildren;
  /**
   * Optional icon component (lucide-preact or any component returning SVG).
   */
  icon?: FunctionalComponent<any>;
  /**
   * When true and only icon is present, you MUST provide an accessible label.
   */
  "aria-label"?: string;
  /**
   * Badge numeric value. If > 99 displays "99+".
   */
  badgeCount?: number;
  /**
   * Disabled state.
   */
  disabled?: boolean;
  /**
   * Visual variant.
   */
  variant?: ButtonVariant;
  /**
   * Visual size.
   */
  size?: ButtonSize;
  /**
   * Expands to full width container.
   */
  fullWidth?: boolean;
  /**
   * Icon position when both icon and text present.
   */
  iconPosition?: "left" | "right";
  /**
   * Optional additional Tailwind classes.
   */
  className?: string;
  /**
   * Click handler.
   */
  onClick?: (evt: MouseEvent) => void;
  /**
   * Whether to render a native button or an anchor (useful for links).
   */
  as?: "button" | "a";
  /**
   * Href (only if as="a").
   */
  href?: string;
  /**
   * Target (only if as="a").
   */
  target?: string;
  /**
   * Rel (only if as="a").
   */
  rel?: string;
}

/**
 * Internal utility: join class names without depending on external libs.
 */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

/**
 * Variant styling map.
 * All variants should include dark mode overrides via Tailwind dark: prefix.
 */
const variantClasses: Record<ButtonVariant, string> = {
  primary:
    "bg-blue-600 hover:bg-blue-700 text-white dark:bg-blue-500 dark:hover:bg-blue-600 focus-visible:ring-blue-500",
  secondary:
    "bg-neutral-200 hover:bg-neutral-300 text-neutral-900 dark:bg-neutral-700 dark:hover:bg-neutral-600 dark:text-neutral-100 focus-visible:ring-neutral-500",
  danger:
    "bg-red-600 hover:bg-red-700 text-white dark:bg-red-500 dark:hover:bg-red-600 focus-visible:ring-red-500",
  neutral:
    "bg-white hover:bg-neutral-100 text-neutral-900 border border-neutral-300 dark:bg-neutral-800 dark:hover:bg-neutral-700 dark:text-neutral-100 dark:border-neutral-600 focus-visible:ring-neutral-400",
  ghost:
    "bg-transparent hover:bg-neutral-200 text-neutral-800 dark:text-neutral-100 dark:hover:bg-neutral-700 focus-visible:ring-neutral-500",
  outline:
    "bg-transparent border border-neutral-300 text-neutral-800 hover:bg-neutral-100 dark:border-neutral-600 dark:text-neutral-100 dark:hover:bg-neutral-700 focus-visible:ring-neutral-500",
};

/**
 * Size styling map.
 */
const sizeClasses: Record<ButtonSize, string> = {
  sm: "text-xs px-2 py-1 gap-1",
  md: "text-sm px-3 py-2 gap-2",
  lg: "text-base px-4 py-2.5 gap-3",
};

/**
 * Badge styling.
 */
const badgeBase =
  "absolute -top-1 -right-1 min-w-[1.1rem] h-[1.1rem] px-1 rounded-full bg-red-600 text-white text-[0.65rem] font-semibold flex items-center justify-center shadow-sm";

interface ButtonProps extends BaseProps {}

/**
 * Accessible Button component.
 *
 * - If you use icon-only button, supply aria-label.
 * - Badge positioned relative to icon container.
 * - Supports dark mode variants (Tailwind dark: prefix).
 * - Provides keyboard focus styles (focus-visible).
 *
 * Example:
 * <Button icon={Plus} variant="primary">Add Item</Button>
 * <Button icon={Bell} badgeCount={3} aria-label="Notifications" />
 */
export const Button = forwardRef<
  HTMLButtonElement | HTMLAnchorElement,
  ButtonProps
>((props, ref) => {
  const {
    children,
    icon: Icon,
    badgeCount,
    disabled,
    variant = "primary",
    size = "md",
    fullWidth,
    iconPosition = "left",
    className,
    onClick,
    as = "button",
    href,
    target,
    rel,
    ...rest
  } = props;

  const onlyIcon = Icon && !children;

  if (onlyIcon && !rest["aria-label"]) {
    // eslint-disable-next-line no-console
    console.warn(
      "Button: icon-only usage detected without aria-label. Please provide an accessible label.",
    );
  }

  // Content assembly
  const iconEl = Icon ? (
    <span className="relative inline-flex">
      <Icon className={cx(size === "lg" ? "w-5 h-5" : "w-4 h-4")} />
      {typeof badgeCount === "number" && badgeCount > 0 && (
        <span className={badgeBase}>
          {badgeCount > 99 ? "99+" : badgeCount}
        </span>
      )}
    </span>
  ) : null;

  const content =
    iconEl && children ? (
      iconPosition === "left" ? (
        <>
          {iconEl}
          <span className="truncate">{children}</span>
        </>
      ) : (
        <>
          <span className="truncate">{children}</span>
          {iconEl}
        </>
      )
    ) : iconEl ? (
      iconEl
    ) : (
      <span className="truncate">{children}</span>
    );

  const baseClasses =
    "inline-flex items-center justify-center font-medium rounded transition-colors select-none focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed relative";

  const finalClassName = cx(
    baseClasses,
    variantClasses[variant],
    sizeClasses[size],
    fullWidth && "w-full",
    className,
  );

  if (as === "a") {
    return (
      <a
        ref={ref as any}
        href={href}
        target={target}
        rel={rel}
        onClick={(e) => {
          if (disabled) {
            e.preventDefault();
            return;
          }
          onClick?.(e as any);
        }}
        aria-disabled={disabled || undefined}
        className={finalClassName}
        {...rest}
      >
        {content}
      </a>
    );
  }

  return (
    <button
      ref={ref as any}
      type="button"
      disabled={disabled}
      onClick={(e) => {
        if (disabled) return;
        onClick?.(e as any);
      }}
      className={finalClassName}
      {...rest}
    >
      {content}
    </button>
  );
});

Button.displayName = "Button";

export default Button;
