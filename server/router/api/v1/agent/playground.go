package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/usememos/memos/store"
)

type PlaygroundCapability struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type PlaygroundScenario struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Highlights  []string `json:"highlights"`
}

type PlaygroundDemoTenant struct {
	Slug         string                 `json:"slug"`
	CompanyName  string                 `json:"company_name"`
	Vertical     string                 `json:"vertical"`
	Summary      string                 `json:"summary"`
	Available    bool                   `json:"available"`
	Capabilities []PlaygroundCapability `json:"capabilities"`
	Scenarios    []PlaygroundScenario   `json:"scenarios"`
}

type PlaygroundCatalogResponse struct {
	Demos       []PlaygroundDemoTenant `json:"demos"`
	SelfHosting []string               `json:"self_hosting"`
	Support     PlaygroundSupport      `json:"support"`
}

type PlaygroundSupport struct {
	Partner  string   `json:"partner"`
	Message  string   `json:"message"`
	Services []string `json:"services"`
}

type PlaygroundSeedResponse struct {
	Seeded []PlaygroundSeedResult `json:"seeded"`
}

type PlaygroundSeedResult struct {
	Slug       string `json:"slug"`
	TenantID   int32  `json:"tenant_id"`
	Created    bool   `json:"created"`
	Imported   bool   `json:"imported"`
	Reindexed  bool   `json:"reindexed"`
	ImportNote string `json:"import_note,omitempty"`
}

type PlaygroundRunRequest struct {
	SessionID       string `json:"session_id"`
	Message         string `json:"message"`
	ScenarioID      string `json:"scenario_id"`
	ClientMessageID string `json:"client_message_id"`
}

type PlaygroundRunResponse struct {
	Demo      PlaygroundDemoTenant `json:"demo"`
	Chat      *ChatResponse        `json:"chat"`
	Artifacts PlaygroundArtifacts  `json:"artifacts"`
}

type PlaygroundArtifacts struct {
	Intent       string                 `json:"intent"`
	Phase        string                 `json:"phase"`
	Urgency      int                    `json:"urgency"`
	RAG          PlaygroundRAGTrace     `json:"rag"`
	Lead         *store.AgentLead       `json:"lead,omitempty"`
	Transcript   *PlaygroundTranscript  `json:"transcript,omitempty"`
	Escalation   PlaygroundEscalation   `json:"escalation"`
	Capabilities []PlaygroundCapability `json:"capabilities"`
}

type PlaygroundRAGTrace struct {
	Enabled bool                  `json:"enabled"`
	Query   string                `json:"query"`
	Results []PlaygroundRAGResult `json:"results"`
	Error   string                `json:"error,omitempty"`
}

type PlaygroundRAGResult struct {
	Rank           int     `json:"rank"`
	Score          float64 `json:"score"`
	Title          string  `json:"title"`
	ContentPreview string  `json:"content_preview"`
	ContentType    string  `json:"content_type"`
	AudienceType   string  `json:"audience_type"`
}

type PlaygroundTranscript struct {
	ID           string               `json:"id"`
	SessionID    string               `json:"session_id"`
	MessageCount int                  `json:"message_count"`
	Messages     []store.AgentMessage `json:"messages"`
}

type PlaygroundEscalation struct {
	Active      bool   `json:"active"`
	Status      string `json:"status,omitempty"`
	RoutingMode string `json:"routing_mode,omitempty"`
	HandoffID   string `json:"handoff_id,omitempty"`
}

