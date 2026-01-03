import { type FC, useMemo, memo } from "react";
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

interface HeatmapCellProps {
    dp: UsageHeatmapDataPoint | null;
    getColor: (manHours: number) => string;
    formatDate: (timestamp: number) => string;
    sizeClass: string;
    t: (key: string) => string;
}

const HeatmapCell = memo(({ dp, getColor, formatDate, sizeClass, t }: HeatmapCellProps) => {
    if (!dp) return <div className={cn(sizeClass, "rounded-sm bg-muted/5")} />;

    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <div
                    className={cn(
                        sizeClass,
                        "rounded-sm transition-colors cursor-help",
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
});

const HeatmapChartComponent: FC<HeatmapChartProps> = ({ data, resolution }) => {
    const { t, i18n } = useTranslation();

    const maxManHours = useMemo(() => {
        return Math.max(...data.map((d) => d.manHours), 0.1);
    }, [data]);

    const getColor = useMemo(() => (manHours: number) => {
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
    }, [maxManHours]);

    const formatDate = useMemo(() => (timestamp: number) => {
        return new Date(timestamp).toLocaleString(i18n.language, {
            weekday: "short",
            month: "short",
            day: "numeric",
            hour: resolution === "hour" ? "numeric" : undefined,
        });
    }, [i18n.language, resolution]);

    const dailyConfig = useMemo(() => {
        if (resolution !== "day" || data.length === 0) return null;
        const days = [t("Mon"), t("Tue"), t("Wed"), t("Thu"), t("Fri"), t("Sat"), t("Sun")];
        const firstDataPoint = data[0];
        const lastDataPoint = data[data.length - 1];
        const firstDate = new Date(firstDataPoint.startsAt);
        const getWeekDay = (d: Date) => (d.getDay() + 6) % 7;
        const firstMonDate = new Date(firstDate);
        firstMonDate.setDate(firstDate.getDate() - getWeekDay(firstDate));
        firstMonDate.setHours(0, 0, 0, 0);
        const firstMonTs = firstMonDate.getTime();
        const lastDate = new Date(lastDataPoint.startsAt);
        const totalDays = Math.ceil((lastDate.getTime() - firstMonTs) / (24 * 60 * 60 * 1000)) + 1;
        const numCols = Math.ceil(totalDays / 7);
        const grid: (UsageHeatmapDataPoint | null)[][] = Array.from({ length: 7 }, () => Array.from({ length: numCols }, () => null));
        data.forEach(dp => {
            const d = new Date(dp.startsAt);
            const col = Math.floor((dp.startsAt - firstMonTs + 3600000) / (7 * 24 * 60 * 60 * 1000));
            const row = getWeekDay(d);
            if (col >= 0 && col < numCols) grid[row][col] = dp;
        });
        return { days, numCols, grid };
    }, [data, resolution, t]);

    const hourlyConfig = useMemo(() => {
        if (resolution !== "hour" || data.length === 0) return null;
        const hours = Array.from({ length: 24 }).map((_, i) => `${i}:00`);
        const firstDataPoint = data[0];
        const lastDataPoint = data[data.length - 1];
        const firstDate = new Date(firstDataPoint.startsAt);
        const dayStartTs = new Date(firstDate).setHours(0, 0, 0, 0);
        const lastDate = new Date(lastDataPoint.startsAt);
        const numDays = Math.ceil((lastDate.getTime() - dayStartTs) / (24 * 60 * 60 * 1000)) + 1;
        const grid: (UsageHeatmapDataPoint | null)[][] = Array.from({ length: 24 }, () => Array.from({ length: numDays }, () => null));
        data.forEach(dp => {
            const date = new Date(dp.startsAt);
            const dayIdx = Math.floor((dp.startsAt - dayStartTs + 3600000) / (24 * 60 * 60 * 1000));
            const hourIdx = date.getHours();
            if (dayIdx >= 0 && dayIdx < numDays) grid[hourIdx][dayIdx] = dp;
        });
        const dayHeaders = Array.from({ length: numDays }).map((_, i) => {
            const d = new Date(dayStartTs + i * 24 * 60 * 60 * 1000);
            return d.toLocaleDateString(i18n.language, { weekday: "short", day: "numeric" });
        });
        return { hours, numDays, grid, dayHeaders };
    }, [data, resolution, i18n.language]);

    if (data.length === 0) return <div className="p-8 text-center text-muted-foreground">{t("No data available")}</div>;

    return (
        <TooltipProvider>
            <div className="flex flex-col gap-2 overflow-x-auto pb-4">
                <div className="flex gap-2 min-w-max">
                    {resolution === "day" && dailyConfig && (
                        <>
                            <div className="flex flex-col gap-1 pr-2">
                                {dailyConfig.days.map((d) => (
                                    <div key={d} className="h-4 text-[10px] flex items-center text-muted-foreground font-medium uppercase tracking-tighter">
                                        {d}
                                    </div>
                                ))}
                            </div>
                            <div className="flex gap-1">
                                {Array.from({ length: dailyConfig.numCols }).map((_, c) => (
                                    <div key={c} className="flex flex-col gap-1">
                                        {dailyConfig.grid.map((row, r) => (
                                            <HeatmapCell
                                                key={r}
                                                dp={row[c]}
                                                getColor={getColor}
                                                formatDate={formatDate}
                                                sizeClass="w-4 h-4"
                                                t={t}
                                            />
                                        ))}
                                    </div>
                                ))}
                            </div>
                        </>
                    )}
                    {resolution === "hour" && hourlyConfig && (
                        <>
                            <div className="flex flex-col gap-1 pr-2 pt-5">
                                {hourlyConfig.hours.map((h, i) => (
                                    <div key={h} className="h-4 text-[10px] flex items-center text-muted-foreground font-medium uppercase tracking-tighter">
                                        {i % 4 === 0 ? h : ""}
                                    </div>
                                ))}
                            </div>
                            <div className="flex gap-1">
                                {Array.from({ length: hourlyConfig.numDays }).map((_, d) => (
                                    <div key={d} className="flex flex-col gap-1 items-center">
                                        <div className="text-[10px] text-muted-foreground font-medium mb-1 truncate w-10 text-center">
                                            {hourlyConfig.dayHeaders[d]}
                                        </div>
                                        {Array.from({ length: 24 }).map((_, h) => (
                                            <HeatmapCell
                                                key={h}
                                                dp={hourlyConfig.grid[h][d]}
                                                getColor={getColor}
                                                formatDate={formatDate}
                                                sizeClass="w-10 h-4"
                                                t={t}
                                            />
                                        ))}
                                    </div>
                                ))}
                            </div>
                        </>
                    )}
                </div>
            </div>
        </TooltipProvider>
    );
};

export const HeatmapChart = memo(HeatmapChartComponent);
