import { Button, Input, Textarea } from "@mui/joy";
import { ArrowLeftIcon, MessageSquareIcon, SendIcon, SettingsIcon, Trash2Icon } from "lucide-react";
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

  const { messages, isLoading, error, selectedTenantSlug, isTenantValidated, isTenantLoading } = chatStore.state;

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
    <section className="@container w-full max-w-4xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full h-full px-4 sm:px-6 flex flex-col" style={{ minHeight: "calc(100vh - 200px)" }}>
        {/* Header */}
        <div className="w-full flex flex-row justify-between items-center mb-4">
          <div className="flex items-center gap-2">
            <MessageSquareIcon className="w-6 h-6 opacity-70" />
            <h1 className="text-xl font-semibold text-gray-800 dark:text-gray-200">{t("chat.title")}</h1>
          </div>
          {isTenantValidated && messages.length > 0 && (
            <Button variant="plain" color="neutral" size="sm" startDecorator={<Trash2Icon className="w-4 h-4" />} onClick={handleClear}>
              {t("common.clear")}
            </Button>
          )}
        </div>

        {/* Show tenant selection or chat based on validation state */}
        {!isTenantValidated ? (
          // Tenant Selection
          <div className="flex-1 w-full flex flex-col items-center justify-center bg-white dark:bg-zinc-800 rounded-xl shadow-sm border border-gray-200 dark:border-zinc-700 p-8 mb-4">
            <SettingsIcon className="w-12 h-12 mb-4 opacity-30 text-gray-500" />
            <h2 className="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">{t("internal-agent.select-tenant")}</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-6 text-center max-w-md">{t("internal-agent.select-tenant-description")}</p>

            <div className="flex gap-2 w-full max-w-sm">
              <Input
                className="flex-1"
                placeholder={t("internal-agent.tenant-placeholder")}
                value={tenantInput}
                onChange={(e) => setTenantInput(e.target.value)}
                onKeyDown={handleTenantKeyDown}
                disabled={isTenantLoading}
              />
              <Button onClick={handleConnect} disabled={!tenantInput.trim() || isTenantLoading} loading={isTenantLoading}>
                {t("internal-agent.connect")}
              </Button>
            </div>

            {error && <p className="mt-4 text-sm text-red-600 dark:text-red-400">{error}</p>}
          </div>
        ) : (
          // Chat Interface
          <>
            {/* Tenant Header */}
            <div className="w-full flex flex-row items-center gap-3 mb-4 p-3 bg-white dark:bg-zinc-800 rounded-lg border border-gray-200 dark:border-zinc-700">
              <Button variant="plain" color="neutral" size="sm" startDecorator={<ArrowLeftIcon className="w-4 h-4" />} onClick={chatStore.resetTenant}>
                {t("internal-agent.disconnect")}
              </Button>
              <span className="text-gray-300 dark:text-gray-600">|</span>
              <span className="text-gray-700 dark:text-gray-300 font-medium">{selectedTenantSlug}</span>
            </div>

            {/* Messages Container */}
            <div className="flex-1 w-full overflow-y-auto bg-white dark:bg-zinc-800 rounded-xl shadow-sm border border-gray-200 dark:border-zinc-700 p-4 mb-4">
              {messages.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-gray-500 min-h-[300px]">
                  <MessageSquareIcon className="w-12 h-12 mb-4 opacity-30" />
                  <p>{t("internal-agent.empty-conversation")}</p>
                  <p className="text-sm mt-2 opacity-70">{t("internal-agent.start-hint")}</p>
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
                          : "mr-auto bg-gray-100 dark:bg-zinc-700 text-gray-800 dark:text-gray-200"
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
                  {error && <div className="mr-auto bg-red-100 dark:bg-red-900/30 p-3 rounded-lg text-red-600 dark:text-red-300">{error}</div>}
                  <div ref={messagesEndRef} />
                </div>
              )}
            </div>

            {/* Input Area */}
            <div className="w-full flex flex-row gap-2">
              <Textarea
                className="flex-1"
                placeholder={t("internal-agent.placeholder")}
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
          </>
        )}
      </div>
    </section>
  );
});

export default Chat;
