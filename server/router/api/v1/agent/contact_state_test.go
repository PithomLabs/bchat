package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestGetContactState_BasicAndNegation(t *testing.T) {
	// 1. Basic extraction (Name + Email)
	messages := []store.AgentMessage{
		{Role: "user", Content: "Hi my name is Ada Lovelace and my email is ada@example.org"},
	}
	state := getContactState(&store.AgentSession{Messages: messages}, "")
	require.True(t, state.HasName)
	require.True(t, state.HasEmailOrPhone)
	require.True(t, state.IsComplete)
	require.Equal(t, "Ada Lovelace", state.Name)
	require.Equal(t, "ada@example.org", state.Email)

	// 2. Negation/Retraction of email (User role)
	messages = []store.AgentMessage{
		{Role: "user", Content: "Hi my name is Ada Lovelace and my email is ada@example.org"},
		{Role: "user", Content: "Actually don't email me anymore"},
	}
	state = getContactState(&store.AgentSession{Messages: messages}, "")
	require.True(t, state.HasName)
	require.False(t, state.HasEmailOrPhone) // Email got cleared by retraction
	require.False(t, state.IsComplete)
	require.Equal(t, "Ada Lovelace", state.Name)
	require.Equal(t, "", state.Email)

	// 3. Negation of phone (User role)
	messages = []store.AgentMessage{
		{Role: "user", Content: "This is Grace Hopper. Phone is 415-555-1212"},
		{Role: "user", Content: "forget my phone number"},
	}
	state = getContactState(&store.AgentSession{Messages: messages}, "")
	require.True(t, state.HasName)
	require.False(t, state.HasEmailOrPhone) // Phone cleared
	require.Equal(t, "Grace Hopper", state.Name)
	require.Equal(t, "", state.Phone)

	// 4. Assistant retraction message must be ignored (No retraction effect)
	messages = []store.AgentMessage{
		{Role: "user", Content: "Hi my name is Ada Lovelace and my email is ada@example.org"},
		{Role: "assistant", Content: "Got it, I won't contact you at ada@example.org anymore"},
	}
	state = getContactState(&store.AgentSession{Messages: messages}, "")
	require.True(t, state.HasName)
	require.True(t, state.HasEmailOrPhone) // Email stays since retraction was by assistant
	require.True(t, state.IsComplete)
	require.Equal(t, "ada@example.org", state.Email)

	// 5. Retraction in same message as correction shouldn't clear
	messages = []store.AgentMessage{
		{Role: "user", Content: "Forget my number, it is actually 415-555-1212"},
	}
	state = getContactState(&store.AgentSession{Messages: messages}, "")
	require.True(t, state.HasEmailOrPhone)
	require.Equal(t, "415-555-1212", state.Phone)
}

func TestBuildContactInstruction(t *testing.T) {
	// Helper to check contains
	contains := func(t *testing.T, s, substr string) {
		require.Contains(t, s, substr)
	}
	notContains := func(t *testing.T, s, substr string) {
		require.NotContains(t, s, substr)
	}

	// Permutation 1: Complete state, Fallback intent
	state := ContactState{
		Name:            "Ada Lovelace",
		Email:           "ada@example.org",
		HasName:         true,
		HasEmailOrPhone: true,
		IsComplete:      true,
	}
	class := &Classification{PrimaryIntent: "out_of_coverage"}
	instructions := buildContactInstruction(state, class, "555-000-1111", true)

	// Section 0 should be present
	contains(t, instructions.Section0Addition, "=== CUSTOMER INFO ALREADY PROVIDED")
	contains(t, instructions.Section0Addition, "Ada Lovelace")

	// Rule 1 should say to acknowledge info and NOT ask again
	contains(t, instructions.Rule1Text, "Since you already have their contact info, do NOT ask for it again")
	notContains(t, instructions.Rule1Text, "offer to collect")

	// Rule 8 should tell agent not to collect again
	contains(t, instructions.Rule8Text, "Do NOT ask to collect contact details because the customer has already provided")

	// RAG fallback should not ask again
	contains(t, instructions.RAGFallbackText, "acknowledge you have their contact information so a team member can follow up. Do not ask for it again.")

	// Permutation 2: Complete state, In-scope intent (should not have fallback prompts)
	classInScope := &Classification{PrimaryIntent: "pricing"}
	instructionsInScope := buildContactInstruction(state, classInScope, "555-000-1111", true)
	contains(t, instructionsInScope.Rule1Text, "Since they have already provided their contact information, do NOT ask for it again")
	notContains(t, instructionsInScope.Rule1Text, "simply state that our team will follow up") // Fallback-specific phrasing is absent

	// Permutation 3: Partial state (Name only), Fallback intent
	stateNameOnly := ContactState{
		Name:            "Ada Lovelace",
		HasName:         true,
		HasEmailOrPhone: false,
		IsComplete:      false,
	}
	instructionsPartialName := buildContactInstruction(stateNameOnly, class, "555-000-1111", true)
	contains(t, instructionsPartialName.Rule1Text, "ask only for their email or phone for follow-up")
	contains(t, instructionsPartialName.Rule8Text, "ask only for their email or phone. Do not ask for their name again")
	contains(t, instructionsPartialName.RAGFallbackText, "ask only for their email or phone for follow-up. Do not ask for their name again")

	// Permutation 4: Partial state (Email only), Fallback intent
	stateEmailOnly := ContactState{
		Email:           "ada@example.org",
		HasName:         false,
		HasEmailOrPhone: true,
		IsComplete:      false,
	}
	instructionsPartialEmail := buildContactInstruction(stateEmailOnly, class, "555-000-1111", true)
	contains(t, instructionsPartialEmail.Rule1Text, "ask only for their name for follow-up")
	contains(t, instructionsPartialEmail.Rule8Text, "ask only for their name. Do not ask for their email or phone again")
	contains(t, instructionsPartialEmail.RAGFallbackText, "ask only for their name for follow-up. Do not ask for their email or phone again")

	// Permutation 5: No contact info, Fallback intent
	stateEmpty := ContactState{}
	instructionsEmpty := buildContactInstruction(stateEmpty, class, "555-000-1111", true)
	require.Empty(t, instructionsEmpty.Section0Addition)
	contains(t, instructionsEmpty.Rule1Text, "offer to collect their name plus email or phone for follow-up")
	contains(t, instructionsEmpty.Rule8Text, "ask for their name and either email or phone")
	contains(t, instructionsEmpty.RAGFallbackText, "offer to collect the customer's name plus email or phone for follow-up")

	// Permutation 6: Contact collection disabled should suppress all follow-up capture prompts.
	instructionsDisabled := buildContactInstruction(state, class, "555-000-1111", false)
	require.Empty(t, instructionsDisabled.Section0Addition)
	contains(t, instructionsDisabled.Rule1Text, "Do not ask for contact information for follow-up")
	contains(t, instructionsDisabled.Rule8Text, "Do NOT ask to collect customer contact details")
	contains(t, instructionsDisabled.RAGFallbackText, "without asking for customer contact information")
	notContains(t, instructionsDisabled.Rule1Text, "offer to collect")
	notContains(t, instructionsDisabled.RAGFallbackText, "team member can follow up")
}
