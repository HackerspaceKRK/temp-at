import { useEffect, useState, type FC, type PropsWithChildren } from "react";
import { apiPath } from "../../config";

type AuthStatus = "loading" | "authorized" | "unauthorized";

/**
 * TabletAuthGate authenticates the kiosk by calling `/api/v1/auth/tablet-auth`
 * when a tablet page is opened. The backend grants a long-lived control session
 * if the request originates from a configured trusted subnet.
 *
 * On 401 (not a trusted subnet) it replaces the whole tablet UI with a
 * full-screen "Unauthorized" banner.
 */
export const TabletAuthGate: FC<PropsWithChildren> = ({ children }) => {
  const [status, setStatus] = useState<AuthStatus>("loading");

  useEffect(() => {
    let cancelled = false;
    fetch(apiPath("/api/v1/auth/tablet-auth"), { method: "POST" })
      .then((res) => {
        if (cancelled) return;
        setStatus(res.ok ? "authorized" : "unauthorized");
      })
      .catch(() => {
        if (!cancelled) setStatus("unauthorized");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (status === "unauthorized") {
    return (
      <div className="flex h-screen w-screen items-center justify-center bg-background text-foreground">
        <div className="text-5xl font-bold tracking-tight">Unauthorized</div>
      </div>
    );
  }

  if (status === "loading") {
    return <div className="h-screen w-screen bg-background" />;
  }

  return <>{children}</>;
};
