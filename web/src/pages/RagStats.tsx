import { Button, Chip, DialogContent, DialogTitle, Divider, FormControl, FormLabel, Input, Modal, ModalDialog, Option, Select, Slider, Switch, Table } from "@mui/joy";
import { DatabaseIcon, RefreshCwIcon, SearchIcon, XIcon, InfoIcon, ChevronRightIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import MobileHeader from "@/components/MobileHeader";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { ragStatsStore, userStore } from "@/store/v2";
import type { TenantRAGInfo, SearchParams, TenantRAGDetails } from "@/store/v2/ragStats";
import { cn } from "@/utils";
import { useTranslate } from "@/utils/i18n";
import { User_Role } from "@/types/proto/api/v1/user_service";

// Content type colors for visual distinction
const CONTENT_TYPE_COLORS: Record<string, string> = {
  service: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  faq: "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  rule: "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  coverage: "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200",
  safety: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  exclusion: "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200",
  kb_section: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
  intent: "bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-200",
};

const RagStats = observer(() => {
  const t = useTranslate();
  const { md } = useResponsiveWidth();
  const [showTenantModal, setShowTenantModal] = useState<TenantRAGInfo | null>(null);

  // Search form state
  const [searchTenantId, setSearchTenantId] = useState<number | null>(null);
  const [searchAudience, setSearchAudience] = useState<string>("external");
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [searchTopK, setSearchTopK] = useState<number>(5);
  const [searchMinScore, setSearchMinScore] = useState<number>(0.3);
  const [useHybrid, setUseHybrid] = useState<boolean>(true);
  const [vectorWeight, setVectorWeight] = useState<number>(0.7);
  const [textWeight, setTextWeight] = useState<number>(0.3);

  const { ragStats, selectedTenantDetails, searchResults, isLoading, isLoadingDetails, isSearching, error } = ragStatsStore.state;

  // Get current user and determine if they're an admin
  const currentUserName = userStore.state.currentUser;
  const currentUser = currentUserName ? userStore.state.userMapByName[currentUserName] : null;
  const isAdmin = currentUser && (currentUser.role === User_Role.HOST || currentUser.role === User_Role.ADMIN);

  useEffect(() => {
    if (isAdmin) {
      ragStatsStore.fetchRAGStats();
    }
  }, [isAdmin]);

  useEffect(() => {
    if (error) {
      toast.error(error);
      ragStatsStore.clearError();
    }
  }, [error]);

  // Set default search tenant when stats load
  useEffect(() => {
    if (ragStats?.tenants.length && !searchTenantId) {
      setSearchTenantId(ragStats.tenants[0].id);
    }
  }, [ragStats?.tenants, searchTenantId]);

  const handleRefresh = async () => {
    await ragStatsStore.fetchRAGStats();
    toast.success(t("rag-stats.refreshed"));
  };

  const handleViewTenantDetails = async (tenant: TenantRAGInfo) => {
    setShowTenantModal(tenant);
    await ragStatsStore.fetchTenantDetails(tenant.id);
  };

  const handleCloseTenantModal = () => {
    setShowTenantModal(null);
    ragStatsStore.clearTenantDetails();
  };

  const handleSearch = async () => {
    if (!searchTenantId || !searchQuery.trim()) {
      toast.error(t("rag-stats.search-query-required"));
      return;
    }

    const params: SearchParams = {
      tenantId: searchTenantId,
      audienceType: searchAudience,
      query: searchQuery.trim(),
      topK: searchTopK,
      minScore: searchMinScore,
      useHybridSearch: useHybrid,
      vectorWeight: vectorWeight,
      textWeight: textWeight,
    };

    await ragStatsStore.testSearch(params);
  };

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  };

  const formatTimeAgo = (dateString?: string): string => {
    if (!dateString) return "-";
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffMins < 1) return "just now";
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  };

  // If not admin, show access denied
  if (!isAdmin) {
    return (
      <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-center items-center">
        <div className="text-center p-8">
          <DatabaseIcon className="w-16 h-16 mx-auto mb-4 opacity-30" />
          <h2 className="text-xl font-semibold text-gray-600 dark:text-gray-400">
            {t("rag-stats.admin-required")}
          </h2>
        </div>
      </section>
    );
  }

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full h-full px-4 sm:px-6 flex flex-col">
        {/* Header */}
        <div className="w-full flex flex-row justify-between items-center mb-6">
          <div className="flex items-center gap-2">
            <DatabaseIcon className="w-6 h-6 opacity-70" />
            <h1 className="text-xl font-semibold text-gray-800 dark:text-gray-200">
              {t("rag-stats.title")}
            </h1>
          </div>
          <Button
            variant="outlined"
            color="neutral"
            size="sm"
            onClick={handleRefresh}
            loading={isLoading}
          >
            <RefreshCwIcon className="w-4 h-4 mr-1" />
            {t("rag-stats.refresh")}
          </Button>
        </div>

        {isLoading && !ragStats ? (
          <div className="flex justify-center items-center py-12">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-gray-100"></div>
          </div>
        ) : !ragStats?.enabled ? (
          <div className="text-center p-8 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg border border-yellow-200 dark:border-yellow-800">
            <InfoIcon className="w-12 h-12 mx-auto mb-4 text-yellow-600 dark:text-yellow-400" />
            <h2 className="text-lg font-semibold text-yellow-800 dark:text-yellow-200 mb-2">
              {t("rag-stats.rag-disabled")}
            </h2>
            <p className="text-yellow-700 dark:text-yellow-300 text-sm">
              {t("rag-stats.rag-disabled-hint")}
            </p>
          </div>
        ) : (
          <>
            {/* Status Banner */}
            <div className="w-full p-4 bg-gray-50 dark:bg-gray-800 rounded-lg mb-6">
              <div className="flex flex-wrap gap-4 items-center">
                <Chip color="success" variant="soft" size="sm">
                  {t("rag-stats.enabled")}
                </Chip>
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  {t("rag-stats.storage")}: <strong>{ragStats.storageProvider}</strong>
                </span>
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  {t("rag-stats.embedding")}: <strong>{ragStats.embeddingProvider}</strong>
                </span>
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  {t("rag-stats.hybrid")}:{" "}
                  <Chip color={ragStats.hybridSearchEnabled ? "success" : "neutral"} variant="soft" size="sm">
                    {ragStats.hybridSearchEnabled ? "ON" : "OFF"}
                  </Chip>
                </span>
              </div>
            </div>

            {/* Stats Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
              {/* Total Chunks Card */}
              <div className="p-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">
                  {t("rag-stats.total-chunks")}
                </h3>
                <p className="text-3xl font-bold text-gray-900 dark:text-gray-100">
                  {ragStats.stats.totalChunks.toLocaleString()}
                </p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  {t("rag-stats.index-size")}: {formatBytes(ragStats.stats.indexSize)}
                </p>
              </div>

              {/* Content Distribution Card */}
              <div className="p-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">
                  {t("rag-stats.content-distribution")}
                </h3>
                <div className="space-y-1">
                  {Object.entries(ragStats.stats.contentCounts || {}).map(([type, count]) => (
                    <div key={type} className="flex items-center gap-2">
                      <span className={cn("px-2 py-0.5 rounded text-xs font-medium", CONTENT_TYPE_COLORS[type] || "bg-gray-100 text-gray-800")}>
                        {type}
                      </span>
                      <div className="flex-1 bg-gray-200 dark:bg-gray-700 rounded-full h-2">
                        <div
                          className="bg-blue-500 h-2 rounded-full"
                          style={{ width: `${Math.min(100, (count / ragStats.stats.totalChunks) * 100)}%` }}
                        />
                      </div>
                      <span className="text-sm text-gray-600 dark:text-gray-400 w-12 text-right">
                        {count}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            {/* Per-Tenant Table */}
            <div className="mb-6">
              <h2 className="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">
                {t("rag-stats.per-tenant")}
              </h2>
              <div className="overflow-x-auto">
                <Table
                  aria-label="Tenant RAG stats"
                  sx={{
                    "& th": { bgcolor: "background.level1" },
                  }}
                >
                  <thead>
                    <tr>
                      <th>{t("rag-stats.tenant")}</th>
                      <th style={{ width: 100 }}>{t("rag-stats.chunks")}</th>
                      <th style={{ width: 120 }}>{t("rag-stats.last-indexed")}</th>
                      <th style={{ width: 80 }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {ragStats.tenants.map((tenant) => (
                      <tr key={tenant.id}>
                        <td>
                          <div>
                            <span className="font-medium">{tenant.companyName}</span>
                            <span className="text-gray-500 dark:text-gray-400 text-sm ml-2">
                              ({tenant.slug})
                            </span>
                          </div>
                        </td>
                        <td>{tenant.chunkCount.toLocaleString()}</td>
                        <td className="text-sm text-gray-500">{formatTimeAgo(tenant.lastIndexed)}</td>
                        <td>
                          <Button
                            variant="plain"
                            color="neutral"
                            size="sm"
                            onClick={() => handleViewTenantDetails(tenant)}
                          >
                            <ChevronRightIcon className="w-4 h-4" />
                          </Button>
                        </td>
                      </tr>
                    ))}
                    {ragStats.tenants.length === 0 && (
                      <tr>
                        <td colSpan={4} className="text-center text-gray-500 py-4">
                          {t("rag-stats.no-tenants")}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </Table>
              </div>
            </div>

            <Divider sx={{ my: 2 }} />

            {/* Search Testing Section */}
            <div className="mb-6">
              <h2 className="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">
                {t("rag-stats.search-testing")}
              </h2>
              <div className="p-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                {/* Search Form */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                  <FormControl size="sm">
                    <FormLabel>{t("rag-stats.tenant")}</FormLabel>
                    <Select
                      value={searchTenantId?.toString() || ""}
                      onChange={(_, value) => setSearchTenantId(value ? parseInt(value as string, 10) : null)}
                    >
                      {ragStats.tenants.map((tenant) => (
                        <Option key={tenant.id} value={tenant.id.toString()}>
                          {tenant.companyName}
                        </Option>
                      ))}
                    </Select>
                  </FormControl>
                  <FormControl size="sm">
                    <FormLabel>{t("rag-stats.audience")}</FormLabel>
                    <Select
                      value={searchAudience}
                      onChange={(_, value) => setSearchAudience(value as string)}
                    >
                      <Option value="external">External</Option>
                      <Option value="internal">Internal</Option>
                    </Select>
                  </FormControl>
                </div>

                <div className="flex gap-2 mb-4">
                  <FormControl size="sm" sx={{ flex: 1 }}>
                    <Input
                      placeholder={t("rag-stats.query-placeholder")}
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      onKeyDown={(e) => e.key === "Enter" && handleSearch()}
                    />
                  </FormControl>
                  <Button
                    variant="solid"
                    color="primary"
                    size="sm"
                    onClick={handleSearch}
                    loading={isSearching}
                    disabled={!searchQuery.trim()}
                  >
                    <SearchIcon className="w-4 h-4 mr-1" />
                    {t("rag-stats.search")}
                  </Button>
                </div>

                {/* Advanced Options */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 p-3 bg-gray-50 dark:bg-gray-900 rounded">
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={useHybrid}
                      onChange={(e) => setUseHybrid(e.target.checked)}
                      size="sm"
                    />
                    <span className="text-sm">{t("rag-stats.hybrid-mode")}</span>
                  </div>
                  <FormControl size="sm">
                    <FormLabel>{t("rag-stats.top-k")}</FormLabel>
                    <Input
                      type="number"
                      value={searchTopK}
                      onChange={(e) => setSearchTopK(parseInt(e.target.value) || 5)}
                      slotProps={{ input: { min: 1, max: 20 } }}
                    />
                  </FormControl>
                  <FormControl size="sm">
                    <FormLabel>{t("rag-stats.min-score")}</FormLabel>
                    <Input
                      type="number"
                      value={searchMinScore}
                      onChange={(e) => setSearchMinScore(parseFloat(e.target.value) || 0)}
                      slotProps={{ input: { min: 0, max: 1, step: 0.1 } }}
                    />
                  </FormControl>
                  {useHybrid && (
                    <FormControl size="sm">
                      <FormLabel>{t("rag-stats.weights")} (V:{vectorWeight}/T:{textWeight})</FormLabel>
                      <Slider
                        value={vectorWeight}
                        onChange={(_, value) => {
                          const v = value as number;
                          setVectorWeight(v);
                          setTextWeight(parseFloat((1 - v).toFixed(1)));
                        }}
                        min={0}
                        max={1}
                        step={0.1}
                        size="sm"
                      />
                    </FormControl>
                  )}
                </div>

                {/* Search Results */}
                {searchResults && (
                  <div className="mt-4">
                    <div className="flex items-center gap-2 mb-2 text-sm text-gray-600 dark:text-gray-400">
                      <span>
                        {t("rag-stats.results-found", { count: searchResults.totalResults })}
                      </span>
                      <span>|</span>
                      <span>{t("rag-stats.latency", { ms: searchResults.latencyMs })}</span>
                      <span>|</span>
                      <Chip size="sm" variant="soft">
                        {searchResults.searchMode}
                      </Chip>
                    </div>
                    <div className="space-y-2">
                      {searchResults.results.map((result, idx) => (
                        <div
                          key={idx}
                          className="p-3 bg-gray-50 dark:bg-gray-900 rounded border border-gray-200 dark:border-gray-700"
                        >
                          <div className="flex items-start justify-between gap-2 mb-1">
                            <div className="flex items-center gap-2">
                              <span className={cn("px-2 py-0.5 rounded text-xs font-medium", CONTENT_TYPE_COLORS[result.chunk.contentType] || "bg-gray-100 text-gray-800")}>
                                {result.chunk.contentType}
                              </span>
                              <span className="font-medium text-sm">{result.chunk.title}</span>
                            </div>
                            <div className="flex items-center gap-2 text-xs text-gray-500">
                              <span className="font-mono bg-gray-200 dark:bg-gray-700 px-1 rounded">
                                {result.score.toFixed(3)}
                              </span>
                              {useHybrid && result.vectorScore !== undefined && (
                                <span className="text-blue-600 dark:text-blue-400">
                                  V:{result.vectorScore.toFixed(2)}
                                </span>
                              )}
                              {useHybrid && result.bm25Score !== undefined && (
                                <span className="text-green-600 dark:text-green-400">
                                  T:{result.bm25Score.toFixed(2)}
                                </span>
                              )}
                            </div>
                          </div>
                          <p className="text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
                            {result.chunk.content}
                          </p>
                        </div>
                      ))}
                      {searchResults.results.length === 0 && (
                        <div className="text-center text-gray-500 py-4">
                          {t("rag-stats.no-results")}
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </div>

      {/* Tenant Details Modal */}
      <Modal open={!!showTenantModal} onClose={handleCloseTenantModal}>
        <ModalDialog sx={{ maxWidth: 600, maxHeight: "80vh", overflow: "auto" }}>
          <DialogTitle>
            {t("rag-stats.tenant-details")}: {showTenantModal?.companyName}
          </DialogTitle>
          <DialogContent>
            {isLoadingDetails ? (
              <div className="flex justify-center py-8">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-gray-100"></div>
              </div>
            ) : selectedTenantDetails ? (
              <div className="space-y-4">
                <div>
                  <p className="text-sm text-gray-500">Slug: <strong>{selectedTenantDetails.slug}</strong></p>
                </div>

                <div>
                  <h4 className="font-medium mb-2">{t("rag-stats.chunks-by-type")}</h4>
                  <div className="grid grid-cols-2 gap-2">
                    {Object.entries(selectedTenantDetails.chunksByType || {}).map(([type, count]) => (
                      <div key={type} className="flex justify-between items-center">
                        <span className={cn("px-2 py-0.5 rounded text-xs font-medium", CONTENT_TYPE_COLORS[type] || "bg-gray-100 text-gray-800")}>
                          {type}
                        </span>
                        <span className="text-sm">{count}</span>
                      </div>
                    ))}
                  </div>
                </div>

                <div>
                  <h4 className="font-medium mb-2">{t("rag-stats.chunks-by-audience")}</h4>
                  <div className="grid grid-cols-2 gap-2">
                    {Object.entries(selectedTenantDetails.chunksByAudience || {}).map(([audience, count]) => (
                      <div key={audience} className="flex justify-between items-center">
                        <span className="capitalize">{audience}</span>
                        <span className="text-sm">{count}</span>
                      </div>
                    ))}
                  </div>
                </div>

                {selectedTenantDetails.sampleChunks.length > 0 && (
                  <div>
                    <h4 className="font-medium mb-2">{t("rag-stats.sample-chunks")}</h4>
                    <div className="space-y-2 max-h-48 overflow-y-auto">
                      {selectedTenantDetails.sampleChunks.map((chunk) => (
                        <div key={chunk.id} className="p-2 bg-gray-50 dark:bg-gray-800 rounded text-sm">
                          <div className="flex items-center gap-2 mb-1">
                            <span className={cn("px-1.5 py-0.5 rounded text-xs", CONTENT_TYPE_COLORS[chunk.contentType] || "bg-gray-100 text-gray-800")}>
                              {chunk.contentType}
                            </span>
                            <span className="font-medium truncate">{chunk.title}</span>
                          </div>
                          <p className="text-gray-600 dark:text-gray-400 text-xs line-clamp-2">
                            {chunk.content}
                          </p>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ) : null}
          </DialogContent>
          <div className="flex justify-end pt-2">
            <Button variant="plain" color="neutral" onClick={handleCloseTenantModal}>
              <XIcon className="w-4 h-4 mr-1" />
              {t("common.close")}
            </Button>
          </div>
        </ModalDialog>
      </Modal>
    </section>
  );
});

export default RagStats;