func playgroundDemoDefinitions() []PlaygroundDemoTenant {
	capabilities := []PlaygroundCapability{
		{ID: "knowledge", Label: "AI knowledge management", Description: "Answers are grounded in tenant KB, policy, script, and RAG retrieval."},
		{ID: "lead_capture", Label: "Lead capture", Description: "Customer name, contact, location, and intent are extracted from natural conversation."},
		{ID: "service", Label: "Customer service", Description: "The agent follows tenant-specific tone, coverage, safety, and escalation rules."},
		{ID: "tickets", Label: "Ticket and handoff flow", Description: "Urgent or unresolved conversations can create operational follow-up artifacts."},
		{ID: "integration", Label: "Backend integration", Description: "Widget, bridge, transcript, and API endpoints connect bchat to existing systems."},
	}

	return []PlaygroundDemoTenant{
		{
			Slug:         "demo-home-services",
			CompanyName:  "Harbor Home Services",
			Vertical:     "Home services",
			Summary:      "Emergency repair, service-area triage, contact capture, and escalation.",
			Capabilities: capabilities,
			Scenarios: []PlaygroundScenario{
				{
					ID:          "emergency-lead",
					Title:       "Emergency service lead",
					Description: "A homeowner needs urgent help and gives contact details naturally.",
					Prompt:      "My basement has standing water and I need help today. I'm Maya Chen, 415-555-0198, in Daly City.",
					Highlights:  []string{"Intent detection", "Urgency scoring", "Lead capture", "Escalation readiness"},
				},
				{
					ID:          "coverage",
					Title:       "Coverage and policy answer",
					Description: "Ask a practical service question that should be answered from the knowledge base.",
					Prompt:      "Do you service my area and can you explain what happens before the technician arrives?",
					Highlights:  []string{"RAG retrieval", "Policy compliance", "Customer-service flow"},
				},
			},
		},
		{
			Slug:         "demo-clinic",
			CompanyName:  "Northstar Clinic",
			Vertical:     "Healthcare",
			Summary:      "Policy-bound answers, safe triage language, scheduling intent, and retention follow-up.",
			Capabilities: capabilities,
			Scenarios: []PlaygroundScenario{
				{
					ID:          "safe-triage",
					Title:       "Policy-safe triage",
					Description: "The agent should avoid diagnosis while guiding the visitor to the right next step.",
					Prompt:      "I have a persistent cough and want to know if I should book a visit or wait it out.",
					Highlights:  []string{"Safety policy", "Boundary handling", "Appointment intent"},
				},
				{
					ID:          "retention",
					Title:       "Retention and follow-up",
					Description: "A returning patient asks about preparation and next actions.",
					Prompt:      "I visited last month and need the prep instructions before scheduling my follow-up.",
					Highlights:  []string{"Knowledge retrieval", "Customer retention", "Scripted next step"},
				},
			},
		},
		{
			Slug:         "demo-saas",
			CompanyName:  "AtlasOps SaaS",
			Vertical:     "B2B SaaS",
			Summary:      "Technical support, internal knowledge lookup, ticket-style escalation, and integration hooks.",
			Capabilities: capabilities,
			Scenarios: []PlaygroundScenario{
				{
					ID:          "support-ticket",
					Title:       "Support issue to ticket",
					Description: "A user reports an integration problem that should become a support workflow.",
					Prompt:      "Our webhook stopped syncing invoices after the API key rotation. Can you help?",
					Highlights:  []string{"Technical support", "Ticket workflow", "Backend integration"},
				},
				{
					ID:          "knowledge-base",
					Title:       "Knowledge-base answer",
					Description: "Ask for a product answer that should come from tenant documentation.",
					Prompt:      "What is the best way to connect the agent to our CRM and support queue?",
					Highlights:  []string{"RAG search", "Integration guidance", "Internal enablement"},
				},
			},
		},
	}
}

