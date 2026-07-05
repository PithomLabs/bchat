import { Button, Checkbox, Input } from "@usememos/mui";
import { LoaderIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { ClientError } from "nice-grpc-web";
import { useState } from "react";
import { toast } from "react-hot-toast";
import { authServiceClient } from "@/grpcweb";
import useLoading from "@/hooks/useLoading";
import useNavigateTo from "@/hooks/useNavigateTo";
import { workspaceStore } from "@/store/v2";
import { initialUserStore } from "@/store/v2/user";
import { useTranslate } from "@/utils/i18n";

interface TenantInfo {
  id: number;
  name: string;
  slug: string;
}

const PasswordSignInForm = observer(() => {
  const t = useTranslate();
  const navigateTo = useNavigateTo();
  const actionBtnLoadingState = useLoading(false);
  const [username, setUsername] = useState(workspaceStore.state.profile.mode === "demo" ? "yourselfhosted" : "");
  const [password, setPassword] = useState(workspaceStore.state.profile.mode === "demo" ? "yourselfhosted" : "");
  const [remember, setRemember] = useState(true);
  const [selectionToken, setSelectionToken] = useState<string | null>(null);
  const [tenants, setTenants] = useState<TenantInfo[]>([]);
  const [selectedTenantId, setSelectedTenantId] = useState<number | null>(null);

  const handleUsernameInputChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    const text = e.target.value as string;
    setUsername(text);
  };

  const handlePasswordInputChanged = (e: React.ChangeEvent<HTMLInputElement>) => {
    const text = e.target.value as string;
    setPassword(text);
  };

  const handleFormSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    handleSignInButtonClick();
  };

  const handleSignInButtonClick = async () => {
    if (username === "" || password === "") {
      return;
    }

    if (actionBtnLoadingState.isLoading) {
      return;
    }

    try {
      actionBtnLoadingState.setLoading();

      // First, try the standard gRPC sign-in
      try {
        await authServiceClient.signIn({ passwordCredentials: { username, password }, neverExpire: remember });
        await initialUserStore();
        navigateTo("/");
        return;
      } catch (error: any) {
        // If the error indicates multiple tenants, fall back to REST flow
        const details = (error as ClientError).details || "";
        if (!details.includes("multiple tenants")) {
          throw error;
        }
      }

      // Multi-tenant flow: get tenant list via REST
      const response = await fetch("/api/v1/auth/tenants", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.message || "Failed to get tenants");
      }

      const data = await response.json();
      setTenants(data.tenants);
      setSelectionToken(data.selection_token);

      if (data.tenants.length === 1) {
        // Auto-select single tenant
        await selectTenant(data.selection_token, data.tenants[0].id);
      }
    } catch (error: any) {
      console.error(error);
      toast.error(error.message || "Failed to sign in.");
    } finally {
      actionBtnLoadingState.setFinish();
    }
  };

  const selectTenant = async (token: string, tenantId: number) => {
    try {
      const response = await fetch("/api/v1/auth/select-tenant", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ selection_token: token, tenant_id: tenantId }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.message || "Failed to select tenant");
      }

      // Store tenant_id and refresh user data
      localStorage.setItem("tenant_id", tenantId.toString());
      await initialUserStore();
      navigateTo("/");
    } catch (error: any) {
      toast.error(error.message || "Failed to select tenant");
    }
  };

  const handleTenantSelect = async () => {
    if (!selectionToken || !selectedTenantId) {
      return;
    }
    await selectTenant(selectionToken, selectedTenantId);
  };

  // Show tenant selector if we have tenants
  if (tenants.length > 0 && !selectionToken) {
    return (
      <div className="w-full mt-2">
        <div className="flex flex-col justify-start items-start w-full gap-4">
          <div className="w-full flex flex-col justify-start items-start">
            <span className="leading-8 text-gray-600">{t("auth.select-tenant")}</span>
            <select
              className="w-full bg-white dark:bg-black border border-gray-300 rounded px-3 py-2"
              value={selectedTenantId?.toString() || ""}
              onChange={(e) => setSelectedTenantId(Number(e.target.value))}
            >
              <option value="">{t("auth.select-tenant-tip")}</option>
              {tenants.map((tenant) => (
                <option key={tenant.id} value={tenant.id}>
                  {tenant.name}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="flex flex-row justify-end items-center w-full mt-6">
          <Button
            color="primary"
            size="lg"
            fullWidth
            disabled={!selectedTenantId || actionBtnLoadingState.isLoading}
            onClick={handleTenantSelect}
          >
            {t("common.sign-in")}
            {actionBtnLoadingState.isLoading && <LoaderIcon className="w-5 h-auto ml-2 animate-spin opacity-60" />}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <form className="w-full mt-2" onSubmit={handleFormSubmit}>
      <div className="flex flex-col justify-start items-start w-full gap-4">
        <div className="w-full flex flex-col justify-start items-start">
          <span className="leading-8 text-gray-600">{t("common.username")}</span>
          <Input
            className="w-full bg-white dark:bg-black"
            size="lg"
            type="text"
            readOnly={actionBtnLoadingState.isLoading}
            placeholder={t("common.username")}
            value={username}
            autoComplete="username"
            autoCapitalize="off"
            spellCheck={false}
            onChange={handleUsernameInputChanged}
            required
          />
        </div>
        <div className="w-full flex flex-col justify-start items-start">
          <span className="leading-8 text-gray-600">{t("common.password")}</span>
          <Input
            className="w-full bg-white dark:bg-black"
            size="lg"
            type="password"
            readOnly={actionBtnLoadingState.isLoading}
            placeholder={t("common.password")}
            value={password}
            autoComplete="password"
            autoCapitalize="off"
            spellCheck={false}
            onChange={handlePasswordInputChanged}
            required
          />
        </div>
      </div>
      <div className="flex flex-row justify-start items-center w-full mt-6">
        <Checkbox label={t("common.remember-me")} checked={remember} onChange={(e) => setRemember(e.target.checked)} />
      </div>
      <div className="flex flex-row justify-end items-center w-full mt-6">
        <Button
          type="submit"
          color="primary"
          size="lg"
          fullWidth
          disabled={actionBtnLoadingState.isLoading}
          onClick={handleSignInButtonClick}
        >
          {t("common.sign-in")}
          {actionBtnLoadingState.isLoading && <LoaderIcon className="w-5 h-auto ml-2 animate-spin opacity-60" />}
        </Button>
      </div>
    </form>
  );
});

export default PasswordSignInForm;
