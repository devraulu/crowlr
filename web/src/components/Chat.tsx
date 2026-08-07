import { useState } from "react";
import useStreamingChat from "../hooks/use-streaming-chat";
import { cn } from "../utils.ts";

type Props = {};

export default function Chat({}: Props) {
  const [input, setInput] = useState("");
  const {
    messages,
    state: { error, isStreaming, usage },
    send,
    cancel,
    clear,
  } = useStreamingChat();

  return (
    <div className="h-full">
      {messages.map((m) => (
        <>{m.content}</>
      ))}
      <form
        onSubmit={() => send(input)}
        className={cn(
          "md:w-4/5 mx-auto",
          messages.length > 0 ? "pt-[20vh]" : "",
        )}
      >
        <div className="">ask questions about the crawled data...</div>
        <div className="flex gap-4">
          <input
            id="chat-input"
            type="search"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            className="w-full border p-1"
          />
          <button
            disabled={isStreaming}
            type="submit"
            className="rotate-180 border bg-transparent cursor-pointer px-4 py-0.5"
          >
            v
          </button>
        </div>
      </form>
    </div>
  );
}