func playgroundSeedFiles(slug string) (externalKB, externalPolicy, internalKB, internalPolicy, script string) {
	switch slug {
	case "demo-home-services":
		return `# Harbor Home Services Knowledge Base

## Emergency Water Extraction
Harbor Home Services provides emergency water extraction, moisture inspection, drying equipment setup, and follow-up restoration planning for homes in Daly City, South San Francisco, San Mateo, and nearby Peninsula communities.

## What To Do Before Arrival
If it is safe, shut off the water source, avoid standing water near electrical outlets, move valuables away from wet areas, and take photos for documentation. Customers should not enter unsafe rooms.

## Contact And Scheduling
The dispatch line is 415-555-0198. The agent should collect the customer's name, phone number, address or city, service concern, urgency, and preferred callback window.

## Exclusions
Harbor Home Services does not perform mold lab testing, major structural engineering, or insurance claim adjustment. The agent may explain that partner referrals can be discussed by staff.
`, `# Harbor Home Services Policy

## Identity
You are a professional customer service representative for Harbor Home Services. Be calm, concise, and practical.

## Lead Capture
For service requests, collect name plus phone or email, location, and the problem. Confirm urgent issues and offer dispatch follow-up.

## Safety
Do not tell customers to enter standing water, touch electrical equipment, or perform unsafe repairs. For immediate danger, recommend emergency services.

## Escalation
Escalate when water is active, electrical safety is mentioned, the customer asks for a human, or the urgency is high.
`, `# Harbor Home Services Internal Knowledge

## Dispatch Notes
High-urgency water calls require customer name, phone, city, loss source, active leak status, and whether electricity is affected.

## CRM Integration
Create a lead and attach transcript notes before assigning a dispatcher.
`, `# Harbor Home Services Internal Policy

## Internal Assistant
Help staff summarize conversations, prepare dispatch notes, and identify missing lead fields. Never invent pricing or availability.
`, `# Harbor Home Services Script

## Stage: Opening
Greet the customer and identify the service concern.

## Stage: Triage
Check urgency, safety, location, and contact details.

## Stage: Resolution
Confirm next step, escalation, or dispatch follow-up.
`
	case "demo-clinic":
		return `# Northstar Clinic Knowledge Base

## Appointments
Northstar Clinic helps patients schedule primary care visits, follow-ups, wellness checks, and nurse callbacks. Patients can request appointment guidance through chat.

## Preparation
For follow-up visits, patients should bring recent medication lists, discharge notes if applicable, and questions for the clinician.

## Safety Boundaries
The chat agent does not diagnose, prescribe medication, interpret test results, or replace clinical judgment. Urgent symptoms should be directed to emergency care or the clinic's urgent line.

## Contact
The clinic scheduling desk is 212-555-0142. The agent may collect name, phone or email, preferred visit type, and general reason for visit.
`, `# Northstar Clinic Policy

## Identity
You are a careful clinic support assistant. Use warm, plain language.

## Medical Safety
Do not provide diagnosis, treatment plans, medication changes, or certainty about symptoms. Encourage clinician review.

## Lead And Retention
When a visitor wants an appointment or follow-up, collect contact details and the reason for the visit.

## Escalation
Escalate for severe symptoms, urgent safety concerns, medication emergencies, or requests for a clinician.
`, `# Northstar Clinic Internal Knowledge

## Staff Workflow
Appointment leads should include patient name, contact, preferred visit type, general concern, and urgency.

## Compliance Reminder
Keep chat notes factual and avoid diagnostic conclusions.
`, `# Northstar Clinic Internal Policy

## Internal Assistant
Help staff triage administrative follow-up while preserving medical safety boundaries.
`, `# Northstar Clinic Script

## Stage: Opening
Welcome the patient and ask what kind of help they need.

## Stage: Safety Boundary
Avoid diagnosis and guide urgent concerns to appropriate care.

## Stage: Scheduling
Collect contact information and appointment preference.
`
	case "demo-saas":
		return `# AtlasOps SaaS Knowledge Base

## Product Overview
AtlasOps SaaS connects AI support agents to CRM records, support queues, webhooks, billing systems, and internal knowledge bases.

## Webhook Troubleshooting
If webhooks stop syncing after API key rotation, verify the active key, webhook endpoint URL, HMAC signature settings, retry logs, and recent deployment changes.

## CRM Integration
Recommended CRM setup includes a tenant-scoped API key, lead field mapping, transcript attachment, and status synchronization.

## Support Workflow
For integration incidents, collect account name, contact email, affected integration, last successful sync time, error message, and business impact.
`, `# AtlasOps SaaS Policy

## Identity
You are a technical support representative for AtlasOps SaaS. Be direct, specific, and careful.

## Support Capture
For support issues, collect account, contact, affected system, impact, and evidence. Offer to create a support workflow when impact is high.

## Boundaries
Do not claim backend access, rotate credentials, or guarantee fixes. Provide safe diagnostic steps and escalate when needed.

## Escalation
Escalate billing outages, data sync failures, security concerns, and urgent production incidents.
`, `# AtlasOps SaaS Internal Knowledge

## Integration Handoff
Attach transcript summary, RAG citations, suspected component, and customer impact to the support ticket.

## Backend Services
Bridge handoff and webhook delivery are used to connect visitor chat to operator tools.
`, `# AtlasOps SaaS Internal Policy

## Internal Assistant
Help operators summarize incidents, identify missing diagnostic fields, and prepare ticket updates.
`, `# AtlasOps SaaS Script

## Stage: Opening
Confirm the integration or product area.

## Stage: Diagnose
Ask for impact, timing, error messages, and recent changes.

## Stage: Escalate
Create a support handoff when business impact or security risk is high.
`
	default:
		return "", "", "", "", ""
	}
}

