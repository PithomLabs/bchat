package store

import (
	"context"
	"errors"
	"sync"
	"time"
)

// AgentTenant represents a tenant (business) using the agent system.
type AgentTenant struct {
	ID                int32
	Slug              string
	CompanyName       string
	GUID              string // Unique identifier for security (used in widget embed)
	Vertical          string
	IsActive          bool
	ProcessingOptions string // JSON-encoded ProcessingOptions for Format for RAG
	AllowedDomains    string // JSON array of allowed domains for widget embedding, empty = allow all
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// FindAgentTenant contains filters for finding tenants.
type FindAgentTenant struct {
	ID       *int32
	Slug     *string
	IsActive *bool
}

// AgentAudience represents audience-specific configuration.
type AgentAudience struct {
	ID           int32
	TenantID     int32
	AudienceType string // "external" or "internal"

	// Identity
	Role       string
	Tone       string
	BrandVoice string
	Guidelines []string

	// Contact
	EmergencyPhone  string
	SecondaryPhones []string
	Email           string
	Address         string

	// Thresholds
	EmergencyUrgencyThreshold     int
	EscalationConfidenceThreshold float64

	// Rate limiting
	RateLimitRPM int

	UpdatedAt time.Time
}

// FindAgentAudience contains filters for finding audiences.
type FindAgentAudience struct {
	TenantID     *int32
	AudienceType *string
}

// AgentService represents a service offered by the tenant.
type AgentService struct {
	ID           int32
	TenantID     int32
	AudienceType string
	Code         string
	Name         string
	Description  string
	IsEmergency  bool
	ResponseTime string
	IsActive     bool
}

// FindAgentService contains filters for finding services.
type FindAgentService struct {
	TenantID     *int32
	AudienceType *string
	Code         *string
	IsActive     *bool
}

// AgentExclusion represents a service the tenant doesn't provide.
type AgentExclusion struct {
	ID            int32
	TenantID      int32
	AudienceType  string
	Code          string
	Name          string
	Description   string
	ExceptionRule string
	Referral      string
	IsActive      bool
}

// FindAgentExclusion contains filters for finding exclusions.
type FindAgentExclusion struct {
	TenantID     *int32
	AudienceType *string
	IsActive     *bool
}

// AgentCoverage represents a service area.
type AgentCoverage struct {
	ID         int32
	TenantID   int32
	AreaType   string
	AreaName   string
	StateCode  string
	IsIncluded bool
}

// FindAgentCoverage contains filters for finding coverage areas.
type FindAgentCoverage struct {
	TenantID   *int32
	IsIncluded *bool
}

// AgentFAQ represents a frequently asked question.
type AgentFAQ struct {
	ID           int32
	TenantID     int32
	AudienceType string
	Code         string
	Question     string
	Answer       string
	IsActive     bool
}

// FindAgentFAQ contains filters for finding FAQs.
type FindAgentFAQ struct {
	TenantID     *int32
	AudienceType *string
	IsActive     *bool
}

// AgentSafetyProtocol represents safety instructions for specific intents.
type AgentSafetyProtocol struct {
	ID             int32
	TenantID       int32
	AudienceType   string
	Code           string
	Name           string
	TriggerIntents []string
	Instructions   []string
	IsActive       bool
}

// FindAgentSafetyProtocol contains filters for finding safety protocols.
type FindAgentSafetyProtocol struct {
	TenantID     *int32
	AudienceType *string
	IsActive     *bool
}

// AgentKBSection represents a generic knowledge base section.
type AgentKBSection struct {
	ID           int32
	TenantID     int32
	AudienceType string
	Code         string
	Title        string
	Content      string
	SectionType  string
	IsActive     bool
}

// FindAgentKBSection contains filters for finding KB sections.
type FindAgentKBSection struct {
	TenantID     *int32
	AudienceType *string
	SectionType  *string
	IsActive     *bool
}

// AgentIntent represents an intent that can be detected from user messages.
type AgentIntent struct {
	ID                  int32
	TenantID            *int32  // nil = global intent
	AudienceType        *string // nil = applies to all audiences
	Code                string
	Name                string
	Category            string
	Description         string
	Examples            []string
	CounterExamples     []string
	Urgency             int
	Action              string
	ConfidenceThreshold float64
	IsActive            bool
}

// FindAgentIntent contains filters for finding intents.
type FindAgentIntent struct {
	TenantID     *int32
	AudienceType *string
	Category     *string
	IsActive     *bool
}

// AgentRule represents a policy rule.
type AgentRule struct {
	ID           int32
	TenantID     int32
	AudienceType string
	Code         string
	Name         string
	Description  string
	Priority     int
	AppliesTo    string
	IsActive     bool
}

// FindAgentRule contains filters for finding rules.
type FindAgentRule struct {
	TenantID     *int32
	AudienceType *string
	IsActive     *bool
}

// AgentSession represents a chat session (internal sessions stored in SQLite).
type AgentSession struct {
	ID           string
	TenantID     int32
	UserID       *int32
	AudienceType string

	// State
	Phase          string
	CurrentIntent  string
	UrgencyLevel   int
	CoverageStatus string

	// Extracted data
	CustomerName     string
	CustomerPhone    string
	CustomerLocation string
	DetectedService  string

	// History
	MessageCount int
	Messages     []AgentMessage

	// Timestamps
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	IsCompleted      bool
	CompletionReason string

	// Session state tracking (in-memory only, not persisted)
	OutOfCoverageCount int    // Tracks how many times out-of-coverage was explained
	SafetyGiven        bool   // Tracks if full safety instructions were given
	EscalationTicket   string // Ticket number if escalation was created

	// Not persisted; guards concurrent ChatExternal calls for the same session.
	IdempotencyMu sync.Mutex `json:"-"`
}

// AgentMessage represents a single message in a chat session.
type AgentMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source,omitempty"`
	SourceID  string    `json:"source_id,omitempty"`
}

