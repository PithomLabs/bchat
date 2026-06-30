import axios from "axios";
import { makeAutoObservable, runInAction } from "mobx";

export interface PlaygroundCapability {
  id: string;
  label: string;
  description: string;
}

export interface PlaygroundScenario {
  id: string;
  title: string;
  description: string;
  prompt: string;
  highlights: string[];
}

export interface PlaygroundDemoTenant {
  slug: string;
  company_name: string;
  vertical: string;
  summary: string;
  available: boolean;
  capabilities: PlaygroundCapability[];
  scenarios: PlaygroundScenario[];
}

export interface PlaygroundSupport {
  partner: string;
  message: string;
  services: string[];
}

export interface PlaygroundCatalog {
  demos: PlaygroundDemoTenant[];
  self_hosting: string[];
  support: PlaygroundSupport;
}

export interface PlaygroundMessage {
  role: "user" | "assistant" | "system";
  content: string;
  timestamp: Date;
}

export interface PlaygroundRAGResult {
  rank: number;
  score: number;
  title: string;
  content_preview: string;
  content_type: string;
  audience_type: string;
}

export interface PlaygroundArtifacts {
  intent: string;
  phase: string;
  urgency: number;
  rag: {
    enabled: boolean;
    query: string;
    results: PlaygroundRAGResult[];
    error?: string;
  };
  lead?: {
    id: string;
    name: string;
    email?: string;
    phone?: string;
    topic?: string;
    location?: string;
    detectedIntent?: string;
    status: string;
  };
  transcript?: {
    id: string;
    session_id: string;
    message_count: number;
  };
  escalation: {
    active: boolean;
    status?: string;
    routing_mode?: string;
    handoff_id?: string;
  };
  capabilities: PlaygroundCapability[];
}

interface PlaygroundRunResponse {
  chat: {
    session_id: string;
    message: {
      role: "assistant" | "system";
      content: string;
      timestamp: string;
    };
  };
  artifacts: PlaygroundArtifacts;
}

class LocalState {
  catalog: PlaygroundCatalog | null = null;
  selectedDemo: PlaygroundDemoTenant | null = null;
  selectedScenario: PlaygroundScenario | null = null;
  sessionId: string | null = null;
  messages: PlaygroundMessage[] = [];
  artifacts: PlaygroundArtifacts | null = null;
  input: string = "";
  isLoadingCatalog: boolean = false;
  isSending: boolean = false;
  error: string | null = null;

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const playgroundStore = (() => {
  const state = new LocalState();

  const fetchCatalog = async () => {
    state.setPartial({ isLoadingCatalog: true, error: null });
    try {
      const response = await axios.get<PlaygroundCatalog>("/api/v1/agent/playground/catalog");
      runInAction(() => {
        state.catalog = response.data;
        state.isLoadingCatalog = false;
        if (!state.selectedDemo && response.data.demos.length > 0) {
          const firstAvailable = response.data.demos.find((demo) => demo.available) || response.data.demos[0];
          state.selectedDemo = firstAvailable;
          state.selectedScenario = firstAvailable.scenarios[0] || null;
        }
      });
    } catch (error: any) {
      state.setPartial({
        isLoadingCatalog: false,
        error: error.response?.data?.message || "Failed to load playground catalog",
      });
    }
  };

  const selectDemo = (demo: PlaygroundDemoTenant) => {
    state.setPartial({
      selectedDemo: demo,
      selectedScenario: demo.scenarios[0] || null,
      sessionId: null,
      messages: [],
      artifacts: null,
      error: null,
      input: "",
    });
  };

  const selectScenario = (scenario: PlaygroundScenario) => {
    state.setPartial({
      selectedScenario: scenario,
      input: scenario.prompt,
      error: null,
    });
  };

  const sendMessage = async (message?: string) => {
    if (!state.selectedDemo || state.isSending) {
      return;
    }

    const content = (message ?? state.input).trim();
    if (!content) {
      return;
    }

    const userMessage: PlaygroundMessage = {
      role: "user",
      content,
      timestamp: new Date(),
    };

    state.setPartial({
      isSending: true,
      error: null,
      input: "",
      messages: [...state.messages, userMessage],
    });

    try {
      const response = await axios.post<PlaygroundRunResponse>(
        `/api/v1/agent/${state.selectedDemo.slug}/playground/run`,
        {
          session_id: state.sessionId || "",
          message: content,
          scenario_id: state.selectedScenario?.id || "",
          client_message_id: `playground-${Date.now()}-${Math.random().toString(36).slice(2)}`,
        },
      );

      const assistantMessage: PlaygroundMessage = {
        role: response.data.chat.message.role,
        content: response.data.chat.message.content,
        timestamp: new Date(response.data.chat.message.timestamp),
      };

      runInAction(() => {
        state.sessionId = response.data.chat.session_id;
        state.messages = [...state.messages, assistantMessage];
        state.artifacts = response.data.artifacts;
        state.isSending = false;
      });
    } catch (error: any) {
      state.setPartial({
        isSending: false,
        error: error.response?.data?.message || "Playground request failed",
      });
    }
  };

  const runScenario = async (scenario?: PlaygroundScenario) => {
    const selected = scenario || state.selectedScenario;
    if (!selected) {
      return;
    }
    state.setPartial({
      selectedScenario: selected,
      sessionId: null,
      messages: [],
      artifacts: null,
    });
    await sendMessage(selected.prompt);
  };

  return {
    state,
    fetchCatalog,
    selectDemo,
    selectScenario,
    sendMessage,
    runScenario,
  };
})();

export default playgroundStore;
