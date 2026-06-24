import { Button, Input } from "@mui/joy";
import {
  ArrowLeftIcon,
  BotIcon,
  MessageSquareIcon,
  SendIcon,
  SettingsIcon,
  Trash2Icon,
  UserIcon,
} from "lucide-react";
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
  const [tenantInput, setTenantInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const {
    messages,
    isLoading,
    error,
    selectedTenantSlug,
    isTenantValidated,
    isTenantLoading,
  } = chatStore.state;

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

  const handleConnect = () => {
    if (tenantInput.trim()) {
      chatStore.validateAndSetTenant(tenantInput.trim());
    }
  };

  const handleTenantKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      handleConnect();
    }
  };

  return (
    <section className="dark @container chat-font w-full max-w-[700px] min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8 bg-zinc-950 text-stone-100 border border-zinc-800 rounded-2xl shadow-sm">
      {!md && <MobileHeader />}
      <div
        className="w-full h-full px-3 sm:px-6 flex flex-col"
        style={{ minHeight: "calc(100vh - 180px)" }}
      >
        {/* Header */}
        <div className="w-full flex flex-row justify-between items-center mb-5 px-1">
          <div className="flex items-center gap-2">
            <MessageSquareIcon className="w-6 h-6 opacity-70" />
            <h1 className="text-xl font-semibold tracking-[-0.02em] text-stone-900 dark:text-stone-100">
              {t("chat.title")}
            </h1>
          </div>
          {isTenantValidated && messages.length > 0 && (
            <Button
              variant="plain"
              color="neutral"
              size="sm"
              startDecorator={<Trash2Icon className="w-4 h-4" />}
              onClick={handleClear}
            >
              {t("common.clear")}
            </Button>
          )}
        </div>

        {/* Show tenant selection or chat based on validation state */}
        {!isTenantValidated ? (
          // Tenant Selection
          <div className="flex-1 w-full flex flex-col items-center justify-center bg-white dark:bg-zinc-900 rounded-2xl shadow-sm border border-stone-200 dark:border-zinc-700 p-8 mb-4">
            <SettingsIcon className="w-12 h-12 mb-4 opacity-30 text-gray-500" />
            <h2 className="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">
              {t("internal-agent.select-tenant")}
            </h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-6 text-center max-w-md">
              {t("internal-agent.select-tenant-description")}
            </p>

            <div className="flex gap-2 w-full max-w-sm">
              <Input
                className="flex-1"
                placeholder={t("internal-agent.tenant-placeholder")}
                value={tenantInput}
                onChange={(e) => setTenantInput(e.target.value)}
                onKeyDown={handleTenantKeyDown}
                disabled={isTenantLoading}
              />
              <Button
                onClick={handleConnect}
                disabled={!tenantInput.trim() || isTenantLoading}
                loading={isTenantLoading}
              >
                {t("internal-agent.connect")}
              </Button>
            </div>

            {error && (
              <p className="mt-4 text-sm text-red-600 dark:text-red-400">
                {error}
              </p>
            )}
          </div>
        ) : (
          // Chat Interface
          <>
            {/* Tenant Header */}
            <div className="w-full flex flex-row items-center gap-3 mb-3 px-2 py-2">
              <Button
                variant="plain"
                color="neutral"
                size="sm"
                startDecorator={<ArrowLeftIcon className="w-4 h-4" />}
                onClick={chatStore.resetTenant}
              >
                {t("internal-agent.disconnect")}
              </Button>
              <span className="h-4 w-px bg-stone-200 dark:bg-zinc-700" />
              <span className="text-sm text-stone-700 dark:text-stone-300 font-medium">
                {selectedTenantSlug}
              </span>
            </div>

            {/* Messages Container */}
            <div className="flex-1 w-full overflow-y-auto bg-transparent rounded-xl p-4 mb-3">
              {messages.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-stone-500 dark:text-stone-400 min-h-[320px]">
                  <MessageSquareIcon className="w-12 h-12 mb-4 opacity-30" />
                  <p className="text-base font-medium">
                    {t("internal-agent.empty-conversation")}
                  </p>
                  <p className="text-sm leading-relaxed mt-2 opacity-70">
                    {t("internal-agent.start-hint")}
                  </p>
                </div>
              ) : (
                <div className="mx-auto flex w-full flex-col gap-3">
                  {messages.map((message, index) => (
                    <div
                      key={index}
                      className={cn(
                        "flex w-full items-start gap-3",
                        message.role === "user" && "flex-row-reverse ml-8",
                        message.role === "assistant" && "mr-8",
                      )}
                    >
                      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-500 shadow-sm dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300">
                        {message.role === "user" ? (
                          <UserIcon className="h-4 w-4" />
                        ) : (
                          <BotIcon className="h-4 w-4" />
                        )}
                      </div>
                      <div
                        className={cn(
                          "min-w-0 rounded-lg border p-3 text-sm leading-relaxed max-w-[85%]",
                          message.role === "user"
                            ? "bg-blue-100 dark:bg-blue-950/40 text-gray-800 dark:text-gray-200 border-blue-200 dark:border-blue-900"
                            : "bg-zinc-100 dark:bg-zinc-800 text-gray-800 dark:text-gray-200 border-zinc-200 dark:border-zinc-700",
                        )}
                      >
                        <div className="flex justify-between items-center mb-1">
                          <span
                            className={cn(
                              "text-xs font-medium",
                              message.role === "user"
                                ? "text-blue-600 dark:text-blue-400"
                                : "text-zinc-600 dark:text-zinc-400",
                            )}
                          >
                            {message.role === "user" ? "Customer" : "Agent"}
                          </span>
                          <span className="text-xs text-zinc-500 dark:text-zinc-400">
                            {message.timestamp.toLocaleTimeString([], {
                              hour: "numeric",
                              minute: "2-digit",
                            })}
                          </span>
                        </div>
                        <p className="whitespace-pre-wrap break-words">
                          {message.content}
                        </p>
                      </div>
                    </div>
                  ))}
                  {isLoading && (
                    <div className="flex items-start gap-3 mr-8">
                      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-500 dark:border-zinc-700 dark:bg-zinc-900">
                        <BotIcon className="h-4 w-4" />
                      </div>
                      <div className="rounded-lg border border-zinc-200 dark:border-zinc-700 bg-zinc-100 dark:bg-zinc-800 p-3 text-sm text-zinc-500 dark:text-zinc-400 max-w-[85%]">
                        <span className="animate-pulse">
                          {t("chat.loading")}
                        </span>
                      </div>
                    </div>
                  )}
                  {error && (
                    <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300">
                      {error}
                    </div>
                  )}
                  <div ref={messagesEndRef} />
                </div>
              )}
            </div>

            {/* Input Area */}
            <div className="w-full rounded-lg border border-zinc-300 dark:border-zinc-700 bg-zinc-100 dark:bg-zinc-800 p-2 transition focus-within:border-teal-500">
              <div className="flex items-end gap-2">
                <textarea
                  className="min-h-10 max-h-36 flex-1 resize-none bg-transparent px-3 py-2 text-sm leading-relaxed font-sans text-gray-800 dark:text-gray-200 outline-none placeholder:text-gray-400 dark:placeholder:text-gray-500 disabled:opacity-60"
                  placeholder={t("internal-agent.placeholder")}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  rows={1}
                  disabled={isLoading}
                />
                <Button
                  onClick={handleSend}
                  disabled={!input.trim() || isLoading}
                  loading={isLoading}
                  color="primary"
                  sx={{
                    minWidth: 36,
                    width: 36,
                    height: 36,
                    borderRadius: 8,
                    alignSelf: "flex-end",
                  }}
                >
                  <SendIcon className="w-4 h-4" />
                </Button>
              </div>
            </div>
          </>
        )}
      </div>
    </section>
  );
});

export default Chat;
