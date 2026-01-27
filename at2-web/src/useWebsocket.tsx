import { useEffect, useState } from "react";

export const ReadyState = {
  UNINSTANTIATED: -1,
  CONNECTING: 0,
  OPEN: 1,
  CLOSING: 2,
  CLOSED: 3,
} as const;

export type ReadyState = (typeof ReadyState)[keyof typeof ReadyState];

export interface UseWebsocketOptions {
  binaryType: BinaryType;
  onMessage?: (message: MessageEvent) => void;
  autoReconnect?: boolean;
}

export default function useWebsocket(
  webSocketUrl: string | null,
  options: UseWebsocketOptions = { binaryType: "blob", autoReconnect: true },
) {
  const [readyState, setReadyState] = useState<ReadyState>(
    ReadyState.UNINSTANTIATED,
  );

  const [webSocket, setWebSocket] = useState<WebSocket | null>(null);
  const [reconnectCount, setReconnectCount] = useState(0);
  const [reconnectDelay, setReconnectDelay] = useState(2000);

  useEffect(() => {
    if (webSocketUrl === null) {
      if (webSocket) {
        webSocket.close();
        setWebSocket(null);
      }
      setReadyState(ReadyState.UNINSTANTIATED);
      return;
    }

    const ws = new WebSocket(webSocketUrl);
    if (options?.binaryType) {
      ws.binaryType = options.binaryType;
    }
    setWebSocket(ws);
    setReadyState(ReadyState.CONNECTING);

    const handleOpen = () => {
      setReadyState(ReadyState.OPEN);
      setReconnectDelay(2000); // Reset delay on successful connection
    };

    const handleClose = () => {
      setReadyState(ReadyState.CLOSED);
      if (options.autoReconnect !== false) {
        const timeout = setTimeout(() => {
          setReconnectCount((prev) => prev + 1);
          setReconnectDelay((prev) => Math.min(prev * 2, 60000));
        }, reconnectDelay);
        return () => clearTimeout(timeout);
      }
    };

    ws.addEventListener("open", handleOpen);
    ws.addEventListener("close", handleClose);
    ws.addEventListener("error", handleClose); // Treat error as close for simplicity in reconnect
    ws.addEventListener("message", (message) => {
      options?.onMessage?.(message);
    });

    return () => {
      ws.close();
      setWebSocket(null);
      setReadyState(ReadyState.CLOSED);
      ws.removeEventListener("open", handleOpen);
      ws.removeEventListener("close", handleClose);
      ws.removeEventListener("error", handleClose);
    };
  }, [webSocketUrl, reconnectCount]);
  const sendMessage = (
    message: string | ArrayBuffer | Blob | ArrayBufferView,
  ) => {
    if (webSocket && readyState === ReadyState.OPEN) {
      webSocket.send(message);
    }
  };
  return { sendMessage, readyState };
}