// FindAgentMessage contains filters for finding agent messages.
type FindAgentMessage struct {
	SessionID *string
	Source    *string
	SourceID  *string
	TenantID  *int32
}

// AgentMessageRecord represents a durable message row in the database.
type AgentMessageRecord struct {
	ID        int32
	SessionID string
	TenantID  int32
	Source    string
	SourceID  string
	Role      string
	Content   string
	CreatedAt time.Time
}

// FindAgentSession contains filters for finding sessions.
type FindAgentSession struct {
	ID       *string
	TenantID *int32
	UserID   *int32
	Limit    *int
}

// UpdateAgentSession contains fields to update on a session.
type UpdateAgentSession struct {
	ID               string
	Phase            *string
	CurrentIntent    *string
	UrgencyLevel     *int
	CoverageStatus   *string
	CustomerName     *string
	CustomerPhone    *string
	CustomerLocation *string
	DetectedService  *string
	MessageCount     *int
	Messages         []AgentMessage
	UpdatedAt        *time.Time
	CompletedAt      *time.Time
	IsCompleted      *bool
	CompletionReason *string
}

// AgentSourceFile represents an imported MD file.
type AgentSourceFile struct {
	ID           int32
	TenantID     int32
	AudienceType string
	FileType     string // "kb", "policy", or "script"
	Content      string
	ContentHash  string
	Version      int32 // Auto-incremented version number per tenant+audience+filetype
	ImportedAt   time.Time
}

// FindAgentSourceFile contains filters for finding source files.
type FindAgentSourceFile struct {
	ID           *int32
	TenantID     *int32
	AudienceType *string
	FileType     *string
	Version      *int32 // Specific version to retrieve
	LatestOnly   bool   // If true, only return the latest version
}

// AgentRateLimit tracks rate limiting per client.
type AgentRateLimit struct {
	ID           int32
	TenantID     int32
	AudienceType string
	ClientIP     string
	RequestCount int
	WindowStart  time.Time
}

// FindAgentRateLimit contains filters for finding rate limits.
type FindAgentRateLimit struct {
	TenantID     *int32
	AudienceType *string
	ClientIP     *string
}

// AgentSimulationTranscript represents a saved simulation transcript.
type AgentSimulationTranscript struct {
	ID            string
	TenantID      int32
	UserID        int32
	InitialPrompt string
	PersonaHint   string
	TotalTurns    int
	EndReason     string
	Messages      []SimulationMessage
	CreatedAt     time.Time
}

// SimulationMessage represents a single message in a simulation.
type SimulationMessage struct {
	Role      string              `json:"role"` // "human_sim" or "agent"
	Content   string              `json:"content"`
	TurnNum   int                 `json:"turn_num"`
	Timestamp time.Time           `json:"timestamp"`
	Metadata  *SimulationMetadata `json:"metadata,omitempty"`
}

