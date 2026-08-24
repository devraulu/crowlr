import { memo, useMemo, useState } from "react";
import useStreamingChat, {
  type Message as MessageObj,
} from "../hooks/use-streaming-chat";
import { cn } from "../utils.ts";
import Button from "./ui/button.tsx";
import Message from "./ui/message.tsx";
import { marked } from "marked";
import DOMPurify from "dompurify";
import React from "react";

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
    <div className="h-full mt-4">
      <div className="space-y-4 relative">
        {messages.map((m, i) => (
          <React.Fragment key={m.id}>
            {m.role === "system" ? (
              <SystemMessage isStreaming={isStreaming} content={m.content} />
            ) : (
              <Message key={m.id} variant={m.role}>
                {m.content}
              </Message>
            )}
          </React.Fragment>
        ))}
      </div>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          send(input);
          setInput("");
        }}
        className={cn(" mx-auto", messages.length > 0 ? "pt-[20vh]" : "")}
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
          {isStreaming && <Button onClick={cancel}>x</Button>}
          <Button disabled={isStreaming} type="submit" className="rotate-180 ">
            v
          </Button>
        </div>
      </form>
    </div>
  );
}

const SystemMessage = memo(
  ({ content, isStreaming }: { content: string; isStreaming: boolean }) => {
    const renderedContent = useMemo(() => {
      if (isStreaming) {
        const lines = content.split("\n");
        return (
          <Message variant={"system"}>
            {lines.map((line, i) => (
              <span key={line}>
                {line}
                {i < lines.length - 1 && <br />}
              </span>
            ))}
          </Message>
        );
      }

      const html = marked(content, { breaks: true, gfm: true }) as string;
      const sanitized = DOMPurify.sanitize(html);
      return <div dangerouslySetInnerHTML={{ __html: sanitized }} />;
    }, [content, isStreaming]);

    return (
      <Message variant={"system"}>
        {renderedContent}
        {isStreaming && <div className="animate-spin">|</div>}
      </Message>
    );
  },
);
