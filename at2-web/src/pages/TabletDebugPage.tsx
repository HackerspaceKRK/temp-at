import { useEffect, useRef, useState, type FC, type ReactNode } from "react";
import {
  KIOSK_EVENT_NAMES,
  isKioskAvailable,
  kioskAnswerCall,
  kioskGetProximity,
  kioskHangupCall,
  kioskListAudioDevices,
  kioskMakeCall,
  kioskProximityStart,
  kioskProximityStop,
  kioskRejectCall,
  kioskScreenOff,
  kioskScreenOn,
  kioskSetBrightness,
  kioskWatchdogEnable,
  kioskWatchdogFeed,
  onKioskEvent,
  type KioskAudioDevice,
} from "../lib/kioskApi";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Label } from "../components/ui/label";
import { Switch } from "../components/ui/switch";

interface LogEntry {
  time: string;
  name: string;
  detail: string;
}

const Panel: FC<{ title: string; children: ReactNode }> = ({
  title,
  children,
}) => (
  <Card className="flex flex-col gap-3 p-4">
    <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
      {title}
    </h2>
    {children}
  </Card>
);

export const TabletDebugPage: FC = () => {
  const available = isKioskAvailable();

  const [brightness, setBrightness] = useState(1);
  const [proximity, setProximity] = useState<number | null>(null);
  const [autoFeed, setAutoFeed] = useState(false);
  const [callDest, setCallDest] = useState("1001");
  const [audioDevices, setAudioDevices] = useState<KioskAudioDevice[] | null>(
    null,
  );
  const [registration, setRegistration] = useState<string>("unknown");
  const [log, setLog] = useState<LogEntry[]>([]);

  const addLog = (name: string, detail: unknown) => {
    setLog((prev) =>
      [
        {
          time: new Date().toLocaleTimeString("pl-PL"),
          name,
          detail: JSON.stringify(detail),
        },
        ...prev,
      ].slice(0, 200),
    );
  };

  // Subscribe to all kiosk events for the live event log.
  useEffect(() => {
    const unsubscribers = KIOSK_EVENT_NAMES.map((name) =>
      onKioskEvent(name, (detail) => {
        addLog(name, detail);
        if (name === "kiosk_sip_registration") {
          const d = detail as { registered: boolean; text: string };
          setRegistration(`${d.registered ? "registered" : "unregistered"} (${d.text})`);
        }
        if (name === "kiosk_proximity") {
          setProximity((detail as { value: number }).value);
        }
      }),
    );
    return () => unsubscribers.forEach((u) => u());
  }, []);

  // Optional watchdog auto-feed (feed every 5s, comfortably within 20s).
  const autoFeedRef = useRef(autoFeed);
  autoFeedRef.current = autoFeed;
  useEffect(() => {
    if (!autoFeed) return;
    const id = window.setInterval(() => {
      kioskWatchdogFeed();
      addLog("watchdog_feed", "auto");
    }, 5000);
    return () => window.clearInterval(id);
  }, [autoFeed]);

  return (
    <main className="mx-auto w-full max-w-[1280px] px-4 py-4">
      <div
        className={`mb-4 rounded-md border px-4 py-2 text-sm font-medium ${
          available
            ? "border-emerald-500 bg-emerald-500/10 text-emerald-600"
            : "border-rose-500 bg-rose-500/10 text-rose-600"
        }`}
      >
        {available
          ? "Kiosk bridge detected — controls are live."
          : "Kiosk bridge NOT detected — you are not running inside the kiosk. Controls are no-ops."}
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Panel title="Screen & brightness">
          <div className="flex gap-2">
            <Button onClick={kioskScreenOn}>Screen On</Button>
            <Button variant="outline" onClick={kioskScreenOff}>
              Screen Off
            </Button>
          </div>
          <div className="flex flex-col gap-1">
            <Label>Brightness: {Math.round(brightness * 100)}%</Label>
            <input
              type="range"
              min={0}
              max={1}
              step={0.01}
              value={brightness}
              onChange={(e) => {
                const v = Number(e.target.value);
                setBrightness(v);
                kioskSetBrightness(v);
              }}
            />
          </div>
        </Panel>

        <Panel title="Proximity">
          <div className="flex flex-wrap gap-2">
            <Button onClick={kioskProximityStart}>Start</Button>
            <Button variant="outline" onClick={kioskProximityStop}>
              Stop
            </Button>
            <Button
              variant="outline"
              onClick={() => setProximity(kioskGetProximity())}
            >
              Get
            </Button>
          </div>
          <div className="text-sm text-muted-foreground">
            Latest value:{" "}
            <span className="font-mono text-foreground">
              {proximity ?? "—"}
            </span>
          </div>
        </Panel>

        <Panel title="Watchdog">
          <div className="flex gap-2">
            <Button onClick={kioskWatchdogEnable}>Enable</Button>
            <Button variant="outline" onClick={kioskWatchdogFeed}>
              Feed
            </Button>
          </div>
          <div className="flex items-center gap-2">
            <Switch
              id="auto-feed"
              checked={autoFeed}
              onCheckedChange={setAutoFeed}
            />
            <Label htmlFor="auto-feed">Auto-feed every 5s</Label>
          </div>
        </Panel>

        <Panel title="SIP call">
          <div className="text-sm text-muted-foreground">
            Registration:{" "}
            <span className="font-mono text-foreground">{registration}</span>
          </div>
          <div className="flex gap-2">
            <input
              type="text"
              value={callDest}
              onChange={(e) => setCallDest(e.target.value)}
              placeholder="sip:1001@pbx or 1001"
              className="flex-1 rounded-md border border-border bg-background px-2 py-1 text-sm"
            />
            <Button onClick={() => kioskMakeCall(callDest)}>Call</Button>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={kioskAnswerCall}>
              Answer
            </Button>
            <Button variant="outline" onClick={kioskRejectCall}>
              Reject
            </Button>
            <Button variant="outline" onClick={kioskHangupCall}>
              Hangup
            </Button>
          </div>
        </Panel>

        <Panel title="Audio devices">
          <Button onClick={() => setAudioDevices(kioskListAudioDevices())}>
            List devices
          </Button>
          {audioDevices && (
            <div className="flex flex-col gap-1 text-sm">
              {audioDevices.length === 0 ? (
                <span className="text-muted-foreground">No devices</span>
              ) : (
                audioDevices.map((d) => (
                  <div key={d.index} className="font-mono">
                    [{d.index}] {d.name} (in:{d.inputCount} out:{d.outputCount})
                  </div>
                ))
              )}
            </div>
          )}
        </Panel>

        <Panel title="Event log">
          <div className="flex justify-end">
            <Button variant="outline" size="sm" onClick={() => setLog([])}>
              Clear
            </Button>
          </div>
          <div className="max-h-64 overflow-y-auto rounded-md bg-muted/40 p-2 font-mono text-xs">
            {log.length === 0 ? (
              <span className="text-muted-foreground">No events yet</span>
            ) : (
              log.map((entry, i) => (
                <div key={i} className="border-b border-border/40 py-0.5">
                  <span className="text-muted-foreground">{entry.time}</span>{" "}
                  <span className="font-semibold">{entry.name}</span>{" "}
                  {entry.detail}
                </div>
              ))
            )}
          </div>
        </Panel>
      </div>
    </main>
  );
};
