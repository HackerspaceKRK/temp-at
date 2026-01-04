import { type FC, useMemo, memo } from "react";
import type { UsageHeatmapDataPoint } from "../schema";
import { useTranslation } from "react-i18next";
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import moment from "moment";
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
                        "rounded-sm transition-colors cursor-help border border-transparent hover:border-border"
                    )}
                    style={{ backgroundColor: getColor(dp.manHours) }}
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
                </div>
            </TooltipContent>
        </Tooltip>
    );
});

const Legend = memo(({ maxManHours, getColor }: { maxManHours: number; getColor: (val: number) => string }) => {
    const steps = 10;
    return (
        <div className="flex flex-col gap-1 pr-4 items-end select-none">
            <span className="text-[10px] text-muted-foreground font-medium mb-1">{maxManHours.toFixed(1)}h</span>
            <div className="flex flex-col-reverse gap-0.5 h-[100px] w-2 border border-border/50 rounded-full overflow-hidden">
                {Array.from({ length: steps }).map((_, i) => (
                    <div
                        key={i}
                        className="flex-1 w-full"
                        style={{ backgroundColor: getColor((i / steps) * maxManHours) }}
                    />
                ))}
            </div>
            <span className="text-[10px] text-muted-foreground font-medium mt-1">0h</span>
        </div>
    );
});

const HeatmapChartComponent: FC<HeatmapChartProps> = ({ data, resolution }) => {
    const { t, i18n } = useTranslation();

    const maxManHours = useMemo(() => {
        return Math.max(...data.map((d) => d.manHours), 0.1);
    }, [data]);

    const getColor = useMemo(() => (manHours: number) => {
        if (manHours === 0) return "transparent";
        const ratio = Math.min(manHours / maxManHours, 1);
        const opacity = 0.1 + ratio * 0.9;
        return `oklch(from var(--primary) l c h / ${opacity})`;
    }, [maxManHours]);

    const formatDate = useMemo(() => (timestamp: number) => {
        return moment(timestamp).locale(i18n.language).format(resolution === "hour" ? "ddd, MMM D, HH:mm" : "ddd, MMM D");
    }, [i18n.language, resolution]);

    const dailyConfig = useMemo(() => {
        if (resolution !== "day" || data.length === 0) return null;

        const days = [t("Mon"), t("Tue"), t("Wed"), t("Thu"), t("Fri"), t("Sat"), t("Sun")];
        const firstDataMoment = moment(data[0].startsAt).startOf('isoWeek');
        const lastDataMoment = moment(data[data.length - 1].startsAt);

        const numCols = lastDataMoment.diff(firstDataMoment, 'weeks') + 1;
        const grid: (UsageHeatmapDataPoint | null)[][] = Array.from({ length: 7 }, () => Array.from({ length: numCols }, () => null));

        data.forEach(dp => {
            const m = moment(dp.startsAt);
            const col = m.diff(firstDataMoment, 'weeks');
            const row = (m.isoWeekday() - 1); // 0=Mon, 6=Sun
            if (col >= 0 && col < numCols) grid[row][col] = dp;
        });

        return { days, numCols, grid };
    }, [data, resolution, t]);

    const hourlyConfig = useMemo(() => {
        if (resolution !== "hour" || data.length === 0) return null;

        const hours = Array.from({ length: 24 }).map((_, i) => `${i}:00`);
        const firstDataMoment = moment(data[0].startsAt).startOf('day');
        const lastDataMoment = moment(data[data.length - 1].startsAt);

        const numDays = lastDataMoment.diff(firstDataMoment, 'days') + 1;
        const grid: (UsageHeatmapDataPoint | null)[][] = Array.from({ length: 24 }, () => Array.from({ length: numDays }, () => null));

        data.forEach(dp => {
            const m = moment(dp.startsAt);
            const dayIdx = m.diff(firstDataMoment, 'days');
            const hourIdx = m.hour();
            if (dayIdx >= 0 && dayIdx < numDays) grid[hourIdx][dayIdx] = dp;
        });

        const dayHeaders = Array.from({ length: numDays }).map((_, i) => {
            return moment(firstDataMoment).add(i, 'days').locale(i18n.language).format("ddd D");
        });

        return { hours, numDays, grid, dayHeaders, firstDataMoment };
    }, [data, resolution, i18n.language]);

    if (data.length === 0) return <div className="p-8 text-center text-muted-foreground">{t("No data available")}</div>;

    return (
        <TooltipProvider>
            <div className="flex items-start">
                <Legend maxManHours={maxManHours} getColor={getColor} />
                <div className="flex flex-col gap-2 overflow-x-auto pb-4 flex-1">
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
                                                    sizeClass="w-6 h-4"
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
            </div>
        </TooltipProvider>
    );
};

export const HeatmapChart = memo(HeatmapChartComponent);