// SimulationMetadata contains agent response metadata.
type SimulationMetadata struct {
	Intent  string `json:"intent,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Urgency int    `json:"urgency,omitempty"`
}

// FindAgentSimulationTranscript contains filters for finding simulation transcripts.
type FindAgentSimulationTranscript struct {
	ID       *string
	TenantID *int32
	UserID   *int32
	Limit    int
	Offset   int
}

// AgentTenantScript represents a tenant's conversation flow script (SCRIPT.MD).
type AgentTenantScript struct {
	ID          int32
	TenantID    int32
	Content     string
	ContentHash string
	Summary     string // Condensed version for system prompt
	ImportedAt  time.Time
	Version     int
}

// FindAgentTenantScript contains filters for finding tenant scripts.
type FindAgentTenantScript struct {
	ID       *int32
	TenantID *int32
}

// AgentAnalysisResult represents the result of analyzing a transcript against benchmarks.
type AgentAnalysisResult struct {
	ID               string
	TenantID         int32
	ConversationID   string
	ConversationType string // "simulation" or "chat"
	UserID           int32
	Score            int
	Grade            string
	Breakdown        AnalysisBreakdown
	Issues           []AnalysisIssue
	Suggestions      []string
	BenchmarkVersion string
	CreatedAt        time.Time
}

// AnalysisBreakdown contains the score breakdown by category.
type AnalysisBreakdown struct {
	IntentRecognition    CategoryScore `json:"intent_recognition"`
	ServiceAlignment     CategoryScore `json:"service_alignment"`
	PolicyCompliance     CategoryScore `json:"policy_compliance"`
	ConversationFlow     CategoryScore `json:"conversation_flow"`
	InformationGathering CategoryScore `json:"information_gathering"`
	ToneResolution       CategoryScore `json:"tone_resolution"`
}

// CategoryScore represents a single category's score in the analysis.
type CategoryScore struct {
	Score int    `json:"score"`
	Max   int    `json:"max"`
	Notes string `json:"notes"`
}

// AnalysisIssue represents a specific issue found during analysis.
type AnalysisIssue struct {
	Severity string `json:"severity"` // "critical", "warning", "info"
	Turn     int    `json:"turn"`
	Message  string `json:"message"`
}

// FindAgentAnalysisResult contains filters for finding analysis results.
type FindAgentAnalysisResult struct {
	ID             *string
	TenantID       *int32
	ConversationID *string
	UserID         *int32
	Limit          int
	Offset         int
}

// AgentLearningMemory stores aggregated insights from analysis results for agent improvement.
type AgentLearningMemory struct {
	ID                 int32               `json:"id"`
	TenantID           int32               `json:"tenant_id"`
	CommonIssues       []CommonIssue       `json:"common_issues"`
	LearnedBehaviors   []LearnedBehavior   `json:"learned_behaviors"`
	ImprovementAreas   []ImprovementArea   `json:"improvement_areas"`
	PendingSuggestions []PendingSuggestion `json:"pending_suggestions"`
	AnalysisCount      int                 `json:"analysis_count"`
	LastUpdated        time.Time           `json:"last_updated"`
	Version            int                 `json:"version"`
}

// CommonIssue represents a frequently occurring issue from analysis.
type CommonIssue struct {
	Category    string `json:"category"`    // e.g., "information_gathering", "tone_resolution"
	Description string `json:"description"` // What the issue is
	Occurrences int    `json:"occurrences"` // How many times it appeared
	LastSeen    string `json:"last_seen"`   // Date of last occurrence
}

// LearnedBehavior represents a specific behavioral improvement to apply.
type LearnedBehavior struct {
	ID         string `json:"id"`                    // Unique identifier
	Content    string `json:"content"`               // The learning text (v2 simplified)
	Type       string `json:"type,omitempty"`        // "issue" or "suggestion" (v2)
	SourceID   string `json:"source_id,omitempty"`   // Analysis result ID (v2)
	SourceTurn int    `json:"source_turn,omitempty"` // Turn number for issues (v2)
	Trigger    string `json:"trigger,omitempty"`     // Legacy: When to apply (v1)
	Behavior   string `json:"behavior,omitempty"`    // Legacy: What to do (v1)
	Source     string `json:"source,omitempty"`      // How this was learned
	AddedAt    string `json:"added_at"`              // When it was added
	IsActive   bool   `json:"is_active"`             // Whether it's currently applied
}

// ImprovementArea represents a category that needs attention.
type ImprovementArea struct {
	Category     string  `json:"category"`      // Category name
	AverageScore float64 `json:"average_score"` // Average score in this category
	MaxScore     int     `json:"max_score"`     // Maximum possible score
	TrendPercent float64 `json:"trend_percent"` // Improvement/decline trend
}

// PendingSuggestion represents a suggestion awaiting admin approval.
type PendingSuggestion struct {
	ID          string `json:"id"`          // Unique identifier
	Category    string `json:"category"`    // Related category
	Trigger     string `json:"trigger"`     // Suggested trigger
	Behavior    string `json:"behavior"`    // Suggested behavior
	Occurrences int    `json:"occurrences"` // Times this issue was seen
	SourceIDs   string `json:"source_ids"`  // Comma-separated analysis IDs
	CreatedAt   string `json:"created_at"`  // When suggestion was generated
}

// FindAgentLearningMemory contains filters for finding learning memory.
type FindAgentLearningMemory struct {
	ID       *int32
	TenantID *int32
}

// AgentComplianceAudit represents the result of a compliance audit on a conversation.
type AgentComplianceAudit struct {
	ID               string
	TenantID         int32
	ConversationID   string
	ConversationType string // "simulation" or "chat"
	Score            int    // 0-100
	Checks           string // JSON array of check results
	OverallPassed    bool
	AuditedAt        time.Time
}

// FindAgentComplianceAudit contains filters for finding compliance audits.
type FindAgentComplianceAudit struct {
	ID               *string
	TenantID         *int32
	ConversationID   *string
	ConversationType *string
	Limit            *int
	Offset           *int
}

// AgentScoringConfig represents the heuristic scoring configuration for a tenant.
type AgentScoringConfig struct {
	ID        int32
	TenantID  int32
	Version   string
	Config    string // JSON configuration
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FindAgentScoringConfig contains filters for finding scoring configs.
type FindAgentScoringConfig struct {
	ID       *int32
	TenantID *int32
}

// AgentQAPair represents a Q&A test pair for evaluating embedding/retrieval quality.
type AgentQAPair struct {
	ID             int32     `json:"id"`
	TenantID       int32     `json:"tenant_id"`
	Question       string    `json:"question"`
	ExpectedAnswer string    `json:"expected_answer"`
	SourceSection  string    `json:"source_section,omitempty"`
	SourceChunkID  string    `json:"source_chunk_id,omitempty"`
	Difficulty     string    `json:"difficulty"` // easy, medium, hard
	Category       string    `json:"category"`   // faq, service, policy, coverage, etc.
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FindAgentQAPair contains filters for finding Q&A pairs.
type FindAgentQAPair struct {
	ID       *int32
	TenantID *int32
	Category *string
	IsActive *bool
}

// AgentTranscript represents a recorded chat conversation transcript.
type AgentTranscript struct {
	ID               string         `json:"id"`
	TenantID         int32          `json:"tenantId"`
	SessionID        string         `json:"sessionId"`
	AudienceType     string         `json:"audienceType"`
	Messages         []AgentMessage `json:"messages"`
	MessageCount     int            `json:"messageCount"`
	ClientIP         string         `json:"clientIp,omitempty"`
	UserAgent        string         `json:"userAgent,omitempty"`
	CustomerName     string         `json:"customerName,omitempty"`
	CustomerPhone    string         `json:"customerPhone,omitempty"`
	CustomerEmail    string         `json:"customerEmail,omitempty"`
	CustomerLocation string         `json:"customerLocation,omitempty"`
	DetectedIntent   string         `json:"detectedIntent,omitempty"`
	StartedAt        time.Time      `json:"startedAt"`
	EndedAt          *time.Time     `json:"endedAt,omitempty"`
	LastMessageAt    time.Time      `json:"lastMessageAt"`
	IsCompleted      bool           `json:"isCompleted"`
	CompletionReason string         `json:"completionReason,omitempty"`
}

// FindAgentTranscript contains filters for finding transcripts.
type FindAgentTranscript struct {
	ID           *string
	TenantID     *int32
	SessionID    *string
	AudienceType *string
	Limit        int
	Offset       int
}

// AgentLead represents a tenant-scoped sales/support lead captured from chat.
type AgentLead struct {
	ID             string     `json:"id"`
	TenantID       int32      `json:"tenantId"`
	SessionID      string     `json:"sessionId"`
	TranscriptID   string     `json:"transcriptId,omitempty"`
	Name           string     `json:"name"`
	Email          string     `json:"email,omitempty"`
	Phone          string     `json:"phone,omitempty"`
	Topic          string     `json:"topic,omitempty"`
	Location       string     `json:"location,omitempty"`
	DetectedIntent string     `json:"detectedIntent,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	LastMessageAt  time.Time  `json:"lastMessageAt"`
	ConvertedAt    *time.Time `json:"convertedAt,omitempty"`
}

