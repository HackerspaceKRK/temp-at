import { type FC, useMemo, memo, useRef, useLayoutEffect, useState, useEffect } from "react";
import type { UsageHeatmapDataPoint } from "../schema";
import { useTranslation } from "react-i18next";
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { format, startOfISOWeek, differenceInCalendarWeeks, getISODay, startOfDay, differenceInCalendarDays, getHours, getMinutes, addDays } from "date-fns";
import { enUS, pl } from "date-fns/locale";
import { setChartCursor, useChartCursor } from "../hooks/useChartCursor";
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
        <Tooltip disableHoverableContent>
            <TooltipTrigger asChild>
                <div
                    className={cn(
                        sizeClass,
                        "rounded-sm transition-colors cursor-help hover:border-border",
                        dp.manHours === 0 ? "border-2 border-border/25" : "border border-transparent"
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

const HOUR_MS = 60 * 60 * 1000;
const ROW_STRIDE = 20;
const CELL_HEIGHT = 16;
const DAILY_COL_STRIDE = 28;

const HourlyCursorLine = memo(({ dayStart }: { dayStart: number }) => {
    const cursor = useChartCursor();
    if (cursor === null || cursor < dayStart || cursor >= dayStart + 24 * HOUR_MS) return null;
    const hours = (cursor - dayStart) / HOUR_MS;
    const top = Math.floor(hours) * ROW_STRIDE + (hours % 1) * CELL_HEIGHT;
    return (
        <div className="absolute left-0 right-0 h-0.5 bg-muted-foreground/70 pointer-events-none z-10" style={{ top }} />
    );
});

const DailyCursorCell = memo(({ weekStart, numCols }: { weekStart: Date; numCols: number }) => {
    const cursor = useChartCursor();
    if (cursor === null) return null;
    const idx = differenceInCalendarDays(cursor, weekStart);
    const col = Math.floor(idx / 7);
    if (idx < 0 || col >= numCols) return null;
    return (
        <div
            className="absolute w-6 h-4 rounded-sm ring-2 ring-muted-foreground/70 pointer-events-none z-10"
            style={{ left: col * DAILY_COL_STRIDE, top: (idx % 7) * ROW_STRIDE }}
        />
    );
});

const clearCursor = () => setChartCursor(null);

const HeatmapChartComponent: FC<HeatmapChartProps> = ({ data, resolution }) => {
    const { t, i18n } = useTranslation();
    const scrollContainerRef = useRef<HTMLDivElement>(null);

    const [now, setNow] = useState(() => new Date());
    useEffect(() => {
        const interval = setInterval(() => setNow(new Date()), 60_000);
        return () => clearInterval(interval);
    }, []);

    useLayoutEffect(() => {
        if (scrollContainerRef.current) {
            scrollContainerRef.current.scrollLeft = scrollContainerRef.current.scrollWidth;
        }
    }, [data, resolution]);

    const maxManHours = useMemo(() => {
        return Math.max(...data.map((d) => d.manHours), 0.1);
    }, [data]);

    const getColor = useMemo(() => (manHours: number) => {
        if (manHours === 0) return "transparent";
        // Limit max man hours to 5 for hourly resolution to increase contrast
        const max = resolution == "hour" ? Math.min(maxManHours, 5) : maxManHours; 
        const ratio = Math.min(manHours / max, 1);
        const opacity = 0.1 + ratio * 0.9;
        return `oklch(from var(--primary) l c h / ${opacity})`;
    }, [maxManHours]);

    const getDateFnsLocale = (lang: string) => {
        if (lang.startsWith("pl")) return pl;
        return enUS;
    };

    const formatDate = useMemo(() => (timestamp: number) => {
        return format(timestamp, resolution === "hour" ? "EEE, MMM d, HH:mm" : "EEE, MMM d", {
            locale: getDateFnsLocale(i18n.language),
        });
    }, [i18n.language, resolution]);

    const dailyConfig = useMemo(() => {
        if (resolution !== "day" || data.length === 0) return null;

        const days = [t("Mon"), t("Tue"), t("Wed"), t("Thu"), t("Fri"), t("Sat"), t("Sun")];
        const firstDataMoment = startOfISOWeek(data[0].startsAt);
        const lastDataMoment = data[data.length - 1].startsAt;

        const numCols = differenceInCalendarWeeks(lastDataMoment, firstDataMoment, { weekStartsOn: 1 }) + 1;
        const grid: (UsageHeatmapDataPoint | null)[][] = Array.from({ length: 7 }, () => Array.from({ length: numCols }, () => null));

        data.forEach(dp => {
            const m = dp.startsAt;
            const col = differenceInCalendarWeeks(m, firstDataMoment, { weekStartsOn: 1 });
            const row = (getISODay(m) - 1); // 0=Mon, 6=Sun
            if (col >= 0 && col < numCols) grid[row][col] = dp;
        });

        return { days, numCols, grid, weekStart: firstDataMoment };
    }, [data, resolution, t]);

    const hourlyConfig = useMemo(() => {
        if (resolution !== "hour" || data.length === 0) return null;

        const hours = Array.from({ length: 24 }).map((_, i) => `${i}:00`);
        const firstDataMoment = startOfDay(data[0].startsAt);
        const lastDataMoment = data[data.length - 1].startsAt;

        const numDays = differenceInCalendarDays(lastDataMoment, firstDataMoment) + 1;
        const grid: (UsageHeatmapDataPoint | null)[][] = Array.from({ length: 24 }, () => Array.from({ length: numDays }, () => null));

        data.forEach(dp => {
            const m = dp.startsAt;
            const dayIdx = differenceInCalendarDays(m, firstDataMoment);
            const hourIdx = getHours(m);
            if (dayIdx >= 0 && dayIdx < numDays) grid[hourIdx][dayIdx] = dp;
        });

        const dayHeaders = Array.from({ length: numDays }).map((_, i) => {
            return format(addDays(firstDataMoment, i), "EEE d", {
                locale: getDateFnsLocale(i18n.language),
            });
        });

        return { hours, numDays, grid, dayHeaders, firstDataMoment };
    }, [data, resolution, i18n.language]);

    // cell height h-4=16px, gap-1=4px → stride 20px per row
    const todayLine = useMemo(() => {
        if (resolution !== "hour" || !hourlyConfig) return null;
        const todayIdx = differenceInCalendarDays(startOfDay(now), hourlyConfig.firstDataMoment);
        if (todayIdx < 0 || todayIdx >= hourlyConfig.numDays) return null;
        const top = getHours(now) * 20 + (getMinutes(now) / 60) * 16;
        return { todayIdx, top };
    }, [now, resolution, hourlyConfig]);

    if (data.length === 0) return <div className="p-8 text-center text-muted-foreground">{t("No data available")}</div>;

    return (
        <TooltipProvider>
            <div className="flex items-start">
                <Legend maxManHours={maxManHours} getColor={getColor} />
                <div ref={scrollContainerRef} className="flex flex-col gap-2 overflow-x-auto pb-4 flex-1">
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
                                <div
                                    className="relative flex gap-1"
                                    onMouseLeave={clearCursor}
                                    onMouseMove={(e) => {
                                        const rect = e.currentTarget.getBoundingClientRect();
                                        const col = Math.floor((e.clientX - rect.left) / DAILY_COL_STRIDE);
                                        const row = Math.min(Math.floor((e.clientY - rect.top) / ROW_STRIDE), 6);
                                        setChartCursor(addDays(dailyConfig.weekStart, col * 7 + row).getTime() + 12 * HOUR_MS);
                                    }}
                                >
                                    <DailyCursorCell weekStart={dailyConfig.weekStart} numCols={dailyConfig.numCols} />
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
                                            <div
                                                className="relative flex flex-col gap-1"
                                                onMouseLeave={clearCursor}
                                                onMouseMove={(e) => {
                                                    const y = e.clientY - e.currentTarget.getBoundingClientRect().top;
                                                    const hour = Math.min(Math.floor(y / ROW_STRIDE), 23);
                                                    const frac = Math.min((y - hour * ROW_STRIDE) / CELL_HEIGHT, 1);
                                                    const dayStart = addDays(hourlyConfig.firstDataMoment, d).getTime();
                                                    setChartCursor(dayStart + (hour + frac) * HOUR_MS);
                                                }}
                                            >
                                                <HourlyCursorLine dayStart={addDays(hourlyConfig.firstDataMoment, d).getTime()} />
                                                {todayLine && d === todayLine.todayIdx && (
                                                    <div
                                                        className="absolute left-0 right-0 flex items-center pointer-events-none z-10"
                                                        style={{ top: todayLine.top }}
                                                    >
                                                        <div className="flex-1 h-0.5 bg-destructive/50" />
                                                        <div className="w-2 h-2 rounded-full bg-destructive/50 shrink-0 -mr-1" />
                                                    </div>
                                                )}
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
