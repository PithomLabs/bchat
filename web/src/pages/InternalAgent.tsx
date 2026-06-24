import { Button, Input, Textarea } from "@mui/joy";
import { BotIcon, MessageSquareIcon, SendIcon, SettingsIcon, Trash2Icon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { KeyboardEvent, useEffect, useRef, useState } from "react";
import MobileHeader from "@/components/MobileHeader";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { agentChatStore } from "@/store/v2";
import { cn } from "@/utils";
import { useTranslate } from "@/utils/i18n";

const InternalAgent = observer(() => {
  const t = useTranslate();
  const { md } = useResponsiveWidth();
  const [input, setInput] = useState("");
  const [tenantInput, setTenantInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const { messages, isLoading, isValidating, error, tenantSlug, tenantCompanyName, metadata } = agentChatStore.state;

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSend = async () => {
    const trimmedInput = input.trim();
    if (!trimmedInput || isLoading) return;

    setInput("");
    await agentChatStore.sendMessage(trimmedInput);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleClear = () => {
    agentChatStore.clearChat();
  };

  const handleSetTenant = async () => {
    const slug = tenantInput.trim();
    if (slug && !isValidating) {
      const success = await agentChatStore.validateAndSetTenant(slug);
      if (success) {
        setTenantInput("");
      }
    }
  };

  const handleDisconnect = () => {
    agentChatStore.disconnectTenant();
  };

  const handleTenantKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleSetTenant();
    }
  };

  return (
    <section className="chat-font @container w-full max-w-[700px] min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full h-full px-4 sm:px-6 flex flex-col" style={{ minHeight: "calc(100vh - 200px)" }}>
        {/* Header */}
        <div className="w-full flex flex-row justify-between items-center mb-4">
          <div className="flex items-center gap-2">
            <BotIcon className="w-6 h-6 opacity-70" />
            <h1 className="text-xl font-semibold text-gray-800 dark:text-gray-200">{t("internal-agent.title")}</h1>
          </div>
          <div className="flex items-center gap-2">
            {tenantSlug && (
              <>
                <span className="text-sm px-2 py-1 bg-teal-100 dark:bg-teal-900 text-teal-700 dark:text-teal-300 rounded">
                  {tenantCompanyName || tenantSlug}
                </span>
                <Button variant="plain" color="neutral" size="sm" onClick={handleDisconnect}>
                  {t("internal-agent.disconnect")}
                </Button>
              </>
            )}
            {messages.length > 0 && (
              <Button variant="plain" color="neutral" size="sm" startDecorator={<Trash2Icon className="w-4 h-4" />} onClick={handleClear}>
                {t("common.clear")}
              </Button>
            )}
          </div>
        </div>

        {/* Tenant Selection (if no tenant selected) */}
        {!tenantSlug && (
          <div className="w-full flex-1 flex flex-col items-center justify-center bg-white dark:bg-zinc-800 rounded-xl shadow-sm border border-gray-200 dark:border-zinc-700 p-8 mb-4">
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
                disabled={isValidating}
              />
              <Button onClick={handleSetTenant} disabled={!tenantInput.trim() || isValidating} loading={isValidating}>
                {t("internal-agent.connect")}
              </Button>
            </div>
            {error && !tenantSlug && (
              <p className="mt-4 text-sm text-red-600 dark:text-red-400">{error}</p>
            )}
          </div>
        )}

        {/* Chat Interface (when tenant is selected) */}
        {tenantSlug && (
          <>
            {/* Metadata Badge */}
            {metadata && (
              <div className="flex gap-2 mb-2 text-xs">
                <span className="px-2 py-1 bg-gray-100 dark:bg-zinc-700 rounded text-gray-600 dark:text-gray-400">
                  Intent: {metadata.intent}
                </span>
                <span className="px-2 py-1 bg-gray-100 dark:bg-zinc-700 rounded text-gray-600 dark:text-gray-400">
                  Phase: {metadata.phase}
                </span>
                {metadata.urgency > 0 && (
                  <span
                    className={cn(
                      "px-2 py-1 rounded",
                      metadata.urgency >= 4
                        ? "bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300"
                        : "bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-300",
                    )}
                  >
                    Urgency: {metadata.urgency}
                  </span>
                )}
              </div>
            )}

            {/* Messages Container */}
            <div className="flex-1 w-full overflow-y-auto bg-transparent p-4 mb-4">
              {messages.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-gray-500 min-h-[300px]">
                  <MessageSquareIcon className="w-12 h-12 mb-4 opacity-30" />
                  <p>{t("internal-agent.empty-conversation")}</p>
                  <p className="text-sm mt-2 text-gray-400">{t("internal-agent.start-hint")}</p>
                </div>
              ) : (
                <div className="flex flex-col gap-3">
                  {messages.map((message, index) => (
                    <div
                      key={index}
                      className={cn(
                        "max-w-[85%] p-3 rounded-lg",
                        message.role === "user"
                          ? "ml-auto ml-8 bg-blue-100 dark:bg-blue-950/40 text-gray-800 dark:text-gray-200 border border-blue-200 dark:border-blue-900"
                          : "mr-auto mr-8 bg-zinc-100 dark:bg-zinc-800 text-gray-800 dark:text-gray-200 border border-zinc-200 dark:border-zinc-700",
                      )}
                    >
                      <div className="flex justify-between items-center mb-1">
                        <span className={cn("text-xs font-medium", message.role === "user" ? "text-blue-600 dark:text-blue-400" : "text-zinc-600 dark:text-zinc-400")}>
                          {message.role === "user" ? "Customer" : "Agent"}
                        </span>
                        <span className="text-xs text-zinc-500 dark:text-zinc-400">{message.timestamp.toLocaleTimeString()}</span>
                      </div>
                      <p className="whitespace-pre-wrap">{message.content}</p>
                    </div>
                  ))}
                  {isLoading && (
                    <div className="mr-auto mr-8 bg-zinc-100 dark:bg-zinc-800 p-3 rounded-lg text-gray-500 max-w-[85%]">
                      <span className="animate-pulse">{t("chat.loading")}</span>
                    </div>
                  )}
                  {error && (
                    <div className="mr-auto mr-8 bg-red-100 dark:bg-red-900/30 p-3 rounded-lg text-red-600 dark:text-red-300 max-w-[85%]">{error}</div>
                  )}
                  <div ref={messagesEndRef} />
                </div>
              )}
            </div>

            {/* Input Area */}
            <div className="w-full flex flex-row gap-2">
              <Textarea
                className="flex-1 bg-transparent text-gray-800 dark:text-gray-200 outline-none placeholder:text-gray-400 dark:placeholder:text-gray-500"
                placeholder={t("internal-agent.placeholder")}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                minRows={1}
                maxRows={4}
                disabled={isLoading}
                sx={{ 
                  '& .MuiTextarea-root': { p: 0 },
                  '& .MuiTextarea-input': { py: 1, px: 2, fontSize: '0.875rem', bgcolor: 'transparent' },
                }}
              />
              <Button onClick={handleSend} disabled={!input.trim() || isLoading} loading={isLoading} color="primary" sx={{ borderRadius: 1, minWidth: 36, height: 36 }}>
                <SendIcon className="w-4 h-4" />
              </Button>
            </div>
          </>
        )}
      </div>
    </section>
  );
});

export default InternalAgent;
