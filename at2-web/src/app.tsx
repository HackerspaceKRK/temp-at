import { useEffect, useState } from "preact/hooks";
import "./app.css";
import useWebsocket from "./useWebsocket";
import type { RoomState } from "./schema";

export function App() {
  const { lastMessage, readyState, sendMessage } = useWebsocket(
    "ws://localhost:8080/api/v1/live-ws",
  );

  const [roomStates, setRoomStates] = useState<{
    [key: string]: RoomState;
  }>({});
  useEffect(() => {
    if (lastMessage?.data) {
      const data = JSON.parse(lastMessage.data);
      setRoomStates({
        ...roomStates,
        [data.id]: data,
      });
    }
  }, [lastMessage]);
  return (
    <>
      <pre>{JSON.stringify(roomStates, null, 2)}</pre>
    </>
  );
}
