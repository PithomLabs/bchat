import { MessageSquareIcon, MinusIcon, SendIcon, XIcon } from "lucide-react";
import {
  KeyboardEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import { cn } from "@/utils";

interface Message {
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
}

interface AgentChatWidgetProps {
  tenantSlug: string;
  position?: "bottom-right" | "bottom-left";
  primaryColor?: string;
  companyName?: string;
}

const AgentChatWidget = ({
  tenantSlug,
  position = "bottom-right",
  primaryColor = "#0d9488",
  companyName,
}: AgentChatWidgetProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const [isMinimized, setIsMinimized] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [bridgeState, setBridgeState] = useState<{
    status: string;
    handoff_id: string;
    routing_mode: string;
  } | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // Load session from localStorage on init
  useEffect(() => {
    const key = `bchat_session_id:${tenantSlug}`;
    const savedSessionId = localStorage.getItem(key);
    if (savedSessionId) {
      setSessionId(savedSessionId);
      // Fetch transcript immediately
      const fetchTranscript = async () => {
        try {
          const res = await fetch(
            `/api/v1/agent/${tenantSlug}/chat/ext/transcript?session_id=${savedSessionId}`,
          );
          if (res.ok) {
            const data = await res.json();
            if (data.messages) {
              setMessages(
                data.messages.map((m: any) => ({
                  role: m.role === "user" ? "user" : "assistant",
                  content: m.content,
                  timestamp: new Date(m.timestamp),
                })),
              );
            }
            if (data.bridge) {
              setBridgeState(data.bridge);
            } else {
              setBridgeState(null);
            }
          }
        } catch (e) {
          console.error("Failed to load transcript on mount", e);
        }
      };
      fetchTranscript();
    }
  }, [tenantSlug]);

  // Handle localStorage sync
  useEffect(() => {
    const key = `bchat_session_id:${tenantSlug}`;
    if (sessionId) {
      localStorage.setItem(key, sessionId);
    } else {
      localStorage.removeItem(key);
    }
  }, [sessionId, tenantSlug]);

  // Polling loop when handoff is active/queued and widget is open
  useEffect(() => {
    if (!isOpen || isMinimized || !sessionId || !bridgeState) return;
    if (
      bridgeState.status !== "human_handoff_active" &&
      bridgeState.status !== "human_handoff_queued"
    )
      return;

    const interval = setInterval(async () => {
      try {
        const res = await fetch(
          `/api/v1/agent/${tenantSlug}/chat/ext/transcript?session_id=${sessionId}`,
        );
        if (res.ok) {
          const data = await res.json();
          if (data.messages) {
            setMessages(
              data.messages.map((m: any) => ({
                role: m.role === "user" ? "user" : "assistant",
                content: m.content,
                timestamp: new Date(m.timestamp),
              })),
            );
          }
          if (data.bridge) {
            setBridgeState(data.bridge);
          } else {
            setBridgeState(null);
          }
        }
      } catch (e) {
        console.error("Failed to poll transcript", e);
      }
    }, 3000);

    return () => clearInterval(interval);
  }, [isOpen, isMinimized, sessionId, bridgeState?.status, tenantSlug]);

  const sendMessage = async () => {
    if (!input.trim() || isLoading) return;

    const userMessage = input.trim();
    setInput("");
    setError(null);

    // Add user message immediately
    setMessages((prev) => [
      ...prev,
      {
        role: "user",
        content: userMessage,
        timestamp: new Date(),
      },
    ]);

    setIsLoading(true);

    try {
      const response = await fetch(`/api/v1/agent/${tenantSlug}/chat/ext`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          session_id: sessionId,
          message: userMessage,
        }),
      });

      if (response.status === 429) {
        setError("Too many messages. Please wait a moment.");
        return;
      }

      if (!response.ok) {
        throw new Error("Failed to send message");
      }

      const data = await response.json();
      setSessionId(data.session_id);
      if (data.bridge) {
        setBridgeState(data.bridge);
      } else {
        setBridgeState(null);
      }

      // Add assistant message
      setMessages((prev) => [
        ...prev,
        {
          role: "assistant",
          content: data.message.content,
          timestamp: new Date(data.message.timestamp),
        },
      ]);
    } catch {
      setError("Something went wrong. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const positionClasses =
    position === "bottom-right" ? "right-4 bottom-4" : "left-4 bottom-4";

  if (!isOpen) {
    return (
      <button
        onClick={() => setIsOpen(true)}
        className={cn(
          "fixed z-50 p-4 rounded-full shadow-lg text-white transition-transform hover:scale-110",
          positionClasses,
        )}
        style={{ backgroundColor: primaryColor }}
        aria-label="Open chat"
      >
        <MessageSquareIcon className="w-6 h-6" />
      </button>
    );
  }

  return (
    <div
      className={cn(
        "dark chat-font fixed z-50 flex w-[90vw] max-w-[700px] flex-col overflow-hidden rounded-xl border border-stone-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 shadow-[0_18px_60px_rgba(0,0,0,0.5)]",
        positionClasses,
      )}
      style={{
        height: isMinimized ? "auto" : "min(600px, 90vh)",
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between border-b border-stone-200 dark:border-zinc-800 bg-white/95 dark:bg-zinc-900/95 px-4 py-3.5 text-stone-800 dark:text-stone-200 backdrop-blur">
        <div className="flex items-center gap-2">
          <MessageSquareIcon
            className="w-5 h-5"
            style={{ color: primaryColor }}
          />
          <span className="text-[15px] font-semibold tracking-[-0.015em]">
            {companyName ? `Chat with ${companyName}` : "Chat with us"}
          </span>
        </div>
        <div className="flex gap-1">
          <button
            onClick={() => setIsMinimized(!isMinimized)}
            className="rounded-lg border border-transparent bg-stone-100 dark:bg-zinc-800 p-1.5 text-stone-500 dark:text-zinc-400 transition hover:border-stone-200 dark:hover:border-zinc-700 hover:bg-stone-200 dark:hover:bg-zinc-700"
          >
            <MinusIcon className="w-4 h-4" />
          </button>
          <button
            onClick={() => setIsOpen(false)}
            className="rounded-lg border border-transparent bg-stone-100 dark:bg-zinc-800 p-1.5 text-stone-500 dark:text-zinc-400 transition hover:border-stone-200 dark:hover:border-zinc-700 hover:bg-stone-200 dark:hover:bg-zinc-700"
          >
            <XIcon className="w-4 h-4" />
          </button>
        </div>
      </div>

      {!isMinimized && (
        <>
          {/* Messages */}
          <div className="flex-1 space-y-3 overflow-y-auto bg-transparent px-4 py-4">
            {messages.length === 0 && (
              <div className="flex h-full min-h-[260px] flex-col items-center justify-center py-8 text-center text-stone-500 dark:text-zinc-400">
                <MessageSquareIcon className="w-10 h-10 mx-auto mb-2 opacity-30" />
                <p className="text-[15px] leading-relaxed font-medium">
                  How can we help you today?
                </p>
              </div>
            )}
            {messages.map((msg, idx) => (
              <div
                key={idx}
                className={cn(
                  "flex",
                  msg.role === "user" ? "justify-end ml-8" : "justify-start mr-8",
                )}
              >
                <div
                  className={cn(
                    "max-w-[85%] rounded-lg border p-3 text-sm leading-relaxed",
                    msg.role === "user"
                      ? "border-blue-200 dark:border-blue-900 bg-blue-100 dark:bg-blue-950/40 text-gray-800 dark:text-gray-200"
                      : "border-zinc-200 dark:border-zinc-700 bg-zinc-100 dark:bg-zinc-800 text-gray-800 dark:text-gray-200",
                  )}
                >
                  <div className="flex justify-between items-center mb-1">
                    <span
                      className={cn(
                        "text-xs font-medium",
                        msg.role === "user" ? "text-blue-600 dark:text-blue-400" : "text-zinc-600 dark:text-zinc-400",
                      )}
                    >
                      {msg.role === "user" ? "Customer" : "Agent"}
                    </span>
                    <span className="text-xs text-zinc-500 dark:text-zinc-400">
                      {msg.timestamp.toLocaleTimeString([], {
                        hour: "numeric",
                        minute: "2-digit",
                      })}
                    </span>
                  </div>
                  <p className="whitespace-pre-wrap break-words">
                    {msg.content}
                  </p>
                </div>
              </div>
            ))}
            {isLoading && (
              <div className="flex justify-start mr-8">
                <div className="rounded-lg border border-zinc-200 dark:border-zinc-700 bg-zinc-100 dark:bg-zinc-800 p-3 max-w-[85%]">
                  <span className="animate-pulse text-sm text-zinc-500 dark:text-zinc-400">
                    Typing...
                  </span>
                </div>
              </div>
            )}
            {error && (
              <div className="text-center text-red-500 text-sm">{error}</div>
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Input */}
          <div className="border-t border-stone-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-4 pb-4 pt-3">
            <div className="flex items-center gap-2 rounded-lg border border-zinc-300 dark:border-zinc-700 bg-zinc-100 dark:bg-zinc-800 p-2 pl-3.5 transition focus-within:border-teal-500">
              <input
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Type your message..."
                className="min-w-0 flex-1 border-0 bg-transparent py-1.5 text-sm leading-relaxed font-sans text-gray-800 dark:text-gray-200 outline-none placeholder:text-gray-400 dark:placeholder:text-gray-500"
                disabled={isLoading}
              />
              <button
                onClick={sendMessage}
                disabled={!input.trim() || isLoading}
                className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-white transition hover:opacity-90 disabled:opacity-40"
                style={{ backgroundColor: primaryColor }}
              >
                <SendIcon className="w-4 h-4" />
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  );
};

export default AgentChatWidget;
