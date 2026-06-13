import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { formatDistanceToNowStrict } from "date-fns";
import {
  ArrowDown,
  ArrowUp,
  ChevronsUpDown,
  EthernetPort,
  Wifi,
  WifiHigh,
  WifiLow,
  WifiZero,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FC } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "../AuthContext";
import { Alert, AlertDescription } from "../components/ui/alert";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../components/ui/table";
import { apiPath } from "../config";
import { vendorLogoUrl } from "../lib/vendorLogo";

interface DhcpConnection {
  type: "wired" | "wifi" | "";
  source?: string;
  switch_name?: string;
  port?: string;
  ap_name?: string;
  ssid?: string;
  rssi?: number;
  signal_dbm?: number;
}

interface DhcpLease {
  mac_address: string;
  ip_address: string;
  hostname: string;
  comment: string;
  server: string;
  dynamic: boolean;
  vendor: string;
  first_seen: number;
  lease_start: number;
  last_seen: number;
  online: boolean;
  connection: DhcpConnection | null;
}

interface DhcpLeasesResponse {
  leases: DhcpLease[];
  last_scrape_time: number;
  scrape_error: string;
}

const POLL_INTERVAL_MS = 30_000;

export const DhcpPage: FC = () => {
  const { t } = useTranslation();
  const { user, isLoading: authLoading, login } = useAuth();

  const [data, setData] = useState<DhcpLeasesResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);
  const [sessionExpired, setSessionExpired] = useState(false);
  const [loading, setLoading] = useState(true);

  const fetchLeases = useCallback(
    async (signal?: AbortSignal) => {
      try {
        const res = await fetch(apiPath("api/v1/dhcp/leases"), { signal });
        if (res.status === 401) {
          setSessionExpired(true);
          return;
        }
        if (res.status === 403) {
          setForbidden(true);
          setError(null);
          return;
        }
        if (!res.ok) {
          throw new Error(await res.text());
        }
        const json: DhcpLeasesResponse = await res.json();
        setData(json);
        setError(null);
        setForbidden(false);
        setSessionExpired(false);
      } catch (err) {
        if ((err as Error)?.name === "AbortError") return;
        setError(t("Error loading data"));
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    if (!user) return;
    const controller = new AbortController();
    fetchLeases(controller.signal);
    const id = setInterval(() => fetchLeases(controller.signal), POLL_INTERVAL_MS);
    return () => {
      controller.abort();
      clearInterval(id);
    };
  }, [user, fetchLeases]);

  if (authLoading) {
    return (
      <main className="px-4 py-10 text-center text-muted-foreground">
        {t("Loading...")}
      </main>
    );
  }

  if (!user || sessionExpired) {
    return <LoginRequired onLogin={login} />;
  }

  return (
    <main className="px-4 pb-10">
      <Card>
        <CardHeader>
          <CardTitle>{t("DHCP Leases")}</CardTitle>
        </CardHeader>
        <CardContent>
          {forbidden && (
            <Alert className="mb-4">
              <AlertDescription>
                {t("You do not have access to any subnets.")}
              </AlertDescription>
            </Alert>
          )}
          {error && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          {data?.scrape_error && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>
                {t("Scrape error")}: {data.scrape_error}
              </AlertDescription>
            </Alert>
          )}

          {loading && !data ? (
            <div className="py-10 text-center text-muted-foreground">
              {t("Loading...")}
            </div>
          ) : (
            <LeaseDataTable leases={data?.leases ?? []} />
          )}

          {data && (
            <p className="mt-4 text-xs text-muted-foreground">
              {t("Last updated {{time}}", {
                time: data.last_scrape_time
                  ? t("{{time}} ago", {
                      time: formatDistanceToNowStrict(
                        new Date(data.last_scrape_time),
                      ),
                    })
                  : "—",
              })}
            </p>
          )}
        </CardContent>
      </Card>
    </main>
  );
};

const LoginRequired: FC<{ onLogin: () => void }> = ({ onLogin }) => {
  const { t } = useTranslation();
  return (
    <main className="flex justify-center px-4 py-16">
      <Card className="max-w-md text-center">
        <CardHeader>
          <CardTitle>{t("Login Required")}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-4">
          <p className="text-muted-foreground">
            {t("You must be logged in to view DHCP leases.")}
          </p>
          <Button onClick={onLogin}>{t("Log In")}</Button>
        </CardContent>
      </Card>
    </main>
  );
};

// Compares two dotted-quad IPv4 addresses numerically (string sort mis-orders
// e.g. 10.12.20.9 vs 10.12.20.10).
function compareIp(a: string, b: string): number {
  const pa = a.split(".").map(Number);
  const pb = b.split(".").map(Number);
  for (let i = 0; i < 4; i++) {
    const da = pa[i] ?? 0;
    const db = pb[i] ?? 0;
    if (da !== db) return da - db;
  }
  return 0;
}

