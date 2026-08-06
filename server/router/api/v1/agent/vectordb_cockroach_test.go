//go:build cockroach

package agent

import (
	"strings"
	"testing"
)

func TestBuildCockroachSearchQueryEmptyContentTypes(t *testing.T) {
	query := SearchQuery{TenantID: 7, TopK: 5, MinScore: 0.25}
	sqlQ, args, err := buildCockroachSearchQuery(query, "[0.1, 0.2]")
	if err != nil {
		t.Fatalf("buildCockroachSearchQuery returned error: %v", err)
	}

	if strings.Contains(sqlQ, "content_type IN") {
		t.Fatalf("empty ContentTypes must not emit a content_type predicate; got SQL:\n%s", sqlQ)
	}
	if !strings.Contains(sqlQ, "WHERE tenant_id = $2") {
		t.Fatalf("tenant filter missing from SQL:\n%s", sqlQ)
	}
	if !strings.Contains(sqlQ, "ORDER BY embedding <=> $1::VECTOR") {
		t.Fatalf("similarity ordering missing from SQL:\n%s", sqlQ)
	}
	if !strings.Contains(sqlQ, "LIMIT $3") {
		t.Fatalf("LIMIT missing from SQL:\n%s", sqlQ)
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}
	if args[1] != int32(7) {
		t.Fatalf("tenant arg = %v, want 7", args[1])
	}
	if args[2] != 5 {
		t.Fatalf("TopK arg = %v, want 5", args[2])
	}
	if args[3] != 0.25 {
		t.Fatalf("MinScore arg = %v, want 0.25", args[3])
	}
}

func TestBuildCockroachSearchQueryContentTypesNormalized(t *testing.T) {
	query := SearchQuery{TenantID: 1, TopK: 3, MinScore: 0.25, ContentTypes: []string{"kb"}}
	sqlQ, _, err := buildCockroachSearchQuery(query, "[0.1]")
	if err != nil {
		t.Fatalf("buildCockroachSearchQuery returned error: %v", err)
	}

	if !strings.Contains(sqlQ, "content_type IN ('kb', 'kb_section')") {
		t.Fatalf("bare 'kb' should expand to both variants; got SQL:\n%s", sqlQ)
	}
}

func TestBuildCockroachSearchQueryContentTypesDeduplicated(t *testing.T) {
	query := SearchQuery{TenantID: 1, TopK: 5, MinScore: 0, ContentTypes: []string{"kb", "kb_section"}}
	sqlQ, _, err := buildCockroachSearchQuery(query, "[0.1]")
	if err != nil {
		t.Fatalf("buildCockroachSearchQuery returned error: %v", err)
	}

	// "kb" expands to kb + kb_section; the explicit kb_section must be deduped,
	// leaving exactly two quoted values.
	if !strings.Contains(sqlQ, "content_type IN ('kb', 'kb_section')") {
		t.Fatalf("expected deduplicated IN list; got SQL:\n%s", sqlQ)
	}
	if strings.Count(sqlQ, "kb_section')") != 1 {
		t.Fatalf("kb_section should appear exactly once; got SQL:\n%s", sqlQ)
	}
}

func TestBuildCockroachSearchQueryRejectsInvalidContentType(t *testing.T) {
	query := SearchQuery{TenantID: 1, TopK: 5, ContentTypes: []string{"kb'; DROP TABLE agent_vectors; --"}}
	_, _, err := buildCockroachSearchQuery(query, "[0.1]")
	if err == nil {
		t.Fatalf("expected error for injection payload in ContentTypes, got nil")
	}
	if strings.Contains(err.Error(), "DROP TABLE") {
		t.Fatalf("error must not echo the raw payload: %q", err.Error())
	}
}

func TestBuildCockroachSearchQueryTopKDefaults(t *testing.T) {
	query := SearchQuery{TenantID: 1, TopK: 0, MinScore: 0.25}
	_, args, err := buildCockroachSearchQuery(query, "[0.1]")
	if err != nil {
		t.Fatalf("buildCockroachSearchQuery returned error: %v", err)
	}
	if args[2] != 10 {
		t.Fatalf("TopK arg = %v, want default 10", args[2])
	}
}

func TestBuildCockroachSearchQueryMultipleContentTypes(t *testing.T) {
	query := SearchQuery{TenantID: 1, TopK: 5, ContentTypes: []string{"policy", "bug"}}
	sqlQ, _, err := buildCockroachSearchQuery(query, "[0.1]")
	if err != nil {
		t.Fatalf("buildCockroachSearchQuery returned error: %v", err)
	}

	if !strings.Contains(sqlQ, "content_type IN ('policy', 'policy_section', 'bug', 'bug_section')") {
		t.Fatalf("expected all variants expanded; got SQL:\n%s", sqlQ)
	}
}
