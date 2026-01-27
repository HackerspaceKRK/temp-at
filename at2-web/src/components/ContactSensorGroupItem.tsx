import { type FC } from "react";
import { Grid2X2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocale } from "../locale";
import type { ContactEntity } from "../schema";
import { cn } from "@/lib/utils";
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ui/tooltip";

/**
 * Contact sensor group item.
 * Aggregates state of all contact sensors in the room.
 */
export const ContactSensorGroupItem: FC<{
    sensors: ContactEntity[];
}> = ({ sensors }) => {
    const { getName } = useLocale();
    const { t } = useTranslation();

    if (!sensors || sensors.length === 0) return null;

    const openSensors = sensors.filter((s) => s.state === false);
    const unknownSensors = sensors.filter((s) => s.state === null);
    const allClosed = openSensors.length === 0 && unknownSensors.length === 0;

    // Status text and style
    let statusText = "OK";
    let isDanger = false;

    if (unknownSensors.length > 0) {
        statusText = "?";
    } else if (!allClosed) {
        statusText = t("{{count}} open", { count: openSensors.length });
        isDanger = true;
    }

    if (openSensors.length > 0) {
        isDanger = true;
    }

    return (
        <TooltipProvider>
            <Tooltip>
                <TooltipTrigger asChild openOnClick>
                    <div
                        className={cn(
                            "flex items-center gap-1 cursor-pointer",
                            isDanger ? "text-red-500 font-bold" : ""
                        )}
                    >
                        <Grid2X2 className="w-4 h-4" />
                        <span>{statusText}</span>
                    </div>
                </TooltipTrigger>
                <TooltipContent side="bottom">
                    <div className="flex flex-col gap-1">
                        <p className="font-semibold text-xs mb-1">{t("Contact sensors")}</p>
                        {sensors.map((s) => (
                            <div key={s.id} className="flex justify-between gap-4 text-xs">
                                <span>{getName(s.localized_name, s.id)}:</span>
                                <span
                                    className={cn(
                                        "font-bold",
                                        s.state === false ? "text-red-500" : ""
                                    )}
                                >
                                    {s.state === true
                                        ? t("Closed")
                                        : s.state === false
                                            ? t("Open")
                                            : t("Unknown")}
                                </span>
                            </div>
                        ))}
                    </div>
                </TooltipContent>
            </Tooltip>
        </TooltipProvider>
    );
};