const LeaseDataTable: FC<{ leases: DhcpLease[] }> = ({ leases }) => {
  const { t } = useTranslation();
  const [sorting, setSorting] = useState<SortingState>([
    { id: "lease_start", desc: true }, // newest devices first
  ]);

  const columns = useMemo<ColumnDef<DhcpLease>[]>(
    () => [
      {
        accessorKey: "ip_address",
        header: ({ column }) => (
          <SortHeader column={column} label={t("IP address")} />
        ),
        sortingFn: (a, b) =>
          compareIp(a.original.ip_address, b.original.ip_address),
        cell: ({ row }) => (
          <span className="tabular-nums">{row.original.ip_address}</span>
        ),
      },
      {
        accessorKey: "mac_address",
        header: ({ column }) => (
          <SortHeader column={column} label={t("MAC address")} />
        ),
        cell: ({ row }) => (
          <span className="font-mono text-xs">{row.original.mac_address}</span>
        ),
      },
      {
        accessorKey: "vendor",
        header: ({ column }) => (
          <SortHeader column={column} label={t("Vendor")} />
        ),
        cell: ({ row }) => <VendorCell vendor={row.original.vendor} />,
      },
      {
        accessorKey: "hostname",
        header: ({ column }) => (
          <SortHeader column={column} label={t("Hostname")} />
        ),
        cell: ({ row }) => (
          <div>
            <div>{row.original.hostname || "—"}</div>
            {row.original.comment && (
              <div className="text-xs text-muted-foreground">
                {row.original.comment}
              </div>
            )}
          </div>
        ),
      },
      {
        accessorKey: "lease_start",
        header: ({ column }) => (
          <SortHeader column={column} label={t("Lease start")} />
        ),
        cell: ({ row }) => <RelativeTime ms={row.original.lease_start} />,
      },
      {
        accessorKey: "first_seen",
        header: ({ column }) => (
          <SortHeader column={column} label={t("First seen")} />
        ),
        cell: ({ row }) => <RelativeTime ms={row.original.first_seen} />,
      },
      {
        id: "connection",
        accessorFn: (lease) => connectionSortKey(lease),
        header: ({ column }) => (
          <SortHeader column={column} label={t("Connection")} />
        ),
        cell: ({ row }) => <ConnectionCell lease={row.original} />,
      },
    ],
    [t],
  );

  const table = useReactTable({
    data: leases,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  if (leases.length === 0) {
    return (
      <div className="py-10 text-center text-muted-foreground">
        {t("Waiting for data...")}
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((hg) => (
          <TableRow key={hg.id}>
            {hg.headers.map((header) => (
              <TableHead key={header.id}>
                {header.isPlaceholder
                  ? null
                  : flexRender(
                      header.column.columnDef.header,
                      header.getContext(),
                    )}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow
            key={row.id}
            className={row.original.online ? "" : "text-muted-foreground"}
          >
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
};

function connectionSortKey(lease: DhcpLease): string {
  if (!lease.online) return "zzz_offline";
  const c = lease.connection;
  if (!c || c.type === "") return "zzz_unknown";
  if (c.type === "wired") return `wired ${c.switch_name} ${c.port}`;
  return `wifi ${c.ap_name} ${c.ssid}`;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const SortHeader: FC<{ column: any; label: string }> = ({ column, label }) => {
  const sorted = column.getIsSorted();
  return (
    <Button
      variant="ghost"
      size="sm"
      className="-ml-2 h-8 data-[state=open]:bg-accent"
      onClick={() => column.toggleSorting(sorted === "asc")}
    >
      <span>{label}</span>
      {sorted === "asc" ? (
        <ArrowUp className="ml-1 h-3.5 w-3.5" />
      ) : sorted === "desc" ? (
        <ArrowDown className="ml-1 h-3.5 w-3.5" />
      ) : (
        <ChevronsUpDown className="ml-1 h-3.5 w-3.5 opacity-50" />
      )}
    </Button>
  );
};

const VendorCell: FC<{ vendor: string }> = ({ vendor }) => {
  const logo = vendorLogoUrl(vendor);
  if (!vendor) return <span className="text-muted-foreground">—</span>;
  return (
    <span className="flex items-center gap-2">
      {logo && (
        <img
          src={logo}
          alt=""
          className="h-5 w-5 shrink-0 object-contain"
          loading="lazy"
        />
      )}
      <span>{vendor}</span>
    </span>
  );
};

// Picks a wifi signal icon by strength in dBm.
function WifiSignalIcon({ dbm }: { dbm: number }) {
  const cls = "h-6 w-6 shrink-0 text-muted-foreground";
  if (dbm >= -60) return <Wifi className={cls} />;
  if (dbm >= -70) return <WifiHigh className={cls} />;
  if (dbm >= -80) return <WifiLow className={cls} />;
  return <WifiZero className={cls} />;
}

const ConnectionCell: FC<{ lease: DhcpLease }> = ({ lease }) => {
  const { t } = useTranslation();
  const conn = lease.connection;

  if (!lease.online) {
    return <Badge variant="outline">{t("Offline")}</Badge>;
  }
  if (!conn || conn.type === "") {
    return <span className="text-muted-foreground">{t("Unknown")}</span>;
  }
  if (conn.type === "wired") {
    return (
      <div className="flex items-center gap-2">
        <EthernetPort className="h-6 w-6 shrink-0 text-muted-foreground" />
        <div className="flex flex-col leading-tight">
          <span className="font-medium">{conn.switch_name}</span>
          <span className="text-xs text-muted-foreground">{conn.port}</span>
        </div>
      </div>
    );
  }
  // wifi: row 1 = AP name, row 2 = SSID + RSSI
  const dbm = conn.signal_dbm ?? 0;
  const secondLine = [conn.ssid, dbm !== 0 ? `${dbm} dBm` : ""]
    .filter(Boolean)
    .join(" · ");
  return (
    <div className="flex items-center gap-2">
      <WifiSignalIcon dbm={dbm} />
      <div className="flex flex-col leading-tight">
        <span className="font-medium">{conn.ap_name}</span>
        <span className="text-xs text-muted-foreground">{secondLine}</span>
      </div>
    </div>
  );
};

const RelativeTime: FC<{ ms: number }> = ({ ms }) => {
  const { t } = useTranslation();
  if (!ms) return <span className="text-muted-foreground">—</span>;
  const date = new Date(ms);
  return (
    <span title={date.toLocaleString()} className="whitespace-nowrap">
      {t("{{time}} ago", {
        time: formatDistanceToNowStrict(date),
      })}
    </span>
  );
};
