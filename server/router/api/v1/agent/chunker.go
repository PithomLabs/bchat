package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

// DocumentChunk represents a single chunk of content for vector indexing.
type DocumentChunk struct {
	// Identity
	ID           string // Unique identifier: tenantID:audience:type:code
	TenantID     int32
	AudienceType string

	// Content
	ContentType string // service, faq, exclusion, coverage, rule, safety, kb_section
	Title       string
	Content     string
	Code        string // service code, rule code, etc.

	// Metadata
	IsEmergency   bool
	IsActive      bool
	Priority      int32
	SourceVersion int32

	// Vector (populated after embedding)
	Embedding []float32

	// Timestamps
	IndexedAt time.Time
}

// ChunkID generates a unique ID for a chunk.
func ChunkID(tenantID int32, audience, contentType, code string) string {
	return fmt.Sprintf("%d:%s:%s:%s", tenantID, audience, contentType, code)
}

// ChunkedDocument holds all chunks extracted from a tenant's documents.
type ChunkedDocument struct {
	TenantID     int32
	AudienceType string
	Chunks       []DocumentChunk
	SourceHash   string // Combined hash for change detection
}

// Chunker handles document chunking for vector indexing.
type Chunker struct {
	// Configuration
	maxChunkSize int // Maximum chunk size in characters (for future use)
}

// NewChunker creates a new document chunker.
func NewChunker() *Chunker {
	return &Chunker{
		maxChunkSize: 2000, // Default max chunk size
	}
}

