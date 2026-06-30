import { Button, Chip, CircularProgress, Tab, TabList, TabPanel, Tabs, Textarea } from "@mui/joy";
import {
  ArrowRightIcon,
  BotIcon,
  CheckCircleIcon,
  CloudIcon,
  DatabaseIcon,
  LifeBuoyIcon,
  MessageSquareIcon,
  PlayIcon,
  RefreshCwIcon,
  SearchIcon,
  SendIcon,
  ServerIcon,
  SparklesIcon,
  TicketIcon,
  UserRoundIcon,
  WorkflowIcon,
} from "lucide-react";
import { observer } from "mobx-react-lite";
import { KeyboardEvent, ReactNode, RefObject, useEffect, useRef } from "react";
import MobileHeader from "@/components/MobileHeader";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { playgroundStore } from "@/store/v2";
import type { PlaygroundArtifacts, PlaygroundCatalog, PlaygroundDemoTenant, PlaygroundScenario } from "@/store/v2/playground";
import { cn } from "@/utils";

const Playground = observer(() => {
  const { md } = useResponsiveWidth();
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const {
    catalog,
    selectedDemo,
    selectedScenario,
    messages,
    artifacts,
    input,
    isLoadingCatalog,
    isSending,
    error,
  } = playgroundStore.state;

  useEffect(() => {
    playgroundStore.fetchCatalog();
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleInputKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      playgroundStore.sendMessage();
    }
  };

  const selectScenario = (scenario: PlaygroundScenario) => {
    playgroundStore.selectScenario(scenario);
  };

  return (
    <section className="@container w-full min-h-full flex flex-col items-center bg-zinc-50 dark:bg-zinc-950 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full max-w-7xl px-4 sm:px-6 pt-5 md:pt-8">
        <header className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <div className="mb-2 flex items-center gap-2 text-sm font-medium text-teal-700 dark:text-teal-300">
              <SparklesIcon className="h-4 w-4" />
              bchat AI automation playground
            </div>
            <h1 className="text-3xl font-semibold text-zinc-950 dark:text-zinc-50">
              Explore tenant-aware AI from knowledge to operations
            </h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600 dark:text-zinc-400">
              Try customer service, retrieval, lead capture, transcripts, escalation signals, and deployment paths from one guided cockpit.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Chip variant="soft" color="success" startDecorator={<CheckCircleIcon className="h-4 w-4" />}>
              Self-host ready
            </Chip>
            <Chip variant="soft" color="primary" startDecorator={<WorkflowIcon className="h-4 w-4" />}>
              Integration-first
            </Chip>
          </div>
        </header>
        {error && !selectedDemo && (
          <div className="mb-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200">
            {error}
          </div>
        )}

        {isLoadingCatalog && !catalog ? (
          <div className="flex min-h-[420px] items-center justify-center">
            <CircularProgress />
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-[280px_minmax(0,1fr)_340px]">
            <DemoRail demos={catalog?.demos || []} selectedDemo={selectedDemo} />
            <ChatWorkbench
              selectedDemo={selectedDemo}
              selectedScenario={selectedScenario}
              input={input}
              isSending={isSending}
              error={error}
              messages={messages}
              messagesEndRef={messagesEndRef}
              onInputKeyDown={handleInputKeyDown}
              onSelectScenario={selectScenario}
            />
            <InsightPanel catalog={catalog} artifacts={artifacts} selectedDemo={selectedDemo} />
          </div>
        )}
      </div>
    </section>
  );
});

