package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScriptWithSkills(t *testing.T) {
	content := `## Opening
- Greet the customer warmly

<!-- @trigger: start, type: "chat" -->
## Workflow Opening
Adopt persona and process the conversation.

<!-- @skill: classify_intent, handler: "builtin:classify_intent", timeout: "30s", max_retries: 3 -->
## Classify Intent
Determine what the customer wants.

<!-- @skill: search_kb, handler: "builtin:search_kb", depends_on: "classify_intent", timeout: "10s" -->
## Search Knowledge Base
Search for relevant solutions.

<!-- @skill: create_ticket, handler: "builtin:create_ticket", depends_on: "search_kb", condition: "search_kb.found == false", timeout: "15s" -->
## Create Support Ticket
If no solution found, create a ticket.

<!-- @skill: respond, handler: "llm:respond", depends_on: "classify_intent, search_kb, create_ticket" -->
## Respond to Customer
Provide a helpful response.

<!-- @signal: stop, condition: "create_ticket.ticket_id != ''", emit_event: "pipeline_completed" -->
## Workflow Completion
Stop once ticket is logged.
`

	parser := NewParser()
	parsed, graph, err := parser.ParseScriptWithSkills(content)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.NotNil(t, graph)
	assert.True(t, graph.HasSkills)

	// Check trigger
	assert.NotNil(t, graph.Trigger)
	assert.Equal(t, "chat", graph.Trigger.Type)

	// Check stop signal
	assert.NotNil(t, graph.Stop)
	assert.Equal(t, "create_ticket.ticket_id != ''", graph.Stop.Condition)
	assert.Equal(t, "pipeline_completed", graph.Stop.EmitEvent)

	// Check skills
	assert.Len(t, graph.Nodes, 4)

	classify := graph.Nodes["classify_intent"]
	require.NotNil(t, classify)
	assert.Equal(t, "builtin:classify_intent", classify.Handler)
	assert.Equal(t, "30s", classify.Timeout)
	assert.Equal(t, 3, classify.MaxRetries)
	assert.Nil(t, classify.DependsOn) // no depends_on = entry point

	search := graph.Nodes["search_kb"]
	require.NotNil(t, search)
	assert.Equal(t, "builtin:search_kb", search.Handler)
	assert.Equal(t, []string{"classify_intent"}, search.DependsOn)

	ticket := graph.Nodes["create_ticket"]
	require.NotNil(t, ticket)
	assert.Equal(t, "builtin:create_ticket", ticket.Handler)
	assert.Equal(t, []string{"search_kb"}, ticket.DependsOn)
	assert.Equal(t, "search_kb.found == false", ticket.Condition)

	respond := graph.Nodes["respond"]
	require.NotNil(t, respond)
	assert.Equal(t, "llm:respond", respond.Handler)
	assert.Equal(t, []string{"classify_intent", "search_kb", "create_ticket"}, respond.DependsOn)

	// Check entry points (nodes with no dependencies)
	assert.Contains(t, graph.EntryPoints, "classify_intent")
}

func TestParseScriptWithSkillsNoAnnotations(t *testing.T) {
	content := `## Opening
- Greet the customer
## Closing
- Say goodbye
`
	parser := NewParser()
	parsed, graph, err := parser.ParseScriptWithSkills(content)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.NotNil(t, graph)
	assert.False(t, graph.HasSkills)
	assert.Empty(t, graph.Nodes)
}

func TestSkillGraphCycleDetection(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"a": {Name: "a", DependsOn: []string{"b"}},
			"b": {Name: "b", DependsOn: []string{"a"}},
		},
	}
	err := graph.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Message, "cycle detected")
}

func TestSkillGraphMissingDependency(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"a": {Name: "a", DependsOn: []string{"nonexistent"}},
		},
	}
	err := graph.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Message, "does not exist")
}

func TestSkillGraphValid(t *testing.T) {
	graph := &SkillGraph{
		Nodes: map[string]*SkillDefinition{
			"a": {Name: "a"},
			"b": {Name: "b", DependsOn: []string{"a"}},
			"c": {Name: "c", DependsOn: []string{"a", "b"}},
		},
	}
	err := graph.Validate()
	assert.Nil(t, err)
	assert.Contains(t, graph.EntryPoints, "a")
	assert.NotContains(t, graph.EntryPoints, "b")
	assert.NotContains(t, graph.EntryPoints, "c")
}

func TestParseScriptWithSkillsLineNumbers(t *testing.T) {
	content := `## Opening

<!-- @skill: test_skill, handler: "builtin:test" -->
## Test Section
Some content.
`
	parser := NewParser()
	_, graph, err := parser.ParseScriptWithSkills(content)
	require.NoError(t, err)
	require.NotNil(t, graph)
	require.True(t, graph.HasSkills)

	skill := graph.Nodes["test_skill"]
	require.NotNil(t, skill)
	assert.Equal(t, 3, skill.LineStart) // line 3 (1-based)
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"a", []string{"a"}},
		{"", nil},
		{"  a  ,  b  ", []string{"a", "b"}},
		{"a,,b", []string{"a", "b"}},
	}
	for _, tt := range tests {
		result := parseCommaSeparated(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestAnnotationBlockLineStart(t *testing.T) {
	content := `## Section 1
Some content.

<!-- @skill: my_skill, handler: "builtin:test" -->
## Section 2
More content.
`
	blocks := extractAnnotationBlocks(content)
	require.Len(t, blocks, 1)
	assert.Equal(t, 4, blocks[0].lineStart) // line 4 (1-based)
	assert.Equal(t, "skill", blocks[0].annotationType)
	assert.Equal(t, "my_skill", blocks[0].params["code"])
	assert.Equal(t, "builtin:test", blocks[0].params["handler"])
}
