import { Button, Textarea } from "@mui/joy";
import { MessageSquareIcon, SendIcon, Trash2Icon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { KeyboardEvent, useEffect, useRef, useState } from "react";
import MobileHeader from "@/components/MobileHeader";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { chatStore } from "@/store/v2";
import { cn } from "@/utils";
import { useTranslate } from "@/utils/i18n";

const Chat = observer(() => {
  const t = useTranslate();
  const { md } = useResponsiveWidth();
  const [input, setInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const { messages, isLoading, error } = chatStore.state;

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSend = async () => {
    const trimmedInput = input.trim();
    if (!trimmedInput || isLoading) return;

    setInput("");
    await chatStore.sendMessage(trimmedInput);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleClear = () => {
    chatStore.clearChat();
  };

  return (
    <section className="@container w-full max-w-4xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full h-full px-4 sm:px-6 flex flex-col" style={{ minHeight: "calc(100vh - 200px)" }}>
        {/* Header */}
        <div className="w-full flex flex-row justify-between items-center mb-4">
          <div className="flex items-center gap-2">
            <MessageSquareIcon className="w-6 h-6 opacity-70" />
            <h1 className="text-xl font-semibold text-gray-800 dark:text-gray-200">{t("chat.title")}</h1>
          </div>
          {messages.length > 0 && (
            <Button variant="plain" color="neutral" size="sm" startDecorator={<Trash2Icon className="w-4 h-4" />} onClick={handleClear}>
              {t("common.clear")}
            </Button>
          )}
        </div>

        {/* Messages Container */}
        <div className="flex-1 w-full overflow-y-auto bg-white dark:bg-zinc-800 rounded-xl shadow-sm border border-gray-200 dark:border-zinc-700 p-4 mb-4">
          {messages.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-gray-500 min-h-[300px]">
              <MessageSquareIcon className="w-12 h-12 mb-4 opacity-30" />
              <p>{t("chat.empty-conversation")}</p>
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              {messages.map((message, index) => (
                <div
                  key={index}
                  className={cn(
                    "max-w-[80%] p-3 rounded-lg",
                    message.role === "user"
                      ? "ml-auto bg-teal-500 text-white"
                      : "mr-auto bg-gray-100 dark:bg-zinc-700 text-gray-800 dark:text-gray-200",
                  )}
                >
                  <p className="whitespace-pre-wrap">{message.content}</p>
                  <span className="text-xs opacity-60 mt-1 block">{message.timestamp.toLocaleTimeString()}</span>
                </div>
              ))}
              {isLoading && (
                <div className="mr-auto bg-gray-100 dark:bg-zinc-700 p-3 rounded-lg text-gray-500">
                  <span className="animate-pulse">{t("chat.loading")}</span>
                </div>
              )}
              {error && (
                <div className="mr-auto bg-red-100 dark:bg-red-900/30 p-3 rounded-lg text-red-600 dark:text-red-300">{error}</div>
              )}
              <div ref={messagesEndRef} />
            </div>
          )}
        </div>

        {/* Input Area */}
        <div className="w-full flex flex-row gap-2">
          <Textarea
            className="flex-1"
            placeholder={t("chat.placeholder")}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            minRows={1}
            maxRows={4}
            disabled={isLoading}
          />
          <Button onClick={handleSend} disabled={!input.trim() || isLoading} loading={isLoading} color="primary">
            <SendIcon className="w-5 h-5" />
          </Button>
        </div>
      </div>
    </section>
  );
});

export default Chat;
