import { type FC, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Area, AreaChart, CartesianGrid, ReferenceLine, XAxis, YAxis } from "recharts";
import { format } from "date-fns";
import { enUS, pl } from "date-fns/locale";
import { Loader2 } from "lucide-react";
import type { PowerStatsResponse } from "../schema";
import { API_URL } from "../config";
import { useLocale } from "../locale";
import { useHasBeenInView } from "../hooks/useHasBeenInView";
import { setChartCursor, useChartCursor } from "../hooks/useChartCursor";
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./ui/select";
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "./ui/chart";

type PowerRange = "48h" | "7d";

const CursorLine: FC<{ from: number; to: number }> = ({ from, to }) => {
  const cursor = useChartCursor();
  if (cursor === null || cursor < from || cursor > to) return null;
  return <ReferenceLine x={cursor} stroke="var(--muted-foreground)" strokeOpacity={0.7} strokeWidth={2} ifOverflow="hidden" />;
};

export const PowerUsageChart: FC = () => {
  const { t, i18n } = useTranslation();
  const { getName } = useLocale();
  const [range, setRange] = useState<PowerRange>("48h");
  const [data, setData] = useState<PowerStatsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { ref, hasBeenInView } = useHasBeenInView<HTMLDivElement>();

  useEffect(() => {
    if (!hasBeenInView) return;
    const controller = new AbortController();
    setIsLoading(true);
    setError(null);
    fetch(`${API_URL.replace(/\/$/, "")}/api/v1/stats/power?range=${range}`, {
      signal: controller.signal,
    })
      .then(async (res) => {
        if (!res.ok) throw new Error(await res.text());
        setData(await res.json());
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(err.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoading(false);
      });
    return () => controller.abort();
  }, [range, hasBeenInView]);

  const dateLocale = i18n.language.startsWith("pl") ? pl : enUS;
  const tickFormat = range === "48h" ? "HH:mm" : "EEE HH:mm";

  const { rows, config } = useMemo(() => {
    const config: ChartConfig = {};
    data?.series.forEach((s, i) => {
      config[`s${i}`] = {
        label: getName(s.localized_name, s.id),
        color: s.color || `var(--chart-${(i % 5) + 1})`,
      };
    });
    const rows = (data?.timestamps ?? []).map((ts, idx) => {
      const row: Record<string, number | null> = { t: ts };
      data?.series.forEach((s, i) => {
        const v = s.values[idx];
        row[`s${i}`] = v == null ? null : Math.round(v);
      });
      return row;
    });
    return { rows, config };
  }, [data, getName, i18n.language]);

  const seriesKeys = Object.keys(config);

  const scrollRef = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollLeft = scrollRef.current.scrollWidth;
  }, [rows]);

  return (
    <div ref={ref} className="col-span-full">
      <Card>
        <CardHeader className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 space-y-0 pb-7">
          <CardTitle className="text-xl font-bold">{t("Power usage")}</CardTitle>
          <div className="flex items-center gap-2">
            {isLoading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
            <Select value={range} onValueChange={(v) => setRange(v as PowerRange)}>
              <SelectTrigger className="w-[180px] h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="48h">{t("Last 48 Hours")}</SelectItem>
                <SelectItem value="7d">{t("Last 7 Days")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent className="min-h-[450px] flex flex-col justify-center">
          {error ? (
            <div className="text-destructive text-sm p-4 text-center">
              {t("Error loading data")}: {error}
            </div>
          ) : seriesKeys.length > 0 ? (
            <>
              <div ref={scrollRef} className="overflow-x-auto pb-2">
                <ChartContainer config={config} className="aspect-auto h-[450px] w-full min-w-[720px]">
                  <AreaChart
                    data={rows}
                    margin={{ left: 12, right: 12 }}
                    onMouseMove={(state) => {
                      const idx = Number(state.activeTooltipIndex);
                      setChartCursor(Number.isInteger(idx) && rows[idx] ? (rows[idx].t as number) : null);
                    }}
                    onMouseLeave={() => setChartCursor(null)}
                  >
                    <CartesianGrid vertical={false} />
                    <XAxis
                      dataKey="t"
                      type="number"
                      scale="time"
                      domain={["dataMin", "dataMax"]}
                      tickLine={false}
                      axisLine={false}
                      tickMargin={8}
                      minTickGap={32}
                      tickFormatter={(v) => format(v, tickFormat, { locale: dateLocale })}
                    />
                    <YAxis
                      tickLine={false}
                      axisLine={false}
                      tickMargin={8}
                      tickFormatter={(v) => `${v} W`}
                    />
                    <ChartTooltip
                      cursor={false}
                      content={
                        <ChartTooltipContent
                          indicator="dot"
                          labelFormatter={(_, payload) =>
                            format(payload[0]?.payload.t, "EEE HH:mm", { locale: dateLocale })
                          }
                        />
                      }
                    />
                    {seriesKeys.map((key) => (
                      <Area
                        key={key}
                        dataKey={key}
                        type="monotone"
                        stackId="power"
                        fill={`var(--color-${key})`}
                        fillOpacity={0.4}
                        stroke={`var(--color-${key})`}
                        isAnimationActive={false}
                      />
                    ))}
                    {rows.length > 0 && (
                      <CursorLine from={rows[0].t as number} to={rows[rows.length - 1].t as number} />
                    )}
                  </AreaChart>
                </ChartContainer>
              </div>
              <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1 pt-3 text-xs">
                {seriesKeys.map((key) => (
                  <div key={key} className="flex items-center gap-1.5">
                    <div className="h-2 w-2 shrink-0 rounded-[2px]" style={{ backgroundColor: config[key].color }} />
                    {config[key].label}
                  </div>
                ))}
              </div>
            </>
          ) : isLoading || !hasBeenInView ? (
            <div className="flex flex-col items-center justify-center gap-2 text-muted-foreground animate-pulse">
              <Loader2 className="h-8 w-8 animate-spin" />
              <span className="text-sm font-medium">{t("Loading statistics...")}</span>
            </div>
          ) : (
            <div className="text-muted-foreground text-sm p-4 text-center">
              {t("No data available")}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};
