import { Button, Checkbox, Chip, DialogActions, DialogContent, DialogTitle, Divider, FormControl, FormHelperText, FormLabel, Input, Modal, ModalClose, ModalDialog, Option, Select, Slider, Switch, Textarea } from "@mui/joy";
import { ArrowLeftIcon, BuildingIcon, CheckIcon, ChevronDownIcon, ChevronUpIcon, CodeIcon, CopyIcon, EditIcon, EyeIcon, EyeOffIcon, FileTextIcon, HistoryIcon, MessageCircleIcon, PlusIcon, RefreshCwIcon, SearchIcon, SettingsIcon, SparklesIcon, Trash2Icon, UploadIcon, XIcon, ZapIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { ChangeEvent, useEffect, useRef, useState } from "react";
import toast from "react-hot-toast";
import MobileHeader from "@/components/MobileHeader";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { agentAdminStore, userStore } from "@/store/v2";
import type { AgentTenant, AgentTranscript, CreateTenantRequest, LLMConfig, SetLLMConfigRequest, UserPermission, GrantPermissionRequest, ProcessingOptions, FormatForRAGResponse } from "@/store/v2/agentAdmin";
import { LLM_MODEL_OPTIONS, PERMISSION_PRESETS, DEFAULT_PROCESSING_OPTIONS } from "@/store/v2/agentAdmin";
import { cn } from "@/utils";
import { useTranslate } from "@/utils/i18n";
import { User_Role } from "@/types/proto/api/v1/user_service";

const AgentAdmin = observer(() => {
  const t = useTranslate();
  const { md } = useResponsiveWidth();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState<AgentTenant | null>(null);
  const [showVersionsModal, setShowVersionsModal] = useState<{ slug: string; audienceType: string; fileType: string } | null>(null);
  const [isRebuilding, setIsRebuilding] = useState(false);
  // Auto-generate annotated content state
  const [showGeneratedContent, setShowGeneratedContent] = useState<{ type: "kb" | "policy" | "rag"; content: string; stats?: FormatForRAGResponse["stats"] } | null>(null);
  const [isGenerating, setIsGenerating] = useState<"kb" | "policy" | "rag" | null>(null);
  // Processing options for Format for RAG
  const [processingOptions, setProcessingOptions] = useState<ProcessingOptions>(DEFAULT_PROCESSING_OPTIONS);
  const [showProcessingOptions, setShowProcessingOptions] = useState(false);
  const [formatFileType, setFormatFileType] = useState<"kb" | "policy">("kb");
  const [hasCustomOptions, setHasCustomOptions] = useState(false);
  const [isSavingOptions, setIsSavingOptions] = useState(false);
  // Q&A Pairs state
  const [showQAModal, setShowQAModal] = useState(false);

  // RAG Search Explorer state
  const [showSearchExplorer, setShowSearchExplorer] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [searchTopK, setSearchTopK] = useState(5);

  // Transcripts state
  const [showTranscripts, setShowTranscripts] = useState(false);
  const [selectedTranscript, setSelectedTranscript] = useState<AgentTranscript | null>(null);
  const [showTranscriptModal, setShowTranscriptModal] = useState(false);

  // Widget embed state
  const [widgetColor, setWidgetColor] = useState("#0d9488");
  const [widgetPosition, setWidgetPosition] = useState<"bottom-right" | "bottom-left">("bottom-right");
  const [widgetWelcome, setWidgetWelcome] = useState("Hi! How can I help you today?");
  const [showWidgetPreview, setShowWidgetPreview] = useState(true);

  const { tenants, selectedTenant, isLoading, isSaving, error, fileVersions, llmConfig, tenantPermissions, myPermissions, script, isLoadingScript, qaPairs, qaTestResults, isGeneratingQA, isTestingQA, ragSearchResults, isSearchingRAG, transcripts, isLoadingTranscripts, tenantSettings } = agentAdminStore.state;

  // Get current user and determine if they're an admin
  const currentUserName = userStore.state.currentUser;
  const currentUser = currentUserName ? userStore.state.userMapByName[currentUserName] : null;
  const isAdmin = currentUser && (currentUser.role === User_Role.HOST || currentUser.role === User_Role.ADMIN);

  // Permission helpers for non-admin users
  const canRead = isAdmin || agentAdminStore.hasPermission("tenant:read");
  const canWrite = isAdmin || agentAdminStore.hasPermission("tenant:write");
  const canUpload = isAdmin || agentAdminStore.hasPermission("files:upload");
  const canRestore = isAdmin || agentAdminStore.hasPermission("files:restore");
  const canConfigApi = isAdmin || agentAdminStore.hasPermission("api:config");
  const canManagePermissions = isAdmin || agentAdminStore.hasPermission("tenant:admin");

  useEffect(() => {
    // Fetch tenants based on user role
    if (isAdmin) {
      agentAdminStore.fetchTenants();
    } else {
      agentAdminStore.fetchUserTenants();
    }
  }, [isAdmin]);

  useEffect(() => {
    if (selectedTenant) {
      // Set user's permissions for this tenant (for non-admin users)
      agentAdminStore.setMyPermissions(selectedTenant.tenant.id);
      // Fetch LLM config if user can read or configure API
      if (isAdmin || agentAdminStore.hasPermission("tenant:read") || agentAdminStore.hasPermission("api:config")) {
        agentAdminStore.fetchLLMConfig(selectedTenant.tenant.slug);
      }
      // Only fetch tenant permissions if user can manage them
      if (isAdmin || agentAdminStore.hasPermission("tenant:admin")) {
        agentAdminStore.fetchTenantPermissions(selectedTenant.tenant.slug);
      }
      // Fetch script if user can read
      if (isAdmin || agentAdminStore.hasPermission("tenant:read")) {
        agentAdminStore.fetchScript(selectedTenant.tenant.slug);
      }
      // Load saved processing options for this tenant
      if (isAdmin) {
        agentAdminStore.loadProcessingOptions(selectedTenant.tenant.slug).then((result) => {
          if (!result.error) {
            setProcessingOptions(result.options);
            setHasCustomOptions(result.hasCustom);
          }
        });
        // Fetch Q&A pairs for this tenant
        agentAdminStore.fetchQAPairs(selectedTenant.tenant.slug);
        // Fetch tenant settings and transcripts
        agentAdminStore.fetchTenantSettings(selectedTenant.tenant.slug);
        agentAdminStore.fetchTranscripts(selectedTenant.tenant.slug);
      }
    }
  }, [selectedTenant?.tenant.slug, selectedTenant?.tenant.id, isAdmin]);

  useEffect(() => {
    if (error) {
      toast.error(error);
      agentAdminStore.clearError();
    }
  }, [error]);

  const handleSelectTenant = async (slug: string, tenantId: number) => {
    // Set permissions first so they're available when tenant loads
    agentAdminStore.setMyPermissions(tenantId);
    await agentAdminStore.fetchTenantConfig(slug);
  };

  const handleBackToList = () => {
    agentAdminStore.clearSelectedTenant();
    agentAdminStore.clearMyPermissions();
  };

  const handleDeleteTenant = async () => {
    if (showDeleteModal) {
      const success = await agentAdminStore.deleteTenant(showDeleteModal.slug);
      if (success) {
        toast.success(t("agent-admin.deleted-successfully"));
        setShowDeleteModal(null);
      }
    }
  };

  const handleRebuildIndex = async () => {
    if (!selectedTenant) return;
    setIsRebuilding(true);
    const result = await agentAdminStore.reindexTenant(selectedTenant.tenant.slug);
    setIsRebuilding(false);
    if (result.success) {
      toast.success(t("agent-admin.rebuild-index-success", { chunks: result.chunks || 0 }));
    } else {
      toast.error(result.error || t("agent-admin.rebuild-index-failed"));
    }
  };

  const handleGenerateQAPairs = async () => {
    if (!selectedTenant) return;
    const result = await agentAdminStore.generateQAPairs(selectedTenant.tenant.slug, 50);
    if (result.success) {
      toast.success(t("agent-admin.qa-generated", { count: result.count || 0 }));
    } else {
      toast.error(result.error || t("agent-admin.qa-generate-failed"));
    }
  };

  const handleTestAllQAPairs = async () => {
    if (!selectedTenant) return;
    const result = await agentAdminStore.testAllQAPairs(selectedTenant.tenant.slug);
    if (result.success && result.results) {
      const recall = (result.results.recall_at_5 * 100).toFixed(0);
      toast.success(t("agent-admin.qa-test-complete", {
        found: result.results.found,
        total: result.results.total_pairs,
        recall,
      }));
    } else {
      toast.error(result.error || t("agent-admin.qa-test-failed"));
    }
  };

  const handleDeleteQAPair = async (pairId: number) => {
    if (!selectedTenant) return;
    await agentAdminStore.deleteQAPair(selectedTenant.tenant.slug, pairId);
  };

  // RAG Search Explorer handler
  const handleRAGSearch = async () => {
    if (!selectedTenant || !searchQuery.trim()) return;
    const result = await agentAdminStore.searchRAG(
      selectedTenant.tenant.slug,
      searchQuery.trim(),
      "internal",
      searchTopK
    );
    if (!result.success) {
      toast.error(result.error || "Search failed");
    }
  };

  const handleViewVersions = async (slug: string, audienceType: string, fileType: string) => {
    await agentAdminStore.fetchFileVersions(slug, audienceType, fileType);
    setShowVersionsModal({ slug, audienceType, fileType });
  };

  const handleRestoreVersion = async (versionId: number) => {
    if (showVersionsModal) {
      const success = await agentAdminStore.restoreFileVersion(
        showVersionsModal.slug,
        showVersionsModal.audienceType,
        showVersionsModal.fileType,
        versionId
      );
      if (success) {
        toast.success("Version restored successfully");
        setShowVersionsModal(null);
      }
    }
  };

  // Auto-generate annotated KB.MD
  const handleGenerateKB = async () => {
    if (!selectedTenant) return;
    setIsGenerating("kb");
    const result = await agentAdminStore.generateKB(selectedTenant.tenant.slug);
    setIsGenerating(null);
    if (result.content) {
      setShowGeneratedContent({ type: "kb", content: result.content });
    } else {
      toast.error(result.error || "Failed to generate KB.MD");
    }
  };

  // Auto-generate annotated POLICY.MD
  const handleGeneratePolicy = async () => {
    if (!selectedTenant) return;
    setIsGenerating("policy");
    const result = await agentAdminStore.generatePolicy(selectedTenant.tenant.slug);
    setIsGenerating(null);
    if (result.content) {
      setShowGeneratedContent({ type: "policy", content: result.content });
    } else {
      toast.error(result.error || "Failed to generate POLICY.MD");
    }
  };

  // Format content for RAG (rule-based, no LLM)
  const handleFormatForRAG = async () => {
    if (!selectedTenant) return;
    setIsGenerating("rag");
    const result = await agentAdminStore.formatForRAG(
      selectedTenant.tenant.slug,
      formatFileType,
      processingOptions
    );
    setIsGenerating(null);
    if (result.result) {
      setShowGeneratedContent({
        type: "rag",
        content: result.result.content,
        stats: result.result.stats,
      });
    } else {
      toast.error(result.error || "Failed to format content");
    }
  };

  // Update a processing option
  const updateProcessingOption = <K extends keyof ProcessingOptions>(key: K, value: ProcessingOptions[K]) => {
    setProcessingOptions((prev) => ({ ...prev, [key]: value }));
  };

  // Save processing options as tenant defaults
  const handleSaveProcessingOptions = async () => {
    if (!selectedTenant) return;
    setIsSavingOptions(true);
    const result = await agentAdminStore.saveProcessingOptions(
      selectedTenant.tenant.slug,
      processingOptions
    );
    setIsSavingOptions(false);
    if (result.success) {
      setHasCustomOptions(true);
      toast.success(t("agent-admin.processing-options-saved"));
    } else {
      toast.error(result.error || t("agent-admin.processing-options-save-failed"));
    }
  };

  // Reset processing options to defaults
  const handleResetProcessingOptions = () => {
    setProcessingOptions(DEFAULT_PROCESSING_OPTIONS);
  };

  // Copy generated content to clipboard
  const handleCopyGeneratedContent = () => {
    if (showGeneratedContent) {
      navigator.clipboard.writeText(showGeneratedContent.content);
      toast.success(t("agent-admin.copied-to-clipboard"));
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success("Copied to clipboard");
  };

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full h-full px-4 sm:px-6 flex flex-col">
        {/* Header */}
        <div className="w-full flex flex-row justify-between items-center mb-6">
          <div className="flex items-center gap-2">
            {selectedTenant && (
              <Button variant="plain" color="neutral" size="sm" onClick={handleBackToList}>
                <ArrowLeftIcon className="w-4 h-4" />
              </Button>
            )}
            <SettingsIcon className="w-6 h-6 opacity-70" />
            <h1 className="text-xl font-semibold text-gray-800 dark:text-gray-200">
              {selectedTenant ? selectedTenant.tenant.companyName : t("agent-admin.title")}
            </h1>
          </div>
          {!selectedTenant && isAdmin && (
            <Button color="primary" startDecorator={<PlusIcon className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
              {t("agent-admin.create-tenant")}
            </Button>
          )}
        </div>

        {!selectedTenant && <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">{t("agent-admin.description")}</p>}

        {/* Tenant List View */}
        {!selectedTenant && (
          <div className="w-full">
            {isLoading ? (
              <div className="text-center py-8 text-gray-500">Loading...</div>
            ) : tenants.length === 0 ? (
              <div className="text-center py-12 bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700">
                <BuildingIcon className="w-12 h-12 mx-auto mb-4 opacity-30 text-gray-500" />
                <p className="text-gray-500">{t("agent-admin.no-tenants")}</p>
              </div>
            ) : (
              <div className="grid gap-4">
                {tenants.map((tenant) => (
                  <div
                    key={tenant.id}
                    className="bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4 hover:border-teal-500 transition-colors cursor-pointer"
                    onClick={() => handleSelectTenant(tenant.slug, tenant.id)}
                  >
                    <div className="flex justify-between items-start">
                      <div>
                        <div className="flex items-center gap-2 mb-1">
                          <h3 className="font-semibold text-gray-800 dark:text-gray-200">{tenant.companyName}</h3>
                          <Chip size="sm" color={tenant.isActive ? "success" : "neutral"} variant="soft">
                            {tenant.isActive ? t("agent-admin.active") : t("agent-admin.inactive")}
                          </Chip>
                        </div>
                        <p className="text-sm text-gray-500 dark:text-gray-400">
                          <code className="bg-gray-100 dark:bg-zinc-700 px-1 rounded">{tenant.slug}</code>
                          {tenant.vertical && <span className="ml-2">• {tenant.vertical}</span>}
                        </p>
                      </div>
                      <div className="flex gap-2" onClick={(e) => e.stopPropagation()}>
                        <Button
                          variant="plain"
                          color="neutral"
                          size="sm"
                          onClick={() => handleSelectTenant(tenant.slug, tenant.id)}
                        >
                          <EditIcon className="w-4 h-4" />
                        </Button>
                        {isAdmin && (
                          <Button
                            variant="plain"
                            color="danger"
                            size="sm"
                            onClick={() => setShowDeleteModal(tenant)}
                          >
                            <Trash2Icon className="w-4 h-4" />
                          </Button>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Tenant Detail View */}
        {selectedTenant && (
          <div className="w-full space-y-6">
            {/* Status Toggle */}
            <div className="bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4">
              <div className="flex justify-between items-center">
                <div>
                  <h3 className="font-medium text-gray-800 dark:text-gray-200">{t("agent-admin.status")}</h3>
                  <p className="text-sm text-gray-500">
                    Slug: <code className="bg-gray-100 dark:bg-zinc-700 px-1 rounded">{selectedTenant.tenant.slug}</code>
                    {selectedTenant.tenant.vertical && <span className="ml-2">• {selectedTenant.tenant.vertical}</span>}
                  </p>
                </div>
                <Switch
                  checked={selectedTenant.tenant.isActive}
                  onChange={(e) => agentAdminStore.toggleTenantActive(selectedTenant.tenant.slug, e.target.checked)}
                  disabled={isSaving}
                />
              </div>
            </div>

            {/* LLM Configuration - visible to users with tenant:read or api:config */}
            {(canRead || canConfigApi) && (
              <LLMConfigSection
                tenantSlug={selectedTenant.tenant.slug}
                config={llmConfig}
                isSaving={isSaving}
                canEdit={canConfigApi}
                t={t}
              />
            )}

            {/* User Permissions - only visible to admins and tenant:admin */}
            {canManagePermissions && (
              <UserPermissionsSection
                tenantSlug={selectedTenant.tenant.slug}
                permissions={tenantPermissions}
                isSaving={isSaving}
              />
            )}

            {/* SCRIPT.MD - Conversation Flow Guide (tenant-level) */}
            {(canRead || canUpload) && (
              <ScriptSection
                tenantSlug={selectedTenant.tenant.slug}
                script={script}
                isLoading={isLoadingScript}
                isSaving={isSaving}
                canUpload={canUpload}
                t={t}
              />
            )}

            {/* External Configuration */}
            <AudienceSection
              title={t("agent-admin.external")}
              audienceType="external"
              tenant={selectedTenant}
              onViewVersions={handleViewVersions}
              isSaving={isSaving}
              t={t}
              canUpload={canUpload}
              canRestore={canRestore}
            />

            {/* Auto-Generate & Format Content - Admin only */}
            {isAdmin && (
              <div className="bg-purple-50 dark:bg-purple-900/20 rounded-xl border border-purple-200 dark:border-purple-800 p-4">
                <div className="flex flex-col gap-3">
                  <div>
                    <h3 className="font-medium text-purple-700 dark:text-purple-300 flex items-center gap-2">
                      <SparklesIcon className="w-4 h-4" />
                      {t("agent-admin.auto-generate-title")}
                    </h3>
                    <p className="text-sm text-purple-600 dark:text-purple-400">{t("agent-admin.auto-generate-desc")}</p>
                  </div>

                  {/* Reasoning Model for Generate KB/Policy */}
                  <ReasoningModelInput
                    tenantSlug={selectedTenant.tenant.slug}
                    config={llmConfig}
                    isSaving={isSaving}
                    t={t}
                  />

                  {/* Processing Options Toggle */}
                  <button
                    onClick={() => setShowProcessingOptions(!showProcessingOptions)}
                    className="flex items-center gap-2 text-sm text-purple-600 dark:text-purple-400 hover:text-purple-800 dark:hover:text-purple-200"
                  >
                    {showProcessingOptions ? <ChevronUpIcon className="w-4 h-4" /> : <ChevronDownIcon className="w-4 h-4" />}
                    {t("agent-admin.processing-options")}
                  </button>

                  {/* Processing Options Panel */}
                  {showProcessingOptions && (
                    <div className="bg-white dark:bg-zinc-900 rounded-lg border border-purple-200 dark:border-purple-700 p-4 space-y-4">
                      {/* File Type Selection */}
                      <div className="flex items-center gap-4 pb-3 border-b border-purple-100 dark:border-purple-800">
                        <span className="text-sm font-medium text-purple-700 dark:text-purple-300">Process:</span>
                        <div className="flex gap-2">
                          <Button
                            size="sm"
                            variant={formatFileType === "kb" ? "solid" : "outlined"}
                            color="primary"
                            onClick={() => setFormatFileType("kb")}
                          >
                            KB.MD
                          </Button>
                          <Button
                            size="sm"
                            variant={formatFileType === "policy" ? "solid" : "outlined"}
                            color="primary"
                            onClick={() => setFormatFileType("policy")}
                          >
                            POLICY.MD
                          </Button>
                        </div>
                      </div>

                      {/* Content Extraction */}
                      <div>
                        <h4 className="text-xs font-semibold text-purple-700 dark:text-purple-300 mb-2 uppercase tracking-wide">
                          {t("agent-admin.content-extraction")}
                        </h4>
                        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                          <Checkbox
                            label={t("agent-admin.extract-faqs")}
                            checked={processingOptions.extract_faqs}
                            onChange={(e) => updateProcessingOption("extract_faqs", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.extract-services")}
                            checked={processingOptions.extract_services}
                            onChange={(e) => updateProcessingOption("extract_services", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.extract-exclusions")}
                            checked={processingOptions.extract_exclusions}
                            onChange={(e) => updateProcessingOption("extract_exclusions", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.extract-coverage")}
                            checked={processingOptions.extract_coverage}
                            onChange={(e) => updateProcessingOption("extract_coverage", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.extract-safety")}
                            checked={processingOptions.extract_safety}
                            onChange={(e) => updateProcessingOption("extract_safety", e.target.checked)}
                            size="sm"
                          />
                        </div>
                      </div>

                      {/* Text Normalization */}
                      <div>
                        <h4 className="text-xs font-semibold text-purple-700 dark:text-purple-300 mb-2 uppercase tracking-wide">
                          {t("agent-admin.text-normalization")}
                        </h4>
                        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                          <Checkbox
                            label={t("agent-admin.remove-whitespace")}
                            checked={processingOptions.remove_whitespace}
                            onChange={(e) => updateProcessingOption("remove_whitespace", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.strip-html")}
                            checked={processingOptions.strip_html}
                            onChange={(e) => updateProcessingOption("strip_html", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.fix-encoding")}
                            checked={processingOptions.fix_encoding}
                            onChange={(e) => updateProcessingOption("fix_encoding", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.remove-page-numbers")}
                            checked={processingOptions.remove_page_numbers}
                            onChange={(e) => updateProcessingOption("remove_page_numbers", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.remove-headers-footers")}
                            checked={processingOptions.remove_header_footer}
                            onChange={(e) => updateProcessingOption("remove_header_footer", e.target.checked)}
                            size="sm"
                          />
                        </div>
                      </div>

                      {/* Structure Splitting */}
                      <div>
                        <h4 className="text-xs font-semibold text-purple-700 dark:text-purple-300 mb-2 uppercase tracking-wide">
                          {t("agent-admin.structure-splitting")}
                        </h4>
                        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                          <Checkbox
                            label={t("agent-admin.split-h2")}
                            checked={processingOptions.split_h2}
                            onChange={(e) => updateProcessingOption("split_h2", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.split-h3")}
                            checked={processingOptions.split_h3}
                            onChange={(e) => updateProcessingOption("split_h3", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.split-paragraphs")}
                            checked={processingOptions.split_paragraphs}
                            onChange={(e) => updateProcessingOption("split_paragraphs", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.split-sentences")}
                            checked={processingOptions.split_sentences}
                            onChange={(e) => updateProcessingOption("split_sentences", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.preserve-lists")}
                            checked={processingOptions.preserve_lists}
                            onChange={(e) => updateProcessingOption("preserve_lists", e.target.checked)}
                            size="sm"
                          />
                        </div>
                      </div>

                      {/* Chunking Controls */}
                      <div>
                        <h4 className="text-xs font-semibold text-purple-700 dark:text-purple-300 mb-2 uppercase tracking-wide">
                          {t("agent-admin.chunking-controls")}
                        </h4>
                        <div className="space-y-3">
                          <div className="flex items-center gap-4">
                            <span className="text-xs text-gray-600 dark:text-gray-400 w-24">{t("agent-admin.max-chunk-size")}</span>
                            <Slider
                              value={processingOptions.max_chunk_size}
                              onChange={(_, value) => updateProcessingOption("max_chunk_size", value as number)}
                              min={400}
                              max={1200}
                              step={50}
                              valueLabelDisplay="auto"
                              sx={{ flex: 1 }}
                            />
                            <span className="text-xs text-gray-500 w-16">{processingOptions.max_chunk_size} {t("agent-admin.tokens")}</span>
                          </div>
                          <div className="flex items-center gap-4">
                            <span className="text-xs text-gray-600 dark:text-gray-400 w-24">{t("agent-admin.min-chunk-size")}</span>
                            <Slider
                              value={processingOptions.min_chunk_size}
                              onChange={(_, value) => updateProcessingOption("min_chunk_size", value as number)}
                              min={50}
                              max={200}
                              step={10}
                              valueLabelDisplay="auto"
                              sx={{ flex: 1 }}
                            />
                            <span className="text-xs text-gray-500 w-16">{processingOptions.min_chunk_size} {t("agent-admin.tokens")}</span>
                          </div>
                          <div className="flex items-center gap-4">
                            <span className="text-xs text-gray-600 dark:text-gray-400 w-24">{t("agent-admin.chunk-overlap")}</span>
                            <Slider
                              value={processingOptions.chunk_overlap}
                              onChange={(_, value) => updateProcessingOption("chunk_overlap", value as number)}
                              min={0}
                              max={100}
                              step={10}
                              valueLabelDisplay="auto"
                              sx={{ flex: 1 }}
                            />
                            <span className="text-xs text-gray-500 w-16">{processingOptions.chunk_overlap} {t("agent-admin.tokens")}</span>
                          </div>
                          <Checkbox
                            label={t("agent-admin.merge-small-chunks")}
                            checked={processingOptions.merge_small_chunks}
                            onChange={(e) => updateProcessingOption("merge_small_chunks", e.target.checked)}
                            size="sm"
                          />
                        </div>
                      </div>

                      {/* Metadata */}
                      <div>
                        <h4 className="text-xs font-semibold text-purple-700 dark:text-purple-300 mb-2 uppercase tracking-wide">
                          {t("agent-admin.metadata-options")}
                        </h4>
                        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                          <Checkbox
                            label={t("agent-admin.generate-titles")}
                            checked={processingOptions.generate_titles}
                            onChange={(e) => updateProcessingOption("generate_titles", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.add-source-ref")}
                            checked={processingOptions.add_source_ref}
                            onChange={(e) => updateProcessingOption("add_source_ref", e.target.checked)}
                            size="sm"
                          />
                          <Checkbox
                            label={t("agent-admin.preserve-hierarchy")}
                            checked={processingOptions.preserve_hierarchy}
                            onChange={(e) => updateProcessingOption("preserve_hierarchy", e.target.checked)}
                            size="sm"
                          />
                        </div>
                      </div>

                      {/* Save/Reset Options */}
                      <div className="flex items-center justify-between pt-3 border-t border-purple-100 dark:border-purple-800">
                        <div className="flex items-center gap-2">
                          <Button
                            size="sm"
                            variant="outlined"
                            color="primary"
                            onClick={handleSaveProcessingOptions}
                            loading={isSavingOptions}
                            disabled={isSavingOptions}
                          >
                            {t("agent-admin.save-as-default")}
                          </Button>
                          <Button
                            size="sm"
                            variant="plain"
                            color="neutral"
                            onClick={handleResetProcessingOptions}
                            disabled={isSavingOptions}
                          >
                            {t("agent-admin.reset-to-defaults")}
                          </Button>
                        </div>
                        {hasCustomOptions && (
                          <Chip size="sm" variant="soft" color="success">
                            {t("agent-admin.custom-options-saved")}
                          </Chip>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Action Buttons */}
                  <div className="flex gap-2 flex-wrap">
                    <Button
                      variant="solid"
                      color="success"
                      onClick={handleFormatForRAG}
                      loading={isGenerating === "rag"}
                      disabled={isGenerating !== null}
                      startDecorator={<ZapIcon className="w-4 h-4" />}
                    >
                      {isGenerating === "rag" ? t("agent-admin.formatting") : t("agent-admin.format-for-rag")}
                    </Button>
                    <Button
                      variant="outlined"
                      color="primary"
                      onClick={handleGenerateKB}
                      loading={isGenerating === "kb"}
                      disabled={isGenerating !== null}
                      startDecorator={<SparklesIcon className="w-4 h-4" />}
                    >
                      {isGenerating === "kb" ? t("agent-admin.generating") : t("agent-admin.generate-kb")}
                    </Button>
                    <Button
                      variant="outlined"
                      color="primary"
                      onClick={handleGeneratePolicy}
                      loading={isGenerating === "policy"}
                      disabled={isGenerating !== null}
                      startDecorator={<SparklesIcon className="w-4 h-4" />}
                    >
                      {isGenerating === "policy" ? t("agent-admin.generating") : t("agent-admin.generate-policy")}
                    </Button>
                  </div>
                  <p className="text-xs text-purple-500 dark:text-purple-400">
                    {t("agent-admin.format-for-rag-desc")}
                  </p>
                </div>
              </div>
            )}

            {/* Internal Configuration */}
            <AudienceSection
              title={t("agent-admin.internal")}
              audienceType="internal"
              tenant={selectedTenant}
              onViewVersions={handleViewVersions}
              isSaving={isSaving}
              t={t}
              canUpload={canUpload}
              canRestore={canRestore}
            />

            {/* Rebuild Index - Admin or api:config permission */}
            {(isAdmin || canConfigApi) && (
              <div className="bg-blue-50 dark:bg-blue-900/20 rounded-xl border border-blue-200 dark:border-blue-800 p-4">
                <div className="flex justify-between items-center">
                  <div>
                    <h3 className="font-medium text-blue-700 dark:text-blue-300">{t("agent-admin.rebuild-index")}</h3>
                    <p className="text-sm text-blue-600 dark:text-blue-400">{t("agent-admin.rebuild-index-desc")}</p>
                  </div>
                  <Button color="primary" onClick={handleRebuildIndex} loading={isRebuilding}>
                    <RefreshCwIcon className="w-4 h-4 mr-2" />
                    {t("agent-admin.rebuild-index")}
                  </Button>
                </div>
              </div>
            )}

            {/* Widget Embed Code - Admin or api:config permission */}
            {(isAdmin || canConfigApi) && (
              <div className="bg-gray-50 dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4">
                <div className="flex items-center gap-2 mb-4">
                  <CodeIcon className="w-5 h-5 text-gray-600 dark:text-gray-400" />
                  <div>
                    <h3 className="font-medium text-gray-800 dark:text-gray-200">{t("agent-admin.widget-embed-title")}</h3>
                    <p className="text-sm text-gray-500 dark:text-gray-400">{t("agent-admin.widget-embed-desc")}</p>
                  </div>
                </div>

                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                  {/* Customization Panel */}
                  <div className="space-y-4">
                    {/* Color Picker */}
                    <FormControl>
                      <FormLabel>{t("agent-admin.widget-color")}</FormLabel>
                      <div className="flex items-center gap-3">
                        <input
                          type="color"
                          value={widgetColor}
                          onChange={(e) => setWidgetColor(e.target.value)}
                          className="w-10 h-10 rounded-lg border border-gray-300 dark:border-zinc-600 cursor-pointer"
                        />
                        <Input
                          value={widgetColor}
                          onChange={(e) => setWidgetColor(e.target.value)}
                          placeholder="#0d9488"
                          sx={{ fontFamily: "monospace", width: 120 }}
                        />
                      </div>
                    </FormControl>

                    {/* Position */}
                    <FormControl>
                      <FormLabel>{t("agent-admin.widget-position")}</FormLabel>
                      <div className="flex gap-2">
                        <Button
                          variant={widgetPosition === "bottom-right" ? "solid" : "outlined"}
                          color="neutral"
                          size="sm"
                          onClick={() => setWidgetPosition("bottom-right")}
                        >
                          {t("agent-admin.widget-position-right")}
                        </Button>
                        <Button
                          variant={widgetPosition === "bottom-left" ? "solid" : "outlined"}
                          color="neutral"
                          size="sm"
                          onClick={() => setWidgetPosition("bottom-left")}
                        >
                          {t("agent-admin.widget-position-left")}
                        </Button>
                      </div>
                    </FormControl>

                    {/* Welcome Message */}
                    <FormControl>
                      <FormLabel>{t("agent-admin.widget-welcome")}</FormLabel>
                      <Input
                        value={widgetWelcome}
                        onChange={(e) => setWidgetWelcome(e.target.value)}
                        placeholder={t("agent-admin.widget-welcome-placeholder")}
                      />
                    </FormControl>
                  </div>

                  {/* Live Preview */}
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <FormLabel>{t("agent-admin.widget-preview")}</FormLabel>
                      <Switch
                        checked={showWidgetPreview}
                        onChange={(e) => setShowWidgetPreview(e.target.checked)}
                        size="sm"
                      />
                    </div>
                    {showWidgetPreview && (
                      <div className="relative bg-white dark:bg-zinc-900 rounded-xl border border-gray-200 dark:border-zinc-700 h-80 overflow-hidden">
                        {/* Mini Preview Panel */}
                        <div
                          className="absolute w-64 bg-white rounded-2xl shadow-lg border overflow-hidden"
                          style={{
                            bottom: 16,
                            [widgetPosition === "bottom-right" ? "right" : "left"]: 16,
                            borderColor: "#e5e5e5"
                          }}
                        >
                          {/* Header */}
                          <div
                            className="px-4 py-3 flex items-center justify-between"
                            style={{ background: "#fff", borderBottom: "1px solid #f0f0f0" }}
                          >
                            <div className="flex items-center gap-2">
                              <svg viewBox="0 0 24 24" className="w-5 h-5" style={{ fill: widgetColor }}>
                                <path d="M12 2C6.48 2 2 6.48 2 12c0 1.85.5 3.58 1.36 5.07L2 22l4.93-1.36C8.42 21.5 10.15 22 12 22c5.52 0 10-4.48 10-10S17.52 2 12 2zm0 18c-1.58 0-3.08-.38-4.4-1.06l-.31-.17-3.23.89.89-3.23-.17-.31C4.38 15.08 4 13.58 4 12c0-4.41 3.59-8 8-8s8 3.59 8 8-3.59 8-8 8z"/>
                              </svg>
                              <span className="font-semibold text-sm text-gray-800">{selectedTenant?.tenant.companyName || "Company"}</span>
                            </div>
                            <div className="flex gap-1">
                              <div className="w-6 h-6 rounded-md hover:bg-gray-100 flex items-center justify-center cursor-pointer">
                                <span className="text-gray-400 text-xs">─</span>
                              </div>
                              <div className="w-6 h-6 rounded-md hover:bg-gray-100 flex items-center justify-center cursor-pointer">
                                <span className="text-gray-400 text-xs">✕</span>
                              </div>
                            </div>
                          </div>
                          {/* Messages Area */}
                          <div className="p-4 bg-gray-50 h-32 flex items-center justify-center">
                            <div className="text-center text-gray-400">
                              <svg viewBox="0 0 24 24" className="w-8 h-8 mx-auto mb-2 fill-gray-300">
                                <path d="M12 2C6.48 2 2 6.48 2 12c0 1.85.5 3.58 1.36 5.07L2 22l4.93-1.36C8.42 21.5 10.15 22 12 22c5.52 0 10-4.48 10-10S17.52 2 12 2zm0 18c-1.58 0-3.08-.38-4.4-1.06l-.31-.17-3.23.89.89-3.23-.17-.31C4.38 15.08 4 13.58 4 12c0-4.41 3.59-8 8-8s8 3.59 8 8-3.59 8-8 8z"/>
                              </svg>
                              <p className="text-xs">{widgetWelcome}</p>
                            </div>
                          </div>
                          {/* Input Area */}
                          <div className="p-3 border-t border-gray-100 flex gap-2">
                            <div className="flex-1 bg-gray-100 rounded-full px-4 py-2 text-xs text-gray-400">
                              Message...
                            </div>
                            <div
                              className="w-8 h-8 rounded-full flex items-center justify-center"
                              style={{ background: widgetColor }}
                            >
                              <svg viewBox="0 0 24 24" className="w-4 h-4 fill-white">
                                <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
                              </svg>
                            </div>
                          </div>
                        </div>
                        {/* Toggle Button */}
                        <div
                          className="absolute w-12 h-12 rounded-full shadow-lg flex items-center justify-center cursor-pointer"
                          style={{
                            background: widgetColor,
                            bottom: 16,
                            [widgetPosition === "bottom-right" ? "right" : "left"]: 16,
                            display: "none"
                          }}
                        >
                          <svg viewBox="0 0 24 24" className="w-6 h-6 fill-white">
                            <path d="M12 2C6.48 2 2 6.48 2 12c0 1.85.5 3.58 1.36 5.07L2 22l4.93-1.36C8.42 21.5 10.15 22 12 22c5.52 0 10-4.48 10-10S17.52 2 12 2zm0 18c-1.58 0-3.08-.38-4.4-1.06l-.31-.17-3.23.89.89-3.23-.17-.31C4.38 15.08 4 13.58 4 12c0-4.41 3.59-8 8-8s8 3.59 8 8-3.59 8-8 8z"/>
                          </svg>
                        </div>
                      </div>
                    )}
                  </div>
                </div>

                {/* Generated Code */}
                <div className="mt-4">
                  <div className="flex items-center justify-between mb-2">
                    <FormLabel>{t("agent-admin.widget-code")}</FormLabel>
                    <Button
                      size="sm"
                      variant="plain"
                      color="neutral"
                      startDecorator={<CopyIcon className="w-4 h-4" />}
                      onClick={() => {
                        const code = `<script src="${window.location.origin}/widget/${selectedTenant?.tenant.slug}/embed.js"></script>
<script>
  AgentChatWidget.init({
    tenant: "${selectedTenant?.tenant.slug}",
    baseUrl: "${window.location.origin}",
    color: "${widgetColor}",
    position: "${widgetPosition}",
    welcomeMessage: "${widgetWelcome}",
    companyName: "${selectedTenant?.tenant.companyName || ""}"
  });
</script>`;
                        navigator.clipboard.writeText(code);
                        toast.success(t("agent-admin.widget-copied"));
                      }}
                    >
                      {t("agent-admin.widget-copy")}
                    </Button>
                  </div>
                  <pre className="bg-gray-900 text-gray-100 rounded-lg p-4 text-xs overflow-x-auto font-mono">
{`<script src="${window.location.origin}/widget/${selectedTenant?.tenant.slug}/embed.js"></script>
<script>
  AgentChatWidget.init({
    tenant: "${selectedTenant?.tenant.slug}",
    baseUrl: "${window.location.origin}",
    color: "${widgetColor}",
    position: "${widgetPosition}",
    welcomeMessage: "${widgetWelcome}",
    companyName: "${selectedTenant?.tenant.companyName || ""}"
  });
</script>`}
                  </pre>
                </div>
              </div>
            )}

            {/* Q&A Pairs Testing - Admin only */}
            {isAdmin && (
              <div className="bg-purple-50 dark:bg-purple-900/20 rounded-xl border border-purple-200 dark:border-purple-800 p-4">
                <div className="flex justify-between items-center mb-3">
                  <div>
                    <h3 className="font-medium text-purple-700 dark:text-purple-300">
                      {t("agent-admin.qa-pairs-title")}
                    </h3>
                    <p className="text-sm text-purple-600 dark:text-purple-400">
                      {t("agent-admin.qa-pairs-desc")}
                    </p>
                  </div>
                  <div className="flex gap-2">
                    <Button
                      color="primary"
                      variant="outlined"
                      onClick={handleGenerateQAPairs}
                      loading={isGeneratingQA}
                    >
                      <SparklesIcon className="w-4 h-4 mr-2" />
                      {t("agent-admin.generate-qa")}
                    </Button>
                    <Button
                      color="primary"
                      onClick={() => setShowQAModal(true)}
                      disabled={qaPairs.length === 0}
                    >
                      <ZapIcon className="w-4 h-4 mr-2" />
                      {t("agent-admin.test-qa")} ({qaPairs.length})
                    </Button>
                  </div>
                </div>

                {/* Quick stats if test results exist */}
                {qaTestResults && (
                  <div className="grid grid-cols-4 gap-3 mt-3 p-3 bg-white dark:bg-gray-800 rounded-lg">
                    <div className="text-center">
                      <div className="text-2xl font-bold text-purple-600">{qaTestResults.total_pairs}</div>
                      <div className="text-xs text-gray-500">Total Pairs</div>
                    </div>
                    <div className="text-center">
                      <div className="text-2xl font-bold text-green-600">{qaTestResults.found}</div>
                      <div className="text-xs text-gray-500">Found</div>
                    </div>
                    <div className="text-center">
                      <div className="text-2xl font-bold text-red-600">{qaTestResults.not_found}</div>
                      <div className="text-xs text-gray-500">Not Found</div>
                    </div>
                    <div className="text-center">
                      <div className="text-2xl font-bold text-blue-600">
                        {(qaTestResults.recall_at_5 * 100).toFixed(0)}%
                      </div>
                      <div className="text-xs text-gray-500">Recall@5</div>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* RAG Search Explorer - Admin only */}
            {isAdmin && (
              <div className="bg-cyan-50 dark:bg-cyan-900/20 rounded-xl border border-cyan-200 dark:border-cyan-800 p-4">
                <div className="flex justify-between items-center mb-3">
                  <div>
                    <h3 className="font-medium text-cyan-700 dark:text-cyan-300">
                      {t("agent-admin.search-explorer-title")}
                    </h3>
                    <p className="text-sm text-cyan-600 dark:text-cyan-400">
                      {t("agent-admin.search-explorer-desc")}
                    </p>
                  </div>
                  <Button
                    color="primary"
                    variant="outlined"
                    onClick={() => setShowSearchExplorer(!showSearchExplorer)}
                  >
                    <SearchIcon className="w-4 h-4 mr-2" />
                    {showSearchExplorer ? t("common.close") : t("agent-admin.open-explorer")}
                  </Button>
                </div>

                {/* Search Explorer Panel */}
                {showSearchExplorer && (
                  <div className="mt-4 space-y-4">
                    {/* Search Input */}
                    <div className="flex gap-2">
                      <Input
                        placeholder={t("agent-admin.search-placeholder")}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        onKeyDown={(e) => e.key === "Enter" && handleRAGSearch()}
                        className="flex-1"
                      />
                      <Select
                        value={searchTopK}
                        onChange={(_, v) => v && setSearchTopK(v)}
                        sx={{ minWidth: 80 }}
                      >
                        <Option value={3}>Top 3</Option>
                        <Option value={5}>Top 5</Option>
                        <Option value={10}>Top 10</Option>
                      </Select>
                      <Button
                        color="primary"
                        onClick={handleRAGSearch}
                        loading={isSearchingRAG}
                        disabled={!searchQuery.trim()}
                      >
                        <SearchIcon className="w-4 h-4" />
                      </Button>
                    </div>

                    {/* Search Results */}
                    {ragSearchResults && (
                      <div className="space-y-3">
                        <div className="flex justify-between items-center text-sm text-gray-600 dark:text-gray-400">
                          <span>{ragSearchResults.total_results} results for "{ragSearchResults.query}"</span>
                          <span>{ragSearchResults.latency_ms}ms</span>
                        </div>

                        {ragSearchResults.results.map((result) => (
                          <div
                            key={result.chunk_id}
                            className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4"
                          >
                            {/* Score Bar */}
                            <div className="flex items-center gap-3 mb-2">
                              <span className="font-medium text-gray-700 dark:text-gray-300">
                                #{result.rank}
                              </span>
                              <div className="flex-1 h-3 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                                <div
                                  className={cn(
                                    "h-full rounded-full transition-all",
                                    result.score_percent >= 80 ? "bg-green-500" :
                                    result.score_percent >= 60 ? "bg-yellow-500" :
                                    result.score_percent >= 40 ? "bg-orange-500" : "bg-red-500"
                                  )}
                                  style={{ width: `${result.score_percent}%` }}
                                />
                              </div>
                              <span className={cn(
                                "font-bold min-w-[3rem] text-right",
                                result.score_percent >= 80 ? "text-green-600" :
                                result.score_percent >= 60 ? "text-yellow-600" :
                                result.score_percent >= 40 ? "text-orange-600" : "text-red-600"
                              )}>
                                {result.score_percent}%
                              </span>
                            </div>

                            {/* Title */}
                            <div className="font-medium text-gray-800 dark:text-gray-200 mb-1">
                              {result.title || "Untitled Section"}
                            </div>

                            {/* Content Preview */}
                            <div className="text-sm text-gray-600 dark:text-gray-400 mb-2 line-clamp-3">
                              {result.content_preview}
                            </div>

                            {/* Keywords & Meta */}
                            <div className="flex flex-wrap gap-2 items-center text-xs">
                              {result.matched_keywords.length > 0 && (
                                <div className="flex gap-1 items-center">
                                  <span className="text-gray-500">Matched:</span>
                                  {result.matched_keywords.slice(0, 5).map((kw) => (
                                    <Chip key={kw} size="sm" color="success" variant="soft">
                                      {kw}
                                    </Chip>
                                  ))}
                                </div>
                              )}
                              <Chip size="sm" variant="outlined">
                                {result.content_type}
                              </Chip>
                            </div>
                          </div>
                        ))}

                        {ragSearchResults.results.length === 0 && (
                          <div className="text-center py-8 text-gray-500">
                            No results found. Try a different query or rebuild the index.
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Chat Transcripts - Admin only */}
            {isAdmin && (
              <div className="bg-green-50 dark:bg-green-900/20 rounded-xl border border-green-200 dark:border-green-800 p-4">
                <div className="flex justify-between items-center mb-3">
                  <div>
                    <h3 className="font-medium text-green-700 dark:text-green-300">
                      {t("agent-admin.transcripts")}
                    </h3>
                    <p className="text-sm text-green-600 dark:text-green-400">
                      {t("agent-admin.transcripts-desc")}
                    </p>
                  </div>
                  <div className="flex items-center gap-4">
                    <Checkbox
                      label={t("agent-admin.record-transcripts")}
                      checked={tenantSettings?.recordTranscripts ?? true}
                      onChange={async (e) => {
                        const success = await agentAdminStore.updateTenantSettings(
                          selectedTenant.tenant.slug,
                          { recordTranscripts: e.target.checked }
                        );
                        if (success) {
                          toast.success(t("agent-admin.settings-saved"));
                        }
                      }}
                      size="sm"
                    />
                    <Button
                      color="primary"
                      variant="outlined"
                      onClick={() => setShowTranscripts(!showTranscripts)}
                    >
                      <MessageCircleIcon className="w-4 h-4 mr-2" />
                      {showTranscripts ? t("common.close") : t("agent-admin.transcript-view")} ({transcripts.length})
                    </Button>
                  </div>
                </div>

                {/* Transcripts List */}
                {showTranscripts && (
                  <div className="mt-4 space-y-3">
                    {isLoadingTranscripts ? (
                      <div className="text-center py-4 text-gray-500">Loading...</div>
                    ) : transcripts.length === 0 ? (
                      <div className="text-center py-4 text-gray-500">
                        {t("agent-admin.transcripts-empty")}
                      </div>
                    ) : (
                      <div className="space-y-2 max-h-96 overflow-y-auto">
                        {transcripts.map((transcript) => (
                          <div
                            key={transcript.id}
                            className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 hover:border-green-300 dark:hover:border-green-700 transition-colors"
                          >
                            <div className="flex justify-between items-start">
                              <div className="flex-1">
                                <div className="flex items-center gap-2 mb-1">
                                  <Chip size="sm" variant="soft" color="primary">
                                    {transcript.audienceType}
                                  </Chip>
                                  <span className="text-xs text-gray-500">
                                    {new Date(transcript.startedAt).toLocaleString()}
                                  </span>
                                  {transcript.isCompleted && (
                                    <Chip size="sm" variant="soft" color="success">
                                      Completed
                                    </Chip>
                                  )}
                                </div>
                                <div className="text-sm text-gray-700 dark:text-gray-300">
                                  {transcript.customerName && (
                                    <span className="font-medium">{transcript.customerName}</span>
                                  )}
                                  {transcript.detectedIntent && (
                                    <span className="ml-2 text-gray-500">- {transcript.detectedIntent}</span>
                                  )}
                                  <span className="ml-2 text-gray-400">({transcript.messageCount} messages)</span>
                                </div>
                              </div>
                              <div className="flex gap-2">
                                <Button
                                  size="sm"
                                  variant="plain"
                                  color="primary"
                                  onClick={() => {
                                    setSelectedTranscript(transcript);
                                    setShowTranscriptModal(true);
                                  }}
                                >
                                  <EyeIcon className="w-4 h-4" />
                                </Button>
                                <Button
                                  size="sm"
                                  variant="plain"
                                  color="danger"
                                  onClick={async () => {
                                    if (window.confirm(t("agent-admin.transcript-delete-confirm"))) {
                                      const success = await agentAdminStore.deleteTranscript(
                                        selectedTenant.tenant.slug,
                                        transcript.id
                                      );
                                      if (success) {
                                        toast.success(t("agent-admin.transcript-deleted"));
                                      }
                                    }
                                  }}
                                >
                                  <Trash2Icon className="w-4 h-4" />
                                </Button>
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Delete Tenant - Admin only */}
            {isAdmin && (
              <div className="bg-red-50 dark:bg-red-900/20 rounded-xl border border-red-200 dark:border-red-800 p-4">
                <div className="flex justify-between items-center">
                  <div>
                    <h3 className="font-medium text-red-700 dark:text-red-300">{t("agent-admin.delete-tenant")}</h3>
                    <p className="text-sm text-red-600 dark:text-red-400">This action cannot be undone.</p>
                  </div>
                  <Button color="danger" onClick={() => setShowDeleteModal(selectedTenant.tenant)}>
                    {t("agent-admin.delete-tenant")}
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Create Tenant Modal */}
        <CreateTenantModal
          open={showCreateModal}
          onClose={() => setShowCreateModal(false)}
          t={t}
        />

        {/* Delete Confirmation Modal */}
        <Modal open={showDeleteModal !== null} onClose={() => setShowDeleteModal(null)}>
          <ModalDialog variant="outlined" role="alertdialog">
            <DialogTitle>{t("agent-admin.delete-tenant")}</DialogTitle>
            <Divider />
            <DialogContent>
              {t("agent-admin.delete-confirm", { name: showDeleteModal?.companyName || "" })}
            </DialogContent>
            <DialogActions>
              <Button variant="plain" color="neutral" onClick={() => setShowDeleteModal(null)}>
                {t("common.cancel")}
              </Button>
              <Button color="danger" onClick={handleDeleteTenant} loading={isSaving}>
                {t("common.delete")}
              </Button>
            </DialogActions>
          </ModalDialog>
        </Modal>

        {/* Version History Modal */}
        <Modal open={showVersionsModal !== null} onClose={() => setShowVersionsModal(null)}>
          <ModalDialog sx={{ maxWidth: 600, width: "100%" }}>
            <DialogTitle>{t("agent-admin.version-history")}</DialogTitle>
            <Divider />
            <DialogContent>
              {showVersionsModal && (
                <VersionHistoryList
                  versions={fileVersions[`${showVersionsModal.slug}-${showVersionsModal.audienceType}-${showVersionsModal.fileType}`] || []}
                  onRestore={handleRestoreVersion}
                  isSaving={isSaving}
                  t={t}
                />
              )}
            </DialogContent>
            <DialogActions>
              <Button variant="plain" color="neutral" onClick={() => setShowVersionsModal(null)}>
                {t("common.close")}
              </Button>
            </DialogActions>
          </ModalDialog>
        </Modal>

        {/* Generated Content Modal */}
        <Modal open={showGeneratedContent !== null} onClose={() => setShowGeneratedContent(null)}>
          <ModalDialog sx={{ maxWidth: 900, width: "90vw", maxHeight: "90vh" }}>
            <ModalClose />
            <DialogTitle>
              {showGeneratedContent?.type === "kb"
                ? t("agent-admin.generated-kb-title")
                : showGeneratedContent?.type === "policy"
                  ? t("agent-admin.generated-policy-title")
                  : t("agent-admin.formatted-content-title")}
            </DialogTitle>
            <Divider />
            <DialogContent sx={{ overflow: "auto" }}>
              {/* Stats for RAG formatting */}
              {showGeneratedContent?.type === "rag" && showGeneratedContent.stats && (
                <div className="mb-4 p-3 bg-green-50 dark:bg-green-900/20 rounded-lg border border-green-200 dark:border-green-700">
                  <h4 className="text-sm font-medium text-green-700 dark:text-green-300 mb-2">
                    {t("agent-admin.processing-stats")}
                  </h4>
                  <div className="grid grid-cols-2 sm:grid-cols-5 gap-3 text-sm">
                    <div>
                      <span className="text-gray-500 dark:text-gray-400">{t("agent-admin.original-tokens")}:</span>
                      <span className="ml-1 font-medium">{showGeneratedContent.stats.original_tokens}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 dark:text-gray-400">{t("agent-admin.processed-tokens")}:</span>
                      <span className="ml-1 font-medium">{showGeneratedContent.stats.processed_tokens}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 dark:text-gray-400">{t("agent-admin.chunks-created")}:</span>
                      <span className="ml-1 font-medium">{showGeneratedContent.stats.chunks_created}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 dark:text-gray-400">{t("agent-admin.faqs-found")}:</span>
                      <span className="ml-1 font-medium">{showGeneratedContent.stats.faqs_extracted}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 dark:text-gray-400">{t("agent-admin.services-found")}:</span>
                      <span className="ml-1 font-medium">{showGeneratedContent.stats.services_extracted}</span>
                    </div>
                  </div>
                </div>
              )}
              <Textarea
                value={showGeneratedContent?.content || ""}
                readOnly
                minRows={20}
                maxRows={40}
                sx={{ fontFamily: "monospace", fontSize: "12px" }}
              />
            </DialogContent>
            <DialogActions>
              <Button variant="plain" color="neutral" onClick={() => setShowGeneratedContent(null)}>
                {t("common.close")}
              </Button>
              <Button
                variant="solid"
                color="primary"
                startDecorator={<CopyIcon className="w-4 h-4" />}
                onClick={handleCopyGeneratedContent}
              >
                {t("agent-admin.copy-to-clipboard")}
              </Button>
            </DialogActions>
          </ModalDialog>
        </Modal>

        {/* Q&A Pairs Modal */}
        <Modal open={showQAModal} onClose={() => setShowQAModal(false)}>
          <ModalDialog sx={{ maxWidth: 900, width: "90vw", maxHeight: "90vh" }}>
            <ModalClose />
            <DialogTitle>{t("agent-admin.qa-pairs-title")}</DialogTitle>
            <Divider />
            <DialogContent sx={{ overflow: "auto" }}>
              {/* Test button and results summary */}
              <div className="flex justify-between items-center mb-4">
                <div className="text-sm text-gray-600 dark:text-gray-400">
                  {t("agent-admin.qa-pairs-loaded", { count: qaPairs.length })}
                </div>
                <Button
                  color="primary"
                  onClick={handleTestAllQAPairs}
                  loading={isTestingQA}
                  disabled={qaPairs.length === 0}
                >
                  <ZapIcon className="w-4 h-4 mr-2" />
                  {t("agent-admin.qa-run-tests")}
                </Button>
              </div>

              {/* No pairs message */}
              {qaPairs.length === 0 && (
                <div className="text-center py-8 text-gray-500">
                  {t("agent-admin.qa-no-pairs")}
                </div>
              )}

              {/* Pairs list with results */}
              <div className="space-y-2">
                {qaPairs.map((pair) => {
                  const result = qaTestResults?.results?.find((r) => r.pair_id === pair.id);
                  return (
                    <div key={pair.id} className="p-3 border rounded-lg dark:border-gray-700">
                      <div className="flex justify-between">
                        <div className="flex-1 pr-4">
                          <div className="font-medium text-gray-900 dark:text-gray-100">{pair.question}</div>
                          <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">{pair.expected_answer}</div>
                          <div className="flex gap-2 mt-2">
                            <Chip size="sm" color="neutral">{pair.difficulty}</Chip>
                            <Chip size="sm" color="primary">{pair.category}</Chip>
                            {pair.source_section && (
                              <Chip size="sm" variant="outlined">{pair.source_section}</Chip>
                            )}
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          {result && (
                            <Chip
                              size="sm"
                              color={result.found ? "success" : "danger"}
                            >
                              {result.found ? `Rank #${result.rank}` : "Not Found"}
                            </Chip>
                          )}
                          <Button
                            size="sm"
                            variant="plain"
                            color="danger"
                            onClick={() => handleDeleteQAPair(pair.id)}
                          >
                            <Trash2Icon className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </DialogContent>
            <DialogActions>
              <Button variant="plain" color="neutral" onClick={() => setShowQAModal(false)}>
                {t("common.close")}
              </Button>
            </DialogActions>
          </ModalDialog>
        </Modal>

        {/* Transcript View Modal */}
        <Modal open={showTranscriptModal && selectedTranscript !== null} onClose={() => setShowTranscriptModal(false)}>
          <ModalDialog sx={{ maxWidth: 700, width: "90vw", maxHeight: "90vh" }}>
            <ModalClose />
            <DialogTitle className="flex items-center gap-2">
              <MessageCircleIcon className="w-5 h-5" />
              {t("agent-admin.transcript-view")}
            </DialogTitle>
            <Divider />
            <DialogContent sx={{ overflow: "auto" }}>
              {selectedTranscript && (
                <div className="space-y-4">
                  {/* Transcript metadata */}
                  <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-3 text-sm">
                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <span className="text-gray-500">Session:</span>{" "}
                        <span className="font-mono text-xs">{selectedTranscript.sessionId}</span>
                      </div>
                      <div>
                        <span className="text-gray-500">Audience:</span>{" "}
                        <Chip size="sm" variant="soft" color="primary">
                          {selectedTranscript.audienceType}
                        </Chip>
                      </div>
                      <div>
                        <span className="text-gray-500">Started:</span>{" "}
                        {new Date(selectedTranscript.startedAt).toLocaleString()}
                      </div>
                      <div>
                        <span className="text-gray-500">Messages:</span>{" "}
                        {selectedTranscript.messageCount}
                      </div>
                      {selectedTranscript.customerName && (
                        <div>
                          <span className="text-gray-500">Customer:</span>{" "}
                          {selectedTranscript.customerName}
                        </div>
                      )}
                      {selectedTranscript.detectedIntent && (
                        <div>
                          <span className="text-gray-500">Intent:</span>{" "}
                          {selectedTranscript.detectedIntent}
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Messages */}
                  <div className="space-y-3">
                    {selectedTranscript.messages.map((msg, idx) => (
                      <div
                        key={idx}
                        className={cn(
                          "p-3 rounded-lg",
                          msg.role === "user"
                            ? "bg-blue-50 dark:bg-blue-900/20 ml-8"
                            : "bg-gray-100 dark:bg-gray-800 mr-8"
                        )}
                      >
                        <div className="flex justify-between items-center mb-1">
                          <span className={cn(
                            "text-xs font-medium",
                            msg.role === "user" ? "text-blue-600" : "text-gray-600"
                          )}>
                            {msg.role === "user" ? "Customer" : "Agent"}
                          </span>
                          {msg.timestamp && (
                            <span className="text-xs text-gray-400">
                              {new Date(msg.timestamp).toLocaleTimeString()}
                            </span>
                          )}
                        </div>
                        <div className="text-sm text-gray-800 dark:text-gray-200 whitespace-pre-wrap">
                          {msg.content}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </DialogContent>
            <DialogActions>
              <Button variant="plain" color="neutral" onClick={() => setShowTranscriptModal(false)}>
                {t("common.close")}
              </Button>
            </DialogActions>
          </ModalDialog>
        </Modal>
      </div>
    </section>
  );
});

// Sub-components

interface EndpointRowProps {
  label: string;
  value: string;
  onCopy: (text: string) => void;
}

const EndpointRow = ({ label, value, onCopy }: EndpointRowProps) => (
  <div className="flex justify-between items-center">
    <span className="text-gray-600 dark:text-gray-400">{label}</span>
    <div className="flex items-center gap-2">
      <code className="bg-gray-100 dark:bg-zinc-700 px-2 py-1 rounded text-xs">{value}</code>
      <button onClick={() => onCopy(value)} className="text-gray-400 hover:text-gray-600">
        <CopyIcon className="w-4 h-4" />
      </button>
    </div>
  </div>
);

interface AudienceSectionProps {
  title: string;
  audienceType: "external" | "internal";
  tenant: any;
  onViewVersions: (slug: string, audienceType: string, fileType: string) => void;
  isSaving: boolean;
  t: (key: string) => string;
  canUpload?: boolean;
  canRestore?: boolean;
}

const AudienceSection = ({ title, audienceType, tenant, onViewVersions, isSaving, t, canUpload = false, canRestore = false }: AudienceSectionProps) => {
  const data = audienceType === "external" ? tenant.external : tenant.internal;
  const kbFileRef = useRef<HTMLInputElement>(null);
  const policyFileRef = useRef<HTMLInputElement>(null);
  const [previewContent, setPreviewContent] = useState<string | null>(null);
  const [previewTitle, setPreviewTitle] = useState<string>("");

  const handleFileUpload = async (fileType: "kb" | "policy", event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      const success = await agentAdminStore.updateTenantFiles({
        slug: tenant.tenant.slug,
        audienceType,
        fileType,
        file,
      });
      if (success) {
        toast.success("File uploaded successfully");
      }
      // Reset the input
      event.target.value = "";
    }
  };

  const handlePreview = async (fileType: "kb" | "policy") => {
    const content = await agentAdminStore.fetchFileContent(tenant.tenant.slug, audienceType, fileType);
    if (content) {
      const typeLabel = fileType === "kb" ? t("agent-admin.preview-kb") : t("agent-admin.preview-policy");
      setPreviewTitle(`${typeLabel} (${audienceType})`);
      setPreviewContent(content);
    } else {
      toast.error("No content available to preview");
    }
  };

  return (
    <div className="bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4">
      <h3 className="font-medium text-gray-800 dark:text-gray-200 mb-4">{title}</h3>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4 mb-4">
        <div className="text-center p-3 bg-gray-50 dark:bg-zinc-700 rounded-lg">
          <div className="text-2xl font-bold text-teal-600">{data?.stats?.servicesCount || 0}</div>
          <div className="text-xs text-gray-500">{t("agent-admin.services-count")}</div>
        </div>
        <div className="text-center p-3 bg-gray-50 dark:bg-zinc-700 rounded-lg">
          <div className="text-2xl font-bold text-teal-600">{data?.stats?.intentsCount || 0}</div>
          <div className="text-xs text-gray-500">{t("agent-admin.intents-count")}</div>
        </div>
        <div className="text-center p-3 bg-gray-50 dark:bg-zinc-700 rounded-lg">
          <div className="text-2xl font-bold text-teal-600">{data?.stats?.faqsCount || 0}</div>
          <div className="text-xs text-gray-500">{t("agent-admin.faqs-count")}</div>
        </div>
      </div>

      {/* File Uploads */}
      <div className="space-y-3">
        <FileUploadRow
          label={audienceType === "external" ? t("agent-admin.external-kb") : t("agent-admin.internal-kb")}
          hint={audienceType === "external" ? t("agent-admin.external-kb-hint") : t("agent-admin.internal-kb-hint")}
          fileRef={kbFileRef}
          onUpload={(e) => handleFileUpload("kb", e)}
          onViewVersions={() => onViewVersions(tenant.tenant.slug, audienceType, "kb")}
          onPreview={() => handlePreview("kb")}
          isSaving={isSaving}
          t={t}
          canUpload={canUpload}
          canRestore={canRestore}
        />
        <FileUploadRow
          label={audienceType === "external" ? t("agent-admin.external-policy") : t("agent-admin.internal-policy")}
          hint={audienceType === "external" ? t("agent-admin.external-policy-hint") : t("agent-admin.internal-policy-hint")}
          fileRef={policyFileRef}
          onUpload={(e) => handleFileUpload("policy", e)}
          onViewVersions={() => onViewVersions(tenant.tenant.slug, audienceType, "policy")}
          onPreview={() => handlePreview("policy")}
          isSaving={isSaving}
          t={t}
          canUpload={canUpload}
          canRestore={canRestore}
        />
      </div>

      {/* Preview Modal */}
      <Modal open={!!previewContent} onClose={() => setPreviewContent(null)}>
        <ModalDialog sx={{ maxWidth: 900, width: '90vw', maxHeight: '90vh', overflow: 'hidden' }}>
          <ModalClose />
          <DialogTitle>{previewTitle}</DialogTitle>
          <DialogContent sx={{ overflow: 'auto' }}>
            <Textarea
              value={previewContent || ""}
              readOnly
              minRows={20}
              maxRows={40}
              sx={{ fontFamily: 'monospace', fontSize: 12 }}
            />
          </DialogContent>
        </ModalDialog>
      </Modal>
    </div>
  );
};

interface FileUploadRowProps {
  label: string;
  hint: string;
  fileRef: React.RefObject<HTMLInputElement>;
  onUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onViewVersions: () => void;
  onPreview: () => void;
  isSaving: boolean;
  t: (key: string) => string;
  canUpload?: boolean;
  canRestore?: boolean;
}

const FileUploadRow = ({ label, hint, fileRef, onUpload, onViewVersions, onPreview, isSaving, t, canUpload = false, canRestore = false }: FileUploadRowProps) => (
  <div className="flex justify-between items-center p-3 bg-gray-50 dark:bg-zinc-700 rounded-lg">
    <div>
      <div className="font-medium text-sm text-gray-800 dark:text-gray-200">{label}</div>
      <div className="text-xs text-gray-500">{hint}</div>
    </div>
    <div className="flex gap-2">
      {canUpload && (
        <input
          type="file"
          ref={fileRef}
          onChange={onUpload}
          accept=".md,.txt"
          className="hidden"
        />
      )}
      <Button
        size="sm"
        variant="outlined"
        color="neutral"
        startDecorator={<EyeIcon className="w-3 h-3" />}
        onClick={onPreview}
      >
        {t("agent-admin.preview")}
      </Button>
      {canRestore && (
        <Button
          size="sm"
          variant="outlined"
          color="neutral"
          startDecorator={<HistoryIcon className="w-3 h-3" />}
          onClick={onViewVersions}
        >
          Versions
        </Button>
      )}
      {canUpload && (
        <Button
          size="sm"
          variant="solid"
          color="primary"
          startDecorator={<UploadIcon className="w-3 h-3" />}
          onClick={() => fileRef.current?.click()}
          loading={isSaving}
        >
          {t("agent-admin.upload-file")}
        </Button>
      )}
    </div>
  </div>
);

interface CreateTenantModalProps {
  open: boolean;
  onClose: () => void;
  t: (key: string) => string;
}

const CreateTenantModal = ({ open, onClose, t }: CreateTenantModalProps) => {
  const [formData, setFormData] = useState<CreateTenantRequest>({
    tenantSlug: "",
    companyName: "",
    vertical: "",
    externalKbFile: null,
    externalPolicyFile: null,
    internalKbFile: null,
    internalPolicyFile: null,
  });

  const externalKbRef = useRef<HTMLInputElement>(null);
  const externalPolicyRef = useRef<HTMLInputElement>(null);
  const internalKbRef = useRef<HTMLInputElement>(null);
  const internalPolicyRef = useRef<HTMLInputElement>(null);

  const { isSaving } = agentAdminStore.state;

  const handleSubmit = async () => {
    if (!formData.tenantSlug || !formData.companyName) {
      toast.error("Tenant slug and company name are required");
      return;
    }

    const success = await agentAdminStore.createTenant(formData);
    if (success) {
      toast.success(t("agent-admin.created-successfully"));
      setFormData({
        tenantSlug: "",
        companyName: "",
        vertical: "",
        externalKbFile: null,
        externalPolicyFile: null,
        internalKbFile: null,
        internalPolicyFile: null,
      });
      onClose();
    }
  };

  const handleFileChange = (field: keyof CreateTenantRequest, file: File | null) => {
    setFormData((prev) => ({ ...prev, [field]: file }));
  };

  return (
    <Modal open={open} onClose={onClose}>
      <ModalDialog sx={{ maxWidth: 600, width: "100%", maxHeight: "90vh", overflow: "auto" }}>
        <DialogTitle>{t("agent-admin.create-tenant")}</DialogTitle>
        <Divider />
        <DialogContent>
          <div className="space-y-4">
            <FormControl required>
              <FormLabel>{t("agent-admin.tenant-slug")}</FormLabel>
              <Input
                placeholder={t("agent-admin.tenant-slug-placeholder")}
                value={formData.tenantSlug}
                onChange={(e) => setFormData((prev) => ({ ...prev, tenantSlug: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "") }))}
              />
              <div className="text-xs text-gray-500 mt-1">{t("agent-admin.tenant-slug-hint")}</div>
            </FormControl>

            <FormControl required>
              <FormLabel>{t("agent-admin.company-name")}</FormLabel>
              <Input
                placeholder={t("agent-admin.company-name-placeholder")}
                value={formData.companyName}
                onChange={(e) => setFormData((prev) => ({ ...prev, companyName: e.target.value }))}
              />
            </FormControl>

            <FormControl>
              <FormLabel>{t("agent-admin.vertical")}</FormLabel>
              <Input
                placeholder={t("agent-admin.vertical-placeholder")}
                value={formData.vertical}
                onChange={(e) => setFormData((prev) => ({ ...prev, vertical: e.target.value }))}
              />
            </FormControl>

            <Divider>External Files</Divider>

            <div className="grid grid-cols-2 gap-4">
              <FormControl>
                <FormLabel>{t("agent-admin.external-kb")}</FormLabel>
                <input
                  type="file"
                  ref={externalKbRef}
                  onChange={(e) => handleFileChange("externalKbFile", e.target.files?.[0] || null)}
                  accept=".md,.txt"
                  className="hidden"
                />
                <Button
                  variant="outlined"
                  color="neutral"
                  fullWidth
                  onClick={() => externalKbRef.current?.click()}
                  startDecorator={<UploadIcon className="w-4 h-4" />}
                >
                  {formData.externalKbFile?.name || "Choose KB.MD"}
                </Button>
              </FormControl>

              <FormControl>
                <FormLabel>{t("agent-admin.external-policy")}</FormLabel>
                <input
                  type="file"
                  ref={externalPolicyRef}
                  onChange={(e) => handleFileChange("externalPolicyFile", e.target.files?.[0] || null)}
                  accept=".md,.txt"
                  className="hidden"
                />
                <Button
                  variant="outlined"
                  color="neutral"
                  fullWidth
                  onClick={() => externalPolicyRef.current?.click()}
                  startDecorator={<UploadIcon className="w-4 h-4" />}
                >
                  {formData.externalPolicyFile?.name || "Choose POLICY.MD"}
                </Button>
              </FormControl>
            </div>

            <Divider>Internal Files</Divider>

            <div className="grid grid-cols-2 gap-4">
              <FormControl>
                <FormLabel>{t("agent-admin.internal-kb")}</FormLabel>
                <input
                  type="file"
                  ref={internalKbRef}
                  onChange={(e) => handleFileChange("internalKbFile", e.target.files?.[0] || null)}
                  accept=".md,.txt"
                  className="hidden"
                />
                <Button
                  variant="outlined"
                  color="neutral"
                  fullWidth
                  onClick={() => internalKbRef.current?.click()}
                  startDecorator={<UploadIcon className="w-4 h-4" />}
                >
                  {formData.internalKbFile?.name || "Choose KB.MD"}
                </Button>
              </FormControl>

              <FormControl>
                <FormLabel>{t("agent-admin.internal-policy")}</FormLabel>
                <input
                  type="file"
                  ref={internalPolicyRef}
                  onChange={(e) => handleFileChange("internalPolicyFile", e.target.files?.[0] || null)}
                  accept=".md,.txt"
                  className="hidden"
                />
                <Button
                  variant="outlined"
                  color="neutral"
                  fullWidth
                  onClick={() => internalPolicyRef.current?.click()}
                  startDecorator={<UploadIcon className="w-4 h-4" />}
                >
                  {formData.internalPolicyFile?.name || "Choose POLICY.MD"}
                </Button>
              </FormControl>
            </div>
          </div>
        </DialogContent>
        <DialogActions>
          <Button variant="plain" color="neutral" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button color="primary" onClick={handleSubmit} loading={isSaving}>
            {t("common.create")}
          </Button>
        </DialogActions>
      </ModalDialog>
    </Modal>
  );
};

interface VersionHistoryListProps {
  versions: any[];
  onRestore: (versionId: number) => void;
  isSaving: boolean;
  t: (key: string) => string;
}

const VersionHistoryList = ({ versions, onRestore, isSaving, t }: VersionHistoryListProps) => {
  if (versions.length === 0) {
    return <div className="text-center text-gray-500 py-4">No version history available</div>;
  }

  return (
    <div className="space-y-2">
      {versions.map((version, index) => (
        <div
          key={version.id}
          className={cn(
            "flex justify-between items-center p-3 rounded-lg",
            index === 0 ? "bg-teal-50 dark:bg-teal-900/20 border border-teal-200 dark:border-teal-800" : "bg-gray-50 dark:bg-zinc-700"
          )}
        >
          <div>
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">Version {versions.length - index}</span>
              {index === 0 && (
                <Chip size="sm" color="success" variant="soft">
                  {t("agent-admin.current-version")}
                </Chip>
              )}
            </div>
            <div className="text-xs text-gray-500">
              {t("agent-admin.imported-at")}: {new Date(version.importedAt).toLocaleString()}
            </div>
            <div className="text-xs text-gray-400">Hash: {version.contentHash?.slice(0, 12)}...</div>
          </div>
          {index !== 0 && (
            <Button size="sm" variant="outlined" color="neutral" onClick={() => onRestore(version.id)} loading={isSaving}>
              {t("agent-admin.restore-version")}
            </Button>
          )}
        </div>
      ))}
    </div>
  );
};

// ============================================================================
// SCRIPT.MD SECTION (tenant-level conversation flow guide)
// ============================================================================

interface ScriptSectionProps {
  tenantSlug: string;
  script: any | null;
  isLoading: boolean;
  isSaving: boolean;
  canUpload?: boolean;
  t: (key: string) => string;
}

const ScriptSection = ({ tenantSlug, script, isLoading, isSaving, canUpload = false, t }: ScriptSectionProps) => {
  const scriptFileRef = useRef<HTMLInputElement>(null);
  const [showPreview, setShowPreview] = useState(false);

  const handleUpload = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      const success = await agentAdminStore.uploadScript(tenantSlug, file);
      if (success) {
        toast.success(t("agent-admin.script-uploaded"));
      }
      e.target.value = "";
    }
  };

  const handleDelete = async () => {
    if (confirm(t("agent-admin.script-delete-confirm"))) {
      const success = await agentAdminStore.deleteScript(tenantSlug);
      if (success) {
        toast.success(t("agent-admin.script-deleted"));
      }
    }
  };

  return (
    <div className="bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4">
      <div className="flex justify-between items-center mb-3">
        <div>
          <h3 className="font-medium text-gray-800 dark:text-gray-200">{t("agent-admin.script-title")}</h3>
          <p className="text-xs text-gray-500 dark:text-gray-400">{t("agent-admin.script-description")}</p>
        </div>
        {script && (
          <Chip size="sm" color="success" variant="soft">
            {t("agent-admin.configured")}
          </Chip>
        )}
      </div>

      {isLoading ? (
        <div className="text-center py-4 text-gray-500">Loading...</div>
      ) : script ? (
        <div className="space-y-3">
          {/* Script info */}
          <div className="bg-gray-50 dark:bg-zinc-700 rounded-lg p-3">
            <div className="flex justify-between items-start">
              <div>
                <div className="text-sm text-gray-700 dark:text-gray-300">
                  <FileTextIcon className="w-4 h-4 inline mr-1" />
                  SCRIPT.MD
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  {t("agent-admin.imported-at")}: {new Date(script.importedAt).toLocaleString()}
                </div>
                <div className="text-xs text-gray-400">
                  Version: {script.version} • Hash: {script.contentHash?.slice(0, 12)}...
                </div>
              </div>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="outlined"
                  color="neutral"
                  onClick={() => setShowPreview(!showPreview)}
                >
                  {showPreview ? t("common.hide") : t("agent-admin.preview")}
                </Button>
                {canUpload && (
                  <Button
                    size="sm"
                    variant="plain"
                    color="danger"
                    onClick={handleDelete}
                    loading={isSaving}
                  >
                    <Trash2Icon className="w-4 h-4" />
                  </Button>
                )}
              </div>
            </div>
            {showPreview && (
              <div className="mt-3 p-3 bg-white dark:bg-zinc-800 rounded border border-gray-200 dark:border-zinc-600 max-h-64 overflow-y-auto">
                <pre className="text-xs text-gray-600 dark:text-gray-400 whitespace-pre-wrap">
                  {script.content}
                </pre>
              </div>
            )}
          </div>

          {/* Replace button */}
          {canUpload && (
            <div className="flex justify-end">
              <input
                type="file"
                ref={scriptFileRef}
                onChange={handleUpload}
                accept=".md,.txt"
                className="hidden"
              />
              <Button
                size="sm"
                variant="outlined"
                color="primary"
                startDecorator={<UploadIcon className="w-3 h-3" />}
                onClick={() => scriptFileRef.current?.click()}
                loading={isSaving}
              >
                {t("agent-admin.replace-script")}
              </Button>
            </div>
          )}
        </div>
      ) : (
        <div className="text-center py-6">
          <FileTextIcon className="w-10 h-10 mx-auto mb-2 text-gray-300" />
          <p className="text-sm text-gray-500 mb-4">{t("agent-admin.no-script")}</p>
          {canUpload && (
            <>
              <input
                type="file"
                ref={scriptFileRef}
                onChange={handleUpload}
                accept=".md,.txt"
                className="hidden"
              />
              <Button
                color="primary"
                startDecorator={<UploadIcon className="w-4 h-4" />}
                onClick={() => scriptFileRef.current?.click()}
                loading={isSaving}
              >
                {t("agent-admin.upload-script")}
              </Button>
            </>
          )}
        </div>
      )}
    </div>
  );
};

// ============================================================================
// LLM CONFIGURATION SECTION
// ============================================================================

interface LLMConfigSectionProps {
  tenantSlug: string;
  config: LLMConfig | null;
  isSaving: boolean;
  canEdit?: boolean;
  t: (key: string) => string;
}

const LLMConfigSection = ({ tenantSlug, config, isSaving, canEdit = false, t }: LLMConfigSectionProps) => {
  const [model, setModel] = useState(config?.llmModel || "");
  const [customModel, setCustomModel] = useState("");
  const [simHumanModel, setSimHumanModel] = useState(config?.simulationHumanModel || "");
  const [customSimHumanModel, setCustomSimHumanModel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [showApiKey, setShowApiKey] = useState(false);

  useEffect(() => {
    // Handle main model
    const modelValue = config?.llmModel || "";
    setModel(modelValue);
    const isPreset = LLM_MODEL_OPTIONS.some((opt) => opt.value === modelValue);
    if (modelValue && !isPreset) {
      setCustomModel(modelValue);
      setModel("custom");
    }
    // Handle simulation human model
    const simModelValue = config?.simulationHumanModel || "";
    setSimHumanModel(simModelValue);
    const isSimPreset = LLM_MODEL_OPTIONS.some((opt) => opt.value === simModelValue);
    if (simModelValue && !isSimPreset) {
      setCustomSimHumanModel(simModelValue);
      setSimHumanModel("custom");
    }
  }, [config?.llmModel, config?.simulationHumanModel]);

  const handleSave = async () => {
    const selectedModel = model === "custom" ? customModel : model;
    const selectedSimModel = simHumanModel === "custom" ? customSimHumanModel : simHumanModel;
    const success = await agentAdminStore.updateLLMConfig(tenantSlug, {
      llmModel: selectedModel,
      simulationHumanModel: selectedSimModel,
      openrouterApiKey: apiKey || undefined,
    });
    if (success) {
      toast.success("LLM configuration saved");
      setApiKey(""); // Clear input after save
    }
  };

  return (
    <div className="bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4">
      <h3 className="font-medium text-gray-800 dark:text-gray-200 mb-4">LLM Configuration</h3>

      <div className="space-y-4">
        <FormControl>
          <FormLabel>Model</FormLabel>
          <Select
            value={model || "openai/gpt-oss-120b:free"}
            onChange={(_, val) => setModel(val as string)}
            disabled={!canEdit}
          >
            {LLM_MODEL_OPTIONS.map((opt) => (
              <Option key={opt.value} value={opt.value}>
                {opt.label}
              </Option>
            ))}
            <Option value="custom">Custom Model...</Option>
          </Select>
        </FormControl>

        {model === "custom" && (
          <FormControl>
            <FormLabel>Custom Model ID</FormLabel>
            <Input
              placeholder="e.g., openai/gpt-4o"
              value={customModel}
              onChange={(e) => setCustomModel(e.target.value)}
              disabled={!canEdit}
            />
          </FormControl>
        )}

        <FormControl>
          <FormLabel>{t("agent-admin.simulation-human-model")}</FormLabel>
          <Select
            value={simHumanModel || ""}
            onChange={(_, val) => setSimHumanModel(val as string)}
            disabled={!canEdit}
            placeholder="Same as Model (default)"
          >
            <Option value="">Same as Model (default)</Option>
            {LLM_MODEL_OPTIONS.map((opt) => (
              <Option key={opt.value} value={opt.value}>
                {opt.label}
              </Option>
            ))}
            <Option value="custom">Custom Model...</Option>
          </Select>
          <FormHelperText>{t("agent-admin.simulation-human-model-hint")}</FormHelperText>
        </FormControl>

        {simHumanModel === "custom" && (
          <FormControl>
            <FormLabel>Custom Simulation Model ID</FormLabel>
            <Input
              placeholder="e.g., openai/gpt-4o"
              value={customSimHumanModel}
              onChange={(e) => setCustomSimHumanModel(e.target.value)}
              disabled={!canEdit}
            />
          </FormControl>
        )}

        <FormControl>
          <FormLabel>
            OpenRouter API Key
            {config?.hasApiKey && (
              <Chip size="sm" color="success" variant="soft" sx={{ ml: 1 }}>
                Set
              </Chip>
            )}
          </FormLabel>
          <Input
            type={showApiKey ? "text" : "password"}
            placeholder={config?.hasApiKey ? "Enter new key to replace" : "sk-or-v1-..."}
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            disabled={!canEdit}
            endDecorator={
              <Button
                variant="plain"
                color="neutral"
                size="sm"
                onClick={() => setShowApiKey(!showApiKey)}
                tabIndex={-1}
              >
                {showApiKey ? <EyeOffIcon className="w-4 h-4" /> : <EyeIcon className="w-4 h-4" />}
              </Button>
            }
          />
          <FormHelperText>{t("agent-admin.api-key-hint")}</FormHelperText>
        </FormControl>

        {canEdit && (
          <Button color="primary" onClick={handleSave} loading={isSaving}>
            Save Configuration
          </Button>
        )}
      </div>
    </div>
  );
};

// ============================================================================
// REASONING MODEL INPUT (for Auto-Generate section)
// ============================================================================

interface ReasoningModelInputProps {
  tenantSlug: string;
  config: LLMConfig | null;
  isSaving: boolean;
  t: (key: string) => string;
}

const ReasoningModelInput = ({ tenantSlug, config, isSaving, t }: ReasoningModelInputProps) => {
  const [reasoningModel, setReasoningModel] = useState(config?.reasoningModel || "");

  useEffect(() => {
    setReasoningModel(config?.reasoningModel || "");
  }, [config?.reasoningModel]);

  const handleSave = async () => {
    const success = await agentAdminStore.updateLLMConfig(tenantSlug, {
      llmModel: config?.llmModel || "",
      simulationHumanModel: config?.simulationHumanModel || "",
      reasoningModel: reasoningModel,
    });
    if (success) {
      toast.success("Reasoning model saved");
    }
  };

  return (
    <div className="flex items-center gap-3 bg-white dark:bg-zinc-900 rounded-lg border border-purple-200 dark:border-purple-700 p-3">
      <FormControl sx={{ flex: 1 }}>
        <FormLabel sx={{ fontSize: "0.875rem", color: "var(--joy-palette-purple-700)" }}>
          {t("agent-admin.reasoning-model")}
        </FormLabel>
        <Input
          size="sm"
          placeholder="google/gemini-2.5-pro"
          value={reasoningModel}
          onChange={(e) => setReasoningModel(e.target.value)}
        />
        <FormHelperText sx={{ fontSize: "0.75rem" }}>{t("agent-admin.reasoning-model-hint")}</FormHelperText>
      </FormControl>
      <Button
        size="sm"
        color="primary"
        onClick={handleSave}
        loading={isSaving}
        disabled={reasoningModel === (config?.reasoningModel || "")}
      >
        Save
      </Button>
    </div>
  );
};

// ============================================================================
// USER PERMISSIONS SECTION
// ============================================================================

interface UserPermissionsSectionProps {
  tenantSlug: string;
  permissions: UserPermission[];
  isSaving: boolean;
}

const UserPermissionsSection = observer(({ tenantSlug, permissions, isSaving }: UserPermissionsSectionProps) => {
  const [showGrantModal, setShowGrantModal] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState<string>("");
  const [selectedPreset, setSelectedPreset] = useState<string>("tester");
  const [isLoadingUsers, setIsLoadingUsers] = useState(false);
  const allUsers = agentAdminStore.state.allUsers;

  // Filter out users who already have permissions
  const existingUserIds = new Set(permissions.map((p) => p.userId));
  const availableUsers = allUsers.filter((u) => !existingUserIds.has(u.id));

  const handleOpenModal = async () => {
    setIsLoadingUsers(true);
    setShowGrantModal(true);
    await agentAdminStore.fetchUsers();
    setIsLoadingUsers(false);
  };

  const handleGrant = async () => {
    if (!selectedUserId) {
      toast.error("Please select a user");
      return;
    }

    const userId = parseInt(selectedUserId, 10);
    const perms = PERMISSION_PRESETS[selectedPreset as keyof typeof PERMISSION_PRESETS] || [];
    const success = await agentAdminStore.grantPermission(tenantSlug, {
      userId,
      permissions: perms,
    });

    if (success) {
      toast.success("Permission granted");
      setShowGrantModal(false);
      setSelectedUserId("");
    }
  };

  const handleRevoke = async (userId: number) => {
    const success = await agentAdminStore.revokePermission(tenantSlug, userId);
    if (success) {
      toast.success("Permission revoked");
    }
  };

  return (
    <div className="bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4">
      <div className="flex justify-between items-center mb-4">
        <h3 className="font-medium text-gray-800 dark:text-gray-200">User Access</h3>
        <Button
          size="sm"
          startDecorator={<PlusIcon className="w-4 h-4" />}
          onClick={handleOpenModal}
        >
          Add User
        </Button>
      </div>

      {permissions.length === 0 ? (
        <div className="text-center text-gray-500 py-4">
          No users have been granted access to this tenant.
        </div>
      ) : (
        <div className="space-y-2">
          {permissions.map((perm) => (
            <div
              key={perm.userId}
              className="flex justify-between items-center p-3 bg-gray-50 dark:bg-zinc-700 rounded-lg"
            >
              <div>
                <div className="font-medium text-sm">{perm.username}</div>
                <div className="text-xs text-gray-500">
                  {perm.permissions.join(", ")}
                </div>
              </div>
              <Button
                size="sm"
                variant="plain"
                color="danger"
                onClick={() => handleRevoke(perm.userId)}
                loading={isSaving}
              >
                Revoke
              </Button>
            </div>
          ))}
        </div>
      )}

      {/* Grant Permission Modal */}
      <Modal open={showGrantModal} onClose={() => setShowGrantModal(false)}>
        <ModalDialog sx={{ maxWidth: 400, width: "100%" }}>
          <DialogTitle>Grant User Access</DialogTitle>
          <Divider />
          <DialogContent>
            <div className="space-y-4">
              <FormControl>
                <FormLabel>User</FormLabel>
                {isLoadingUsers ? (
                  <div className="text-sm text-gray-500 py-2">Loading users...</div>
                ) : availableUsers.length === 0 ? (
                  <div className="text-sm text-gray-500 py-2">
                    {allUsers.length > 0 ? "All users already have permissions" : "No users found"}
                  </div>
                ) : (
                  <Select
                    placeholder="Select a user"
                    value={selectedUserId || null}
                    onChange={(_, val) => {
                      setSelectedUserId(val as string || "");
                    }}
                    slotProps={{
                      listbox: {
                        sx: { zIndex: 99999 },
                      },
                    }}
                  >
                    {availableUsers.map((user) => (
                      <Option key={user.id} value={String(user.id)}>
                        {user.username} ({user.name || user.role})
                      </Option>
                    ))}
                  </Select>
                )}
              </FormControl>

              <FormControl>
                <FormLabel>Permission Preset</FormLabel>
                <Select
                  value={selectedPreset}
                  onChange={(_, val) => setSelectedPreset(val as string)}
                >
                  <Option value="viewer">Viewer (read-only)</Option>
                  <Option value="tester">Tester (read + test chat)</Option>
                  <Option value="analyst">Analyst (read + view logs)</Option>
                  <Option value="editor">Editor (read + write + upload)</Option>
                  <Option value="tenant_admin">Tenant Admin (full access)</Option>
                </Select>
              </FormControl>
            </div>
          </DialogContent>
          <DialogActions>
            <Button variant="plain" color="neutral" onClick={() => setShowGrantModal(false)}>
              Cancel
            </Button>
            <Button color="primary" onClick={handleGrant} loading={isSaving}>
              Grant Access
            </Button>
          </DialogActions>
        </ModalDialog>
      </Modal>
    </div>
  );
});

export default AgentAdmin;
