import { Button, Chip, DialogActions, DialogContent, DialogTitle, Divider, FormControl, FormHelperText, FormLabel, Input, Modal, ModalClose, ModalDialog, Option, Select, Switch, Textarea } from "@mui/joy";
import { ArrowLeftIcon, BrainIcon, BuildingIcon, CheckIcon, CopyIcon, EditIcon, FileTextIcon, HistoryIcon, PlusIcon, RefreshCwIcon, SettingsIcon, SparklesIcon, Trash2Icon, UploadIcon, XIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { ChangeEvent, useEffect, useRef, useState } from "react";
import toast from "react-hot-toast";
import MobileHeader from "@/components/MobileHeader";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { agentAdminStore, userStore } from "@/store/v2";
import type { AgentTenant, AgentLearningMemory, CreateTenantRequest, LLMConfig, SetLLMConfigRequest, UserPermission, GrantPermissionRequest } from "@/store/v2/agentAdmin";
import { LLM_MODEL_OPTIONS, PERMISSION_PRESETS } from "@/store/v2/agentAdmin";
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
  const [showGeneratedContent, setShowGeneratedContent] = useState<{ type: "kb" | "policy"; content: string } | null>(null);
  const [isGenerating, setIsGenerating] = useState<"kb" | "policy" | null>(null);

  const { tenants, selectedTenant, isLoading, isSaving, error, fileVersions, llmConfig, tenantPermissions, myPermissions, script, isLoadingScript, learningMemory, isLoadingLearning } = agentAdminStore.state;

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
      // Fetch learning memory if user is admin or has tenant:admin
      if (isAdmin || agentAdminStore.hasPermission("tenant:admin")) {
        agentAdminStore.fetchLearningMemory(selectedTenant.tenant.slug);
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

            {/* Endpoints */}
            <div className="bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4">
              <h3 className="font-medium text-gray-800 dark:text-gray-200 mb-3">{t("agent-admin.endpoints")}</h3>
              <div className="space-y-2 text-sm">
                <EndpointRow label={t("agent-admin.external-chat-endpoint")} value={selectedTenant.endpoints.externalChat} onCopy={copyToClipboard} />
                <EndpointRow label={t("agent-admin.internal-chat-endpoint")} value={selectedTenant.endpoints.internalChat} onCopy={copyToClipboard} />
                <EndpointRow label={t("agent-admin.widget-endpoint")} value={selectedTenant.endpoints.widget} onCopy={copyToClipboard} />
              </div>
            </div>

            {/* LLM Configuration - visible to users with tenant:read or api:config */}
            {(canRead || canConfigApi) && (
              <LLMConfigSection
                tenantSlug={selectedTenant.tenant.slug}
                config={llmConfig}
                isSaving={isSaving}
                canEdit={canConfigApi}
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

            {/* Applied Learnings - Agent Self-Improvement (admin/tenant:admin only) */}
            {canManagePermissions && (
              <AppliedLearningsSection
                tenantSlug={selectedTenant.tenant.slug}
                learningMemory={learningMemory}
                isLoading={isLoadingLearning}
                isSaving={isSaving}
                t={t}
              />
            )}

            {/* External Configuration */}
            <AudienceSection
              title="External (Customer-Facing)"
              audienceType="external"
              tenant={selectedTenant}
              onViewVersions={handleViewVersions}
              isSaving={isSaving}
              t={t}
              canUpload={canUpload}
              canRestore={canRestore}
            />

            {/* Auto-Generate Annotated Content - Admin only */}
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
                  <div className="flex gap-2 flex-wrap">
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
                </div>
              </div>
            )}

            {/* Internal Configuration */}
            <AudienceSection
              title="Internal (Employee-Facing)"
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
                : t("agent-admin.generated-policy-title")}
            </DialogTitle>
            <Divider />
            <DialogContent sx={{ overflow: "auto" }}>
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
          isSaving={isSaving}
          t={t}
          canUpload={canUpload}
          canRestore={canRestore}
        />
      </div>
    </div>
  );
};

interface FileUploadRowProps {
  label: string;
  hint: string;
  fileRef: React.RefObject<HTMLInputElement>;
  onUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onViewVersions: () => void;
  isSaving: boolean;
  t: (key: string) => string;
  canUpload?: boolean;
  canRestore?: boolean;
}

