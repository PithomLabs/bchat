import axios from "axios";
import { makeAutoObservable, runInAction } from "mobx";

// Response types matching backend
export interface RAGStats {
  enabled: boolean;
  storageProvider: string;
  embeddingProvider: string;
  embeddingModel: string;
  hybridSearchEnabled: boolean;
  hybridVectorWeight: number;
  hybridTextWeight: number;
  stats: {
    totalChunks: number;
    tenantCounts: Record<number, number>;
    contentCounts: Record<string, number>;
    indexSize: number;
    lastOptimized?: string;
  };
  tenants: TenantRAGInfo[];
}

export interface TenantRAGInfo {
  id: number;
  slug: string;
  companyName: string;
  chunkCount: number;
  lastIndexed?: string;
}

export interface TenantRAGDetails {
  tenantId: number;
  slug: string;
  companyName: string;
  chunksByType: Record<string, number>;
  chunksByAudience: Record<string, number>;
  sampleChunks: ChunkInfo[];
}

export interface ChunkInfo {
  id: string;
  contentType: string;
  audienceType: string;
  title: string;
  content: string;
  code?: string;
  isActive: boolean;
  isEmergency?: boolean;
  priority?: number;
  indexedAt?: string;
}

export interface SearchResult {
  chunk: ChunkInfo;
  score: number;
  vectorScore?: number;
  bm25Score?: number;
}

export interface SearchResponse {
  searchMode: string;
  latencyMs: number;
  totalResults: number;
  results: SearchResult[];
}

export interface SearchParams {
  tenantId: number;
  audienceType: string;
  query: string;
  topK: number;
  minScore: number;
  useHybridSearch: boolean;
  vectorWeight: number;
  textWeight: number;
}

class LocalState {
  ragStats: RAGStats | null = null;
  selectedTenantDetails: TenantRAGDetails | null = null;
  searchResults: SearchResponse | null = null;
  isLoading: boolean = false;
  isLoadingDetails: boolean = false;
  isSearching: boolean = false;
  error: string | null = null;

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const ragStatsStore = (() => {
  const state = new LocalState();

  const fetchRAGStats = async (): Promise<boolean> => {
    state.setPartial({ isLoading: true, error: null });
    try {
      const response = await axios.get<RAGStats>("/api/v1/admin/rag/stats");
      runInAction(() => {
        state.ragStats = response.data;
        state.isLoading = false;
      });
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isLoading = false;
        state.error = error.response?.data?.message || error.message || "Failed to fetch RAG stats";
      });
      return false;
    }
  };

  const fetchTenantDetails = async (tenantId: number): Promise<boolean> => {
    state.setPartial({ isLoadingDetails: true, error: null });
    try {
      const response = await axios.get<TenantRAGDetails>(
        `/api/v1/admin/rag/tenants/${tenantId}`
      );
      runInAction(() => {
        state.selectedTenantDetails = response.data;
        state.isLoadingDetails = false;
      });
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isLoadingDetails = false;
        state.error = error.response?.data?.message || error.message || "Failed to fetch tenant details";
      });
      return false;
    }
  };

  const testSearch = async (params: SearchParams): Promise<boolean> => {
    state.setPartial({ isSearching: true, error: null, searchResults: null });
    try {
      const response = await axios.post<SearchResponse>(
        "/api/v1/admin/rag/search",
        params
      );
      runInAction(() => {
        state.searchResults = response.data;
        state.isSearching = false;
      });
      return true;
    } catch (error: any) {
      runInAction(() => {
        state.isSearching = false;
        state.error = error.response?.data?.message || error.message || "Search failed";
      });
      return false;
    }
  };

  const clearTenantDetails = () => {
    state.setPartial({ selectedTenantDetails: null });
  };

  const clearSearchResults = () => {
    state.setPartial({ searchResults: null });
  };

  const clearError = () => {
    state.setPartial({ error: null });
  };

  const reset = () => {
    state.setPartial({
      ragStats: null,
      selectedTenantDetails: null,
      searchResults: null,
      isLoading: false,
      isLoadingDetails: false,
      isSearching: false,
      error: null,
    });
  };

  return {
    state,
    fetchRAGStats,
    fetchTenantDetails,
    testSearch,
    clearTenantDetails,
    clearSearchResults,
    clearError,
    reset,
  };
})();

export default ragStatsStore;