// FindAgentLead contains filters for finding captured leads.
type FindAgentLead struct {
	ID        *string
	TenantID  *int32
	SessionID *string
	Status    *string
	Limit     int
	Offset    int
}

// ObservationLog represents the compressed memory state for a session.
type ObservationLog struct {
	SessionID            string    `json:"session_id"`
	TenantID             int32     `json:"tenant_id"`
	ResourceID           string    `json:"resource_id"` // NEW: For resource-scoped memory (user_id)
	ObservationLog       string    `json:"observation_log"`
	LastObservedMsgIndex int       `json:"last_observed_msg_index"`
	TokensInLog          int       `json:"tokens_in_log"`
	CurrentTask          string    `json:"current_task"`       // NEW: Current task from Observer
	SuggestedResponse    string    `json:"suggested_response"` // NEW: Suggested response hint
	CreatedAt            time.Time `json:"created_at"`
	LastUpdatedAt        time.Time `json:"last_updated_at"`
}

// AgentStore interface defines all agent-related database operations.
type AgentStore interface {
	// Tenant operations
	CreateAgentTenant(ctx context.Context, tenant *AgentTenant) (*AgentTenant, error)
	GetAgentTenant(ctx context.Context, find *FindAgentTenant) (*AgentTenant, error)
	ListAgentTenants(ctx context.Context, find *FindAgentTenant) ([]*AgentTenant, error)
	UpdateAgentTenant(ctx context.Context, tenant *AgentTenant) (*AgentTenant, error)
	DeleteAgentTenant(ctx context.Context, id int32) error

	// Audience operations
	CreateAgentAudience(ctx context.Context, audience *AgentAudience) (*AgentAudience, error)
	GetAgentAudience(ctx context.Context, find *FindAgentAudience) (*AgentAudience, error)
	ListAgentAudiences(ctx context.Context, find *FindAgentAudience) ([]*AgentAudience, error)
	UpdateAgentAudience(ctx context.Context, audience *AgentAudience) (*AgentAudience, error)
	DeleteAgentAudience(ctx context.Context, tenantID int32, audienceType string) error

	// Service operations
	CreateAgentService(ctx context.Context, service *AgentService) (*AgentService, error)
	ListAgentServices(ctx context.Context, find *FindAgentService) ([]*AgentService, error)
	DeleteAgentServices(ctx context.Context, tenantID int32, audienceType string) error

	// Exclusion operations
	CreateAgentExclusion(ctx context.Context, exclusion *AgentExclusion) (*AgentExclusion, error)
	ListAgentExclusions(ctx context.Context, find *FindAgentExclusion) ([]*AgentExclusion, error)
	DeleteAgentExclusions(ctx context.Context, tenantID int32, audienceType string) error

	// Coverage operations
	CreateAgentCoverage(ctx context.Context, coverage *AgentCoverage) (*AgentCoverage, error)
	ListAgentCoverage(ctx context.Context, find *FindAgentCoverage) ([]*AgentCoverage, error)
	DeleteAgentCoverage(ctx context.Context, tenantID int32) error

	// FAQ operations
	CreateAgentFAQ(ctx context.Context, faq *AgentFAQ) (*AgentFAQ, error)
	ListAgentFAQs(ctx context.Context, find *FindAgentFAQ) ([]*AgentFAQ, error)
	DeleteAgentFAQs(ctx context.Context, tenantID int32, audienceType string) error

	// Safety protocol operations
	CreateAgentSafetyProtocol(ctx context.Context, protocol *AgentSafetyProtocol) (*AgentSafetyProtocol, error)
	ListAgentSafetyProtocols(ctx context.Context, find *FindAgentSafetyProtocol) ([]*AgentSafetyProtocol, error)
	DeleteAgentSafetyProtocols(ctx context.Context, tenantID int32, audienceType string) error

	// KB section operations
	CreateAgentKBSection(ctx context.Context, section *AgentKBSection) (*AgentKBSection, error)
	ListAgentKBSections(ctx context.Context, find *FindAgentKBSection) ([]*AgentKBSection, error)
	DeleteAgentKBSections(ctx context.Context, tenantID int32, audienceType string) error

	// Intent operations
	CreateAgentIntent(ctx context.Context, intent *AgentIntent) (*AgentIntent, error)
	ListAgentIntents(ctx context.Context, find *FindAgentIntent) ([]*AgentIntent, error)
	DeleteAgentIntents(ctx context.Context, tenantID int32, audienceType *string) error

	// Rule operations
	CreateAgentRule(ctx context.Context, rule *AgentRule) (*AgentRule, error)
	ListAgentRules(ctx context.Context, find *FindAgentRule) ([]*AgentRule, error)
	DeleteAgentRules(ctx context.Context, tenantID int32, audienceType string) error

	// Session operations
	CreateAgentSession(ctx context.Context, session *AgentSession) (*AgentSession, error)
	GetAgentSession(ctx context.Context, find *FindAgentSession) (*AgentSession, error)
	ListAgentSessions(ctx context.Context, find *FindAgentSession) ([]*AgentSession, error)
	UpdateAgentSession(ctx context.Context, update *UpdateAgentSession) (*AgentSession, error)
	DeleteAgentSession(ctx context.Context, id string) error

	// Agent message operations
	CreateAgentMessages(ctx context.Context, messages []*AgentMessageRecord) error
	GetAssistantMessageBySourceID(ctx context.Context, sessionID, sourceID string) (*AgentMessageRecord, error)
	GetUserMessageBySourceID(ctx context.Context, sessionID, sourceID string) (*AgentMessageRecord, error)

	// Source file operations
	UpsertAgentSourceFile(ctx context.Context, file *AgentSourceFile) (*AgentSourceFile, error)
	GetAgentSourceFile(ctx context.Context, find *FindAgentSourceFile) (*AgentSourceFile, error)
	ListAgentSourceFiles(ctx context.Context, find *FindAgentSourceFile) ([]*AgentSourceFile, error)
	DeleteAgentSourceFiles(ctx context.Context, tenantID int32, audienceType *string) error

	// Rate limit operations
	GetOrCreateAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) (*AgentRateLimit, error)
	IncrementAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error
	ResetAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error

	// Simulation transcript operations
	CreateAgentSimulationTranscript(ctx context.Context, transcript *AgentSimulationTranscript) (*AgentSimulationTranscript, error)
	GetAgentSimulationTranscript(ctx context.Context, find *FindAgentSimulationTranscript) (*AgentSimulationTranscript, error)
	ListAgentSimulationTranscripts(ctx context.Context, find *FindAgentSimulationTranscript) ([]*AgentSimulationTranscript, int, error)
	DeleteAgentSimulationTranscript(ctx context.Context, id string) error

	// Tenant script operations (SCRIPT.MD)
	UpsertAgentTenantScript(ctx context.Context, script *AgentTenantScript) (*AgentTenantScript, error)
	GetAgentTenantScript(ctx context.Context, find *FindAgentTenantScript) (*AgentTenantScript, error)
	DeleteAgentTenantScript(ctx context.Context, tenantID int32) error

	// Analysis result operations
	CreateAgentAnalysisResult(ctx context.Context, result *AgentAnalysisResult) (*AgentAnalysisResult, error)
	GetAgentAnalysisResult(ctx context.Context, find *FindAgentAnalysisResult) (*AgentAnalysisResult, error)
	ListAgentAnalysisResults(ctx context.Context, find *FindAgentAnalysisResult) ([]*AgentAnalysisResult, int, error)

	// Learning memory operations
	GetOrCreateAgentLearningMemory(ctx context.Context, tenantID int32) (*AgentLearningMemory, error)
	UpdateAgentLearningMemory(ctx context.Context, memory *AgentLearningMemory) (*AgentLearningMemory, error)
	DeleteAgentLearningMemory(ctx context.Context, tenantID int32) error

	// Compliance audit operations
	CreateAgentComplianceAudit(ctx context.Context, audit *AgentComplianceAudit) error
	GetAgentComplianceAudit(ctx context.Context, find *FindAgentComplianceAudit) (*AgentComplianceAudit, error)
	ListAgentComplianceAudits(ctx context.Context, find *FindAgentComplianceAudit) ([]*AgentComplianceAudit, error)

	// Scoring config operations
	GetOrCreateAgentScoringConfig(ctx context.Context, tenantID int32) (*AgentScoringConfig, error)
	UpdateAgentScoringConfig(ctx context.Context, config *AgentScoringConfig) (*AgentScoringConfig, error)

	// Q&A pair operations (for embedding/retrieval testing)
	CreateAgentQAPair(ctx context.Context, pair *AgentQAPair) (*AgentQAPair, error)
	ListAgentQAPairs(ctx context.Context, find *FindAgentQAPair) ([]*AgentQAPair, error)
	UpdateAgentQAPair(ctx context.Context, pair *AgentQAPair, tenantID int32) (*AgentQAPair, error)
	DeleteAgentQAPair(ctx context.Context, id int32, tenantID int32) error
	DeleteAgentQAPairsByTenant(ctx context.Context, tenantID int32) error

	// Transcript operations (chat conversation recording)
	CreateAgentTranscript(ctx context.Context, transcript *AgentTranscript) (*AgentTranscript, error)
	GetAgentTranscript(ctx context.Context, find *FindAgentTranscript) (*AgentTranscript, error)
	ListAgentTranscripts(ctx context.Context, find *FindAgentTranscript) ([]*AgentTranscript, error)
	UpdateAgentTranscript(ctx context.Context, transcript *AgentTranscript) error
	DeleteAgentTranscript(ctx context.Context, id string) error

	// Lead operations
	UpsertAgentLead(ctx context.Context, lead *AgentLead) (*AgentLead, error)
	GetAgentLead(ctx context.Context, find *FindAgentLead) (*AgentLead, error)
	ListAgentLeads(ctx context.Context, find *FindAgentLead) ([]*AgentLead, error)
	UpdateAgentLeadStatus(ctx context.Context, tenantID int32, id string, status string, convertedAt *time.Time) (*AgentLead, error)

	// Observation Log operations (Observational Memory)
	UpsertObservationLog(ctx context.Context, log *ObservationLog) (*ObservationLog, error)
	GetObservationLog(ctx context.Context, sessionID string) (*ObservationLog, error)
	GetObservationLogByResource(ctx context.Context, resourceID string) (*ObservationLog, error)

	// Bridge delivery operations
	SupportsBridgeDelivery() bool
}

