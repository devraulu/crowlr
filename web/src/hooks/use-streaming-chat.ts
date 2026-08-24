import { useCallback, useRef, useState } from "react";
import { API_URL } from "../api";

export interface Message {
  id: string;
  role: "user" | "system";
  content: string;
}

interface StreamState {
  isStreaming: boolean;
  error: string | null;
  usage: { totalTokens: number } | null;
}

export default function useStreamingChat() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [state, setState] = useState<StreamState>({
    isStreaming: false,
    error: null,
    usage: null,
  });

  const abortControllerRef = useRef<AbortController | null>(null);

  const send = useCallback(async (userMessage: string) => {
    abortControllerRef.current?.abort();
    abortControllerRef.current = new AbortController();

    const newMessages: Message[] = [
      ...messages,
      { id: crypto.randomUUID(), role: "user", content: userMessage },
    ];

    const systemMessageId = crypto.randomUUID();
    setMessages([
      ...newMessages,
      { role: "system", content: "", id: systemMessageId },
    ]);
    setState({ isStreaming: true, error: null, usage: null });

    try {
      const response = await fetch(API_URL + "/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ q: newMessages }),
        signal: abortControllerRef.current.signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      if (!response.body) throw new Error("No response body");

      const stream = response.body.pipeThrough(new TextDecoderStream());

      let buffer = "";
      let systemMessage = "";
      let systemThinking = "";

      for await (const value of stream) {
        buffer += value;
        console.log({ buffer });
        const lines = buffer.split("\n\n");
        console.log({ lines });
        buffer = lines.pop() || "";
        console.log({ buffer });
        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const jsonStr = line.slice(6);
          if (jsonStr === "[DONE]") continue;
          try {
            const data = JSON.parse(jsonStr);
            if (data.type === "content") {
              systemMessage += data.content;
              setMessages((prev) =>
                prev.slice(0, -1).concat({
                  id: systemMessageId,
                  role: "system",
                  content: systemMessage,
                }),
              );
            } else if (data.type === "thinking") {
              systemThinking += data.content;
            } else if (data.type === "done") {
              setState((prev) => ({
                ...prev,
                usage: data.usage,
              }));
            } else if (data.type === "error") {
              throw new Error(data.message);
            }
          } catch (e) {
            console.warn("Failed to parse SSE message: ", line);
          }
        }
      }
    } catch (error) {
      if ((error as Error).name === "AbortError") {
        console.log("aborted");
        return;
      }
      console.error(error);
      setState((prev) => ({ ...prev, error: (error as Error).message }));
      setMessages((prev) => prev.slice(0, -1));
    } finally {
      setState((prev) => ({ ...prev, isStreaming: false }));
    }
  }, []);

  const cancel = useCallback(() => {
    abortControllerRef.current?.abort();
    setState((prev) => ({ ...prev, isStreaming: false }));
  }, []);

  const clear = useCallback(() => {
    setMessages([]);
    setState({ isStreaming: false, error: null, usage: null });
  }, []);

  return {
    messages,
    state,
    send,
    cancel,
    clear,
  };
}