func (h *Handler) ensurePlaygroundDemo(ctx context.Context, demo PlaygroundDemoTenant, force bool) (PlaygroundDemoTenant, PlaygroundSeedResult, error) {
	result := PlaygroundSeedResult{Slug: demo.Slug}
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &demo.Slug})
	if err != nil {
		return demo, result, fmt.Errorf("check demo tenant %q: %w", demo.Slug, err)
	}
	if tenant == nil {
		tenant, err = h.store.CreateAgentTenant(ctx, &store.AgentTenant{
			Slug:        demo.Slug,
			CompanyName: demo.CompanyName,
			Vertical:    demo.Vertical,
			IsActive:    true,
		})
		if err != nil {
			return demo, result, fmt.Errorf("create demo tenant %q: %w", demo.Slug, err)
		}
		result.Created = true
		force = true
	} else {
		tenant.CompanyName = demo.CompanyName
		tenant.Vertical = demo.Vertical
		tenant.IsActive = true
		tenant, err = h.store.UpdateAgentTenant(ctx, tenant)
		if err != nil {
			return demo, result, fmt.Errorf("update demo tenant %q: %w", demo.Slug, err)
		}
	}
	result.TenantID = tenant.ID

	shouldImport := force || h.playgroundDemoFilesMissing(ctx, tenant.ID)
	if shouldImport {
		externalKB, externalPolicy, internalKB, internalPolicy, script := playgroundSeedFiles(demo.Slug)
		if externalKB == "" || externalPolicy == "" || internalKB == "" || internalPolicy == "" || script == "" {
			return demo, result, fmt.Errorf("missing bundled files for demo tenant %q", demo.Slug)
		}
		if _, err := h.importFiles(ctx, tenant.ID, "external", externalKB, externalPolicy); err != nil {
			return demo, result, fmt.Errorf("import external files for demo tenant %q: %w", demo.Slug, err)
		}
		if _, err := h.importFiles(ctx, tenant.ID, "internal", internalKB, internalPolicy); err != nil {
			return demo, result, fmt.Errorf("import internal files for demo tenant %q: %w", demo.Slug, err)
		}
		if _, err := h.store.UpsertAgentTenantScript(ctx, &store.AgentTenantScript{
			TenantID:    tenant.ID,
			Content:     script,
			ContentHash: ContentHash(script),
		}); err != nil {
			return demo, result, fmt.Errorf("import script for demo tenant %q: %w", demo.Slug, err)
		}
		result.Imported = true
		result.Reindexed = h.service.IsRAGEnabled()
		h.service.configCache.Invalidate(tenant.Slug)
	} else {
		result.ImportNote = "existing demo files kept"
	}

	demo.Available = true
	demo.CompanyName = tenant.CompanyName
	demo.Vertical = tenant.Vertical
	return demo, result, nil
}

func (h *Handler) playgroundDemoFilesMissing(ctx context.Context, tenantID int32) bool {
	for _, required := range []struct {
		audienceType string
		fileType     string
	}{
		{audienceType: "external", fileType: "kb"},
		{audienceType: "external", fileType: "policy"},
		{audienceType: "internal", fileType: "kb"},
		{audienceType: "internal", fileType: "policy"},
	} {
		audienceType := required.audienceType
		fileType := required.fileType
		latestOnly := true
		file, err := h.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
			TenantID:     &tenantID,
			AudienceType: &audienceType,
			FileType:     &fileType,
			LatestOnly:   latestOnly,
		})
		if err != nil || file == nil || strings.TrimSpace(file.Content) == "" {
			return true
		}
	}

	script, err := h.store.GetAgentTenantScript(ctx, &store.FindAgentTenantScript{TenantID: &tenantID})
	return err != nil || script == nil || strings.TrimSpace(script.Content) == ""
}

