# LanceDB Visualization Feature Plan

**Status:** Planned
**Date:** 2026-01-26
**Related Documents:**
- [DOCS_LANCEDB_STORAGE.MD](./DOCS_LANCEDB_STORAGE.MD) - Storage reference
- [DOCS_LANCEDB.MD](./DOCS_LANCEDB.MD) - RAG implementation plan
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables

---

## Overview

This document describes the implementation plan for a LanceDB visualization page accessible from the left sidebar. The page provides administrators with visibility into the RAG pipeline's vector database for effective troubleshooting.

---

## Table of Contents

1. [Design Decision](#design-decision)
2. [Features](#features)
3. [API Endpoints](#api-endpoints)
4. [Frontend Implementation](#frontend-implementation)
5. [UI Design](#ui-design)
6. [Files to Modify](#files-to-modify)
7. [Verification](#verification)

---

## Design Decision

**Approach:** Standalone page (`/rag-stats`) rather than embedding in Agent Admin.

**Rationale:**
- RAG statistics are cross-tenant (global view) while Agent Admin is tenant-specific
- Dedicated page allows richer visualization and search testing capabilities
- Keeps Agent Admin focused on tenant configuration
- Admin-only access maintains security

---

## Features

### 1. Global Statistics Dashboard

| Metric | Description |
|--------|-------------|
| RAG Status | Enabled/Disabled indicator |
| Storage Provider | memory, local, or s3 |
| Embedding Provider | mock, local, or openrouter |
| Hybrid Search | Enabled/Disabled with weights |
| Total Chunks | Count across all tenants |
| Index Size | Approximate storage consumption |

### 2. Content Distribution

Visual breakdown of indexed content by type:

| Content Type | Description |
|--------------|-------------|
| `service` | Services offered |
| `faq` | Q&A pairs |
| `rule` | Policy rules |
| `coverage` | Coverage areas |
| `safety` | Safety protocols |
| `exclusion` | Services not offered |
| `kb_section` | General KB content |
| `intent` | Intent classifications |

### 3. Per-Tenant Breakdown

Table showing each tenant's RAG usage:

| Column | Description |
|--------|-------------|
| Tenant | Company name and slug |
| Chunks | Total indexed chunks |
| Last Indexed | Time since last reindex |
| Actions | View details button |

### 4. Tenant Detail Modal

Drill-down view for a specific tenant:

- Chunks by content type (service: 45, faq: 120, etc.)
- Chunks by audience (external: 300, internal: 223)
- Sample chunks with full content preview
- Quick actions (reindex, view in Agent Admin)

### 5. Search Testing Tool

Interactive query tester for troubleshooting:

| Field | Description |
|-------|-------------|
| Tenant | Dropdown to select tenant |
| Audience | External or Internal |
| Query | Free-text search input |
| TopK | Number of results (1-20) |
| Min Score | Relevance threshold (0-1) |
| Hybrid Mode | Toggle hybrid search |
| Vector Weight | Weight for semantic (0-1) |
| Text Weight | Weight for BM25 (0-1) |

**Results Display:**
- Combined score
- Vector score (if hybrid)
- BM25 score (if hybrid)
- Content type
- Title
- Content preview
- Search latency

---

## API Endpoints

### 1. Get Global RAG Statistics

```
GET /api/v1/admin/rag/stats
Authorization: Admin role required
```

**Response:**
```json
{
  "enabled": true,
  "storageProvider": "local",
  "embeddingProvider": "openrouter",
  "embeddingModel": "openai/text-embedding-3-small",
  "hybridSearchEnabled": true,
  "hybridVectorWeight": 0.7,
  "hybridTextWeight": 0.3,
  "stats": {
    "totalChunks": 1245,
    "tenantCounts": {
      "1": 523,
      "2": 722
    },
    "contentCounts": {
      "service": 234,
      "faq": 456,
      "rule": 189,
      "coverage": 120,
      "safety": 45,
      "exclusion": 98,
      "kb_section": 103
    },
    "indexSize": 15234567,
    "lastOptimized": "2026-01-26T10:30:00Z"
  },
  "tenants": [
    {
      "id": 1,
      "slug": "acme-restoration",
      "companyName": "ACME Restoration",
      "chunkCount": 523,
      "lastIndexed": "2026-01-26T09:15:00Z"
    },
    {
      "id": 2,
      "slug": "insure-corp",
      "companyName": "Insure Corp",
      "chunkCount": 722,
      "lastIndexed": "2026-01-26T08:30:00Z"
    }
  ]
}
```

### 2. Get Tenant RAG Details

```
GET /api/v1/admin/rag/tenants/:tenantId
Authorization: Admin role required
```

**Response:**
```json
{
  "tenantId": 1,
  "slug": "acme-restoration",
  "companyName": "ACME Restoration",
  "chunksByType": {
    "service": 45,
    "faq": 120,
    "rule": 30,
    "coverage": 25,
    "safety": 8,
    "exclusion": 15,
    "kb_section": 280
  },
  "chunksByAudience": {
    "external": 300,
    "internal": 223
  },
  "sampleChunks": [
    {
      "id": "1:external:service:WATER_DAMAGE",
      "contentType": "service",
      "audienceType": "external",
      "title": "Water Damage Restoration",
      "content": "24/7 emergency water extraction and drying services. Response time: 1-2 hours.",
      "code": "WATER_DAMAGE",
      "isActive": true,
      "isEmergency": true,
      "priority": 100,
      "indexedAt": "2026-01-26T09:15:00Z"
    }
  ]
}
```

### 3. Test RAG Search

```
POST /api/v1/admin/rag/search
Authorization: Admin role required
```

**Request:**
```json
{
  "tenantId": 1,
  "audienceType": "external",
  "query": "water damage in basement",
  "topK": 5,
  "minScore": 0.3,
  "useHybridSearch": true,
  "vectorWeight": 0.7,
  "textWeight": 0.3
}
```

**Response:**
```json
{
  "searchMode": "hybrid",
  "latencyMs": 45,
  "totalResults": 5,
  "results": [
    {
      "chunk": {
        "id": "1:external:service:WATER_DAMAGE",
        "contentType": "service",
        "title": "Water Damage Restoration",
        "content": "24/7 emergency water extraction...",
        "code": "WATER_DAMAGE",
        "isEmergency": true,
        "isActive": true
      },
      "score": 0.87,
      "vectorScore": 0.92,
      "bm25Score": 0.75
    },
    {
      "chunk": {
        "id": "1:external:faq:12",
        "contentType": "faq",
        "title": "What should I do if I have water damage?",
        "content": "First, ensure safety and turn off electricity...",
        "isActive": true
      },
      "score": 0.73,
      "vectorScore": 0.78,
      "bm25Score": 0.62
    }
  ]
}
```

---

## Frontend Implementation

### Navigation Link

**File:** `web/src/components/Navigation.tsx`

```typescript
import { DatabaseIcon } from "lucide-react";

const ragStatsNavLink: NavLinkItem = {
  id: "header-rag-stats",
  path: Routes.RAG_STATS,
  title: t("common.rag-stats"),
  icon: <DatabaseIcon className="w-6 h-auto opacity-70 shrink-0" />,
};

// Add to admin-only links
const adminNavLinks: NavLinkItem[] = isAdmin
  ? [agentAdminNavLink, ragStatsNavLink]
  : [];
```

### Route Definition

**File:** `web/src/router/index.tsx`

```typescript
// Add to Routes enum
export enum Routes {
  // ... existing routes
  RAG_STATS = "/rag-stats",
}

// Add lazy import
const RagStats = lazy(() => import("@/pages/RagStats"));

// Add route config
{
  path: Routes.RAG_STATS,
  element: (
    <Suspense fallback={<Loading />}>
      <RagStats />
    </Suspense>
  ),
}
```

### MobX Store

**File:** `web/src/store/v2/ragStats.ts`

```typescript
import axios from "axios";
import { makeAutoObservable, runInAction } from "mobx";

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
    lastOptimized: string;
  };
  tenants: TenantRAGInfo[];
}

export interface TenantRAGInfo {
  id: number;
  slug: string;
  companyName: string;
  chunkCount: number;
  lastIndexed: string;
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
  indexedAt: string;
}

export interface SearchResult {
  chunk: ChunkInfo;
  score: number;
  vectorScore: number;
  bm25Score: number;
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
        state.error = error.response?.data?.message || "Failed to fetch RAG stats";
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
        state.error = error.response?.data?.message || "Failed to fetch tenant details";
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
        state.error = error.response?.data?.message || "Search failed";
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

  return {
    state,
    fetchRAGStats,
    fetchTenantDetails,
    testSearch,
    clearTenantDetails,
    clearSearchResults,
    clearError,
  };
})();

export default ragStatsStore;
```

---

## UI Design

### Page Layout

```
+------------------------------------------------------------------+
| RAG Statistics                                    [Refresh Button] |
+------------------------------------------------------------------+
| +--------------------------------------------------------------+ |
| | Status Banner                                                 | |
| | [ENABLED] Storage: local | Embedding: openrouter | Hybrid: ON | |
| +--------------------------------------------------------------+ |
|                                                                    |
| +---------------------------+  +--------------------------------+ |
| | Total Chunks              |  | Content Distribution           | |
| |         1,245             |  | service ████████████  234      | |
| |                           |  | faq     ██████████████████ 456 | |
| | Index Size: 14.5 MB       |  | rule    ████████  189          | |
| +---------------------------+  | coverage ██████  120           | |
|                                | safety  ███  45                 | |
|                                | exclusion █████  98             | |
|                                | kb_section █████  103           | |
|                                +--------------------------------+ |
|                                                                    |
| +--------------------------------------------------------------+ |
| | Per-Tenant Breakdown                                          | |
| | +----------------------------------------------------------+ | |
| | | Tenant              | Chunks | Last Indexed    | Actions  | | |
| | |---------------------|--------|-----------------|----------| | |
| | | ACME Restoration    |   523  | 2 hours ago     | [Details]| | |
| | | Insure Corp         |   722  | 30 minutes ago  | [Details]| | |
| | +----------------------------------------------------------+ | |
| +--------------------------------------------------------------+ |
|                                                                    |
| +--------------------------------------------------------------+ |
| | Search Testing                                                | |
| | +----------------------------------------------------------+ | |
| | | Tenant: [ACME Restoration ▼]  Audience: [External ▼]     | | |
| | |                                                          | | |
| | | Query: [water damage in basement________________] [Search]| | |
| | |                                                          | | |
| | | Options:                                                 | | |
| | | [x] Hybrid Search                                        | | |
| | | Vector Weight: [0.7]  Text Weight: [0.3]                | | |
| | | TopK: [5]  Min Score: [0.3]                             | | |
| | +----------------------------------------------------------+ | |
| |                                                              | |
| | Results (5 found in 45ms, mode: hybrid)                     | |
| | +----------------------------------------------------------+ | |
| | | Score | V/BM25  | Type    | Title          | Content     | | |
| | |-------|---------|---------|----------------|-------------| | |
| | | 0.87  | .92/.75 | service | Water Damage   | 24/7 emer...| | |
| | | 0.73  | .78/.62 | faq     | What should... | First, en...| | |
| | | 0.65  | .70/.55 | service | Flood Cleanup  | Comprehens..| | |
| | +----------------------------------------------------------+ | |
| +--------------------------------------------------------------+ |
+------------------------------------------------------------------+
```

### Tenant Detail Modal

```
+--------------------------------------------------+
| Tenant Details: ACME Restoration          [Close] |
+--------------------------------------------------+
| Slug: acme-restoration                            |
| Total Chunks: 523                                 |
+--------------------------------------------------+
| Chunks by Type          | Chunks by Audience     |
| service: 45             | external: 300          |
| faq: 120                | internal: 223          |
| rule: 30                |                        |
| coverage: 25            |                        |
| safety: 8               |                        |
| exclusion: 15           |                        |
| kb_section: 280         |                        |
+--------------------------------------------------+
| Sample Chunks (first 5)                          |
| +----------------------------------------------+ |
| | [service] Water Damage Restoration           | |
| | 24/7 emergency water extraction and drying   | |
| | Code: WATER_DAMAGE | Emergency: Yes          | |
| +----------------------------------------------+ |
| | [faq] Do you offer financing?                | |
| | Yes, we offer 0% financing for qualified...  | |
| +----------------------------------------------+ |
+--------------------------------------------------+
| [Open in Agent Admin]        [Reindex This Tenant]|
+--------------------------------------------------+
```

---

## Files to Modify

### Backend

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/handlers.go` | Add `HandleGetRAGStats`, `HandleGetTenantRAGDetails`, `HandleTestRAGSearch` |
| `server/router/api/v1/v1.go` | Register 3 new routes in adminGroup |

### Frontend

| File | Changes |
|------|---------|
| `web/src/router/index.tsx` | Add `RAG_STATS` to Routes enum, add route config |
| `web/src/components/Navigation.tsx` | Add ragStatsNavLink to adminNavLinks |
| `web/src/store/v2/ragStats.ts` | **New file** - MobX store |
| `web/src/store/v2/index.ts` | Export ragStatsStore |
| `web/src/pages/RagStats.tsx` | **New file** - Page component |
| `web/src/locales/en.json` | Add rag-stats translations |

---

## Verification

### Build and Run

```bash
# Build with RAG support
task build:backend:rag

# Run with RAG enabled
RAG_PIPELINE_ENABLED=true \
LANCEDB_STORAGE_PROVIDER=local \
EMBEDDING_PROVIDER=mock \
./build/memos --mode dev
```

### Test Checklist

1. **Navigation**
   - [ ] RAG Stats link visible in sidebar (admin only)
   - [ ] Non-admin users do not see the link
   - [ ] Link navigates to /rag-stats

2. **Stats Display**
   - [ ] Status banner shows correct provider info
   - [ ] Total chunks count is accurate
   - [ ] Content distribution shows all types
   - [ ] Per-tenant table populates correctly

3. **Tenant Details**
   - [ ] Click tenant row opens detail modal
   - [ ] Chunks by type/audience are accurate
   - [ ] Sample chunks display correctly
   - [ ] Reindex button works

4. **Search Testing**
   - [ ] Tenant dropdown populates
   - [ ] Search executes and returns results
   - [ ] Hybrid mode toggle works
   - [ ] Weight sliders affect results
   - [ ] Latency displayed correctly
   - [ ] Score breakdown shown for hybrid

5. **Error Handling**
   - [ ] RAG disabled shows appropriate message
   - [ ] API errors display user-friendly messages
   - [ ] Loading states show during fetches

---

## Future Enhancements

1. **Export capabilities** - Download stats as JSON/CSV
2. **Historical tracking** - Track index size over time
3. **Bulk reindex** - Reindex all tenants with progress
4. **Query history** - Save and replay test queries
5. **Embedding visualization** - 2D/3D plot of vector space
6. **Performance metrics** - Search latency percentiles

---

*Document Version: 1.0*
*Created: 2026-01-26*
*Author: Claude Code*
*Status: Planned*
