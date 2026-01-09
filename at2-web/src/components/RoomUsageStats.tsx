import { type FC, useEffect, useState, useMemo, memo, useRef } from "react";
import type { RoomState, UsageHeatmapResponse } from "../schema";
import { API_URL } from "../config";
import { HeatmapChart } from "./HeatmapChart";
import { useTranslation } from "react-i18next";
import { useLocale } from "../locale";
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "./ui/dropdown-menu";
import { Button } from "./ui/button";
import { ChevronDown, Loader2 } from "lucide-react";

interface RoomUsageStatsProps {
    rooms: RoomState[];
}

const RoomUsageStatsComponent: FC<RoomUsageStatsProps> = ({ rooms }) => {
    const { t } = useTranslation();
    const { getName } = useLocale();
    const [selectedRoomId, setSelectedRoomId] = useState<string>("");
    const [timeRange, setTimeRange] = useState<"month" | "week">("week");
    const [data, setData] = useState<UsageHeatmapResponse | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [hasBeenInView, setHasBeenInView] = useState(false);
    const [canInitializeObserver, setCanInitializeObserver] = useState(false);
    const containerRef = useRef<HTMLDivElement>(null);

    const resolution = timeRange === "month" ? "day" : "hour";
    const duration = timeRange === "month" ? 60 : 168 * 2;

    useEffect(() => {
        if (canInitializeObserver || rooms.length === 0) return;

        const timeout = setTimeout(() => {
            setCanInitializeObserver(true);
        }, 500);

        return () => clearTimeout(timeout);
    }, [rooms.length, canInitializeObserver]);

    useEffect(() => {
        if (hasBeenInView || !canInitializeObserver) return;

        const observer = new IntersectionObserver((entries) => {
            if (entries[0].isIntersecting) {
                setHasBeenInView(true);
                observer.disconnect();
            }
        });

        if (containerRef.current) {
            observer.observe(containerRef.current);
        }

        return () => observer.disconnect();
    }, [hasBeenInView, canInitializeObserver]);

    useEffect(() => {
        if (!hasBeenInView) return;
        console.log("FETCHING DATA!");
        const fetchData = async () => {
            setIsLoading(true);
            setError(null);
            try {
                const params = new URLSearchParams({
                    resolution,
                    duration: duration.toString(),
                });
                if (selectedRoomId) {
                    params.append("roomId", selectedRoomId);
                }

                const response = await fetch(`${API_URL.replace(/\/$/, "")}/api/v1/stats/usage-heatmap?${params.toString()}`);
                if (!response.ok) {
                    throw new Error(await response.text());
                }
                const json = await response.json();
                setData(json);
            } catch (err: any) {
                setError(err.message);
            } finally {
                setIsLoading(false);
            }
        };

        fetchData();
    }, [selectedRoomId, timeRange, resolution, duration, hasBeenInView]);

    const filteredRooms = useMemo(() => rooms.filter(r =>
        r.entities.some(e => e.representation === "presence" || e.representation === "person")
    ), [rooms]);

    const selectedRoomLabel = useMemo(() => selectedRoomId
        ? getName(rooms.find(r => r.id === selectedRoomId)?.localized_name, selectedRoomId)
        : t("All Rooms"), [selectedRoomId, rooms, getName, t]);

    return (
        <div ref={containerRef} className="col-span-full mt-8">
            <Card>
                <CardHeader className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 space-y-0 pb-7">
                    <CardTitle className="text-xl font-bold">{t("Usage Statistics")}</CardTitle>
                    <div className="flex flex-wrap items-center gap-2 w-full sm:w-auto justify-end">
                        {isLoading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}

                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button variant="outline" size="sm" className="h-8 gap-1">
                                    {selectedRoomLabel}
                                    <ChevronDown className="h-4 w-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuItem onClick={() => setSelectedRoomId("")}>
                                    {t("All Rooms")}
                                </DropdownMenuItem>
                                {filteredRooms.map((room) => (
                                    <DropdownMenuItem key={room.id} onClick={() => setSelectedRoomId(room.id)}>
                                        {getName(room.localized_name, room.id)}
                                    </DropdownMenuItem>
                                ))}
                            </DropdownMenuContent>
                        </DropdownMenu>

                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button variant="outline" size="sm" className="h-8 gap-1">
                                    {timeRange === "month" ? t("Last 60 Days") : t("Last 14 Days")}
                                    <ChevronDown className="h-4 w-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuItem onClick={() => setTimeRange("month")}>
                                    {t("Last 60 Days")} ({t("Daily")})
                                </DropdownMenuItem>
                                <DropdownMenuItem onClick={() => setTimeRange("week")}>
                                    {t("Last 14 Days")} ({t("Hourly")})
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    </div>
                </CardHeader>
                <CardContent className="min-h-[300px] flex flex-col justify-center">
                    {error && (
                        <div className="text-destructive text-sm p-4 text-center">
                            {t("Error loading data")}: {error}
                        </div>
                    )}
                    {!error && isLoading && !data && (
                        <div className="flex flex-col items-center justify-center gap-2 text-muted-foreground animate-pulse">
                            <Loader2 className="h-8 w-8 animate-spin" />
                            <span className="text-sm font-medium">{t("Loading statistics...")}</span>
                        </div>
                    )}
                    {!error && data && (
                        <HeatmapChart data={data.dataPoints} resolution={resolution} />
                    )}
                    {!error && !data && !isLoading && (
                        <div className="text-muted-foreground text-sm p-4 text-center">
                            {t("No data available")}
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    );
};

export const RoomUsageStats = memo(RoomUsageStatsComponent, (prevProps, nextProps) => {
    if (prevProps.rooms.length !== nextProps.rooms.length) return false;
    for (let i = 0; i < prevProps.rooms.length; i++) {
        if (prevProps.rooms[i].id !== nextProps.rooms[i].id) return false;
        // Check localized names for changes
        const prevLoc = prevProps.rooms[i].localized_name;
        const nextLoc = nextProps.rooms[i].localized_name;
        if (Object.keys(prevLoc).length !== Object.keys(nextLoc).length) return false;
        for (const key in prevLoc) {
            if (prevLoc[key] !== nextLoc[key]) return false;
        }
    }
    return true;
});
