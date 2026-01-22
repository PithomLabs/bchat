import axios from "axios";
import { makeAutoObservable, runInAction } from "mobx";

export interface SimulationMetadata {
  intent?: string;
  phase?: string;
  urgency?: number;
}

export interface SimulationMessage {
  role: "human_sim" | "agent";
  content: string;
  turn_num: number;
  timestamp: Date;
  metadata?: SimulationMetadata;
}

export type SimulationStatus = "idle" | "running" | "paused" | "completed" | "stopped";

// Unified conversation types for history
export interface ConversationSummary {
  id: string;
  type: "simulation" | "chat";
  summary: string;
  messageCount: number;
  createdAt: Date;
  // Simulation-specific
  endReason?: string;
  personaHint?: string;
  // Chat-specific
  phase?: string;
  audienceType?: string;
  customerName?: string;
}

export interface UnifiedMessage {
  role: "human" | "agent";
  content: string;
  timestamp: Date;
  turnNum?: number;
  metadata?: SimulationMetadata;
}

export interface ConversationDetail {
  id: string;
  type: "simulation" | "chat";
  messages: UnifiedMessage[];
  createdAt: Date;
  // Simulation-specific
  initialPrompt?: string;
  personaHint?: string;
  totalTurns?: number;
  endReason?: string;
  // Chat-specific
  audienceType?: string;
  customerName?: string;
  phase?: string;
  currentIntent?: string;
}

export type ViewMode = "simulation" | "viewing";

// Analysis types
export interface AnalysisCategoryScore {
  score: number;
  max: number;
  notes: string;
}

export interface AnalysisBreakdown {
  intent_recognition: AnalysisCategoryScore;
  service_alignment: AnalysisCategoryScore;
  policy_compliance: AnalysisCategoryScore;
  conversation_flow: AnalysisCategoryScore;
  information_gathering: AnalysisCategoryScore;
  tone_resolution: AnalysisCategoryScore;
}

export interface AnalysisIssue {
  severity: "critical" | "warning" | "info";
  turn: number;
  message: string;
}

export interface AnalysisResult {
  id: string;
  conversationId: string;
  conversationType: "simulation" | "chat";
  score: number;
  grade: string;
  breakdown: AnalysisBreakdown;
  issues: AnalysisIssue[];
  suggestions?: string[];
  benchmarkVersion: string;
  createdAt: Date;
}

class LocalState {
  // Tenant state
  tenantSlug: string | null = null;
  tenantCompanyName: string | null = null;
  llmModel: string | null = null;
  isValidating: boolean = false;

  // Simulation state
  sessionId: string | null = null;
  status: SimulationStatus = "idle";
  messages: SimulationMessage[] = [];
  currentTurn: number = 0;
  endReason: string | null = null;

  // Configuration
  initialPrompt: string = "";
  personaHint: string = "";

  // UI state
  respondingRole: "human_sim" | "agent" | null = null;
  error: string | null = null;

  // SSE connection
  eventSource: EventSource | null = null;

  // Conversation history state
  conversations: ConversationSummary[] = [];
  selectedConversation: ConversationDetail | null = null;
  viewMode: ViewMode = "simulation";
  isLoadingHistory: boolean = false;
  historyPermissions: { canViewSimulations: boolean; canViewChats: boolean } = {
    canViewSimulations: false,
    canViewChats: false,
  };