// ChunkKBContent extracts chunks from parsed KB content.
func (c *Chunker) ChunkKBContent(
	kb *ParsedKB,
	tenantID int32,
	audience string,
	sourceVersion int32,
) []DocumentChunk {
	var chunks []DocumentChunk
	now := time.Now()

	// Services
	for _, svc := range kb.Services {
		content := buildServiceContent(svc)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "service", svc.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "service",
			Title:         svc.Name,
			Content:       content,
			Code:          svc.Code,
			IsEmergency:   svc.IsEmergency,
			IsActive:      svc.IsActive,
			Priority:      0, // Services don't have priority in store
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// FAQs
	for i, faq := range kb.FAQs {
		code := faq.Code
		if code == "" {
			code = fmt.Sprintf("faq_%d", i)
		}
		content := buildFAQContent(faq)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "faq", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "faq",
			Title:         faq.Question,
			Content:       content,
			Code:          code,
			IsActive:      faq.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Exclusions
	for _, exc := range kb.Exclusions {
		content := buildExclusionContent(exc)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "exclusion", exc.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "exclusion",
			Title:         exc.Name,
			Content:       content,
			Code:          exc.Code,
			IsActive:      exc.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Coverage areas
	for i, cov := range kb.Coverage {
		code := fmt.Sprintf("coverage_%d", i)
		content := buildCoverageContent(cov)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "coverage", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "coverage",
			Title:         cov.AreaName,
			Content:       content,
			Code:          code,
			IsActive:      cov.IsIncluded, // IsIncluded means it's an active coverage area
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Safety protocols
	for i, safety := range kb.Safety {
		code := safety.Code
		if code == "" {
			code = fmt.Sprintf("safety_%d", i)
		}
		content := buildSafetyContent(safety)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "safety", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "safety",
			Title:         safety.Name,
			Content:       content,
			Code:          code,
			IsEmergency:   true, // Safety protocols are always high priority
			IsActive:      safety.IsActive,
			Priority:      100, // High priority
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// KB Sections (general knowledge sections)
	for i, section := range kb.Sections {
		code := section.Code
		if code == "" {
			code = fmt.Sprintf("kb_section_%d", i)
		}
		content := buildKBSectionContent(section)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "kb_section", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "kb_section",
			Title:         section.Title,
			Content:       content,
			Code:          code,
			IsActive:      section.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	return chunks
}

// ChunkPolicyContent extracts chunks from parsed Policy content.
func (c *Chunker) ChunkPolicyContent(
	policy *ParsedPolicy,
	tenantID int32,
	audience string,
	sourceVersion int32,
) []DocumentChunk {
	var chunks []DocumentChunk
	now := time.Now()

	// Rules
	for _, rule := range policy.Rules {
		content := buildRuleContent(rule)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "rule", rule.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "rule",
			Title:         rule.Name,
			Content:       content,
			Code:          rule.Code,
			IsActive:      rule.IsActive,
			Priority:      int32(rule.Priority),
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Intents (for context matching)
	for _, intent := range policy.Intents {
		code := fmt.Sprintf("intent_%s", intent.Code)
		content := buildIntentContent(intent)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "intent", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "intent",
			Title:         intent.Name,
			Content:       content,
			Code:          code,
			IsActive:      intent.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	return chunks
}

// ChunkFromStoreTypes creates chunks from store types (for existing data).
func (c *Chunker) ChunkFromStoreTypes(
	services []*store.AgentService,
	exclusions []*store.AgentExclusion,
	faqs []*store.AgentFAQ,
	coverage []*store.AgentCoverage,
	safety []*store.AgentSafetyProtocol,
	rules []*store.AgentRule,
	tenantID int32,
	audience string,
	sourceVersion int32,
) []DocumentChunk {
	var chunks []DocumentChunk
	now := time.Now()

	// Services
	for _, svc := range services {
		content := buildServiceContent(svc)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "service", svc.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "service",
			Title:         svc.Name,
			Content:       content,
			Code:          svc.Code,
			IsEmergency:   svc.IsEmergency,
			IsActive:      svc.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Exclusions
	for _, exc := range exclusions {
		content := buildExclusionContent(exc)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "exclusion", exc.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "exclusion",
			Title:         exc.Name,
			Content:       content,
			Code:          exc.Code,
			IsActive:      exc.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// FAQs
	for i, faq := range faqs {
		code := faq.Code
		if code == "" {
			code = fmt.Sprintf("faq_%d", i)
		}
		content := buildFAQContent(faq)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "faq", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "faq",
			Title:         faq.Question,
			Content:       content,
			Code:          code,
			IsActive:      faq.IsActive,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Coverage
	for i, cov := range coverage {
		code := fmt.Sprintf("coverage_%d", i)
		content := buildCoverageContent(cov)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "coverage", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "coverage",
			Title:         cov.AreaName,
			Content:       content,
			Code:          code,
			IsActive:      cov.IsIncluded,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Safety protocols
	for i, sp := range safety {
		code := sp.Code
		if code == "" {
			code = fmt.Sprintf("safety_%d", i)
		}
		content := buildSafetyContent(sp)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "safety", code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "safety",
			Title:         sp.Name,
			Content:       content,
			Code:          code,
			IsEmergency:   true,
			IsActive:      sp.IsActive,
			Priority:      100,
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	// Rules
	for _, rule := range rules {
		content := buildRuleContent(rule)
		chunks = append(chunks, DocumentChunk{
			ID:            ChunkID(tenantID, audience, "rule", rule.Code),
			TenantID:      tenantID,
			AudienceType:  audience,
			ContentType:   "rule",
			Title:         rule.Name,
			Content:       content,
			Code:          rule.Code,
			IsActive:      rule.IsActive,
			Priority:      int32(rule.Priority),
			SourceVersion: sourceVersion,
			IndexedAt:     now,
		})
	}

	return chunks
}

// GetTextsForEmbedding extracts text content from chunks for embedding.
func GetTextsForEmbedding(chunks []DocumentChunk) []string {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		// Combine title and content for better semantic matching
		texts[i] = fmt.Sprintf("%s: %s", chunk.Title, chunk.Content)
	}
	return texts
}

// ============================================================================
// CONTENT BUILDERS (from store types)
// ============================================================================

func buildServiceContent(svc *store.AgentService) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", svc.Name, svc.Description))
	if svc.ResponseTime != "" {
		parts = append(parts, fmt.Sprintf("Response time: %s", svc.ResponseTime))
	}
	if svc.IsEmergency {
		parts = append(parts, "This is an emergency service.")
	}
	return strings.Join(parts, ". ")
}

func buildFAQContent(faq *store.AgentFAQ) string {
	return fmt.Sprintf("Question: %s\nAnswer: %s", faq.Question, faq.Answer)
}

func buildExclusionContent(exc *store.AgentExclusion) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", exc.Name, exc.Description))
	if exc.ExceptionRule != "" {
		parts = append(parts, fmt.Sprintf("Exception: %s", exc.ExceptionRule))
	}
	if exc.Referral != "" {
		parts = append(parts, fmt.Sprintf("Referral: %s", exc.Referral))
	}
	return strings.Join(parts, ". ")
}

func buildCoverageContent(cov *store.AgentCoverage) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s (%s)", cov.AreaName, cov.AreaType))
	if cov.StateCode != "" {
		parts = append(parts, fmt.Sprintf("State: %s", cov.StateCode))
	}
	if cov.IsIncluded {
		parts = append(parts, "This area is covered.")
	} else {
		parts = append(parts, "This area is NOT covered.")
	}
	return strings.Join(parts, ". ")
}

func buildSafetyContent(safety *store.AgentSafetyProtocol) string {
	var parts []string
	parts = append(parts, safety.Name)
	if len(safety.TriggerIntents) > 0 {
		parts = append(parts, fmt.Sprintf("Triggers: %s", strings.Join(safety.TriggerIntents, ", ")))
	}
	if len(safety.Instructions) > 0 {
		parts = append(parts, fmt.Sprintf("Instructions: %s", strings.Join(safety.Instructions, "; ")))
	}
	return strings.Join(parts, ". ")
}

func buildKBSectionContent(section *store.AgentKBSection) string {
	return fmt.Sprintf("%s: %s", section.Title, section.Content)
}

func buildRuleContent(rule *store.AgentRule) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", rule.Name, rule.Description))
	if rule.AppliesTo != "" {
		parts = append(parts, fmt.Sprintf("Applies to: %s", rule.AppliesTo))
	}
	return strings.Join(parts, ". ")
}

func buildIntentContent(intent *store.AgentIntent) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", intent.Name, intent.Description))
	if len(intent.Examples) > 0 {
		parts = append(parts, fmt.Sprintf("Examples: %s", strings.Join(intent.Examples, "; ")))
	}
	if intent.Category != "" {
		parts = append(parts, fmt.Sprintf("Category: %s", intent.Category))
	}
	return strings.Join(parts, ". ")
}
