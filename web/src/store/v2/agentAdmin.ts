import axios from "axios";
import { makeAutoObservable, runInAction } from "mobx";
import { userServiceClient } from "@/grpcweb";

export interface AgentTenant {
  id: number;
  slug: string;
  companyName: string;
  vertical: string;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface AgentSourceFile {
  id: number;
  tenantId: number;
  audienceType: string;
  fileType: string;
  content: string;
  contentHash: string;
  importedAt: string;
  version: number;
}

export interface AgentAudienceStats {
  servicesCount: number;
  intentsCount: number;
  faqsCount: number;
}

export interface TenantConfig {
  tenant: AgentTenant;
  external: {
    stats: AgentAudienceStats;
    sourceFiles: AgentSourceFile[];
  };
  internal: {
    stats: AgentAudienceStats;
    sourceFiles: AgentSourceFile[];
  };
  endpoints: {
    externalChat: string;
    internalChat: string;
    widget: string;
  };
}

export interface CreateTenantRequest {
  tenantSlug: string;
  companyName: string;
  vertical?: string;
  externalKbFile: File | null;
  externalPolicyFile: File | null;
  internalKbFile: File | null;
  internalPolicyFile: File | null;
}

export interface UpdateTenantFilesRequest {
  slug: string;
  audienceType: "external" | "internal";
  fileType: "kb" | "policy";
  file: File;
}

export interface LLMConfig {
  tenantSlug: string;
  llmModel: string;
  simulationHumanModel: string;
  hasApiKey: boolean;
  updatedAt?: string;
}

export interface SetLLMConfigRequest {
  llmModel: string;
  simulationHumanModel: string;
  openrouterApiKey?: string;
}

export interface UserPermission {
  userId: number;
  username: string;
  permissions: string[];
  grantedBy?: string;
  grantedAt: string;
}

export interface GrantPermissionRequest {
  userId: number;
  permissions: string[];
}

export interface UserInfo {
  name: string;
  id: number;
  username: string;
  role: string;
}

// SCRIPT.MD (tenant-level conversation flow guide)
export interface AgentScript {
  id: number;
  tenantId: number;
  content: string;
  contentHash: string;
  importedAt: string;
  version: number;
}

// Learning Memory (agent self-improvement)
export interface CommonIssue {
  category: string;
  description: string;
  occurrences: number;
  last_seen: string;
}

export interface LearnedBehavior {
  id: string;
  content?: string;       // v2: The learning text
  type?: string;          // v2: "issue" or "suggestion"
  source_id?: string;     // v2: Analysis result ID
  source_turn?: number;   // v2: Turn number for issues
  trigger?: string;       // v1 legacy
  behavior?: string;      // v1 legacy
  source: string;
  added_at: string;
  is_active: boolean;
}

export interface ImprovementArea {
  category: string;
  average_score: number;
  max_score: number;
  trend_percent: number;
}

export interface PendingSuggestion {
  id: string;
  category: string;
  trigger: string;
  behavior: string;
  occurrences: number;
  source_ids: string;
  created_at: string;
}

export interface AgentLearningMemory {
  id: number;
  tenant_id: number;
  common_issues: CommonIssue[];
  learned_behaviors: LearnedBehavior[];
  improvement_areas: ImprovementArea[];
  pending_suggestions: PendingSuggestion[];
  analysis_count: number;
  last_updated: string;
  version: number;
}

// Available LLM models (free tier)
export const LLM_MODEL_OPTIONS = [
  { value: "openai/gpt-oss-120b:free", label: "GPT-OSS 120B (Default)" },
  { value: "xiaomi/mimo-v2-flash:free", label: "Xiaomi MiMo v2 Flash" },
  { value: "nvidia/nemotron-3-nano-30b-a3b:free", label: "NVIDIA Nemotron-3 Nano 30B" },
  { value: "google/gemma-3-27b-it:free", label: "Google Gemma 3 27B" },
  { value: "qwen/qwen3-coder:free", label: "Qwen 3 Coder" },
  { value: "z-ai/glm-4.5-air:free", label: "Z-AI GLM 4.5 Air" },
  { value: "nousresearch/hermes-3-llama-3.1-405b:free", label: "Hermes 3 LLaMA 405B" },
];

// Permission presets
export const PERMISSION_PRESETS = {
  viewer: ["tenant:read"],
  tester: ["tenant:read", "chat:test"],
  analyst: ["tenant:read", "chat:logs"],
  editor: ["tenant:read", "tenant:write", "files:upload"],
  tenant_admin: ["tenant:admin"],
};

export interface UserTenantAccess {
  tenant: AgentTenant;
  permissions: string[];
}

export interface ChatSession {
  id: string;
  audienceType: string;
  userId?: number;
  phase: string;
  currentIntent: string;
  urgencyLevel: number;
  coverageStatus: string;
  customerName: string;
  customerPhone: string;
  customerLocation: string;
  detectedService: string;
  messageCount: number;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
  isCompleted: boolean;
  completionReason: string;
}

export interface ChatSessionDetail extends ChatSession {
  messages: {
    role: string;
    content: string;
    timestamp: string;
  }[];
}

class LocalState {
  tenants: AgentTenant[] = [];
  selectedTenant: TenantConfig | null = null;
  isLoading: boolean = false;
  isSaving: boolean = false;
  error: string | null = null;
  fileVersions: Record<string, AgentSourceFile[]> = {}; // key: "tenantId-audienceType-fileType"
  llmConfig: LLMConfig | null = null;
  tenantPermissions: UserPermission[] = [];
  allUsers: UserInfo[] = [];
  // Track current user's permissions for each tenant (for non-admin users)
  myTenantAccess: UserTenantAccess[] = [];
  // Current user's permissions for the selected tenant
  myPermissions: string[] = [];
  // SCRIPT.MD (tenant-level)
  script: AgentScript | null = null;
  isLoadingScript: boolean = false;
  // Learning Memory (agent self-improvement)
  learningMemory: AgentLearningMemory | null = null;
  isLoadingLearning: boolean = false;

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const agentAdminStore = (() => {
  const state = new LocalState();

  const fetchTenants = async () => {
    state.setPartial({ isLoading: true, error: null });

    try {
      const response = await axios.get<{ tenants: AgentTenant[] }>("/api/v1/agent/tenants");
      runInAction(() => {
        state.tenants = response.data.tenants || [];
        state.isLoading = false;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isLoading = false;
        state.error = error.response?.data?.message || "Failed to fetch tenants";
      });
    }
  };

  const fetchTenantConfig = async (slug: string) => {
    state.setPartial({ isLoading: true, error: null });

    try {
      const response = await axios.get<TenantConfig>(`/api/v1/agent/${slug}/config`);
      runInAction(() => {
        state.selectedTenant = response.data;
        state.isLoading = false;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isLoading = false;
        state.error = error.response?.data?.message || "Failed to fetch tenant config";
      });
    }
  };

  const fetchFileVersions = async (slug: string, audienceType: string, fileType: string) => {
    try {
      const response = await axios.get<{ versions: AgentSourceFile[] }>(
        `/api/v1/agent/${slug}/files/${audienceType}/${fileType}/versions`
      );
      const key = `${slug}-${audienceType}-${fileType}`;
      runInAction(() => {
        state.fileVersions[key] = response.data.versions || [];
      });
    } catch (error: any) {
      console.error("Failed to fetch file versions:", error);
    }
  };

  const createTenant = async (request: CreateTenantRequest): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });

    try {
      const formData = new FormData();
      formData.append("tenant_slug", request.tenantSlug);
      formData.append("company_name", request.companyName);
      if (request.vertical) {
        formData.append("vertical", request.vertical);
      }
      if (request.externalKbFile) {
        formData.append("external_kb_file", request.externalKbFile);
      }
      if (request.externalPolicyFile) {
        formData.append("external_policy_file", request.externalPolicyFile);
      }
      if (request.internalKbFile) {
        formData.append("internal_kb_file", request.internalKbFile);
      }
      if (request.internalPolicyFile) {
        formData.append("internal_policy_file", request.internalPolicyFile);
      }

      await axios.post("/api/v1/agent/onboard", formData, {
        headers: { "Content-Type": "multipart/form-data" },
      });

      runInAction(() => {
        state.isSaving = false;
      });

      // Refresh tenants list
      await fetchTenants();
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to create tenant";
      });
      return false;
    }
  };

  const updateTenantFiles = async (request: UpdateTenantFilesRequest): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });

    try {
      const formData = new FormData();
      formData.append("audience_type", request.audienceType);
      formData.append("file_type", request.fileType);
      formData.append("file", request.file);

      await axios.post(`/api/v1/agent/${request.slug}/import`, formData, {
        headers: { "Content-Type": "multipart/form-data" },
      });

      runInAction(() => {
        state.isSaving = false;
      });

      // Refresh tenant config
      await fetchTenantConfig(request.slug);
      // Refresh file versions
      await fetchFileVersions(request.slug, request.audienceType, request.fileType);
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to update tenant files";
      });
      return false;
    }
  };

  const restoreFileVersion = async (slug: string, audienceType: string, fileType: string, versionId: number): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });

    try {
      await axios.post(`/api/v1/agent/${slug}/files/${audienceType}/${fileType}/restore`, {
        version_id: versionId,
      });

      runInAction(() => {
        state.isSaving = false;
      });

      // Refresh tenant config and versions
      await fetchTenantConfig(slug);
      await fetchFileVersions(slug, audienceType, fileType);
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to restore file version";
      });
      return false;
    }
  };

  // ============================================================================
  // SCRIPT.MD MANAGEMENT (tenant-level conversation flow guide)
  // ============================================================================

  const fetchScript = async (slug: string) => {
    state.setPartial({ isLoadingScript: true });
    try {
      const response = await axios.get<AgentScript>(`/api/v1/agent/${slug}/script`);
      runInAction(() => {
        state.script = response.data;
        state.isLoadingScript = false;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isLoadingScript = false;
        // 404 means no script exists yet - that's OK
        if (error.response?.status !== 404) {
          console.error("Failed to fetch script:", error);
        }
        state.script = null;
      });
    }
  };

  const uploadScript = async (slug: string, file: File): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });

    try {
      const formData = new FormData();
      formData.append("file", file);

      const response = await axios.post<AgentScript>(`/api/v1/agent/${slug}/script`, formData, {
        headers: { "Content-Type": "multipart/form-data" },
      });

      runInAction(() => {
        state.script = response.data;
        state.isSaving = false;
      });

      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to upload script";
      });
      return false;
    }
  };

  const deleteScript = async (slug: string): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });

    try {
      await axios.delete(`/api/v1/agent/${slug}/script`);

      runInAction(() => {
        state.script = null;
        state.isSaving = false;
      });

      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to delete script";
      });
      return false;
    }
  };

  const clearScript = () => {
    state.setPartial({ script: null });
  };

  // ============================================================================
  // LEARNING MEMORY MANAGEMENT (agent self-improvement)
  // ============================================================================

  const fetchLearningMemory = async (slug: string) => {
    state.setPartial({ isLoadingLearning: true });
    try {
      const response = await axios.get<AgentLearningMemory>(`/api/v1/agent/${slug}/learning`);
      runInAction(() => {
        state.learningMemory = response.data;
        state.isLoadingLearning = false;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isLoadingLearning = false;
        console.error("Failed to fetch learning memory:", error);
        state.learningMemory = null;
      });
    }
  };

  const removeLearnedBehavior = async (slug: string, behaviorId: string): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });
    try {
      const response = await axios.delete<{ memory: AgentLearningMemory }>(`/api/v1/agent/${slug}/learning/behaviors/${behaviorId}`);
      runInAction(() => {
        state.learningMemory = response.data.memory;
        state.isSaving = false;
      });
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to remove behavior";
      });
      return false;
    }
  };

  const clearLearningMemory = async (slug: string): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });
    try {
      await axios.delete(`/api/v1/agent/${slug}/learning`);
      runInAction(() => {
        state.learningMemory = null;
        state.isSaving = false;
      });
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to clear learning memory";
      });
      return false;
    }
  };

  const clearLearningState = () => {
    state.setPartial({ learningMemory: null });
  };

  // ============================================================================
  // AUTO-GENERATE ANNOTATED KB.MD / POLICY.MD
  // ============================================================================

  const generateKB = async (slug: string): Promise<{ content?: string; error?: string }> => {
    state.setPartial({ isSaving: true, error: null });
    try {
      const response = await axios.post<{ content: string }>(
        `/api/v1/agent/${slug}/generate-kb`
      );
      runInAction(() => {
        state.isSaving = false;
      });
      return { content: response.data.content };
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to generate KB";
      });
      return { error: error.response?.data?.message || "Failed to generate KB" };
    }
  };

  const generatePolicy = async (slug: string): Promise<{ content?: string; error?: string }> => {
    state.setPartial({ isSaving: true, error: null });
    try {
      const response = await axios.post<{ content: string }>(
        `/api/v1/agent/${slug}/generate-policy`
      );
      runInAction(() => {
        state.isSaving = false;
      });
      return { content: response.data.content };
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to generate Policy";
      });
      return { error: error.response?.data?.message || "Failed to generate Policy" };
    }
  };

  const deleteTenant = async (slug: string): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });

    try {
      await axios.delete(`/api/v1/agent/${slug}`);

      runInAction(() => {
        state.isSaving = false;
        state.selectedTenant = null;
      });

      // Refresh tenants list
      await fetchTenants();
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to delete tenant";
      });
      return false;
    }
  };

  const reindexTenant = async (slug: string): Promise<{ success: boolean; chunks?: number; error?: string }> => {
    state.setPartial({ isSaving: true, error: null });

    try {
      const response = await axios.post<{ success: boolean; chunks: number; message: string }>(
        `/api/v1/agent/${slug}/reindex`
      );

      runInAction(() => {
        state.isSaving = false;
      });

      return { success: true, chunks: response.data.chunks };
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to rebuild index";
      });
      return { success: false, error: error.response?.data?.message || "Failed to rebuild index" };
    }
  };

  const toggleTenantActive = async (slug: string, isActive: boolean): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });

    try {
      await axios.patch(`/api/v1/agent/${slug}`, { is_active: isActive });

      runInAction(() => {
        state.isSaving = false;
        // Update local state
        const tenant = state.tenants.find((t) => t.slug === slug);
        if (tenant) {
          tenant.isActive = isActive;
        }
        if (state.selectedTenant && state.selectedTenant.tenant.slug === slug) {
          state.selectedTenant.tenant.isActive = isActive;
        }
      });

      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to update tenant status";
      });
      return false;
    }
  };

  const clearSelectedTenant = () => {
    state.setPartial({ selectedTenant: null });
  };

  const clearError = () => {
    state.setPartial({ error: null });
  };

  // ============================================================================
  // LLM CONFIGURATION
  // ============================================================================

  // Transform snake_case API response to camelCase for frontend
  const transformLLMConfig = (data: any): LLMConfig => ({
    tenantSlug: data.tenant_slug,
    llmModel: data.llm_model,
    simulationHumanModel: data.simulation_human_model || "",
    hasApiKey: data.has_api_key,
    updatedAt: data.updated_at,
  });

  const fetchLLMConfig = async (slug: string) => {
    try {
      const response = await axios.get(`/api/v1/agent/${slug}/llm-config`);
      runInAction(() => {
        state.llmConfig = transformLLMConfig(response.data);
      });
    } catch (error: any) {
      console.error("Failed to fetch LLM config:", error);
    }
  };

  const updateLLMConfig = async (slug: string, config: SetLLMConfigRequest): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });
    try {
      const response = await axios.put(`/api/v1/agent/${slug}/llm-config`, {
        llm_model: config.llmModel,
        simulation_human_model: config.simulationHumanModel,
        openrouter_api_key: config.openrouterApiKey,
      });
      runInAction(() => {
        state.llmConfig = transformLLMConfig(response.data);
        state.isSaving = false;
      });
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to update LLM config";
      });
      return false;
    }
  };

  // Fetch file content for preview
  const fetchFileContent = async (slug: string, audienceType: string, fileType: string): Promise<string | null> => {
    try {
      const response = await axios.get(`/api/v1/agent/${slug}/source-file`, {
        params: { audience_type: audienceType, file_type: fileType },
      });
      return response.data.content || null;
    } catch (error: any) {
      console.error("Failed to fetch file content:", error);
      return null;
    }
  };

  // ============================================================================
  // PERMISSION MANAGEMENT
  // ============================================================================

  const fetchTenantPermissions = async (slug: string) => {
    try {
      const response = await axios.get<{ permissions: UserPermission[] }>(`/api/v1/agent/${slug}/permissions`);
      runInAction(() => {
        state.tenantPermissions = response.data.permissions || [];
      });
    } catch (error: any) {
      console.error("Failed to fetch permissions:", error);
    }
  };

  const grantPermission = async (slug: string, request: GrantPermissionRequest): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });
    try {
      await axios.post(`/api/v1/agent/${slug}/permissions`, {
        user_id: request.userId,
        permissions: request.permissions,
      });
      runInAction(() => {
        state.isSaving = false;
      });
      await fetchTenantPermissions(slug);
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to grant permission";
      });
      return false;
    }
  };

  const revokePermission = async (slug: string, userId: number): Promise<boolean> => {
    state.setPartial({ isSaving: true, error: null });
    try {
      await axios.delete(`/api/v1/agent/${slug}/permissions/${userId}`);
      runInAction(() => {
        state.isSaving = false;
      });
      await fetchTenantPermissions(slug);
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSaving = false;
        state.error = error.response?.data?.message || "Failed to revoke permission";
      });
      return false;
    }
  };

  // ============================================================================
  // USER MANAGEMENT (for permission dropdowns)
  // ============================================================================

  const fetchUsers = async () => {
    try {
      const { users } = await userServiceClient.listUsers({});
      runInAction(() => {
        state.allUsers = users.map((u) => ({
          id: parseInt(u.name.replace("users/", ""), 10),
          name: u.nickname || u.username,
          username: u.username,
          role: u.role.toString(),
        }));
      });
    } catch (error: any) {
      console.error("Failed to fetch users:", error);
    }
  };

  // ============================================================================
  // USER TENANT ACCESS (for non-admin users)
  // ============================================================================

  const fetchUserTenants = async () => {
    state.setPartial({ isLoading: true, error: null });

    try {
      const response = await axios.get<{ tenants: UserTenantAccess[] | null }>("/api/v1/user/tenants");
      runInAction(() => {
        // Handle null or undefined tenants array (user has no permissions)
        const tenants = response.data.tenants ?? [];
        state.myTenantAccess = tenants;
        // Also populate the tenants array from user access for unified display
        state.tenants = tenants.map((t) => t.tenant);
        state.isLoading = false;
      });
    } catch (error: any) {
      runInAction(() => {
        state.isLoading = false;
        // Handle 404 as valid "no permissions" case (don't show error)
        if (error.response?.status === 404) {
          state.myTenantAccess = [];
          state.tenants = [];
        } else {
          state.error = error.response?.data?.message || "Failed to fetch user tenants";
        }
      });
    }
  };

  const setMyPermissions = (tenantId: number) => {
    const access = state.myTenantAccess.find((t) => t.tenant.id === tenantId);
    state.setPartial({
      myPermissions: access?.permissions || [],
    });
  };

  const hasPermission = (permission: string): boolean => {
    // Check if user has the specific permission or tenant:admin (which grants all tenant:* permissions)
    if (state.myPermissions.includes(permission)) {
      return true;
    }
    if (state.myPermissions.includes("tenant:admin") && permission.startsWith("tenant:")) {
      return true;
    }
    return false;
  };

  const clearMyPermissions = () => {
    state.setPartial({ myPermissions: [] });
  };

  const hasAnyChatTestPermission = (): boolean => {
    // Check if user has chat:test on ANY tenant
    // Note: tenant:admin does NOT imply chat:test (permissions are intentionally separate)
    return state.myTenantAccess.some((access) =>
      access.permissions.includes("chat:test") ||
      access.permissions.includes("*")
    );
  };

  return {
    state,
    fetchTenants,
    fetchTenantConfig,
    fetchFileVersions,
    createTenant,
    updateTenantFiles,
    restoreFileVersion,
    deleteTenant,
    reindexTenant,
    toggleTenantActive,
    clearSelectedTenant,
    clearError,
    fetchLLMConfig,
    updateLLMConfig,
    fetchTenantPermissions,
    grantPermission,
    revokePermission,
    fetchUsers,
    fetchUserTenants,
    setMyPermissions,
    hasPermission,
    clearMyPermissions,
    hasAnyChatTestPermission,
    // Script methods
    fetchScript,
    uploadScript,
    deleteScript,
    clearScript,
    // Learning memory methods (v2 simplified)
    fetchLearningMemory,
    removeLearnedBehavior,
    clearLearningMemory,
    clearLearningState,
    // Auto-generate annotated content
    generateKB,
    generatePolicy,
    // File content preview
    fetchFileContent,
  };
})();

export default agentAdminStore;
