import { type FC, useEffect, useState } from "react";
import type { RoomState, UsageHeatmapResponse } from "../schema";
import { API_URL } from "../config";
import { HeatmapChart } from "./HeatmapChart";
import { useTranslation } from "react-i18next";
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

export const RoomUsageStats: FC<RoomUsageStatsProps> = ({ rooms }) => {
    const { t, i18n } = useTranslation();
    const [selectedRoomId, setSelectedRoomId] = useState<string>("");
    const [timeRange, setTimeRange] = useState<"month" | "week">("month");
    const [data, setData] = useState<UsageHeatmapResponse | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const resolution = timeRange === "month" ? "day" : "hour";
    const duration = timeRange === "month" ? 30 : 168;

    useEffect(() => {
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
    }, [selectedRoomId, timeRange, resolution, duration]);

    const selectedRoomLabel = selectedRoomId
        ? (rooms.find(r => r.id === selectedRoomId)?.localized_name[i18n.language] || selectedRoomId)
        : t("All Rooms");

    return (
        <Card className="col-span-full mt-8">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-7">
                <CardTitle className="text-xl font-bold">{t("Usage Statistics")}</CardTitle>
                <div className="flex items-center gap-2">
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
                            {rooms.map((room) => (
                                <DropdownMenuItem key={room.id} onClick={() => setSelectedRoomId(room.id)}>
                                    {room.localized_name[i18n.language] || room.id}
                                </DropdownMenuItem>
                            ))}
                        </DropdownMenuContent>
                    </DropdownMenu>

                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <Button variant="outline" size="sm" className="h-8 gap-1">
                                {timeRange === "month" ? t("Last 30 Days") : t("Last 7 Days")}
                                <ChevronDown className="h-4 w-4" />
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                            <DropdownMenuItem onClick={() => setTimeRange("month")}>
                                {t("Last 30 Days")} ({t("Daily")})
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => setTimeRange("week")}>
                                {t("Last 7 Days")} ({t("Hourly")})
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                </div>
            </CardHeader>
            <CardContent>
                {error && (
                    <div className="text-destructive text-sm p-4 text-center">
                        {t("Error loading data")}: {error}
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
    );
};
