package agent

import (
	"testing"
	"time"

	"github.com/PithomLabs/memstate"
	"github.com/usememos/memos/store"
)

// ============================================================================
// Supersession acceptance tests
// These MUST pass before MEMSTATE_ENABLED=true in any environment.
// ============================================================================

func countCurrent(facts []*memstate.Fact) int {
	n := 0
	for _, f := range facts {
		if f.Current() {
			n++
		}
	}
	return n
}

func currentTexts(facts []*memstate.Fact) []string {
	var texts []string
	for _, f := range facts {
		if f.Current() {
			texts = append(texts, f.Text)
		}
	}
	return texts
}

func TestSupersessionSameField(t *testing.T) {
	mem := store.NewSafeMemory()
	mem.Add("Customer name is John")
	mem.Add("Customer name is Jonathan")
	facts := mem.Facts(false)
	if n := countCurrent(facts); n != 1 {
		t.Fatalf("expected 1 current fact, got %d (all: %v)", n, currentTexts(facts))
	}
	if facts[len(facts)-1].Text != "Customer name is Jonathan" {
		t.Errorf("expected current fact %q, got %q", "Customer name is Jonathan", facts[len(facts)-1].Text)
	}
}

func TestSupersessionSameFieldPhone(t *testing.T) {
	mem := store.NewSafeMemory()
	mem.Add("Customer phone is (555) 123-4567")
	mem.Add("Customer phone is (555) 567-8901")
	facts := mem.Facts(false)
	if n := countCurrent(facts); n != 1 {
		t.Fatalf("expected 1 current fact, got %d (all: %v)", n, currentTexts(facts))
	}
	if facts[len(facts)-1].Text != "Customer phone is (555) 567-8901" {
		t.Errorf("expected current fact %q, got %q", "Customer phone is (555) 567-8901", facts[len(facts)-1].Text)
	}
}

func TestSupersessionCrossTopic(t *testing.T) {
	mem := store.NewSafeMemory()
	mem.Add("Customer name is John Smith")
	mem.Add("Customer location is Rome")
	facts := mem.Facts(false)
	if n := countCurrent(facts); n != 2 {
		t.Fatalf("expected 2 current facts (no supersession), got %d (all: %v)", n, currentTexts(facts))
	}
}

func TestSupersessionDifferentTopic(t *testing.T) {
	mem := store.NewSafeMemory()
	mem.Add("Customer name is John")
	mem.Add("I need help with billing")
	facts := mem.Facts(false)
	if n := countCurrent(facts); n != 2 {
		t.Fatalf("expected 2 current facts (no supersession), got %d (all: %v)", n, currentTexts(facts))
	}
}

// ============================================================================
// Facts-nil-by-default test
// ============================================================================

func TestFactsNilByDefault(t *testing.T) {
	if isMemstateEnabled() {
		t.Skip("MEMSTATE_ENABLED is true; testing nil guard is not applicable")
	}
	orig := isMemstateEnabled
	isMemstateEnabled = func() bool { return false }
	defer func() { isMemstateEnabled = orig }()

	ms := NewMemorySessionStore(time.Minute)
	session := ms.GetOrCreate(1, "test-nil")
	if session.Facts != nil {
		t.Error("expected Facts to be nil when MEMSTATE_ENABLED is false")
	}
}

func TestFactsInitializedWhenEnabled(t *testing.T) {
	orig := isMemstateEnabled
	isMemstateEnabled = func() bool { return true }
	defer func() { isMemstateEnabled = orig }()

	mem := store.NewSafeMemory()
	if mem == nil {
		t.Fatal("NewSafeMemory returned nil")
	}
	mem.Add("Customer name is Alice")
	facts := mem.Facts(false)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Text != "Customer name is Alice" {
		t.Errorf("expected %q, got %q", "Customer name is Alice", facts[0].Text)
	}
}

// ============================================================================
// extractLatest* function tests
// ============================================================================

func TestExtractLatestName(t *testing.T) {
	msgs := []store.AgentMessage{
		{Role: "user", Content: "Hi there"},
		{Role: "user", Content: "My name is John"},
		{Role: "assistant", Content: "Nice to meet you John"},
		{Role: "user", Content: "Actually call me Jonathan"},
	}
	got := extractLatestName(msgs)
	if got != "Jonathan" {
		t.Errorf("expected %q, got %q", "Jonathan", got)
	}
}

func TestExtractLatestNameSkipsAssistant(t *testing.T) {
	msgs := []store.AgentMessage{
		{Role: "assistant", Content: "Is your name John?"},
		{Role: "user", Content: "yes"},
	}
	got := extractLatestName(msgs)
	if got != "" {
		t.Errorf("expected empty (no explicit name), got %q", got)
	}
}

func TestExtractLatestPhone(t *testing.T) {
	msgs := []store.AgentMessage{
		{Role: "user", Content: "My number is 555-123-4567"},
		{Role: "user", Content: "Wait, actually it's 555-567-8901"},
	}
	got := extractLatestPhone(msgs, "")
	if got == "" {
		t.Fatal("expected a phone number, got empty")
	}
	if got != "555-567-8901" {
		t.Errorf("expected %q, got %q", "555-567-8901", got)
	}
}

func TestExtractLatestPhoneExcludesTenant(t *testing.T) {
	msgs := []store.AgentMessage{
		{Role: "user", Content: "Your number is 555-999-0000"},
	}
	got := extractLatestPhone(msgs, "555-999-0000")
	if got != "" {
		t.Errorf("expected empty (tenant phone excluded), got %q", got)
	}
}

func TestExtractLatestPhoneCorrection(t *testing.T) {
	msgs := []store.AgentMessage{
		{Role: "user", Content: "My number is 555-123-4567"},
		{Role: "user", Content: "Actually, correct my number to 555-567-8901"},
	}
	got := extractLatestPhone(msgs, "")
	if got != "555-567-8901" {
		t.Errorf("expected %q, got %q", "555-567-8901", got)
	}
}

func TestExtractLatestAddress(t *testing.T) {
	msgs := []store.AgentMessage{
		{Role: "user", Content: "I live at 123 Main St, Springfield, IL 62701"},
		{Role: "user", Content: "Actually moved to 456 Oak Ave, Chicago, IL 60601"},
	}
	got := extractLatestAddress(msgs)
	if got == "" {
		t.Fatal("expected an address, got empty")
	}
	if got != "456 Oak Ave, Chicago, IL 60601" {
		t.Errorf("expected %q, got %q", "456 Oak Ave, Chicago, IL 60601", got)
	}
}
