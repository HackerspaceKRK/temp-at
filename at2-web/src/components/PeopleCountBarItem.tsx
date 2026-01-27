import { useState, useEffect, type FC } from "react";
import { User } from "lucide-react";
import { useTranslation } from "react-i18next";

/**
 * People count item.
 * Always renders (since count is always present); red when count > 0.
 */
export const PeopleCountBarItem: FC<{
    count: number;
    lastSeen?: string | null;
    title: string;
}> = ({ count, lastSeen, title }) => {
    const [now, setNow] = useState(new Date());
    const { t } = useTranslation();

    useEffect(() => {
        const interval = setInterval(() => {
            setNow(new Date());
        }, 10000);
        return () => clearInterval(interval);
    }, []);

    if (count === 0 && lastSeen) {
        const date = new Date(lastSeen);
        const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

        let timeString = "";
        if (diffInSeconds < 60) {
            timeString = `${Math.max(0, diffInSeconds)}s`;
        } else if (diffInSeconds < 3600) {
            const mins = Math.floor(diffInSeconds / 60);
            timeString = `${mins}m`;
        } else if (diffInSeconds < 86400) {
            const hours = Math.floor(diffInSeconds / 3600);
            timeString = `${hours}h`;
        } else {
            const days = Math.floor(diffInSeconds / 86400);
            timeString = `${days}d`;
        }

        return (
            <div
                className="flex items-center gap-1 text-neutral-300"
                title={t("Last seen: {{date}}", { date: date.toLocaleString() })}
            >
                <User className="w-4 h-4" />
                <span className="text-xs">{t("{{time}} ago", { time: timeString })}</span>
            </div>
        );
    }

    return (
        <div
            className={`flex items-center gap-1 ${count > 0 ? "text-red-400" : "text-neutral-300"
                }`}
            title={title}
        >
            <User className="w-4 h-4" />
            <span>{count}</span>
        </div>
    );
};