// HandlePlaygroundCatalog returns public-safe demo tenant metadata.
func (h *Handler) HandlePlaygroundCatalog(c echo.Context) error {
	ctx := c.Request().Context()
	demos := playgroundDemoDefinitions()
	for i := range demos {
		demo, _, err := h.ensurePlaygroundDemo(ctx, demos[i], false)
		if err != nil {
			slog.Error("failed to provision playground demo", "slug", demos[i].Slug, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Playground demo is preparing. Please refresh in a moment.")
		}
		demos[i] = demo
	}

	return c.JSON(http.StatusOK, PlaygroundCatalogResponse{
		Demos: demos,
		SelfHosting: []string{
			"Run bchat on-prem with SQLite or a managed SQL database.",
			"Deploy to a private cloud or public cloud with RAG storage attached.",
			"Bring tenant KB, policy, script, widget, bridge, and webhook integrations under your own control.",
		},
		Support: PlaygroundSupport{
			Partner: "Pithom Labs",
			Message: "Self-host bchat independently, or bring in Pithom Labs for implementation, integrations, migration, training, and ongoing support.",
			Services: []string{
				"Architecture and deployment planning",
				"Knowledge-base migration and RAG tuning",
				"CRM, support desk, webhook, and backend integration",
				"Operational support from prototype to production",
			},
		},
	})
}

// HandleSeedPlaygroundDemos creates or refreshes the built-in demo tenants.
func (h *Handler) HandleSeedPlaygroundDemos(c echo.Context) error {
	ctx := c.Request().Context()
	if !h.isAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Admin role required")
	}

	var req struct {
		Force bool `json:"force"`
	}
	if c.Request().Body != nil {
		_ = c.Bind(&req)
	}

	results := make([]PlaygroundSeedResult, 0, len(playgroundDemoDefinitions()))
	for _, demo := range playgroundDemoDefinitions() {
		_, result, err := h.ensurePlaygroundDemo(ctx, demo, req.Force)
		if err != nil {
			slog.Error("failed to seed playground demo", "slug", demo.Slug, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to seed playground demos")
		}
		results = append(results, result)
	}

	return c.JSON(http.StatusOK, PlaygroundSeedResponse{Seeded: results})
}

// HandlePlaygroundRun runs a public demo chat turn and returns inspection artifacts.
func (h *Handler) HandlePlaygroundRun(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	demo, ok := playgroundDemoBySlug(slug)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "Playground demo not found")
	}

	demo, seedResult, err := h.ensurePlaygroundDemo(ctx, demo, false)
	if err != nil {
		slog.Error("failed to provision playground demo for run", "slug", slug, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Playground demo is preparing. Please try again in a moment.")
	}

	var req PlaygroundRunRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		if scenario, ok := playgroundScenarioByID(demo, req.ScenarioID); ok {
			req.Message = scenario.Prompt
		}
	}
	if req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Message is required")
	}

	clientIP := c.RealIP()
	if clientIP == "" {
		clientIP = c.Request().RemoteAddr
	}
	userAgent := c.Request().UserAgent()
	if userAgent == "" {
		userAgent = "bchat-playground"
	}

	chatResp, err := h.service.ChatExternal(ctx, slug, clientIP, userAgent, ChatRequest{
		SessionID:       req.SessionID,
		Message:         req.Message,
		ClientMessageID: req.ClientMessageID,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalidExternalSessionID) {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid session_id")
		}
		if strings.Contains(err.Error(), "rate limit") {
			return echo.NewHTTPError(http.StatusTooManyRequests, "Too many requests. Please try again later.")
		}
		slog.Error("playground chat failed", "slug", slug, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Playground chat service unavailable")
	}

	artifacts := h.buildPlaygroundArtifacts(ctx, seedResult.TenantID, chatResp.SessionID, req.Message, chatResp, demo.Capabilities)
	return c.JSON(http.StatusOK, PlaygroundRunResponse{
		Demo:      demo,
		Chat:      chatResp,
		Artifacts: artifacts,
	})
}

