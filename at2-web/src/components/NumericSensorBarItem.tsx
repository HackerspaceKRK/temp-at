import { type FC } from "react";
import { cn } from "@/lib/utils";

/**
 * Numeric sensor item.
 * Supports single or combined values (e.g. CO / Gas).
 * If isAlarm is true, renders with red pulsating style.
 */
export const NumericSensorBarItem: FC<{
    icon: FC<any>;
    value?: number | null;
    unit?: string;
    title: string;
    precision?: number;
    // Optional secondary value (e.g. for Gas when combined with CO)
    secondaryValue?: number | null;
    secondaryUnit?: string;
    secondaryPrecision?: number;
    isAlarm?: boolean;
}> = ({
    icon: Icon,
    value,
    unit,
    title,
    precision = 1,
    secondaryValue,
    secondaryUnit,
    secondaryPrecision = 1,
    isAlarm,
}) => {
        const hasValue = value !== null && value !== undefined && !isNaN(value);
        const hasSecondary =
            secondaryValue !== null &&
            secondaryValue !== undefined &&
            !isNaN(secondaryValue);

        if (!hasValue && !hasSecondary) return null;

        return (
            <div
                className={cn(
                    "flex items-center gap-1 transition-all",
                    isAlarm ? "text-red-500 font-bold animate-alarm" : ""
                )}
                title={title}
            >
                <Icon className="w-4 h-4" />
                <span>
                    {hasValue && (
                        <>
                            {value!.toFixed(precision)}
                            <span className="ml-[1px]">{unit}</span>
                        </>
                    )}
                    {hasValue && hasSecondary && <span> / </span>}
                    {hasSecondary && (
                        <>
                            {secondaryValue!.toFixed(secondaryPrecision)}
                            <span className="ml-[1px]">{secondaryUnit}</span>
                        </>
                    )}
                </span>
            </div>
        );
    };