// Store methods that delegate to the driver

func (s *Store) CreateAgentTenant(ctx context.Context, tenant *AgentTenant) (*AgentTenant, error) {
	return s.driver.CreateAgentTenant(ctx, tenant)
}

func (s *Store) GetAgentTenant(ctx context.Context, find *FindAgentTenant) (*AgentTenant, error) {
	return s.driver.GetAgentTenant(ctx, find)
}

func (s *Store) ListAgentTenants(ctx context.Context, find *FindAgentTenant) ([]*AgentTenant, error) {
	return s.driver.ListAgentTenants(ctx, find)
}

func (s *Store) UpdateAgentTenant(ctx context.Context, tenant *AgentTenant) (*AgentTenant, error) {
	return s.driver.UpdateAgentTenant(ctx, tenant)
}

func (s *Store) DeleteAgentTenant(ctx context.Context, id int32) error {
	return s.driver.DeleteAgentTenant(ctx, id)
}

func (s *Store) CreateAgentAudience(ctx context.Context, audience *AgentAudience) (*AgentAudience, error) {
	return s.driver.CreateAgentAudience(ctx, audience)
}

func (s *Store) GetAgentAudience(ctx context.Context, find *FindAgentAudience) (*AgentAudience, error) {
	return s.driver.GetAgentAudience(ctx, find)
}