const FileUploadRow = ({ label, hint, fileRef, onUpload, onViewVersions, isSaving, t, canUpload = false, canRestore = false }: FileUploadRowProps) => (
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
// LEARNING INSIGHTS SECTION (Agent Self-Improvement)
// ============================================================================

interface AppliedLearningsSectionProps {
  tenantSlug: string;
  learningMemory: AgentLearningMemory | null;
  isLoading: boolean;
  isSaving: boolean;
  t: (key: string) => string;
}

const AppliedLearningsSection = ({ tenantSlug, learningMemory, isLoading, isSaving, t }: AppliedLearningsSectionProps) => {
  const handleRemoveBehavior = async (behaviorId: string) => {
    if (confirm(t("agent-admin.remove-behavior-confirm"))) {
      const success = await agentAdminStore.removeLearnedBehavior(tenantSlug, behaviorId);
      if (success) {
        toast.success(t("agent-admin.behavior-removed"));
      }
    }
  };

  const handleClearAll = async () => {
    if (confirm(t("agent-admin.clear-learning-confirm"))) {
      const success = await agentAdminStore.clearLearningMemory(tenantSlug);
      if (success) {
        toast.success(t("agent-admin.learning-cleared"));
      }
    }
  };

  const learnedBehaviors = learningMemory?.learned_behaviors?.filter((b) => b.is_active) || [];

  // Get display text for a behavior (v2 uses content, v1 uses trigger+behavior)
  const getBehaviorText = (behavior: LearnedBehavior) => {
    if (behavior.content) {
      return behavior.content;
    }
    if (behavior.trigger && behavior.behavior) {
      return `${behavior.trigger}: ${behavior.behavior}`;
    }
    return behavior.behavior || "Unknown";
  };

  return (
    <div className="bg-white dark:bg-zinc-800 rounded-xl border border-gray-200 dark:border-zinc-700 p-4">
      <div className="flex justify-between items-center mb-3">
        <div className="flex items-center gap-2">
          <BrainIcon className="w-5 h-5 text-purple-500" />
          <div>
            <h3 className="font-medium text-gray-800 dark:text-gray-200">{t("agent-admin.applied-learnings-title")}</h3>
            <p className="text-xs text-gray-500 dark:text-gray-400">{t("agent-admin.applied-learnings-description")}</p>
          </div>
        </div>
        {learnedBehaviors.length > 0 && (
          <Chip size="sm" color="success" variant="soft">
            {t("agent-admin.active-count", { count: learnedBehaviors.length })}
          </Chip>
        )}
      </div>

      {isLoading ? (
        <div className="text-center py-4 text-gray-500">Loading...</div>
      ) : learnedBehaviors.length === 0 ? (
        <div className="text-center py-6 text-gray-500 dark:text-gray-400">
          <BrainIcon className="w-8 h-8 mx-auto mb-2 opacity-50" />
          <p className="text-sm">{t("agent-admin.no-learnings")}</p>
          <p className="text-xs mt-1">{t("agent-admin.no-learnings-hint")}</p>
        </div>
      ) : (
        <div className="space-y-3">
          {/* Applied Learnings List */}
          <div className="space-y-2">
            {learnedBehaviors.map((behavior) => (
              <div
                key={behavior.id}
                className="bg-green-50 dark:bg-green-900/20 rounded-lg p-3 border border-green-200 dark:border-green-800"
              >
                <div className="flex justify-between items-start gap-2">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-gray-700 dark:text-gray-300">{getBehaviorText(behavior)}</p>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      {t("common.added")}: {behavior.added_at} • {behavior.source}
                    </p>
                  </div>
                  <Button
                    size="sm"
                    variant="plain"
                    color="danger"
                    onClick={() => handleRemoveBehavior(behavior.id)}
                    loading={isSaving}
                  >
                    <Trash2Icon className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            ))}
          </div>

          {/* Clear All Button */}
          <div className="pt-2 border-t border-gray-200 dark:border-zinc-700">
            <Button
              size="sm"
              variant="plain"
              color="danger"
              onClick={handleClearAll}
              loading={isSaving}
            >
              {t("agent-admin.clear-all")}
            </Button>
          </div>
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
}

const LLMConfigSection = ({ tenantSlug, config, isSaving, canEdit = false }: LLMConfigSectionProps) => {
  const [model, setModel] = useState(config?.llmModel || "");
  const [customModel, setCustomModel] = useState("");
  const [apiKey, setApiKey] = useState("");

  useEffect(() => {
    const modelValue = config?.llmModel || "";
    setModel(modelValue);
    // Check if it's a custom model (not in preset list)
    const isPreset = LLM_MODEL_OPTIONS.some((opt) => opt.value === modelValue);
    if (modelValue && !isPreset) {
      setCustomModel(modelValue);
      setModel("custom");
    }
  }, [config?.llmModel]);

  const handleSave = async () => {
    const selectedModel = model === "custom" ? customModel : model;
    const success = await agentAdminStore.updateLLMConfig(tenantSlug, {
      llmModel: selectedModel,
      openrouterApiKey: apiKey || undefined,
    });
    if (success) {
      toast.success("LLM configuration saved");
      setApiKey("");
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

        {canEdit && (
          <FormControl>
            <FormLabel>OpenRouter API Key</FormLabel>
            <Input
              type="password"
              placeholder={config?.hasApiKey ? "••••••••••• (key is set)" : "sk-or-..."}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
            />
            <div className="text-xs text-gray-500 mt-1">
              {config?.hasApiKey ? "Leave empty to keep current key" : "Enter your OpenRouter API key"}
            </div>
          </FormControl>
        )}

        {!canEdit && config?.hasApiKey && (
          <div className="text-sm text-gray-500">
            API key is configured (hidden for security)
          </div>
        )}

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
