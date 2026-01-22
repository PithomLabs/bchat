import { MessageSquareIcon, MinusIcon, SendIcon, XIcon } from "lucide-react";
import { KeyboardEvent, useEffect, useRef, useState } from "react";
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

const AgentChatWidget = ({ tenantSlug, position = "bottom-right", primaryColor = "#0d9488", companyName }: AgentChatWidgetProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const [isMinimized, setIsMinimized] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

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

  const positionClasses = position === "bottom-right" ? "right-4 bottom-4" : "left-4 bottom-4";

  if (!isOpen) {
    return (
      <button
        onClick={() => setIsOpen(true)}
        className={cn("fixed z-50 p-4 rounded-full shadow-lg text-white transition-transform hover:scale-110", positionClasses)}
        style={{ backgroundColor: primaryColor }}
        aria-label="Open chat"
      >
        <MessageSquareIcon className="w-6 h-6" />
      </button>
    );
  }

  return (
    <div
      className={cn("fixed z-50 w-80 sm:w-96 flex flex-col bg-white rounded-lg shadow-2xl overflow-hidden", positionClasses)}
      style={{ height: isMinimized ? "auto" : "500px" }}
    >
      {/* Header */}
      <div className="flex items-center justify-between p-3 text-white" style={{ backgroundColor: primaryColor }}>
        <div className="flex items-center gap-2">
          <MessageSquareIcon className="w-5 h-5" />
          <span className="font-medium">{companyName ? `Chat with ${companyName}` : "Chat with us"}</span>
        </div>
        <div className="flex gap-1">
          <button onClick={() => setIsMinimized(!isMinimized)} className="p-1 hover:bg-white/20 rounded">
            <MinusIcon className="w-4 h-4" />
          </button>
          <button onClick={() => setIsOpen(false)} className="p-1 hover:bg-white/20 rounded">
            <XIcon className="w-4 h-4" />
          </button>
        </div>
      </div>

      {!isMinimized && (
        <>
          {/* Messages */}
          <div className="flex-1 overflow-y-auto p-3 space-y-3 bg-gray-50">
            {messages.length === 0 && (
              <div className="text-center text-gray-500 py-8">
                <MessageSquareIcon className="w-10 h-10 mx-auto mb-2 opacity-30" />
                <p>How can we help you today?</p>
              </div>
            )}
            {messages.map((msg, idx) => (
              <div key={idx} className={cn("flex", msg.role === "user" ? "justify-end" : "justify-start")}>
                <div
                  className={cn(
                    "max-w-[80%] p-2.5 rounded-lg text-sm",
                    msg.role === "user" ? "bg-teal-500 text-white" : "bg-white border border-gray-200 text-gray-800",
                  )}
                >
                  <p className="whitespace-pre-wrap">{msg.content}</p>
                </div>
              </div>
            ))}
            {isLoading && (
              <div className="flex justify-start">
                <div className="bg-white border border-gray-200 p-2.5 rounded-lg">
                  <span className="animate-pulse text-gray-500">Typing...</span>
                </div>
              </div>
            )}
            {error && <div className="text-center text-red-500 text-sm">{error}</div>}
            <div ref={messagesEndRef} />
          </div>

          {/* Input */}
          <div className="p-3 border-t bg-white">
            <div className="flex gap-2">
              <input
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Type your message..."
                className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:border-teal-500"
                disabled={isLoading}
              />
              <button
                onClick={sendMessage}
                disabled={!input.trim() || isLoading}
                className="p-2 rounded-lg text-white disabled:opacity-50"
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