func (s *Store) ListAgentAudiences(ctx context.Context, find *FindAgentAudience) ([]*AgentAudience, error) {
	return s.driver.ListAgentAudiences(ctx, find)
}

func (s *Store) UpdateAgentAudience(ctx context.Context, audience *AgentAudience) (*AgentAudience, error) {
	return s.driver.UpdateAgentAudience(ctx, audience)
}

func (s *Store) DeleteAgentAudience(ctx context.Context, tenantID int32, audienceType string) error {
	return s.driver.DeleteAgentAudience(ctx, tenantID, audienceType)
}

func (s *Store) CreateAgentService(ctx context.Context, service *AgentService) (*AgentService, error) {
	return s.driver.CreateAgentService(ctx, service)
}

func (s *Store) ListAgentServices(ctx context.Context, find *FindAgentService) ([]*AgentService, error) {
	return s.driver.ListAgentServices(ctx, find)
}

func (s *Store) DeleteAgentServices(ctx context.Context, tenantID int32, audienceType string) error {
	return s.driver.DeleteAgentServices(ctx, tenantID, audienceType)
}

func (s *Store) CreateAgentExclusion(ctx context.Context, exclusion *AgentExclusion) (*AgentExclusion, error) {
	return s.driver.CreateAgentExclusion(ctx, exclusion)
}

func (s *Store) ListAgentExclusions(ctx context.Context, find *FindAgentExclusion) ([]*AgentExclusion, error) {
	return s.driver.ListAgentExclusions(ctx, find)
}

func (s *Store) DeleteAgentExclusions(ctx context.Context, tenantID int32, audienceType string) error {
	return s.driver.DeleteAgentExclusions(ctx, tenantID, audienceType)
}

func (s *Store) CreateAgentCoverage(ctx context.Context, coverage *AgentCoverage) (*AgentCoverage, error) {
	return s.driver.CreateAgentCoverage(ctx, coverage)
}

func (s *Store) ListAgentCoverage(ctx context.Context, find *FindAgentCoverage) ([]*AgentCoverage, error) {
	return s.driver.ListAgentCoverage(ctx, find)
}

func (s *Store) DeleteAgentCoverage(ctx context.Context, tenantID int32) error {
	return s.driver.DeleteAgentCoverage(ctx, tenantID)
}

func (s *Store) CreateAgentFAQ(ctx context.Context, faq *AgentFAQ) (*AgentFAQ, error) {
	return s.driver.CreateAgentFAQ(ctx, faq)
}

func (s *Store) ListAgentFAQs(ctx context.Context, find *FindAgentFAQ) ([]*AgentFAQ, error) {
	return s.driver.ListAgentFAQs(ctx, find)
}

func (s *Store) DeleteAgentFAQs(ctx context.Context, tenantID int32, audienceType string) error {
	return s.driver.DeleteAgentFAQs(ctx, tenantID, audienceType)
}

func (s *Store) CreateAgentSafetyProtocol(ctx context.Context, protocol *AgentSafetyProtocol) (*AgentSafetyProtocol, error) {
	return s.driver.CreateAgentSafetyProtocol(ctx, protocol)
}

func (s *Store) ListAgentSafetyProtocols(ctx context.Context, find *FindAgentSafetyProtocol) ([]*AgentSafetyProtocol, error) {
	return s.driver.ListAgentSafetyProtocols(ctx, find)
}

func (s *Store) DeleteAgentSafetyProtocols(ctx context.Context, tenantID int32, audienceType string) error {
	return s.driver.DeleteAgentSafetyProtocols(ctx, tenantID, audienceType)
}

func (s *Store) CreateAgentKBSection(ctx context.Context, section *AgentKBSection) (*AgentKBSection, error) {
	return s.driver.CreateAgentKBSection(ctx, section)
}

func (s *Store) ListAgentKBSections(ctx context.Context, find *FindAgentKBSection) ([]*AgentKBSection, error) {
	return s.driver.ListAgentKBSections(ctx, find)
}

func (s *Store) DeleteAgentKBSections(ctx context.Context, tenantID int32, audienceType string) error {
	return s.driver.DeleteAgentKBSections(ctx, tenantID, audienceType)
}

func (s *Store) CreateAgentIntent(ctx context.Context, intent *AgentIntent) (*AgentIntent, error) {
	return s.driver.CreateAgentIntent(ctx, intent)
}

func (s *Store) ListAgentIntents(ctx context.Context, find *FindAgentIntent) ([]*AgentIntent, error) {
	return s.driver.ListAgentIntents(ctx, find)
}

func (s *Store) DeleteAgentIntents(ctx context.Context, tenantID int32, audienceType *string) error {
	return s.driver.DeleteAgentIntents(ctx, tenantID, audienceType)
}

func (s *Store) CreateAgentRule(ctx context.Context, rule *AgentRule) (*AgentRule, error) {
	return s.driver.CreateAgentRule(ctx, rule)
}

func (s *Store) ListAgentRules(ctx context.Context, find *FindAgentRule) ([]*AgentRule, error) {
	return s.driver.ListAgentRules(ctx, find)
}

func (s *Store) DeleteAgentRules(ctx context.Context, tenantID int32, audienceType string) error {
	return s.driver.DeleteAgentRules(ctx, tenantID, audienceType)
}

