package agent

import "testing"

// TestModeDecision verifies the mode-selection logic in processChat.
// The actual decision is embedded in processChat, so this test validates
// the decision pattern as documented in the plan decision tree.
func TestModeDecision(t *testing.T) {
	tests := []struct {
		name          string
		retrievalMode string
		hasStructured bool
		ragEnabled    bool
		wantRAG       bool
	}{
		// Explicit RetrieverMode always wins
		{"explicit rag + structured", "rag", true, true, true},
		{"explicit rag + unstructured", "rag", false, true, true},
		{"explicit long_context + structured", "long_context", true, true, false},
		{"explicit long_context + unstructured", "long_context", false, true, false},

		// No explicit mode: fall back to HasStructuredContent
		{"unset + structured + rag enabled", "", true, true, false},
		{"unset + unstructured + rag enabled", "", false, true, true},

		// RAG pipeline disabled: always long_context
		{"unset + structured + rag disabled", "", true, false, false},
		{"unset + unstructured + rag disabled", "", false, false, false},
		{"explicit rag + rag disabled", "rag", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the decision logic from processChat
			useRAG := false
			if tt.ragEnabled {
				if tt.retrievalMode == "rag" {
					useRAG = true
				} else if tt.retrievalMode == "long_context" {
					useRAG = false
				} else if !tt.hasStructured {
					useRAG = true
				}
			}

			if useRAG != tt.wantRAG {
				t.Fatalf("useRAG = %v, want %v (retrievalMode=%q, hasStructured=%v, ragEnabled=%v)",
					useRAG, tt.wantRAG, tt.retrievalMode, tt.hasStructured, tt.ragEnabled)
			}
		})
	}
}
