import { useEffect, useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "./ui/alert";
import { Info, TriangleAlert } from "lucide-react";

interface FlashAlert {
    type: "warning" | "info" | "error";
    title: string;
    body: string;
    isCode?: boolean;
}

export function Alerts() {
    const [alerts, setAlerts] = useState<FlashAlert[]>([]);

    useEffect(() => {
        const cookies = document.cookie.split(";");
        const flashAlertCookie = cookies.find((cookie) =>
            cookie.trim().startsWith("at2_flash_alert=")
        );

        if (flashAlertCookie) {
            try {
                const encodedValue = flashAlertCookie.split("=")[1];
                const decodedValue = decodeURIComponent(encodedValue);
                const alertData: FlashAlert = JSON.parse(decodedValue);
                setAlerts((prev) => [...prev, alertData]);

                // Clear the cookie
                document.cookie =
                    "at2_flash_alert=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
            } catch (error) {
                console.error("Failed to parse flash alert cookie:", error);
            }
        }
    }, []);

    if (alerts.length === 0) return null;

    return (
        <div className="flex flex-col gap-2 px-4 pb-4">
            {alerts.map((alert, index) => {
                let variant: "default" | "destructive" = "default";
                let Icon = Info;

                if (alert.type === "error") {
                    variant = "destructive";
                    Icon = TriangleAlert;
                } else if (alert.type === "warning") {
                    Icon = TriangleAlert;
                }

                return (
                    <Alert key={index} variant={variant}>
                        <Icon className="h-4 w-4" />
                        <AlertTitle>{alert.title}</AlertTitle>
                        <AlertDescription>
                            {alert.isCode ? (
                                <pre className="font-mono text-xs mt-2 p-2 bg-muted rounded overflow-auto">
                                    {alert.body}
                                </pre>
                            ) : (
                                alert.body
                            )}
                        </AlertDescription>
                    </Alert>
                );
            })}
        </div>
    );
}