func (s *Store) CreateAgentSession(ctx context.Context, session *AgentSession) (*AgentSession, error) {
	return s.driver.CreateAgentSession(ctx, session)
}

func (s *Store) GetAgentSession(ctx context.Context, find *FindAgentSession) (*AgentSession, error) {
	return s.driver.GetAgentSession(ctx, find)
}

func (s *Store) ListAgentSessions(ctx context.Context, find *FindAgentSession) ([]*AgentSession, error) {
	return s.driver.ListAgentSessions(ctx, find)
}

func (s *Store) UpdateAgentSession(ctx context.Context, update *UpdateAgentSession) (*AgentSession, error) {
	return s.driver.UpdateAgentSession(ctx, update)
}

func (s *Store) DeleteAgentSession(ctx context.Context, id string) error {
	return s.driver.DeleteAgentSession(ctx, id)
}

func (s *Store) CreateAgentMessages(ctx context.Context, messages []*AgentMessageRecord) error {
	return s.driver.CreateAgentMessages(ctx, messages)
}

func (s *Store) GetAssistantMessageBySourceID(ctx context.Context, sessionID, sourceID string) (*AgentMessageRecord, error) {
	return s.driver.GetAssistantMessageBySourceID(ctx, sessionID, sourceID)
}

func (s *Store) GetUserMessageBySourceID(ctx context.Context, sessionID, sourceID string) (*AgentMessageRecord, error) {
	return s.driver.GetUserMessageBySourceID(ctx, sessionID, sourceID)
}

func (s *Store) SupportsBridgeDelivery() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := s.driver.ClaimBridgeReplyOutboxByOutboxID(ctx, 0, "", "", time.Time{}, 0)
	return !errors.Is(err, ErrBridgeUnsupportedDatabase)
}

func (s *Store) UpsertAgentSourceFile(ctx context.Context, file *AgentSourceFile) (*AgentSourceFile, error) {
	return s.driver.UpsertAgentSourceFile(ctx, file)
}

func (s *Store) GetAgentSourceFile(ctx context.Context, find *FindAgentSourceFile) (*AgentSourceFile, error) {
	return s.driver.GetAgentSourceFile(ctx, find)
}

func (s *Store) ListAgentSourceFiles(ctx context.Context, find *FindAgentSourceFile) ([]*AgentSourceFile, error) {
	return s.driver.ListAgentSourceFiles(ctx, find)
}

func (s *Store) DeleteAgentSourceFiles(ctx context.Context, tenantID int32, audienceType *string) error {
	return s.driver.DeleteAgentSourceFiles(ctx, tenantID, audienceType)
}

func (s *Store) GetOrCreateAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) (*AgentRateLimit, error) {
	return s.driver.GetOrCreateAgentRateLimit(ctx, tenantID, audienceType, clientIP)
}

func (s *Store) IncrementAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error {
	return s.driver.IncrementAgentRateLimit(ctx, tenantID, audienceType, clientIP)
}

func (s *Store) ResetAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error {
	return s.driver.ResetAgentRateLimit(ctx, tenantID, audienceType, clientIP)
}

func (s *Store) CreateAgentSimulationTranscript(ctx context.Context, transcript *AgentSimulationTranscript) (*AgentSimulationTranscript, error) {
	return s.driver.CreateAgentSimulationTranscript(ctx, transcript)
}

func (s *Store) GetAgentSimulationTranscript(ctx context.Context, find *FindAgentSimulationTranscript) (*AgentSimulationTranscript, error) {
	return s.driver.GetAgentSimulationTranscript(ctx, find)
}

func (s *Store) ListAgentSimulationTranscripts(ctx context.Context, find *FindAgentSimulationTranscript) ([]*AgentSimulationTranscript, int, error) {
	return s.driver.ListAgentSimulationTranscripts(ctx, find)
}

func (s *Store) DeleteAgentSimulationTranscript(ctx context.Context, id string) error {
	return s.driver.DeleteAgentSimulationTranscript(ctx, id)
}

func (s *Store) UpsertAgentTenantScript(ctx context.Context, script *AgentTenantScript) (*AgentTenantScript, error) {
	return s.driver.UpsertAgentTenantScript(ctx, script)
}

func (s *Store) GetAgentTenantScript(ctx context.Context, find *FindAgentTenantScript) (*AgentTenantScript, error) {
	return s.driver.GetAgentTenantScript(ctx, find)
}

func (s *Store) DeleteAgentTenantScript(ctx context.Context, tenantID int32) error {
	return s.driver.DeleteAgentTenantScript(ctx, tenantID)
}

func (s *Store) CreateAgentAnalysisResult(ctx context.Context, result *AgentAnalysisResult) (*AgentAnalysisResult, error) {
	return s.driver.CreateAgentAnalysisResult(ctx, result)
}

func (s *Store) GetAgentAnalysisResult(ctx context.Context, find *FindAgentAnalysisResult) (*AgentAnalysisResult, error) {
	return s.driver.GetAgentAnalysisResult(ctx, find)
}

func (s *Store) ListAgentAnalysisResults(ctx context.Context, find *FindAgentAnalysisResult) ([]*AgentAnalysisResult, int, error) {
	return s.driver.ListAgentAnalysisResults(ctx, find)
}

func (s *Store) GetOrCreateAgentLearningMemory(ctx context.Context, tenantID int32) (*AgentLearningMemory, error) {
	return s.driver.GetOrCreateAgentLearningMemory(ctx, tenantID)
}

func (s *Store) UpdateAgentLearningMemory(ctx context.Context, memory *AgentLearningMemory) (*AgentLearningMemory, error) {
	return s.driver.UpdateAgentLearningMemory(ctx, memory)
}