  // Analysis state
  analysisResult: AnalysisResult | null = null;
  isAnalyzing: boolean = false;
  showAnalysisModal: boolean = false;

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const agentSimulationStore = (() => {
  const state = new LocalState();

  // Fetch LLM config for the tenant (called after validation)
  const fetchLLMConfig = async (slug: string) => {
    try {
      const response = await axios.get(`/api/v1/agent/${slug}/llm-config`);
      runInAction(() => {
        state.llmModel = response.data.llm_model || null;
      });
    } catch (error: any) {
      // Silently fail - LLM config is optional info
      console.debug("Could not fetch LLM config:", error.message);
    }
  };

  const validateAndSetTenant = async (slug: string): Promise<boolean> => {
    state.setPartial({
      isValidating: true,
      error: null,
    });

    try {
      const response = await axios.get<{ valid: boolean; companyName: string; message?: string }>(
        `/api/v1/agent/${slug}/validate`
      );

      if (response.data.valid) {
        state.setPartial({
          tenantSlug: slug,
          tenantCompanyName: response.data.companyName,
          llmModel: null, // Reset until fetched
          messages: [],
          sessionId: null,
          status: "idle",
          error: null,
          isValidating: false,
          currentTurn: 0,
          endReason: null,
          // Reset history state
          conversations: [],
          selectedConversation: null,
          viewMode: "simulation",
        });
        // Fetch conversation history and LLM config in background (don't await)
        fetchConversationHistory();
        fetchLLMConfig(slug);
        return true;
      } else {
        state.setPartial({
          isValidating: false,
          error: response.data.message || "Tenant not found or inactive",
        });
        return false;
      }
    } catch (error: any) {
      const status = error.response?.status;
      let errorMessage = error.response?.data?.message || "Tenant not found";

      if (status === 403) {
        errorMessage = error.response?.data?.message || "Permission denied";
      }

      state.setPartial({
        isValidating: false,
        error: errorMessage,
      });
      return false;
    }
  };

  const startSimulation = async (initialPrompt: string, personaHint?: string): Promise<boolean> => {
    if (!state.tenantSlug) {
      state.setPartial({ error: "No tenant selected" });
      return false;
    }

    state.setPartial({
      status: "running",
      error: null,
      messages: [],
      currentTurn: 0,
      endReason: null,
      initialPrompt,
      personaHint: personaHint || "",
    });

    try {
      const response = await axios.post<{ session_id: string; status: string; stream_url: string }>(
        `/api/v1/agent/${state.tenantSlug}/simulate`,
        {
          initial_prompt: initialPrompt,
          persona_hint: personaHint || "",
        }
      );

      state.setPartial({
        sessionId: response.data.session_id,
      });

      // Connect to SSE stream
      connectToStream(response.data.stream_url);
      return true;
    } catch (error: any) {
      state.setPartial({
        status: "idle",
        error: error.response?.data?.message || "Failed to start simulation",
      });
      return false;
    }
  };

  const connectToStream = (streamUrl: string) => {
    // Close existing connection
    if (state.eventSource) {
      state.eventSource.close();
    }

    const eventSource = new EventSource(streamUrl);
    state.setPartial({ eventSource });

    eventSource.addEventListener("message", (event) => {
      try {
        const data = JSON.parse(event.data);
        const msg: SimulationMessage = {
          role: data.role,
          content: data.content,
          turn_num: data.turn_num,
          timestamp: new Date(data.timestamp),
          metadata: data.metadata,
        };

        runInAction(() => {
          state.messages = [...state.messages, msg];
          state.respondingRole = null;
        });
      } catch (e) {
        console.error("Failed to parse message:", e);
      }
    });

    eventSource.addEventListener("status", (event) => {
      try {
        const data = JSON.parse(event.data);
        runInAction(() => {
          state.status = data.status as SimulationStatus;
          state.currentTurn = data.current_turn;
          state.respondingRole = data.responding_role || null;
        });
      } catch (e) {
        console.error("Failed to parse status:", e);
      }
    });

    eventSource.addEventListener("complete", (event) => {
      try {
        const data = JSON.parse(event.data);
        runInAction(() => {
          state.status = data.status as SimulationStatus;
          state.currentTurn = data.total_turns;
          state.endReason = data.end_reason;
          state.respondingRole = null;
        });
        eventSource.close();
        state.setPartial({ eventSource: null });
      } catch (e) {
        console.error("Failed to parse complete:", e);
      }
    });

    eventSource.addEventListener("error", () => {
      runInAction(() => {
        if (state.status === "running") {
          state.error = "Connection lost. Simulation may still be running.";
        }
      });
    });
  };

  const pauseSimulation = async () => {
    if (!state.tenantSlug || !state.sessionId) return;

    try {
      await axios.post(`/api/v1/agent/${state.tenantSlug}/simulate/${state.sessionId}/control`, {
        action: "pause",
      });
    } catch (error: any) {
      state.setPartial({
        error: error.response?.data?.message || "Failed to pause simulation",
      });
    }
  };

  const resumeSimulation = async () => {
    if (!state.tenantSlug || !state.sessionId) return;

    try {
      await axios.post(`/api/v1/agent/${state.tenantSlug}/simulate/${state.sessionId}/control`, {
        action: "resume",
      });
    } catch (error: any) {
      state.setPartial({
        error: error.response?.data?.message || "Failed to resume simulation",
      });
    }
  };

  const stopSimulation = async () => {
    if (!state.tenantSlug || !state.sessionId) return;

    try {
      await axios.post(`/api/v1/agent/${state.tenantSlug}/simulate/${state.sessionId}/control`, {
        action: "stop",
      });
    } catch (error: any) {
      state.setPartial({
        error: error.response?.data?.message || "Failed to stop simulation",
      });
    }
  };

  const clearSimulation = () => {
    if (state.eventSource) {
      state.eventSource.close();
    }
    state.setPartial({
      messages: [],
      sessionId: null,
      status: "idle",
      error: null,
      currentTurn: 0,
      endReason: null,
      respondingRole: null,
      eventSource: null,
      initialPrompt: "",
      personaHint: "",
    });
  };

  const disconnectTenant = () => {
    if (state.eventSource) {
      state.eventSource.close();
    }
    state.setPartial({
      tenantSlug: null,
      tenantCompanyName: null,
      llmModel: null,
      messages: [],
      sessionId: null,
      status: "idle",
      error: null,
      currentTurn: 0,
      endReason: null,
      respondingRole: null,
      eventSource: null,
      initialPrompt: "",
      personaHint: "",
      // Clear history state
      conversations: [],
      selectedConversation: null,
      viewMode: "simulation",
      historyPermissions: { canViewSimulations: false, canViewChats: false },
    });
  };

  const clearError = () => {
    state.setPartial({ error: null });
  };

  // Fetch conversation history (simulations + chat sessions based on permissions)
  const fetchConversationHistory = async (): Promise<void> => {
    if (!state.tenantSlug) return;

    state.setPartial({ isLoadingHistory: true });

    try {
      const response = await axios.get<{
        conversations: Array<{
          id: string;
          type: "simulation" | "chat";
          summary: string;
          messageCount: number;
          createdAt: string;
          endReason?: string;
          personaHint?: string;
          phase?: string;
          audienceType?: string;
          customerName?: string;
        }>;
        permissions: { canViewSimulations: boolean; canViewChats: boolean };
      }>(`/api/v1/agent/${state.tenantSlug}/conversations?limit=50`);

      runInAction(() => {
        state.conversations = response.data.conversations.map((c) => ({
          ...c,
          createdAt: new Date(c.createdAt),
        }));
        state.historyPermissions = response.data.permissions;
        state.isLoadingHistory = false;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isLoadingHistory = false;
        // Don't show error for 403 - user just doesn't have permission
        if (error.response?.status !== 403) {
          state.error = error.response?.data?.message || "Failed to fetch conversation history";
        }
      });
    }
  };

  // Select a conversation to view
  const selectConversation = async (id: string): Promise<void> => {
    if (!state.tenantSlug) return;

    state.setPartial({ isLoadingHistory: true, error: null });

    try {
      const response = await axios.get<{
        id: string;
        type: "simulation" | "chat";
        messages: Array<{
          role: "human" | "agent";
          content: string;
          timestamp: string;
          turnNum?: number;
          metadata?: SimulationMetadata;
        }>;
        createdAt: string;
        initialPrompt?: string;
        personaHint?: string;
        totalTurns?: number;
        endReason?: string;
        audienceType?: string;
        customerName?: string;
        phase?: string;
        currentIntent?: string;
      }>(`/api/v1/agent/${state.tenantSlug}/conversations/${id}`);

      runInAction(() => {
        state.selectedConversation = {
          ...response.data,
          createdAt: new Date(response.data.createdAt),
          messages: response.data.messages.map((m) => ({
            ...m,
            timestamp: new Date(m.timestamp),
          })),
        };
        state.viewMode = "viewing";
        state.isLoadingHistory = false;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isLoadingHistory = false;
        state.error = error.response?.data?.message || "Failed to load conversation";
      });
    }
  };

  // Close conversation view and return to simulation mode
  const closeConversationView = () => {
    state.setPartial({
      selectedConversation: null,
      viewMode: "simulation",
    });
  };

  // Analyze a conversation transcript
  const analyzeConversation = async (conversationId: string, includeSuggestions: boolean = true): Promise<void> => {
    if (!state.tenantSlug) return;

    state.setPartial({ isAnalyzing: true, error: null });

    try {
      const response = await axios.post<{
        id: string;
        conversation_id: string;
        conversation_type: "simulation" | "chat";
        score: number;
        grade: string;
        breakdown: AnalysisBreakdown;
        issues: AnalysisIssue[];
        suggestions?: string[];
        benchmark_version: string;
        created_at: string;
      }>(`/api/v1/agent/${state.tenantSlug}/analyze`, {
        conversation_id: conversationId,
        include_suggestions: includeSuggestions,
      });

      runInAction(() => {
        state.analysisResult = {
          id: response.data.id,
          conversationId: response.data.conversation_id,
          conversationType: response.data.conversation_type,
          score: response.data.score,
          grade: response.data.grade,
          breakdown: response.data.breakdown,
          issues: response.data.issues,
          suggestions: response.data.suggestions,
          benchmarkVersion: response.data.benchmark_version,
          createdAt: new Date(response.data.created_at),
        };
        state.isAnalyzing = false;
        state.showAnalysisModal = true;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isAnalyzing = false;
        state.error = error.response?.data?.message || "Analysis failed";
      });
    }
  };

  // Close analysis modal
  const closeAnalysisModal = () => {
    state.setPartial({ showAnalysisModal: false });
  };

  // Clear analysis result
  const clearAnalysisResult = () => {
    state.setPartial({ analysisResult: null, showAnalysisModal: false });
  };

  // Apply selected learnings from analysis (v2 simplified workflow)
  const applySelectedLearnings = async (
    analysisId: string,
    selectedIssues: number[],
    selectedSuggestions: number[]
  ): Promise<{ success: boolean; appliedCount: number; message: string }> => {
    if (!state.tenantSlug) {
      return { success: false, appliedCount: 0, message: "No tenant selected" };
    }

    try {
      const response = await axios.post<{
        applied_count: number;
        learned_behaviors: unknown[];
        message: string;
      }>(`/api/v1/agent/${state.tenantSlug}/learning/apply`, {
        analysis_id: analysisId,
        selected_issues: selectedIssues,
        selected_suggestions: selectedSuggestions,
      });

      return {
        success: true,
        appliedCount: response.data.applied_count,
        message: response.data.message,
      };
    } catch (error: any) {
      return {
        success: false,
        appliedCount: 0,
        message: error.response?.data?.message || "Failed to apply learnings",
      };
    }
  };

  return {
    state,
    validateAndSetTenant,
    startSimulation,
    pauseSimulation,
    resumeSimulation,
    stopSimulation,
    clearSimulation,
    disconnectTenant,
    clearError,
    fetchConversationHistory,
    selectConversation,
    closeConversationView,
    analyzeConversation,
    closeAnalysisModal,
    clearAnalysisResult,
    applySelectedLearnings,
  };
})();

export default agentSimulationStore;
