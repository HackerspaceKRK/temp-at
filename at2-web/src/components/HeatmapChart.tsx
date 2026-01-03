import { type FC, useMemo } from "react";
import type { UsageHeatmapDataPoint } from "../schema";
import { useTranslation } from "react-i18next";
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "./ui/tooltip";

function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs));
}

interface HeatmapChartProps {
    data: UsageHeatmapDataPoint[];
    resolution: "day" | "hour";
}

export const HeatmapChart: FC<HeatmapChartProps> = ({ data, resolution }) => {
    const { t, i18n } = useTranslation();

    const maxManHours = useMemo(() => {
        return Math.max(...data.map((d) => d.manHours), 0.1);
    }, [data]);

    const getColor = (manHours: number) => {
        if (manHours === 0) return "bg-muted/20";
        const intensity = Math.min(Math.ceil((manHours / maxManHours) * 9), 9); // 1-9
        const opacities = [
            "bg-primary/5",
            "bg-primary/10",
            "bg-primary/20",
            "bg-primary/30",
            "bg-primary/40",
            "bg-primary/55",
            "bg-primary/70",
            "bg-primary/85",
            "bg-primary/100",
        ];
        return opacities[intensity - 1];
    };

    const formatDate = (timestamp: number) => {
        return new Date(timestamp).toLocaleString(i18n.language, {
            weekday: "short",
            month: "short",
            day: "numeric",
            hour: resolution === "hour" ? "numeric" : undefined,
        });
    };

    if (data.length === 0) return <div className="p-8 text-center text-muted-foreground">{t("No data available")}</div>;

    if (resolution === "day") {
        const days = [
            t("Mon"),
            t("Tue"),
            t("Wed"),
            t("Thu"),
            t("Fri"),
            t("Sat"),
            t("Sun"),
        ];

        const firstDataPoint = data[0];
        const firstDate = new Date(firstDataPoint.startsAt);
        const getWeekDay = (d: Date) => (d.getDay() + 6) % 7;

        const startWeekDay = getWeekDay(firstDate);
        const lastDataPoint = data[data.length - 1];
        const totalDays = Math.ceil((lastDataPoint.startsAt - firstDataPoint.startsAt) / (24 * 60 * 60 * 1000)) + 1;
        const numCols = Math.ceil((totalDays + startWeekDay) / 7);

        const grid: (UsageHeatmapDataPoint | null)[][] = Array.from({ length: 7 }, () => []);

        for (let c = 0; c < numCols; c++) {
            for (let r = 0; r < 7; r++) {
                const dayIdx = c * 7 + r - startWeekDay;
                if (dayIdx >= 0 && dayIdx < data.length) {
                    grid[r][c] = data[dayIdx];
                } else {
                    grid[r][c] = null;
                }
            }
        }

        return (
            <TooltipProvider>
                <div className="flex flex-col gap-2 overflow-x-auto pb-4">
                    <div className="flex gap-2 min-w-max">
                        <div className="flex flex-col gap-1 pr-2 pt-6">
                            {days.map((d) => (
                                <div key={d} className="h-4 text-[10px] flex items-center text-muted-foreground font-medium uppercase tracking-tighter">
                                    {d}
                                </div>
                            ))}
                        </div>
                        <div className="flex gap-1">
                            {Array.from({ length: numCols }).map((_, c) => (
                                <div key={c} className="flex flex-col gap-1">
                                    {grid.map((row, r) => {
                                        const dp = row[c];
                                        if (!dp) return <div key={r} className="w-4 h-4 rounded-sm bg-transparent" />;
                                        return (
                                            <Tooltip key={r}>
                                                <TooltipTrigger asChild>
                                                    <div
                                                        className={cn(
                                                            "w-4 h-4 rounded-sm transition-colors cursor-help",
                                                            getColor(dp.manHours)
                                                        )}
                                                    />
                                                </TooltipTrigger>
                                                <TooltipContent>
                                                    <div className="space-y-1">
                                                        <div className="font-bold border-b border-border/50 pb-1 mb-1">
                                                            {formatDate(dp.startsAt)}
                                                        </div>
                                                        <div className="flex justify-between gap-4">
                                                            <span>{t("person-hours")}:</span>
                                                            <span className="font-mono">{dp.manHours.toFixed(2)}</span>
                                                        </div>
                                                        <div className="flex justify-between gap-4">
                                                            <span>{t("Max people")}:</span>
                                                            <span className="font-mono">{dp.maxPeople}</span>
                                                        </div>
                                                        <div className="flex justify-between gap-4">
                                                            <span>{t("Active hours")}:</span>
                                                            <span className="font-mono">{dp.activeHours.toFixed(2)}h</span>
                                                        </div>
                                                    </div>
                                                </TooltipContent>
                                            </Tooltip>
                                        );
                                    })}
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
            </TooltipProvider>
        );
    } else {
        // resolution === "hour"
        const hours = Array.from({ length: 24 }).map((_, i) => `${i}:00`);
        const firstDataPoint = data[0];
        const numDays = Math.ceil(data.length / 24);

        const dayHeaders = Array.from({ length: numDays }).map((_, i) => {
            const d = new Date(firstDataPoint.startsAt + i * 24 * 60 * 60 * 1000);
            return d.toLocaleDateString(i18n.language, { weekday: "short", day: "numeric" });
        });

        return (
            <TooltipProvider>
                <div className="flex flex-col gap-2 overflow-x-auto pb-4">
                    <div className="flex gap-2 min-w-max">
                        <div className="flex flex-col gap-1 pr-2 pt-6">
                            {hours.map((h, i) => (
                                <div key={h} className="h-4 text-[10px] flex items-center text-muted-foreground font-medium uppercase tracking-tighter">
                                    {i % 4 === 0 ? h : ""}
                                </div>
                            ))}
                        </div>
                        <div className="flex gap-1">
                            {Array.from({ length: numDays }).map((_, d) => (
                                <div key={d} className="flex flex-col gap-1 items-center">
                                    <div className="text-[10px] text-muted-foreground font-medium mb-1 truncate w-10 text-center">
                                        {dayHeaders[d]}
                                    </div>
                                    {Array.from({ length: 24 }).map((_, h) => {
                                        const idx = d * 24 + h;
                                        const dp = data[idx];
                                        if (!dp) return <div key={h} className="w-10 h-4 rounded-sm bg-transparent" />;
                                        return (
                                            <Tooltip key={h}>
                                                <TooltipTrigger asChild>
                                                    <div
                                                        className={cn(
                                                            "w-10 h-4 rounded-sm transition-colors cursor-help",
                                                            getColor(dp.manHours)
                                                        )}
                                                    />
                                                </TooltipTrigger>
                                                <TooltipContent>
                                                    <div className="space-y-1">
                                                        <div className="font-bold border-b border-border/50 pb-1 mb-1">
                                                            {formatDate(dp.startsAt)}
                                                        </div>
                                                        <div className="flex justify-between gap-4">
                                                            <span>{t("person-hours")}:</span>
                                                            <span className="font-mono">{dp.manHours.toFixed(2)}</span>
                                                        </div>
                                                        <div className="flex justify-between gap-4">
                                                            <span>{t("Max people")}:</span>
                                                            <span className="font-mono">{dp.maxPeople}</span>
                                                        </div>
                                                        <div className="flex justify-between gap-4">
                                                            <span>{t("Active hours")}:</span>
                                                            <span className="font-mono">{dp.activeHours.toFixed(2)}h</span>
                                                        </div>
                                                    </div>
                                                </TooltipContent>
                                            </Tooltip>
                                        );
                                    })}
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
            </TooltipProvider>
        );
    }
};
