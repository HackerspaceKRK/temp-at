import { useEffect, useState } from "preact/hooks";
import preactLogo from "./assets/preact.svg";
import viteLogo from "/vite.svg";
import "./app.css";
import useWebsocket from "./useWebsocket";

export function App() {
  const { lastMessage, readyState, sendMessage } = useWebsocket(
    "ws://localhost:8080/api/v1/live-ws"
  );

  const [roomStates, setRoomStates] = useState<any>({});
  useEffect(() => {
    if (lastMessage?.data) {
      const data = JSON.parse(lastMessage.data);
      setRoomStates({
        ...roomStates,
        [data.name]: data,
      });
    }
  }, [lastMessage]);
  return (
    <>
      <pre>{JSON.stringify(roomStates, null, 2)}</pre>
    </>
  );
}