const DemoRail = ({ demos, selectedDemo }: { demos: PlaygroundDemoTenant[]; selectedDemo: PlaygroundDemoTenant | null }) => {
  return (
    <aside className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-zinc-500 dark:text-zinc-400">Choose an experience</h2>
        <Button variant="plain" color="neutral" size="sm" onClick={() => playgroundStore.fetchCatalog()}>
          <RefreshCwIcon className="h-4 w-4" />
        </Button>
      </div>
      {demos.map((demo) => (
        <button
          key={demo.slug}
          type="button"
          onClick={() => playgroundStore.selectDemo(demo)}
          className={cn(
            "w-full rounded-lg border bg-white p-4 text-left transition dark:bg-zinc-900",
            selectedDemo?.slug === demo.slug
              ? "border-teal-500 shadow-sm ring-2 ring-teal-500/10"
              : "border-zinc-200 hover:border-zinc-300 dark:border-zinc-800 dark:hover:border-zinc-700",
          )}
        >
          <div className="mb-2 flex items-center justify-between gap-2">
            <span className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">{demo.company_name}</span>
            <span
              className={cn(
                "rounded px-2 py-0.5 text-xs",
                demo.available
                  ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"
                  : "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
              )}
            >
              {demo.available ? "Live demo" : "Preparing"}
            </span>
          </div>
          <p className="text-xs font-medium text-teal-700 dark:text-teal-300">{demo.vertical}</p>
          <p className="mt-2 text-sm leading-5 text-zinc-600 dark:text-zinc-400">{demo.summary}</p>
        </button>
      ))}
    </aside>
  );
};

