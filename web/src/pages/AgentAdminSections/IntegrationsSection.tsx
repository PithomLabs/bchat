import { useState } from "react";
import { observer } from "mobx-react-lite";
import {
  Button,
  Card,
  Chip,
  IconButton,
  Input,
  Modal,
  ModalDialog,
  Select,
  Option,
  Switch,
  Textarea,
  Typography,
} from "@mui/joy";
import { PlusIcon, TrashIcon, PlayIcon, RefreshCwIcon } from "lucide-react";
import toast from "react-hot-toast";
import agentAdminStore from "../../store/v2/agentAdmin";
import type { AgentIntegration, AgentEvent } from "../../store/v2/agentAdmin";

interface IntegrationsSectionProps {
  slug: string;
  isAdmin: boolean;
}

const IntegrationsSection = observer(({ slug, isAdmin }: IntegrationsSectionProps) => {
  const { integrations, isLoadingIntegrations, events, isLoadingEvents } = agentAdminStore.state;

  const [showAddModal, setShowAddModal] = useState(false);
  const [newLabel, setNewLabel] = useState("");
  const [newUrl, setNewUrl] = useState("");
  const [newSecret, setNewSecret] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const [testingId, setTestingId] = useState<number | null>(null);
  const [showEvents, setShowEvents] = useState(false);
  const [eventFilter, setEventFilter] = useState<string>("");

  const handleCreate = async () => {
    if (!newLabel.trim() || !newUrl.trim() || !newSecret.trim()) {
      toast.error("All fields are required");
      return;
    }
    setIsCreating(true);
    try {
      const result = await agentAdminStore.createIntegration(slug, {
        integration_type: "webhook",
        label: newLabel.trim(),
        config: { url: newUrl.trim(), secret: newSecret.trim() },
      });
      if (result) {
        toast.success("Webhook created");
        setShowAddModal(false);
        setNewLabel("");
        setNewUrl("");
        setNewSecret("");
      } else {
        toast.error("Failed to create webhook");
      }
    } finally {
      setIsCreating(false);
    }
  };

  const handleToggle = async (ig: AgentIntegration) => {
    const success = await agentAdminStore.updateIntegration(slug, ig.id, {
      is_active: !ig.is_active,
    });
    if (success) {
      toast.success(ig.is_active ? "Webhook disabled" : "Webhook enabled");
    } else {
      toast.error("Failed to update webhook");
    }
  };

  const handleDelete = async (ig: AgentIntegration) => {
    if (!confirm(`Delete webhook "${ig.label}"?`)) return;
    const success = await agentAdminStore.deleteIntegration(slug, ig.id);
    if (success) {
      toast.success("Webhook deleted");
    } else {
      toast.error("Failed to delete webhook");
    }
  };

  const handleTest = async (ig: AgentIntegration) => {
    setTestingId(ig.id);
    try {
      const result = await agentAdminStore.testIntegration(slug, ig.id);
      if (result.success) {
        toast.success("Test webhook delivered successfully");
      } else {
        toast.error(result.error || "Test failed");
      }
    } finally {
      setTestingId(null);
    }
  };

  const handleRefreshEvents = async () => {
    await agentAdminStore.fetchEvents(slug, eventFilter ? { status: eventFilter } : undefined);
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "delivered": return "success";
      case "failed": return "danger";
      case "pending": return "warning";
      case "processing": return "primary";
      default: return "neutral";
    }
  };

  return (
    <div className="bg-purple-50 dark:bg-purple-900/20 rounded-xl border border-purple-200 dark:border-purple-700 p-4 mt-4">
      <div className="flex items-center justify-between mb-3">
        <div>
          <h3 className="text-lg font-semibold text-purple-800 dark:text-purple-200">
            Webhook Integrations
          </h3>
          <p className="text-sm text-purple-600 dark:text-purple-400">
            Send events to external services when leads are captured.
          </p>
        </div>
        <div className="flex gap-2">
          {isAdmin && (
            <Button
              size="sm"
              color="primary"
              startDecorator={<PlusIcon size={16} />}
              onClick={() => setShowAddModal(true)}
            >
              Add Webhook
            </Button>
          )}
        </div>
      </div>

      {/* Integration List */}
      {isLoadingIntegrations ? (
        <div className="text-center py-4 text-purple-600">Loading...</div>
      ) : integrations.length === 0 ? (
        <div className="text-center py-6 text-purple-500">
          No webhooks configured. Add one to start receiving events.
        </div>
      ) : (
        <div className="space-y-3">
          {integrations.map((ig) => (
            <Card key={ig.id} size="sm" className="bg-white dark:bg-gray-800">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Switch
                    checked={ig.is_active}
                    onChange={() => handleToggle(ig)}
                    disabled={!isAdmin}
                    size="sm"
                  />
                  <div>
                    <Typography level="body-sm" fontWeight="bold">
                      {ig.label}
                    </Typography>
                    <Typography level="body-xs" color="neutral">
                      {ig.integration_type} • Created {new Date(ig.created_at * 1000).toLocaleDateString()}
                    </Typography>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <IconButton
                    size="sm"
                    variant="plain"
                    color="primary"
                    onClick={() => handleTest(ig)}
                    disabled={testingId === ig.id || !isAdmin}
                    loading={testingId === ig.id}
                  >
                    <PlayIcon size={16} />
                  </IconButton>
                  {isAdmin && (
                    <IconButton
                      size="sm"
                      variant="plain"
                      color="danger"
                      onClick={() => handleDelete(ig)}
                    >
                      <TrashIcon size={16} />
                    </IconButton>
                  )}
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Event Log Toggle */}
      <div className="mt-4">
        <Button
          size="sm"
          variant="outlined"
          color="neutral"
          onClick={() => {
            setShowEvents(!showEvents);
            if (!showEvents) {
              agentAdminStore.fetchEvents(slug);
            }
          }}
          startDecorator={<RefreshCwIcon size={14} />}
        >
          {showEvents ? "Hide Events" : "Show Events"}
        </Button>
      </div>

      {/* Event Log */}
      {showEvents && (
        <div className="mt-3">
          <div className="flex items-center gap-2 mb-2">
            <Select
              size="sm"
              value={eventFilter}
              onChange={(_, val) => setEventFilter(val as string)}
              placeholder="All statuses"
              sx={{ minWidth: 150 }}
            >
              <Option value="">All</Option>
              <Option value="delivered">Delivered</Option>
              <Option value="failed">Failed</Option>
              <Option value="pending">Pending</Option>
              <Option value="processing">Processing</Option>
            </Select>
            <Button size="sm" variant="plain" onClick={handleRefreshEvents}>
              Refresh
            </Button>
          </div>
          {isLoadingEvents ? (
            <div className="text-center py-2 text-gray-500">Loading...</div>
          ) : events.length === 0 ? (
            <div className="text-center py-2 text-gray-500 text-sm">No events found</div>
          ) : (
            <div className="max-h-64 overflow-y-auto space-y-1">
              {events.map((evt) => (
                <div
                  key={evt.id}
                  className="flex items-center justify-between text-xs py-1 px-2 bg-white dark:bg-gray-800 rounded"
                >
                  <div className="flex items-center gap-2">
                    <Chip size="sm" color={getStatusColor(evt.status)} variant="soft">
                      {evt.status}
                    </Chip>
                    <span className="font-mono">{evt.event_type}</span>
                  </div>
                  <span className="text-gray-500">
                    {new Date(evt.created_at * 1000).toLocaleString()}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Add Webhook Modal */}
      <Modal open={showAddModal} onClose={() => setShowAddModal(false)}>
        <ModalDialog>
          <Typography level="h4">Add Webhook</Typography>
          <Input
            placeholder="Label (e.g., Zapier, n8n)"
            value={newLabel}
            onChange={(e) => setNewLabel(e.target.value)}
          />
          <Input
            placeholder="Webhook URL"
            value={newUrl}
            onChange={(e) => setNewUrl(e.target.value)}
          />
          <Input
            placeholder="Secret (for HMAC signature)"
            type="password"
            value={newSecret}
            onChange={(e) => setNewSecret(e.target.value)}
          />
          <div className="flex gap-2 justify-end">
            <Button variant="outlined" onClick={() => setShowAddModal(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreate} loading={isCreating}>
              Create
            </Button>
          </div>
        </ModalDialog>
      </Modal>
    </div>
  );
});

export default IntegrationsSection;
