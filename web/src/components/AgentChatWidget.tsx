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
        "dark chat-font fixed z-50 flex w-[90vw] max-w-[420px] flex-col overflow-hidden rounded-3xl border border-stone-200 dark:border-zinc-800/80 bg-white/95 dark:bg-zinc-950/95 shadow-[0_20px_50px_rgba(0,0,0,0.3)] transition-all duration-300 ease-in-out",
        positionClasses,
      )}
      style={{
        height: isMinimized ? "auto" : "min(680px, 85vh)",
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between border-b border-stone-200 dark:border-zinc-800/80 bg-white/70 dark:bg-zinc-950/70 px-5 py-4 text-stone-800 dark:text-stone-200 backdrop-blur-xl">
        <div className="flex items-center gap-2">
          <MessageSquareIcon
            className="w-5 h-5"
            style={{ color: primaryColor }}
          />
          <span className="text-base font-semibold tracking-[-0.02em]">
            {companyName ? `Chat with ${companyName}` : "Chat with us"}
          </span>
        </div>
        <div className="flex gap-1">
          <button
            onClick={() => setIsMinimized(!isMinimized)}
            className="rounded-xl border border-transparent bg-stone-100/80 dark:bg-zinc-800/80 p-1.5 text-stone-500 dark:text-zinc-400 transition hover:border-stone-200 dark:hover:border-zinc-700 hover:bg-stone-200 dark:hover:bg-zinc-700 active:scale-95"
          >
            <MinusIcon className="w-4 h-4" />
          </button>
          <button
            onClick={() => setIsOpen(false)}
            className="rounded-xl border border-transparent bg-stone-100/80 dark:bg-zinc-800/80 p-1.5 text-stone-500 dark:text-zinc-400 transition hover:border-stone-200 dark:hover:border-zinc-700 hover:bg-stone-200 dark:hover:bg-zinc-700 active:scale-95"
          >
            <XIcon className="w-4 h-4" />
          </button>
        </div>
      </div>

      {!isMinimized && (
        <>
          {/* Messages */}
          <div className="flex-1 space-y-5 overflow-y-auto bg-transparent px-5 py-6">
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
                    "w-[75%] px-5 py-3.5 text-[15px] leading-relaxed border border-zinc-200 dark:border-zinc-800 transition-all duration-200 shadow-sm",
                    msg.role === "user"
                      ? "bg-[#3478f6] dark:bg-[#4fc3f7] text-white dark:text-zinc-950 rounded-2xl rounded-tr-sm"
                      : "bg-zinc-100 dark:bg-zinc-800/60 text-gray-800 dark:text-gray-200 rounded-2xl rounded-tl-sm",
                  )}
                >
                  <div className="flex justify-between items-center mb-1 gap-4">
                    <span
                      className={cn(
                        "text-xs font-semibold uppercase tracking-wider",
                        msg.role === "user" ? "text-blue-100 dark:text-zinc-900/70" : "text-zinc-500 dark:text-zinc-400",
                      )}
                    >
                      {msg.role === "user" ? "Customer" : "Agent"}
                    </span>
                    <span
                      className={cn(
                        "text-[10px]",
                        msg.role === "user" ? "text-blue-200/90 dark:text-zinc-800/70" : "text-zinc-400 dark:text-zinc-500",
                      )}
                    >
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
                <div className="w-[75%] border border-zinc-200 dark:border-zinc-800 bg-zinc-100 dark:bg-zinc-800/60 rounded-2xl rounded-tl-sm px-5 py-3.5 shadow-sm">
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
          <div className="border-t border-stone-200 dark:border-zinc-800/80 bg-white/70 dark:bg-zinc-950/70 px-5 pb-5 pt-4 backdrop-blur-xl">
            <div className="flex items-center gap-2 rounded-xl border border-zinc-200 dark:border-zinc-800/80 bg-zinc-50/50 dark:bg-zinc-900/60 p-2.5 pl-4 transition-all duration-200 focus-within:border-[#3478f6] dark:focus-within:border-[#4fc3f7] focus-within:ring-1 focus-within:ring-[#3478f6] dark:focus-within:ring-[#4fc3f7] focus-within:shadow-[0_0_12px_rgba(52,120,246,0.15)] dark:focus-within:shadow-[0_0_12px_rgba(79,195,247,0.15)]">
              <input
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Type your message..."
                className="min-w-0 flex-1 border-0 bg-transparent py-1 text-base leading-normal font-sans text-gray-800 dark:text-gray-200 outline-none placeholder:text-gray-400 dark:placeholder:text-gray-500"
                disabled={isLoading}
              />
              <button
                onClick={sendMessage}
                disabled={!input.trim() || isLoading}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-white transition-all duration-200 hover:scale-105 active:scale-95 disabled:opacity-40 disabled:scale-100 hover:shadow-[0_4px_16px_rgba(52,120,246,0.35)] dark:hover:shadow-[0_4px_16px_rgba(79,195,247,0.35)]"
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
