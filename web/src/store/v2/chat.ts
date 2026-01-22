import axios from "axios";
import { makeAutoObservable } from "mobx";

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
}

class LocalState {
  messages: ChatMessage[] = [];
  isLoading: boolean = false;
  error: string | null = null;

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const chatStore = (() => {
  const state = new LocalState();

  const sendMessage = async (content: string) => {
    // Add user message to history
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
      // Prepare messages for API (without timestamps)
      const apiMessages = updatedMessages.map((msg) => ({
        role: msg.role,
        content: msg.content,
      }));

      const response = await axios.post<{ message: { role: string; content: string } }>("/api/v1/chat", {
        messages: apiMessages,
      });

      const assistantMessage: ChatMessage = {
        role: "assistant",
        content: response.data.message.content,
        timestamp: new Date(),
      };

      state.setPartial({
        messages: [...updatedMessages, assistantMessage],
        isLoading: false,
      });
    } catch (error: any) {
      state.setPartial({
        isLoading: false,
        error: error.response?.data?.message || "Failed to get response",
      });
    }
  };

  const clearChat = () => {
    state.setPartial({
      messages: [],
      isLoading: false,
      error: null,
    });
  };

  return {
    state,
    sendMessage,
    clearChat,
  };
})();

export default chatStore;