func playgroundDemoBySlug(slug string) (PlaygroundDemoTenant, bool) {
	for _, demo := range playgroundDemoDefinitions() {
		if demo.Slug == slug {
			return demo, true
		}
	}
	return PlaygroundDemoTenant{}, false
}

func playgroundScenarioByID(demo PlaygroundDemoTenant, scenarioID string) (PlaygroundScenario, bool) {
	for _, scenario := range demo.Scenarios {
		if scenario.ID == scenarioID {
			return scenario, true
		}
	}
	return PlaygroundScenario{}, false
}

func (h *Handler) buildPlaygroundArtifacts(ctx context.Context, tenantID int32, sessionID, query string, chatResp *ChatResponse, capabilities []PlaygroundCapability) PlaygroundArtifacts {
	artifacts := PlaygroundArtifacts{
		Intent:       chatResp.Metadata.Intent,
		Phase:        chatResp.Metadata.Phase,
		Urgency:      chatResp.Metadata.Urgency,
		RAG:          h.buildPlaygroundRAGTrace(ctx, tenantID, query),
		Capabilities: capabilities,
	}

	if chatResp.Bridge != nil {
		artifacts.Escalation = PlaygroundEscalation{
			Active:      true,
			Status:      chatResp.Bridge.Status,
			RoutingMode: chatResp.Bridge.RoutingMode,
			HandoffID:   chatResp.Bridge.HandoffID,
		}
	} else if chatResp.Metadata.Urgency >= 4 || strings.Contains(strings.ToLower(chatResp.Metadata.Phase), "escal") {
		artifacts.Escalation = PlaygroundEscalation{
			Active: true,
			Status: "ai_escalation_ready",
		}
	}

	transcript, err := h.store.GetAgentTranscript(ctx, &store.FindAgentTranscript{
		TenantID:  &tenantID,
		SessionID: &sessionID,
	})
	if err != nil {
		slog.Warn("failed to load playground transcript artifact", "tenant_id", tenantID, "session_id", sessionID, "error", err)
	} else if transcript != nil {
		messages := transcript.Messages
		if len(messages) > 8 {
			messages = messages[len(messages)-8:]
		}
		artifacts.Transcript = &PlaygroundTranscript{
			ID:           transcript.ID,
			SessionID:    transcript.SessionID,
			MessageCount: transcript.MessageCount,
			Messages:     messages,
		}
	}

	lead, err := h.store.GetAgentLead(ctx, &store.FindAgentLead{
		TenantID:  &tenantID,
		SessionID: &sessionID,
	})
	if err != nil {
		slog.Warn("failed to load playground lead artifact", "tenant_id", tenantID, "session_id", sessionID, "error", err)
	} else if lead != nil {
		artifacts.Lead = lead
	}

	return artifacts
}

func (h *Handler) buildPlaygroundRAGTrace(ctx context.Context, tenantID int32, query string) PlaygroundRAGTrace {
	trace := PlaygroundRAGTrace{
		Enabled: h.service.IsRAGEnabled(),
		Query:   query,
	}
	if !trace.Enabled || strings.TrimSpace(query) == "" {
		return trace
	}

	result, err := h.service.SearchVectorDB(ctx, tenantID, "external", query, 3)
	if err != nil {
		trace.Error = "RAG search unavailable for this turn"
		slog.Warn("playground RAG trace failed", "tenant_id", tenantID, "error", err)
		return trace
	}
	if result == nil {
		return trace
	}

	trace.Results = make([]PlaygroundRAGResult, 0, len(result.Chunks))
	for i, chunk := range result.Chunks {
		score := 0.0
		if i < len(result.Scores) {
			score = result.Scores[i]
		}
		preview := chunk.Content
		if len(preview) > 260 {
			preview = preview[:260] + "..."
		}
		trace.Results = append(trace.Results, PlaygroundRAGResult{
			Rank:           i + 1,
			Score:          score,
			Title:          chunk.Title,
			ContentPreview: preview,
			ContentType:    chunk.ContentType,
			AudienceType:   chunk.AudienceType,
		})
	}
	return trace
}