const ChatWorkbench = ({
  selectedDemo,
  selectedScenario,
  input,
  isSending,
  error,
  messages,
  messagesEndRef,
  onInputKeyDown,
  onSelectScenario,
}: {
  selectedDemo: PlaygroundDemoTenant | null;
  selectedScenario: PlaygroundScenario | null;
  input: string;
  isSending: boolean;
  error: string | null;
  messages: { role: string; content: string; timestamp: Date }[];
  messagesEndRef: RefObject<HTMLDivElement>;
  onInputKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  onSelectScenario: (scenario: PlaygroundScenario) => void;
}) => {
  return (
    <main className="min-h-[720px] rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <div className="border-b border-zinc-200 p-4 dark:border-zinc-800">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 className="flex items-center gap-2 text-lg font-semibold text-zinc-950 dark:text-zinc-50">
              <MessageSquareIcon className="h-5 w-5 text-teal-600" />
              {selectedDemo ? selectedDemo.company_name : "Select a demo"}
            </h2>
            <p className="text-sm text-zinc-500 dark:text-zinc-400">{selectedScenario?.description || "Choose a scenario to start."}</p>
          </div>
          <Button
            color="primary"
            startDecorator={<PlayIcon className="h-4 w-4" />}
            disabled={!selectedScenario || !selectedDemo || isSending}
            loading={isSending}
            onClick={() => playgroundStore.runScenario()}
          >
            Run scenario
          </Button>
        </div>
        {selectedDemo && (
          <div className="mt-4 grid grid-cols-1 gap-2 md:grid-cols-2">
            {selectedDemo.scenarios.map((scenario) => (
              <button
                key={scenario.id}
                type="button"
                onClick={() => onSelectScenario(scenario)}
                className={cn(
                  "rounded-lg border p-3 text-left transition",
                  selectedScenario?.id === scenario.id
                    ? "border-teal-500 bg-teal-50 dark:bg-teal-950/30"
                    : "border-zinc-200 bg-zinc-50 hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
                )}
              >
                <div className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">{scenario.title}</div>
                <div className="mt-1 flex flex-wrap gap-1">
                  {scenario.highlights.slice(0, 3).map((item) => (
                    <span key={item} className="rounded bg-white px-2 py-0.5 text-xs text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
                      {item}
                    </span>
                  ))}
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="flex h-[430px] flex-col gap-3 overflow-y-auto p-4">
        {messages.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center text-center text-zinc-500">
            <BotIcon className="mb-3 h-12 w-12 opacity-30" />
            <p className="text-sm">Run a scenario to see chat, retrieval, lead capture, and operations artifacts update live.</p>
          </div>
        ) : (
          messages.map((message, index) => (
            <div key={`${message.timestamp.toISOString()}-${index}`} className={cn("flex items-start gap-3", message.role === "user" && "flex-row-reverse")}>
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-800">
                {message.role === "user" ? <UserRoundIcon className="h-4 w-4" /> : <BotIcon className="h-4 w-4" />}
              </div>
              <div
                className={cn(
                  "max-w-[82%] rounded-lg border p-3 text-sm leading-6",
                  message.role === "user"
                    ? "border-teal-200 bg-teal-50 text-zinc-900 dark:border-teal-900 dark:bg-teal-950/40 dark:text-zinc-100"
                    : "border-zinc-200 bg-zinc-50 text-zinc-800 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-200",
                )}
              >
                <div className="mb-1 text-xs font-medium text-zinc-500">{message.role === "user" ? "Visitor" : "Agent"}</div>
                <p className="whitespace-pre-wrap break-words">{message.content}</p>
              </div>
            </div>
          ))
        )}
        {isSending && (
          <div className="flex items-center gap-2 text-sm text-zinc-500">
            <CircularProgress size="sm" />
            Processing turn
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      <div className="border-t border-zinc-200 p-4 dark:border-zinc-800">
        {error && <div className="mb-3 rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">{error}</div>}
        <div className="flex gap-2">
          <Textarea
            minRows={2}
            maxRows={4}
            className="flex-1"
            value={input}
            placeholder="Ask a follow-up or edit the scenario prompt"
            disabled={!selectedDemo || isSending}
            onChange={(event) => playgroundStore.state.setPartial({ input: event.target.value })}
            onKeyDown={onInputKeyDown}
          />
          <Button
            color="primary"
            disabled={!selectedDemo || isSending || !input.trim()}
            loading={isSending}
            onClick={() => playgroundStore.sendMessage()}
            sx={{ alignSelf: "stretch", minWidth: 44 }}
          >
            <SendIcon className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </main>
  );
};

const InsightPanel = ({
  catalog,
  artifacts,
  selectedDemo,
}: {
  catalog: PlaygroundCatalog | null;
  artifacts: PlaygroundArtifacts | null;
  selectedDemo: PlaygroundDemoTenant | null;
}) => {
  return (
    <aside className="flex flex-col gap-4">
      <Tabs defaultValue={0} sx={{ bgcolor: "transparent" }}>
        <TabList>
          <Tab>
            <DatabaseIcon className="h-4 w-4" />
            Trace
          </Tab>
          <Tab>
            <TicketIcon className="h-4 w-4" />
            Ops
          </Tab>
          <Tab>
            <ServerIcon className="h-4 w-4" />
            Deploy
          </Tab>
        </TabList>
        <TabPanel value={0} sx={{ px: 0 }}>
          <Panel title="Knowledge trace" icon={<SearchIcon className="h-4 w-4" />}>
            {!artifacts ? (
              <EmptyPanel text="RAG hits and response metadata appear after a turn." />
            ) : (
              <div className="space-y-3">
                <div className="grid grid-cols-3 gap-2">
                  <Metric label="Intent" value={artifacts.intent || "unknown"} />
                  <Metric label="Phase" value={artifacts.phase || "triage"} />
                  <Metric label="Urgency" value={`${artifacts.urgency || 0}`} />
                </div>
                {artifacts.rag.enabled ? (
                  artifacts.rag.results.length > 0 ? (
                    <div className="space-y-2">
                      {artifacts.rag.results.map((result) => (
                        <div key={`${result.rank}-${result.title}`} className="rounded-lg border border-zinc-200 p-3 dark:border-zinc-800">
                          <div className="mb-1 flex items-center justify-between gap-2">
                            <span className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">{result.title || `Chunk ${result.rank}`}</span>
                            <span className="text-xs text-zinc-500">{Math.round(result.score * 100)}%</span>
                          </div>
                          <p className="text-xs leading-5 text-zinc-600 dark:text-zinc-400">{result.content_preview}</p>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <EmptyPanel text={artifacts.rag.error || "RAG is enabled, but no chunks matched this turn."} />
                  )
                ) : (
                  <EmptyPanel text="RAG is disabled on this instance; chat still works through tenant prompts and source files." />
                )}
              </div>
            )}
          </Panel>
        </TabPanel>
        <TabPanel value={1} sx={{ px: 0 }}>
          <Panel title="Automation output" icon={<WorkflowIcon className="h-4 w-4" />}>
            {!artifacts ? (
              <EmptyPanel text="Lead, transcript, and escalation artifacts appear after a conversation turn." />
            ) : (
              <div className="space-y-3">
                <ArtifactRow label="Lead" value={artifacts.lead ? `${artifacts.lead.name || "Captured"} · ${artifacts.lead.status}` : "Not captured yet"} />
                <ArtifactRow label="Transcript" value={artifacts.transcript ? `${artifacts.transcript.message_count} messages` : "Waiting for recorded transcript"} />
                <ArtifactRow label="Escalation" value={artifacts.escalation.active ? artifacts.escalation.status || "Active" : "No escalation"} />
                <div className="rounded-lg border border-zinc-200 p-3 dark:border-zinc-800">
                  <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-zinc-500">Capabilities exercised</div>
                  <div className="flex flex-wrap gap-1">
                    {(selectedDemo?.capabilities || artifacts.capabilities).map((capability) => (
                      <Chip key={capability.id} size="sm" variant="soft">
                        {capability.label}
                      </Chip>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </Panel>
        </TabPanel>
        <TabPanel value={2} sx={{ px: 0 }}>
          <Panel title="Self-host and support" icon={<CloudIcon className="h-4 w-4" />}>
            <div className="space-y-3">
              {(catalog?.self_hosting || []).map((item) => (
                <div key={item} className="flex gap-2 text-sm leading-5 text-zinc-700 dark:text-zinc-300">
                  <ArrowRightIcon className="mt-0.5 h-4 w-4 shrink-0 text-teal-600" />
                  <span>{item}</span>
                </div>
              ))}
              {catalog?.support && (
                <div className="rounded-lg border border-teal-200 bg-teal-50 p-3 dark:border-teal-900 dark:bg-teal-950/30">
                  <div className="mb-1 flex items-center gap-2 text-sm font-semibold text-teal-900 dark:text-teal-100">
                    <LifeBuoyIcon className="h-4 w-4" />
                    {catalog.support.partner}
                  </div>
                  <p className="text-sm leading-5 text-teal-900/80 dark:text-teal-100/80">{catalog.support.message}</p>
                </div>
              )}
            </div>
          </Panel>
        </TabPanel>
      </Tabs>
    </aside>
  );
};

const Panel = ({ title, icon, children }: { title: string; icon: ReactNode; children: ReactNode }) => (
  <section className="rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900">
    <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-zinc-900 dark:text-zinc-100">
      {icon}
      {title}
    </h3>
    {children}
  </section>
);

const EmptyPanel = ({ text }: { text: string }) => <p className="rounded-lg bg-zinc-50 p-3 text-sm leading-5 text-zinc-500 dark:bg-zinc-950 dark:text-zinc-400">{text}</p>;

const Metric = ({ label, value }: { label: string; value: string }) => (
  <div className="rounded-lg bg-zinc-50 p-2 dark:bg-zinc-950">
    <div className="text-xs text-zinc-500">{label}</div>
    <div className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">{value}</div>
  </div>
);

const ArtifactRow = ({ label, value }: { label: string; value: string }) => (
  <div className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 p-3 dark:border-zinc-800">
    <span className="text-sm text-zinc-500">{label}</span>
    <span className="text-right text-sm font-semibold text-zinc-900 dark:text-zinc-100">{value}</span>
  </div>
);

export default Playground;
