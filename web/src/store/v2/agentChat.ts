import axios from "axios";
import { makeAutoObservable } from "mobx";

export interface AgentChatMessage {
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
}

export interface AgentChatMetadata {
  intent: string;
  urgency: number;
  phase: string;
}

class LocalState {
  messages: AgentChatMessage[] = [];
  isLoading: boolean = false;
  isValidating: boolean = false;
  error: string | null = null;
  sessionId: string | null = null;
  tenantSlug: string | null = null;
  tenantCompanyName: string | null = null;
  metadata: AgentChatMetadata | null = null;

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const agentChatStore = (() => {
  const state = new LocalState();

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
        // Reset state and set tenant
        state.setPartial({
          tenantSlug: slug,
          tenantCompanyName: response.data.companyName,
          messages: [],
          sessionId: null,
          error: null,
          metadata: null,
          isValidating: false,
        });
        return true;
      } else {
        state.setPartial({
          isValidating: false,
          error: response.data.message || "Tenant not found or inactive",
        });
        return false;
      }
    } catch (error: any) {
      // Handle permission denied (403) and other errors
      const status = error.response?.status;
      let errorMessage = error.response?.data?.message || "Tenant not found";

      if (status === 403) {
        errorMessage = error.response?.data?.message || "Permission denied: you do not have access to this tenant";
      }

      state.setPartial({
        isValidating: false,
        error: errorMessage,
      });
      return false;
    }
  };

  const setTenant = (slug: string) => {
    // Reset state when switching tenants (direct set without validation)
    state.setPartial({
      tenantSlug: slug,
      messages: [],
      sessionId: null,
      error: null,
      metadata: null,
    });
  };

  const sendMessage = async (content: string) => {
    if (!state.tenantSlug) {
      state.setPartial({ error: "No tenant selected" });
      return;
    }

    // Add user message to history
    const userMessage: AgentChatMessage = {
      role: "user",
      content,
      timestamp: new Date(),
    };

    const updatedMessages = [...state.messages, userMessage];

    state.setPartial({
      messages: updatedMessages,
      isLoading: true,
      error: null,
    });

    try {
      // Internal chat endpoint (authenticated)
      const response = await axios.post<{
        session_id: string;
        message: { role: string; content: string; timestamp: string };
        metadata: AgentChatMetadata;
      }>(`/api/v1/agent/${state.tenantSlug}/chat/int`, {
        session_id: state.sessionId,
        message: content,
      });

      const assistantMessage: AgentChatMessage = {
        role: "assistant",
        content: response.data.message.content,
        timestamp: new Date(response.data.message.timestamp),
      };

      state.setPartial({
        messages: [...updatedMessages, assistantMessage],
        isLoading: false,
        sessionId: response.data.session_id,
        metadata: response.data.metadata,
      });
    } catch (error: any) {
      const errorMessage =
        error.response?.status === 429
          ? "Too many requests. Please wait a moment."
          : error.response?.data?.message || "Failed to get response";

      state.setPartial({
        isLoading: false,
        error: errorMessage,
      });
    }
  };

  const clearChat = () => {
    state.setPartial({
      messages: [],
      isLoading: false,
      error: null,
      sessionId: null,
      metadata: null,
    });
  };

  const clearError = () => {
    state.setPartial({ error: null });
  };

  const disconnectTenant = () => {
    state.setPartial({
      tenantSlug: null,
      tenantCompanyName: null,
      messages: [],
      sessionId: null,
      error: null,
      metadata: null,
    });
  };

  return {
    state,
    setTenant,
    validateAndSetTenant,
    sendMessage,
    clearChat,
    clearError,
    disconnectTenant,
  };
})();

export default agentChatStore;
