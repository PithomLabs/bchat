import axios from "axios";
import { makeAutoObservable, runInAction } from "mobx";

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
}

interface ChatApiResponse {
  session_id: string;
  message: {
    role: string;
    content: string;
    timestamp: string;
  };
}

class LocalState {
  messages: ChatMessage[] = [];
  isLoading: boolean = false;
  error: string | null = null;
  // Tenant state
  selectedTenantSlug: string = "";
  isTenantValidated: boolean = false;
  isTenantLoading: boolean = false;
  sessionId: string = "";

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const chatStore = (() => {
  const state = new LocalState();

  const validateAndSetTenant = async (slug: string) => {
    state.setPartial({ isTenantLoading: true, isTenantValidated: false, error: null });
    try {
      await axios.get(`/api/v1/agent/${slug}/validate`);
      runInAction(() => {
        state.selectedTenantSlug = slug;
        state.isTenantValidated = true;
        state.isTenantLoading = false;
        state.messages = [];
        state.sessionId = "";
      });
    } catch (error: any) {
      runInAction(() => {
        state.isTenantLoading = false;
        state.error = error.response?.data?.message || "Failed to validate tenant";
      });
    }
  };

  const resetTenant = () => {
    state.setPartial({
      selectedTenantSlug: "",
      isTenantValidated: false,
      messages: [],
      sessionId: "",
      error: null,
    });
  };

  const sendMessage = async (content: string) => {
    if (!state.selectedTenantSlug) return;

    const userMessage: ChatMessage = {
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
      const response = await axios.post<ChatApiResponse>(
        `/api/v1/agent/${state.selectedTenantSlug}/chat/int`,
        {
          message: content,
          session_id: state.sessionId || undefined,
        }
      );

      const assistantMessage: ChatMessage = {
        role: "assistant",
        content: response.data.message.content,
        timestamp: new Date(response.data.message.timestamp),
      };

      runInAction(() => {
        state.messages = [...updatedMessages, assistantMessage];
        state.sessionId = response.data.session_id;
        state.isLoading = false;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isLoading = false;
        state.error = error.response?.data?.message || "Failed to get response";
      });
    }
  };

  const clearChat = () => {
    state.setPartial({
      messages: [],
      isLoading: false,
      error: null,
      sessionId: "",
    });
  };

  return {
    state,
    validateAndSetTenant,
    resetTenant,
    sendMessage,
    clearChat,
  };
})();

export default chatStore;
