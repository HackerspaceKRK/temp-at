import {
  createContext,
  useContext,
  useState,
  type FC,
  type PropsWithChildren,
} from "react";

interface TabletSessionValue {
  /** The tablet path that was open on initial page load. */
  initialPath: string;
  /** Room id, if the initial path was a /tablet/room/:id page; otherwise null. */
  initialRoomId: string | null;
}

const TabletSessionContext = createContext<TabletSessionValue | null>(null);

const ROOM_PATH_RE = /^\/tablet\/room\/([^/]+)\/?$/;

/**
 * Captures, once at mount, which tablet page the kiosk was opened to. This
 * "initial page" is where the logo and the inactivity auto-return send the
 * user back to. If it was a specific room, that room id is remembered so the
 * navbar can offer a quick link back to it.
 */
export const TabletSessionProvider: FC<PropsWithChildren> = ({ children }) => {
  const [value] = useState<TabletSessionValue>(() => {
    const path = window.location.pathname;
    const match = path.match(ROOM_PATH_RE);
    return {
      initialPath: path,
      initialRoomId: match ? decodeURIComponent(match[1]) : null,
    };
  });

  return (
    <TabletSessionContext.Provider value={value}>
      {children}
    </TabletSessionContext.Provider>
  );
};

export function useTabletSession(): TabletSessionValue {
  const ctx = useContext(TabletSessionContext);
  if (!ctx) {
    throw new Error("useTabletSession must be used within a TabletSessionProvider");
  }
  return ctx;
}
