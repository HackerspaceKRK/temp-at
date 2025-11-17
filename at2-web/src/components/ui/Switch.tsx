import { useState, useCallback, useEffect, useRef } from "preact/hooks";
import { forwardRef } from "preact/compat";
import type { ComponentChildren } from "preact";

/**
 * Switch (toggle) component
 * - Supports controlled (checked) and uncontrolled (defaultChecked) usage
 * - Accessible: role="switch", aria-checked, keyboard (Space/Enter)
 * - Sizes: sm | md | lg
 * - Dark mode via Tailwind dark: variants
 * - Optional external label and inline on/off labels
 */

type SwitchSize = "sm" | "md" | "lg";

export interface SwitchProps {
  checked?: boolean;
  defaultChecked?: boolean;
  onChange?: (checked: boolean, event: Event) => void;
  disabled?: boolean;
  size?: SwitchSize;
  id?: string;
  name?: string;
  value?: string;
  className?: string;
  label?: ComponentChildren; // External label rendered to right
  onLabel?: ComponentChildren; // Inline label when ON (inside track)
  offLabel?: ComponentChildren; // Inline label when OFF (inside track)
  ariaLabel?: string; // Accessible label if no visual label text
}

/**
 * Utility: join class names without dependency on external libraries.
 */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

const sizeConfig: Record<
  SwitchSize,
  {
    track: string;
    knob: string;
    translate: string; // knob translate when ON
    font: string;
    inlineLabel: string;
  }
> = {
  sm: {
    track: "w-10 h-5",
    knob: "w-4 h-4",
    translate: "translate-x-[19px]",
    font: "text-xs",
    inlineLabel: "px-1",
  },
  md: {
    track: "w-12 h-6",
    knob: "w-5 h-5",
    translate: "translate-x-[23px]",
    font: "text-sm",
    inlineLabel: "px-1.5",
  },
  lg: {
    track: "w-16 h-8",
    knob: "w-7 h-7",
    translate: "translate-x-[31px]",
    font: "text-base",
    inlineLabel: "px-2",
  },
};

export const Switch = forwardRef<HTMLButtonElement, SwitchProps>(
  (
    {
      checked,
      defaultChecked,
      onChange,
      disabled = false,
      size = "md" as SwitchSize,
      id,
      name,
      value = "on",
      className,
      label,
      onLabel,
      offLabel,
      ariaLabel,
    }: SwitchProps,
    ref: any,
  ) => {
    const isControlled = typeof checked === "boolean";
    const [internalChecked, setInternalChecked] = useState<boolean>(
      defaultChecked ?? false,
    );
    const currentChecked = isControlled
      ? (checked as boolean)
      : internalChecked;

    const buttonRef = useRef<HTMLButtonElement | null>(null);
    // Forward ref merge
    useEffect(() => {
      if (!ref) return;
      if (typeof ref === "function") {
        ref(buttonRef.current);
      } else {
        (ref as any).current = buttonRef.current;
      }
    }, [ref]);

    const toggle = useCallback(
      (event: Event) => {
        if (disabled) return;
        const next = !currentChecked;
        if (!isControlled) {
          setInternalChecked(next);
        }
        onChange?.(next, event);
      },
      [disabled, currentChecked, isControlled, onChange],
    );

    const onKeyDown = (e: KeyboardEvent) => {
      if (disabled) return;
      if (e.key === " " || e.key === "Enter") {
        e.preventDefault();
        toggle(e);
      }
    };

    const { track, knob, translate, font, inlineLabel } = sizeConfig[size];

    const trackBase =
      "relative rounded-full flex items-center transition-colors select-none border";
    const knobBase =
      "absolute left-px top-px rounded-full bg-white dark:bg-neutral-200 shadow-sm transition-transform";

    const trackClasses = cx(
      trackBase,
      track,
      currentChecked
        ? "bg-blue-600 dark:bg-blue-500 border-blue-600 dark:border-blue-500"
        : "bg-neutral-300 dark:bg-neutral-700 border-neutral-300 dark:border-neutral-600",
      disabled && "opacity-50 cursor-not-allowed",
    );

    const knobClasses = cx(knobBase, knob, currentChecked && translate);

    const inlineLabelClasses = cx(
      "absolute inset-0 flex items-center justify-center pointer-events-none text-white dark:text-neutral-100 font-medium",
      font,
      inlineLabel,
      currentChecked ? "opacity-90" : "opacity-70",
    );

    return (
      <div
        className={cx(
          "inline-flex items-center gap-2",
          disabled && "cursor-not-allowed",
          className,
        )}
      >
        <button
          ref={buttonRef}
          // Using type button to prevent form submission.
          type="button"
          id={id}
          role="switch"
          aria-checked={currentChecked}
          aria-label={ariaLabel}
          aria-disabled={disabled || undefined}
          disabled={disabled}
          onClick={(e) => toggle(e as any)}
          onKeyDown={(e) => onKeyDown(e as any)}
          className={cx(
            "focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 rounded-full",
            "transition-shadow",
            !disabled && "cursor-pointer",
            disabled && "pointer-events-none",
          )}
        >
          <div className={trackClasses}>
            {onLabel && currentChecked && (
              <div className={inlineLabelClasses}>{onLabel}</div>
            )}
            {offLabel && !currentChecked && (
              <div className={inlineLabelClasses}>{offLabel}</div>
            )}
            <div className={knobClasses} />
          </div>
        </button>
        {/* Hidden input for form integration if name provided */}
        {name && (
          <input
            type="hidden"
            name={name}
            value={currentChecked ? value : ""}
            disabled={disabled}
          />
        )}
        {label && (
          <label
            htmlFor={id}
            className={cx(
              "select-none",
              "text-neutral-800 dark:text-neutral-100",
              disabled && "opacity-50",
              font,
            )}
            onClick={(e) => {
              // Provide click toggling if label clicked and associated
              if (!id) return;
              e.preventDefault();
              if (disabled) return;
              toggle(e as any);
            }}
          >
            {label}
          </label>
        )}
      </div>
    );
  },
);

Switch.displayName = "Switch";

export default Switch;

/* Usage Examples:
<Switch defaultChecked label="Notifications" />
<Switch checked={value} onChange={setValue} size="lg" onLabel="On" offLabel="Off" />
<Switch iconOnly ariaLabel="Enable feature" />
*/
