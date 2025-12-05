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
}

export default function useWebsocket(
  webSocketUrl: string | null,
  options?: UseWebsocketOptions,
) {
  const [readyState, setReadyState] = useState<ReadyState>(
    ReadyState.UNINSTANTIATED,
  );
  
  const [webSocket, setWebSocket] = useState<WebSocket | null>(null);

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

    ws.addEventListener("open", () => {
      setReadyState(ReadyState.OPEN);
    });
    ws.addEventListener("close", () => {
      setReadyState(ReadyState.CLOSED);
    });
    ws.addEventListener("error", () => {
      setReadyState(ReadyState.CLOSED);
    });
    ws.addEventListener("message", (message) => {
      options?.onMessage?.(message);
       
    });

    return () => {
      ws.close();
      setWebSocket(null);
      setReadyState(ReadyState.CLOSED);
    };
  }, [webSocketUrl]);
  const sendMessage = (
    message: string | ArrayBuffer | Blob | ArrayBufferView,
  ) => {
    if (webSocket && readyState === ReadyState.OPEN) {
      webSocket.send(message);
    }
  };
  return { sendMessage,  readyState };
}
