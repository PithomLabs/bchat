import { Button, Checkbox, Dropdown, Input, Menu, MenuButton, MenuItem, Modal, ModalClose, ModalDialog, Textarea } from "@mui/joy";
import toast from "react-hot-toast";
import {
  AlertTriangleIcon,
  ArrowLeftIcon,
  BarChart2Icon,
  BrainIcon,
  CheckCircleIcon,
  ChevronDownIcon,
  FlaskConicalIcon,
  InfoIcon,
  MessageSquareIcon,
  PauseIcon,
  PlayIcon,
  ScrollTextIcon,
  SettingsIcon,
  SquareIcon,
  Trash2Icon,
  UsersIcon,
  XCircleIcon,
} from "lucide-react";
import { observer } from "mobx-react-lite";
import { KeyboardEvent, useEffect, useRef, useState } from "react";
import MobileHeader from "@/components/MobileHeader";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { agentSimulationStore } from "@/store/v2";
import type { ConversationSummary, UnifiedMessage } from "@/store/v2/agentSimulation";
import { cn } from "@/utils";
import { useTranslate } from "@/utils/i18n";

const AgentSimulation = observer(() => {
  const t = useTranslate();
  const { md } = useResponsiveWidth();
  const [tenantInput, setTenantInput] = useState("");
  const [promptInput, setPromptInput] = useState("");
  const [personaInput, setPersonaInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Learning application state
  const [selectedIssues, setSelectedIssues] = useState<number[]>([]);
  const [selectedSuggestions, setSelectedSuggestions] = useState<number[]>([]);
  const [isApplying, setIsApplying] = useState(false);

  const {
    tenantSlug,
    tenantCompanyName,
    llmModel,
    isValidating,
    status,
    messages,
    currentTurn,
    endReason,
    respondingRole,
    error,
    // History state
    conversations,
    selectedConversation,
    viewMode,
    isLoadingHistory,
    // Analysis state
    analysisResult,
    isAnalyzing,
    showAnalysisModal,
  } = agentSimulationStore.state;

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSetTenant = async () => {
    const slug = tenantInput.trim();
    if (slug && !isValidating) {
      const success = await agentSimulationStore.validateAndSetTenant(slug);
      if (success) {
        setTenantInput("");
      }
    }
  };

  const handleTenantKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleSetTenant();
    }
  };

  const handleDisconnect = () => {
    agentSimulationStore.disconnectTenant();
  };

  const handleStartSimulation = async () => {
    const prompt = promptInput.trim();
    if (!prompt) return;

    await agentSimulationStore.startSimulation(prompt, personaInput.trim());
  };

  const handlePause = () => {
    agentSimulationStore.pauseSimulation();
  };

  const handleResume = () => {
    agentSimulationStore.resumeSimulation();
  };

  const handleStop = () => {
    agentSimulationStore.stopSimulation();
  };

  const handleClear = () => {
    agentSimulationStore.clearSimulation();
    setPromptInput("");
    setPersonaInput("");
  };

  const handleSelectConversation = (id: string) => {
    agentSimulationStore.selectConversation(id);
  };

  const handleCloseConversationView = () => {
    agentSimulationStore.closeConversationView();
  };

  const handleAnalyze = () => {
    if (selectedConversation) {
      agentSimulationStore.analyzeConversation(selectedConversation.id);
    }
  };

  const handleCloseAnalysisModal = () => {
    agentSimulationStore.closeAnalysisModal();
    // Reset selections when closing
    setSelectedIssues([]);
    setSelectedSuggestions([]);
  };

  // Toggle issue selection
  const toggleIssueSelection = (index: number) => {
    setSelectedIssues((prev) => (prev.includes(index) ? prev.filter((i) => i !== index) : [...prev, index]));
  };

  // Toggle suggestion selection
  const toggleSuggestionSelection = (index: number) => {
    setSelectedSuggestions((prev) => (prev.includes(index) ? prev.filter((i) => i !== index) : [...prev, index]));
  };

  // Select all issues
  const selectAllIssues = () => {
    if (analysisResult?.issues) {
      setSelectedIssues(analysisResult.issues.map((_, i) => i));
    }
  };

  // Select all suggestions
  const selectAllSuggestions = () => {
    if (analysisResult?.suggestions) {
      setSelectedSuggestions(analysisResult.suggestions.map((_, i) => i));
    }
  };

  // Apply selected learnings
  const handleApplyLearnings = async () => {
    if (!analysisResult || (selectedIssues.length === 0 && selectedSuggestions.length === 0)) {
      return;
    }

    setIsApplying(true);
    const result = await agentSimulationStore.applySelectedLearnings(analysisResult.id, selectedIssues, selectedSuggestions);
    setIsApplying(false);

    if (result.success) {
      toast.success(t("agent-simulation.learnings-applied", { count: result.appliedCount }));
      handleCloseAnalysisModal();
    } else {
      toast.error(result.message);
    }
  };

  const totalSelected = selectedIssues.length + selectedSuggestions.length;

  const formatConversationDate = (date: Date) => {
    return date.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const getConversationTypeLabel = (conv: ConversationSummary) => {
    if (conv.type === "simulation") {
      return t("agent-simulation.conversation-type-simulation");
    }
    if (conv.audienceType === "internal") {
      return t("agent-simulation.conversation-type-internal");
    }
    return t("agent-simulation.conversation-type-external");
  };

  const isRunning = status === "running";
  const isPaused = status === "paused";
  const isIdle = status === "idle";
  const isComplete = status === "completed" || status === "stopped";

  const getEndReasonLabel = (reason: string) => {
    switch (reason) {
      case "llm_detected":
        return t("agent-simulation.end-reason-llm");
      case "phase_closed":
        return t("agent-simulation.end-reason-phase");
      case "keyword_match":
        return t("agent-simulation.end-reason-keyword");
      case "max_turns":
        return t("agent-simulation.end-reason-max-turns");
      case "user_stopped":
        return t("agent-simulation.end-reason-user-stopped");
      default:
        return reason;
    }
  };

  return (
    <section className="@container w-full max-w-4xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full h-full px-4 sm:px-6 flex flex-col" style={{ minHeight: "calc(100vh - 200px)" }}>
        {/* Header */}
        <div className="w-full flex flex-row justify-between items-center mb-4">
          <div className="flex items-center gap-2">
            <FlaskConicalIcon className="w-6 h-6 opacity-70" />
            <h1 className="text-xl font-semibold text-gray-800 dark:text-gray-200">{t("agent-simulation.title")}</h1>
          </div>
          <div className="flex items-center gap-2">
            {tenantSlug && (
              <>
                <span className="text-sm px-2 py-1 bg-purple-100 dark:bg-purple-900 text-purple-700 dark:text-purple-300 rounded">
                  {tenantCompanyName || tenantSlug}
                </span>
                {llmModel && (
                  <span className="text-xs px-2 py-1 bg-gray-100 dark:bg-zinc-700 text-gray-600 dark:text-gray-300 rounded flex items-center gap-1" title="LLM Model">
                    <BrainIcon className="w-3 h-3" />
                    {llmModel.split("/").pop()}
                  </span>
                )}
                {/* History Dropdown */}
                {conversations.length > 0 && viewMode === "simulation" && (
                  <Dropdown>
                    <MenuButton
                      variant="outlined"
                      color="neutral"
                      size="sm"
                      endDecorator={<ChevronDownIcon className="w-4 h-4" />}
                      startDecorator={<ScrollTextIcon className="w-4 h-4" />}
                      loading={isLoadingHistory}
                      disabled={isRunning || isPaused}
                    >
                      {t("agent-simulation.history")} ({conversations.length})
                    </MenuButton>
                    <Menu placement="bottom-end" sx={{ maxHeight: 400, overflow: "auto", minWidth: 320 }}>
                      <MenuItem disabled sx={{ fontWeight: "bold", opacity: 1 }}>
                        {t("agent-simulation.recent-conversations")}
                      </MenuItem>
                      {conversations.map((conv) => (
                        <MenuItem key={conv.id} onClick={() => handleSelectConversation(conv.id)}>
                          <div className="flex flex-col w-full">
                            <div className="flex items-center gap-2">
                              {conv.type === "simulation" ? (
                                <FlaskConicalIcon className="w-4 h-4 text-purple-600" />
                              ) : (
                                <MessageSquareIcon className="w-4 h-4 text-green-600" />
                              )}
                              <span className="truncate flex-1">{conv.summary}</span>
                              <span className="text-xs text-gray-500">{conv.messageCount} msgs</span>
                            </div>
                            <span className="text-xs text-gray-400 ml-6">
                              {getConversationTypeLabel(conv)} • {formatConversationDate(conv.createdAt)}
                            </span>
                          </div>
                        </MenuItem>
                      ))}
                      {conversations.length === 0 && (
                        <MenuItem disabled>
                          <span className="text-gray-500">{t("agent-simulation.no-conversations")}</span>
                        </MenuItem>
                      )}
                    </Menu>
                  </Dropdown>
                )}
                <Button variant="plain" color="neutral" size="sm" onClick={handleDisconnect} disabled={isRunning || isPaused}>
                  {t("internal-agent.disconnect")}
                </Button>
              </>
            )}
            {(messages.length > 0 || isComplete) && viewMode === "simulation" && (
              <Button
                variant="plain"
                color="neutral"
                size="sm"
                startDecorator={<Trash2Icon className="w-4 h-4" />}
                onClick={handleClear}
                disabled={isRunning}
              >
                {t("common.clear")}
              </Button>
            )}
          </div>
        </div>

        {/* Viewing Mode - Show selected conversation */}
        {viewMode === "viewing" && selectedConversation && (
          <>
            {/* Back button and info */}
            <div className="w-full bg-gray-50 dark:bg-zinc-700 rounded-lg p-4 mb-4">
              <div className="flex justify-between items-start mb-3">
                <Button
                  variant="plain"
                  color="neutral"
                  size="sm"
                  startDecorator={<ArrowLeftIcon className="w-4 h-4" />}
                  onClick={handleCloseConversationView}
                >
                  {t("agent-simulation.back-to-simulation")}
                </Button>
                <Button
                  variant="solid"
                  color="primary"
                  size="sm"
                  startDecorator={<BarChart2Icon className="w-4 h-4" />}
                  onClick={handleAnalyze}
                  loading={isAnalyzing}
                  disabled={isAnalyzing}
                >
                  {t("agent-simulation.analyze")}
                </Button>
              </div>
              <div className="space-y-1">
                <h3 className="text-md font-medium text-gray-700 dark:text-gray-300">
                  {selectedConversation.type === "simulation"
                    ? t("agent-simulation.viewing-simulation", { date: formatConversationDate(selectedConversation.createdAt) })
                    : t("agent-simulation.viewing-chat", { name: selectedConversation.customerName || "Unknown" })}
                </h3>
                {selectedConversation.type === "simulation" && selectedConversation.initialPrompt && (
                  <p className="text-sm text-gray-500 dark:text-gray-400 truncate">
                    {t("agent-simulation.initial-prompt")}: "{selectedConversation.initialPrompt}"
                  </p>
                )}
                <div className="flex gap-4 text-xs text-gray-500">
                  <span>{selectedConversation.messages.length} messages</span>
                  {selectedConversation.type === "simulation" && selectedConversation.endReason && (
                    <span>{getEndReasonLabel(selectedConversation.endReason)}</span>
                  )}
                  {selectedConversation.type === "chat" && selectedConversation.phase && (
                    <span>Phase: {selectedConversation.phase}</span>
                  )}
                </div>
              </div>
            </div>

            {/* Messages */}
            <div className="flex-1 w-full overflow-y-auto bg-white dark:bg-zinc-800 rounded-xl shadow-sm border border-gray-200 dark:border-zinc-700 p-4 mb-4 min-h-[400px]">
              <div className="flex flex-col gap-4">
                {selectedConversation.messages.map((message: UnifiedMessage, index: number) => (
                  <div
                    key={index}
                    className={cn(
                      "max-w-[85%] p-4 rounded-xl",
                      message.role === "human"
                        ? "ml-auto bg-blue-100 dark:bg-blue-900/30 text-blue-900 dark:text-blue-100"
                        : "mr-auto bg-teal-500 dark:bg-teal-600 text-white"
                    )}
                  >
                    {/* Role badge */}
                    <div className="flex items-center gap-2 mb-2">
                      <span
                        className={cn(
                          "text-xs font-medium px-2 py-0.5 rounded",
                          message.role === "human"
                            ? "bg-blue-200 dark:bg-blue-800 text-blue-800 dark:text-blue-200"
                            : "bg-teal-600 dark:bg-teal-700 text-white"
                        )}
                      >
                        {message.role === "human" ? t("agent-simulation.human-label") : t("agent-simulation.agent-label")}
                      </span>
                      {message.turnNum && (
                        <span className="text-xs opacity-60">
                          {t("agent-simulation.turn")} {message.turnNum}
                        </span>
                      )}
                    </div>

                    {/* Content */}
                    <p className="whitespace-pre-wrap">{message.content}</p>

                    {/* Metadata (for agent messages) */}
                    {message.role === "agent" && message.metadata && (
                      <div className="flex gap-2 mt-2 text-xs opacity-70">
                        {message.metadata.intent && <span>Intent: {message.metadata.intent}</span>}
                        {message.metadata.phase && <span>Phase: {message.metadata.phase}</span>}
                      </div>
                    )}

                    {/* Timestamp */}
                    <span className="text-xs opacity-50 mt-2 block">{message.timestamp.toLocaleTimeString()}</span>
                  </div>
                ))}
                <div ref={messagesEndRef} />
              </div>
            </div>
          </>
        )}

        {/* Tenant Selection (if no tenant selected) */}
        {!tenantSlug && viewMode === "simulation" && (
          <div className="w-full flex-1 flex flex-col items-center justify-center bg-white dark:bg-zinc-800 rounded-xl shadow-sm border border-gray-200 dark:border-zinc-700 p-8 mb-4">
            <SettingsIcon className="w-12 h-12 mb-4 opacity-30 text-gray-500" />
            <h2 className="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">{t("agent-simulation.select-tenant")}</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-6 text-center max-w-md">
              {t("agent-simulation.select-tenant-description")}
            </p>
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
            {error && !tenantSlug && <p className="mt-4 text-sm text-red-600 dark:text-red-400">{error}</p>}
          </div>
        )}

        {/* Simulation Interface (when tenant is selected and in simulation mode) */}
        {tenantSlug && viewMode === "simulation" && (
          <>
            {/* Configuration (when idle) */}
            {isIdle && (
              <div className="w-full bg-white dark:bg-zinc-800 rounded-xl shadow-sm border border-gray-200 dark:border-zinc-700 p-6 mb-4">
                <h3 className="text-md font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                  <UsersIcon className="w-5 h-5" />
                  {t("agent-simulation.configure")}
                </h3>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">
                      {t("agent-simulation.initial-prompt")} *
                    </label>
                    <Textarea
                      placeholder={t("agent-simulation.initial-prompt-placeholder")}
                      value={promptInput}
                      onChange={(e) => setPromptInput(e.target.value)}
                      minRows={2}
                      maxRows={4}
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">
                      {t("agent-simulation.persona-hint")}
                    </label>
                    <Input
                      placeholder={t("agent-simulation.persona-hint-placeholder")}
                      value={personaInput}
                      onChange={(e) => setPersonaInput(e.target.value)}
                    />
                  </div>

                  <Button
                    color="primary"
                    onClick={handleStartSimulation}
                    disabled={!promptInput.trim()}
                    startDecorator={<PlayIcon className="w-4 h-4" />}
                    className="w-full"
                  >
                    {t("agent-simulation.start")}
                  </Button>
                </div>
              </div>
            )}

            {/* Status Bar (when running/paused/complete) */}
            {!isIdle && (
              <div className="w-full flex items-center justify-between bg-gray-50 dark:bg-zinc-700 rounded-lg p-3 mb-4">
                <div className="flex items-center gap-4">
                  <div className="flex items-center gap-2">
                    <div
                      className={cn(
                        "w-3 h-3 rounded-full",
                        isRunning && "bg-green-500 animate-pulse",
                        isPaused && "bg-yellow-500",
                        isComplete && (endReason === "user_stopped" ? "bg-red-500" : "bg-blue-500")
                      )}
                    />
                    <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {isRunning && t("agent-simulation.status-running")}
                      {isPaused && t("agent-simulation.status-paused")}
                      {isComplete && t("agent-simulation.status-completed")}
                    </span>
                  </div>
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    {t("agent-simulation.turn")} {currentTurn}/50
                  </span>
                  {isComplete && endReason && (
                    <span className="text-xs px-2 py-1 bg-gray-200 dark:bg-zinc-600 rounded text-gray-600 dark:text-gray-300">
                      {getEndReasonLabel(endReason)}
                    </span>
                  )}
                </div>

                <div className="flex gap-2">
                  {isRunning && (
                    <Button variant="outlined" color="warning" size="sm" onClick={handlePause} startDecorator={<PauseIcon className="w-4 h-4" />}>
                      {t("agent-simulation.pause")}
                    </Button>
                  )}
                  {isPaused && (
                    <Button variant="outlined" color="success" size="sm" onClick={handleResume} startDecorator={<PlayIcon className="w-4 h-4" />}>
                      {t("agent-simulation.resume")}
                    </Button>
                  )}
                  {(isRunning || isPaused) && (
                    <Button variant="outlined" color="danger" size="sm" onClick={handleStop} startDecorator={<SquareIcon className="w-4 h-4" />}>
                      {t("agent-simulation.stop")}
                    </Button>
                  )}
                </div>
              </div>
            )}

            {/* Messages Container */}
            {!isIdle && (
              <div className="flex-1 w-full overflow-y-auto bg-white dark:bg-zinc-800 rounded-xl shadow-sm border border-gray-200 dark:border-zinc-700 p-4 mb-4 min-h-[400px]">
                {messages.length === 0 && isRunning ? (
                  <div className="h-full flex flex-col items-center justify-center text-gray-500">
                    <FlaskConicalIcon className="w-12 h-12 mb-4 opacity-30 animate-pulse" />
                    <p>{t("agent-simulation.starting")}</p>
                  </div>
                ) : (
                  <div className="flex flex-col gap-4">
                    {messages.map((message, index) => (
                      <div
                        key={index}
                        className={cn(
                          "max-w-[85%] p-4 rounded-xl",
                          message.role === "human_sim"
                            ? "ml-auto bg-blue-100 dark:bg-blue-900/30 text-blue-900 dark:text-blue-100"
                            : "mr-auto bg-teal-500 dark:bg-teal-600 text-white"
                        )}
                      >
                        {/* Role badge */}
                        <div className="flex items-center gap-2 mb-2">
                          <span
                            className={cn(
                              "text-xs font-medium px-2 py-0.5 rounded",
                              message.role === "human_sim"
                                ? "bg-blue-200 dark:bg-blue-800 text-blue-800 dark:text-blue-200"
                                : "bg-teal-600 dark:bg-teal-700 text-white"
                            )}
                          >
                            {message.role === "human_sim" ? t("agent-simulation.human-label") : t("agent-simulation.agent-label")}
                          </span>
                          <span className="text-xs opacity-60">{t("agent-simulation.turn")} {message.turn_num}</span>
                        </div>

                        {/* Content */}
                        <p className="whitespace-pre-wrap">{message.content}</p>

                        {/* Metadata (for agent messages) */}
                        {message.role === "agent" && message.metadata && (
                          <div className="flex gap-2 mt-2 text-xs opacity-70">
                            {message.metadata.intent && <span>Intent: {message.metadata.intent}</span>}
                            {message.metadata.phase && <span>Phase: {message.metadata.phase}</span>}
                          </div>
                        )}

                        {/* Timestamp */}
                        <span className="text-xs opacity-50 mt-2 block">{message.timestamp.toLocaleTimeString()}</span>
                      </div>
                    ))}

                    {/* Responding indicator */}
                    {respondingRole && (
                      <div
                        className={cn(
                          "max-w-[85%] p-4 rounded-xl animate-pulse",
                          respondingRole === "human_sim"
                            ? "ml-auto bg-blue-100 dark:bg-blue-900/30 text-blue-900 dark:text-blue-100"
                            : "mr-auto bg-teal-500 dark:bg-teal-600 text-white"
                        )}
                      >
                        <span className="text-sm">
                          {respondingRole === "human_sim"
                            ? t("agent-simulation.human-responding")
                            : t("agent-simulation.agent-responding")}
                        </span>
                      </div>
                    )}

                    {error && (
                      <div className="mr-auto bg-red-100 dark:bg-red-900/30 p-3 rounded-lg text-red-600 dark:text-red-300">{error}</div>
                    )}

                    <div ref={messagesEndRef} />
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>

      {/* Analysis Results Modal */}
      <Modal open={showAnalysisModal} onClose={handleCloseAnalysisModal}>
        <ModalDialog
          sx={{
            maxWidth: 600,
            maxHeight: "90vh",
            overflow: "auto",
          }}
        >
          <ModalClose />
          <div className="p-4">
            <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-4 flex items-center gap-2">
              <BarChart2Icon className="w-5 h-5" />
              {t("agent-simulation.analysis-title")}
            </h2>

            {analysisResult && (
              <div className="space-y-6">
                {/* Overall Score */}
                <div className="text-center p-4 bg-gray-50 dark:bg-zinc-700 rounded-lg">
                  <div className="text-4xl font-bold text-gray-800 dark:text-gray-200">
                    {analysisResult.score}/100
                  </div>
                  <div
                    className={cn(
                      "inline-block mt-2 px-3 py-1 rounded text-lg font-semibold",
                      analysisResult.grade === "A" && "bg-green-100 text-green-800 dark:bg-green-800 dark:text-green-100",
                      analysisResult.grade === "B" && "bg-blue-100 text-blue-800 dark:bg-blue-800 dark:text-blue-100",
                      analysisResult.grade === "C" && "bg-yellow-100 text-yellow-800 dark:bg-yellow-800 dark:text-yellow-100",
                      analysisResult.grade === "D" && "bg-orange-100 text-orange-800 dark:bg-orange-800 dark:text-orange-100",
                      analysisResult.grade === "F" && "bg-red-100 text-red-800 dark:bg-red-800 dark:text-red-100"
                    )}
                  >
                    {t("agent-simulation.grade")}: {analysisResult.grade}
                  </div>
                </div>

                {/* Breakdown */}
                <div>
                  <h3 className="text-md font-medium text-gray-700 dark:text-gray-300 mb-3">
                    {t("agent-simulation.breakdown")}
                  </h3>
                  <div className="space-y-3">
                    {Object.entries(analysisResult.breakdown).map(([key, category]) => (
                      <div key={key} className="space-y-1">
                        <div className="flex justify-between text-sm">
                          <span className="text-gray-600 dark:text-gray-400 capitalize">
                            {key.replace(/_/g, " ")}
                          </span>
                          <span className="font-medium text-gray-800 dark:text-gray-200">
                            {category.score}/{category.max}
                          </span>
                        </div>
                        <div className="w-full bg-gray-200 dark:bg-zinc-600 rounded-full h-2">
                          <div
                            className={cn(
                              "h-2 rounded-full transition-all",
                              category.score / category.max >= 0.8 && "bg-green-500",
                              category.score / category.max >= 0.6 && category.score / category.max < 0.8 && "bg-blue-500",
                              category.score / category.max >= 0.4 && category.score / category.max < 0.6 && "bg-yellow-500",
                              category.score / category.max < 0.4 && "bg-red-500"
                            )}
                            style={{ width: `${(category.score / category.max) * 100}%` }}
                          />
                        </div>
                        {category.notes && (
                          <p className="text-xs text-gray-500 dark:text-gray-400">{category.notes}</p>
                        )}
                      </div>
                    ))}
                  </div>
                </div>

                {/* Issues with checkboxes */}
                {analysisResult.issues && analysisResult.issues.length > 0 && (
                  <div>
                    <div className="flex justify-between items-center mb-3">
                      <h3 className="text-md font-medium text-gray-700 dark:text-gray-300">
                        {t("agent-simulation.issues-found")}
                      </h3>
                      <Button size="sm" variant="plain" onClick={selectAllIssues}>
                        {t("agent-simulation.select-all")}
                      </Button>
                    </div>
                    <div className="space-y-2">
                      {analysisResult.issues.map((issue, index) => (
                        <div
                          key={index}
                          className={cn(
                            "flex items-start gap-2 p-2 rounded text-sm cursor-pointer",
                            issue.severity === "critical" && "bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-200",
                            issue.severity === "warning" && "bg-yellow-50 dark:bg-yellow-900/20 text-yellow-800 dark:text-yellow-200",
                            issue.severity === "info" && "bg-blue-50 dark:bg-blue-900/20 text-blue-800 dark:text-blue-200",
                            selectedIssues.includes(index) && "ring-2 ring-blue-500"
                          )}
                          onClick={() => toggleIssueSelection(index)}
                        >
                          <Checkbox
                            size="sm"
                            checked={selectedIssues.includes(index)}
                            onChange={() => toggleIssueSelection(index)}
                            onClick={(e) => e.stopPropagation()}
                          />
                          {issue.severity === "critical" && <XCircleIcon className="w-4 h-4 flex-shrink-0 mt-0.5" />}
                          {issue.severity === "warning" && <AlertTriangleIcon className="w-4 h-4 flex-shrink-0 mt-0.5" />}
                          {issue.severity === "info" && <InfoIcon className="w-4 h-4 flex-shrink-0 mt-0.5" />}
                          <div className="flex-1">
                            <span className="font-medium">{t("agent-simulation.turn")} {issue.turn}:</span>{" "}
                            {issue.message}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Suggestions with checkboxes */}
                {analysisResult.suggestions && analysisResult.suggestions.length > 0 && (
                  <div>
                    <div className="flex justify-between items-center mb-3">
                      <h3 className="text-md font-medium text-gray-700 dark:text-gray-300">
                        {t("agent-simulation.suggestions")}
                      </h3>
                      <Button size="sm" variant="plain" onClick={selectAllSuggestions}>
                        {t("agent-simulation.select-all")}
                      </Button>
                    </div>
                    <div className="space-y-2">
                      {analysisResult.suggestions.map((suggestion, index) => (
                        <div
                          key={index}
                          className={cn(
                            "flex items-start gap-2 text-sm text-gray-600 dark:text-gray-400 p-2 rounded cursor-pointer bg-green-50 dark:bg-green-900/20",
                            selectedSuggestions.includes(index) && "ring-2 ring-blue-500"
                          )}
                          onClick={() => toggleSuggestionSelection(index)}
                        >
                          <Checkbox
                            size="sm"
                            checked={selectedSuggestions.includes(index)}
                            onChange={() => toggleSuggestionSelection(index)}
                            onClick={(e) => e.stopPropagation()}
                          />
                          <CheckCircleIcon className="w-4 h-4 flex-shrink-0 mt-0.5 text-green-600" />
                          <span className="flex-1">{suggestion}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Apply Button */}
                {((analysisResult.issues && analysisResult.issues.length > 0) ||
                  (analysisResult.suggestions && analysisResult.suggestions.length > 0)) && (
                  <div className="pt-4 border-t border-gray-200 dark:border-zinc-600">
                    <Button
                      fullWidth
                      color="primary"
                      disabled={totalSelected === 0}
                      loading={isApplying}
                      onClick={handleApplyLearnings}
                    >
                      {totalSelected > 0
                        ? t("agent-simulation.apply-selected", { count: totalSelected })
                        : t("agent-simulation.select-to-apply")}
                    </Button>
                    <p className="text-xs text-gray-500 dark:text-gray-400 text-center mt-2">
                      {t("agent-simulation.apply-hint")}
                    </p>
                  </div>
                )}

                {/* Benchmark Version */}
                <div className="text-xs text-gray-400 dark:text-gray-500 text-center pt-2 border-t border-gray-200 dark:border-zinc-600">
                  {t("agent-simulation.benchmark-version")}: {analysisResult.benchmarkVersion}
                </div>
              </div>
            )}
          </div>
        </ModalDialog>
      </Modal>
    </section>
  );
});

export default AgentSimulation;