func (s *Store) DeleteAgentLearningMemory(ctx context.Context, tenantID int32) error {
	return s.driver.DeleteAgentLearningMemory(ctx, tenantID)
}

func (s *Store) CreateAgentComplianceAudit(ctx context.Context, audit *AgentComplianceAudit) error {
	return s.driver.CreateAgentComplianceAudit(ctx, audit)
}

func (s *Store) GetAgentComplianceAudit(ctx context.Context, find *FindAgentComplianceAudit) (*AgentComplianceAudit, error) {
	return s.driver.GetAgentComplianceAudit(ctx, find)
}

func (s *Store) ListAgentComplianceAudits(ctx context.Context, find *FindAgentComplianceAudit) ([]*AgentComplianceAudit, error) {
	return s.driver.ListAgentComplianceAudits(ctx, find)
}

func (s *Store) GetOrCreateAgentScoringConfig(ctx context.Context, tenantID int32) (*AgentScoringConfig, error) {
	return s.driver.GetOrCreateAgentScoringConfig(ctx, tenantID)
}

func (s *Store) UpdateAgentScoringConfig(ctx context.Context, config *AgentScoringConfig) (*AgentScoringConfig, error) {
	return s.driver.UpdateAgentScoringConfig(ctx, config)
}

func (s *Store) CreateAgentQAPair(ctx context.Context, pair *AgentQAPair) (*AgentQAPair, error) {
	return s.driver.CreateAgentQAPair(ctx, pair)
}

func (s *Store) ListAgentQAPairs(ctx context.Context, find *FindAgentQAPair) ([]*AgentQAPair, error) {
	return s.driver.ListAgentQAPairs(ctx, find)
}

func (s *Store) UpdateAgentQAPair(ctx context.Context, pair *AgentQAPair, tenantID int32) (*AgentQAPair, error) {
	return s.driver.UpdateAgentQAPair(ctx, pair, tenantID)
}

func (s *Store) DeleteAgentQAPair(ctx context.Context, id int32, tenantID int32) error {
	return s.driver.DeleteAgentQAPair(ctx, id, tenantID)
}

func (s *Store) DeleteAgentQAPairsByTenant(ctx context.Context, tenantID int32) error {
	return s.driver.DeleteAgentQAPairsByTenant(ctx, tenantID)
}

func (s *Store) CreateAgentTranscript(ctx context.Context, transcript *AgentTranscript) (*AgentTranscript, error) {
	return s.driver.CreateAgentTranscript(ctx, transcript)
}

func (s *Store) GetAgentTranscript(ctx context.Context, find *FindAgentTranscript) (*AgentTranscript, error) {
	return s.driver.GetAgentTranscript(ctx, find)
}

func (s *Store) ListAgentTranscripts(ctx context.Context, find *FindAgentTranscript) ([]*AgentTranscript, error) {
	return s.driver.ListAgentTranscripts(ctx, find)
}

func (s *Store) UpdateAgentTranscript(ctx context.Context, transcript *AgentTranscript) error {
	return s.driver.UpdateAgentTranscript(ctx, transcript)
}

func (s *Store) DeleteAgentTranscript(ctx context.Context, id string) error {
	return s.driver.DeleteAgentTranscript(ctx, id)
}

func (s *Store) UpsertAgentLead(ctx context.Context, lead *AgentLead) (*AgentLead, error) {
	return s.driver.UpsertAgentLead(ctx, lead)
}

func (s *Store) GetAgentLead(ctx context.Context, find *FindAgentLead) (*AgentLead, error) {
	return s.driver.GetAgentLead(ctx, find)
}

func (s *Store) ListAgentLeads(ctx context.Context, find *FindAgentLead) ([]*AgentLead, error) {
	return s.driver.ListAgentLeads(ctx, find)
}

func (s *Store) UpdateAgentLeadStatus(ctx context.Context, tenantID int32, id string, status string, convertedAt *time.Time) (*AgentLead, error) {
	return s.driver.UpdateAgentLeadStatus(ctx, tenantID, id, status, convertedAt)
}

// ReindexCheckpoint tracks reindex progress for resume-from-error support.
type ReindexCheckpoint struct {
	ID              int32
	TenantID        int32
	Audience        string
	TotalChunks     int32
	ProcessedChunks int32
	CurrentBatch    int32
	TotalBatches    int32
	BatchSize       int32
	Status          string // "in_progress", "completed", "failed"
	ErrorMessage    string
	LastMessage     string // Detailed progress message (e.g. "Processing batch 12/29...")
	ErrorBatch      *int32
	StartedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
}

// FindReindexCheckpoint contains filters for finding reindex checkpoints.
type FindReindexCheckpoint struct {
	TenantID *int32
	Audience *string
	Status   *string
}

func (s *Store) UpsertReindexCheckpoint(ctx context.Context, checkpoint *ReindexCheckpoint) (*ReindexCheckpoint, error) {
	return s.driver.UpsertReindexCheckpoint(ctx, checkpoint)
}

func (s *Store) GetReindexCheckpoint(ctx context.Context, find *FindReindexCheckpoint) (*ReindexCheckpoint, error) {
	return s.driver.GetReindexCheckpoint(ctx, find)
}

func (s *Store) DeleteReindexCheckpoint(ctx context.Context, tenantID int32, audience string) error {
	return s.driver.DeleteReindexCheckpoint(ctx, tenantID, audience)
}

func (s *Store) UpsertObservationLog(ctx context.Context, log *ObservationLog) (*ObservationLog, error) {
	return s.driver.UpsertObservationLog(ctx, log)
}

func (s *Store) GetObservationLog(ctx context.Context, sessionID string) (*ObservationLog, error) {
	return s.driver.GetObservationLog(ctx, sessionID)
}

func (s *Store) GetObservationLogByResource(ctx context.Context, resourceID string) (*ObservationLog, error) {
	return s.driver.GetObservationLogByResource(ctx, resourceID)
}
