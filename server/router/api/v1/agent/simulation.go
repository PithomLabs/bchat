package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/revrost/go-openrouter"

	"github.com/usememos/memos/store"
)

// ============================================================================
// SIMULATION TYPES
// ============================================================================

// SimulationRequest represents a request to start a simulation.
type SimulationRequest struct {
	InitialPrompt string `json:"initial_prompt"`
	PersonaHint   string `json:"persona_hint,omitempty"`
}

// SimulationStartResponse is returned when a simulation starts.
type SimulationStartResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	StreamURL string `json:"stream_url"`
}

// SimulationControlRequest represents control signals for a simulation.
type SimulationControlRequest struct {
	Action string `json:"action"` // "pause", "resume", "stop"
}

// SimulationControlResponse is returned from control actions.
type SimulationControlResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
}

// SimulationMessage represents a message in the simulation.
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

// SimulationStatus represents the current status event.
type SimulationStatus struct {
	Status         string `json:"status"`
	CurrentTurn    int    `json:"current_turn"`
	RespondingRole string `json:"responding_role,omitempty"`
}

// SimulationComplete represents the completion event.
type SimulationComplete struct {
	Status     string `json:"status"`
	TotalTurns int    `json:"total_turns"`
	EndReason  string `json:"end_reason"`
}

// SimulationState holds the runtime state of a simulation.
type SimulationState struct {
	ID            string
	TenantID      int32
	UserID        int32
	TenantSlug    string
	Status        string // "running", "paused", "completed", "stopped"
	CurrentTurn   int
	MaxTurns      int
	MinTurns      int
	Messages      []SimulationMessage
	InitialPrompt string
	PersonaHint   string
	EndReason     string

	// Control channels
	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}

	mu sync.RWMutex
}

// ============================================================================
// SIMULATION SESSION STORE
// ============================================================================

// SimulationSessionStore manages active simulation sessions.
type SimulationSessionStore struct {
	sessions map[string]*SimulationState
	mu       sync.RWMutex
	ttl      time.Duration
}

// NewSimulationSessionStore creates a new simulation session store.
func NewSimulationSessionStore(ttl time.Duration) *SimulationSessionStore {
	store := &SimulationSessionStore{
		sessions: make(map[string]*SimulationState),
		ttl:      ttl,
	}
	go store.cleanupLoop()
	return store
}

// Create creates a new simulation session.
func (s *SimulationSessionStore) Create(tenantID int32, userID int32, tenantSlug, initialPrompt, personaHint string) *SimulationState {
	state := &SimulationState{
		ID:            "sim-" + uuid.New().String(),
		TenantID:      tenantID,
		UserID:        userID,
		TenantSlug:    tenantSlug,
		Status:        "running",
		CurrentTurn:   0,
		MaxTurns:      50,
		MinTurns:      10,
		Messages:      []SimulationMessage{},
		InitialPrompt: initialPrompt,
		PersonaHint:   personaHint,
		pauseCh:       make(chan struct{}, 1),
		resumeCh:      make(chan struct{}, 1),
		stopCh:        make(chan struct{}, 1),
	}

	s.mu.Lock()
	s.sessions[state.ID] = state
	s.mu.Unlock()

	return state
}

// Get retrieves a simulation session by ID.
func (s *SimulationSessionStore) Get(sessionID string) *SimulationState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

// Delete removes a simulation session.
func (s *SimulationSessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *SimulationSessionStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.cleanup()
	}
}

func (s *SimulationSessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up completed/stopped sessions older than TTL
	for id, state := range s.sessions {
		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()
		if status == "completed" || status == "stopped" {
			delete(s.sessions, id)
		}
	}
}

// ============================================================================
// SIMULATION ORCHESTRATION
// ============================================================================

// simulationSessions holds the active simulation sessions (initialized in NewService).
var simulationSessions *SimulationSessionStore

func init() {
	simulationSessions = NewSimulationSessionStore(15 * time.Minute)
}

// GetSimulationSessions returns the simulation session store.
func (s *Service) GetSimulationSessions() *SimulationSessionStore {
	return simulationSessions
}

// RunSimulation orchestrates a simulation conversation.
func (s *Service) RunSimulation(
	ctx context.Context,
	config *AudienceConfig,
	state *SimulationState,
	msgChan chan<- SimulationMessage,
	statusChan chan<- SimulationStatus,
	completeChan chan<- SimulationComplete,
) {
	defer close(msgChan)
	defer close(statusChan)
	defer close(completeChan)

	// Create an internal session for the agent
	agentSession := &store.AgentSession{
		ID:             uuid.New().String(),
		TenantID:       config.TenantID,
		AudienceType:   "internal",
		Phase:          "triage",
		UrgencyLevel:   0,
		CoverageStatus: "unknown",
		MessageCount:   0,
		Messages:       []store.AgentMessage{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Main simulation loop
	for state.CurrentTurn < state.MaxTurns {
		// Check for control signals
		select {
		case <-state.stopCh:
			state.mu.Lock()
			state.Status = "stopped"
			state.EndReason = "user_stopped"
			state.mu.Unlock()
			completeChan <- SimulationComplete{
				Status:     "stopped",
				TotalTurns: state.CurrentTurn,
				EndReason:  "user_stopped",
			}
			return
		case <-state.pauseCh:
			state.mu.Lock()
			state.Status = "paused"
			state.mu.Unlock()
			statusChan <- SimulationStatus{
				Status:      "paused",
				CurrentTurn: state.CurrentTurn,
			}
			// Wait for resume or stop
			select {
			case <-state.resumeCh:
				state.mu.Lock()
				state.Status = "running"
				state.mu.Unlock()
				statusChan <- SimulationStatus{
					Status:      "running",
					CurrentTurn: state.CurrentTurn,
				}
			case <-state.stopCh:
				state.mu.Lock()
				state.Status = "stopped"
				state.EndReason = "user_stopped"
				state.mu.Unlock()
				completeChan <- SimulationComplete{
					Status:     "stopped",
					TotalTurns: state.CurrentTurn,
					EndReason:  "user_stopped",
				}
				return
			case <-ctx.Done():
				return
			}
		default:
			// Continue with simulation
		}

		state.CurrentTurn++

		// Generate human message
		statusChan <- SimulationStatus{
			Status:         "running",
			CurrentTurn:    state.CurrentTurn,
			RespondingRole: "human_sim",
		}

		humanMessage, err := s.generateHumanResponse(ctx, config, state, agentSession)
		if err != nil {
			slog.Error("failed to generate human response", "error", err)
			state.mu.Lock()
			state.Status = "completed"
			state.EndReason = "error"
			state.mu.Unlock()
			completeChan <- SimulationComplete{
				Status:     "completed",
				TotalTurns: state.CurrentTurn,
				EndReason:  "error",
			}
			return
		}

		humanMsg := SimulationMessage{
			Role:      "human_sim",
			Content:   humanMessage,
			TurnNum:   state.CurrentTurn,
			Timestamp: time.Now(),
		}
		state.mu.Lock()
		state.Messages = append(state.Messages, humanMsg)
		state.mu.Unlock()
		msgChan <- humanMsg

		// Add to agent session history for context
		agentSession.Messages = append(agentSession.Messages, store.AgentMessage{
			Role:      "user",
			Content:   humanMessage,
			Timestamp: time.Now(),
		})
		agentSession.MessageCount++

		// Generate agent response
		statusChan <- SimulationStatus{
			Status:         "running",
			CurrentTurn:    state.CurrentTurn,
			RespondingRole: "agent",
		}

		agentResp, err := s.processChat(ctx, config, agentSession, humanMessage)
		if err != nil {
			slog.Error("failed to generate agent response", "error", err)
			state.mu.Lock()
			state.Status = "completed"
			state.EndReason = "error"
			state.mu.Unlock()
			completeChan <- SimulationComplete{
				Status:     "completed",
				TotalTurns: state.CurrentTurn,
				EndReason:  "error",
			}
			return
		}

		agentMsg := SimulationMessage{
			Role:      "agent",
			Content:   agentResp.Message.Content,
			TurnNum:   state.CurrentTurn,
			Timestamp: time.Now(),
			Metadata: &SimulationMetadata{
				Intent:  agentResp.Metadata.Intent,
				Phase:   agentResp.Metadata.Phase,
				Urgency: agentResp.Metadata.Urgency,
			},
		}
		state.mu.Lock()
		state.Messages = append(state.Messages, agentMsg)
		state.mu.Unlock()
		msgChan <- agentMsg

		// Check for end conditions after minimum turns
		if state.CurrentTurn >= state.MinTurns {
			endReason := s.checkEndConditions(ctx, config, state, agentSession, agentResp)
			if endReason != "" {
				state.mu.Lock()
				state.Status = "completed"
				state.EndReason = endReason
				state.mu.Unlock()
				completeChan <- SimulationComplete{
					Status:     "completed",
					TotalTurns: state.CurrentTurn,
					EndReason:  endReason,
				}
				return
			}
		}

		// Small delay between turns
		time.Sleep(500 * time.Millisecond)
	}

	// Reached max turns
	state.mu.Lock()
	state.Status = "completed"
	state.EndReason = "max_turns"
	state.mu.Unlock()
	completeChan <- SimulationComplete{
		Status:     "completed",
		TotalTurns: state.CurrentTurn,
		EndReason:  "max_turns",
	}
}

// ============================================================================
// HUMAN SIMULATOR
// ============================================================================

// generateHumanResponse generates a realistic human response using LLM.
func (s *Service) generateHumanResponse(ctx context.Context, config *AudienceConfig, state *SimulationState, agentSession *store.AgentSession) (string, error) {
	model, apiKey := s.getLLMConfig(ctx, config.TenantID)
	if apiKey == "" {
		return "", fmt.Errorf("no API key configured")
	}

	// Build conversation history
	var historyBuilder strings.Builder
	for _, msg := range state.Messages {
		role := "Human"
		if msg.Role == "agent" {
			role = "Agent"
		}
		historyBuilder.WriteString(fmt.Sprintf("%s: %s\n\n", role, msg.Content))
	}

	// Build persona description
	persona := "a typical customer"
	if state.PersonaHint != "" {
		persona = state.PersonaHint
	}

	vertical := "services"
	if config.Audience != nil && config.Audience.Role != "" {
		vertical = config.Audience.Role
	}

	systemPrompt := fmt.Sprintf(`You are simulating a realistic human customer/user interacting with %s's AI assistant.

CONTEXT:
- Company: %s
- Industry: %s
- Initial scenario: "%s"
- Persona: %s

YOUR ROLE:
You are playing the part of a customer. Your responses should be:

1. NATURAL AND HUMAN-LIKE:
   - Use casual language, contractions, sometimes informal grammar
   - Express emotions naturally (frustration, relief, confusion, gratitude)
   - Ask follow-up questions when clarification is needed
   - Sometimes go slightly off-topic, then return to the main issue

2. REALISTIC BEHAVIOR:
   - Provide information gradually, not all at once
   - React to the agent's tone and helpfulness
   - If asked for details, provide them (but not always completely)
   - Express urgency appropriately based on the situation
   - Occasionally misunderstand or need clarification

3. STAY IN CHARACTER:
   - You are a real person with this problem, not an AI
   - Never mention being a simulation, test, or AI
   - Have realistic knowledge gaps about the company's services
   - React to wait times, policies, processes like a real customer would

4. CONVERSATION DYNAMICS:
   - If satisfied with an answer, express thanks and potentially ask a follow-up
   - If frustrated, express it but remain civil
   - Eventually reach a natural conclusion (resolved, need to call, will think about it)

Generate the next human response. Keep it realistic - usually 1-3 sentences, occasionally longer for complex explanations. Do not include any prefix like "Human:" or quotation marks.`,
		config.CompanyName, config.CompanyName, vertical, state.InitialPrompt, persona)

	userPrompt := "CONVERSATION HISTORY:\n" + historyBuilder.String()
	if state.CurrentTurn == 1 {
		userPrompt = "This is the start of the conversation. Generate the opening message from the human based on the initial scenario."
	} else {
		userPrompt += "\nGenerate the next human response:"
	}

	client := openrouter.NewClient(apiKey)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage(systemPrompt),
			openrouter.UserMessage(userPrompt),
		},
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	response := strings.TrimSpace(resp.Choices[0].Message.Content.Text)
	// Clean up any accidental prefixes
	response = strings.TrimPrefix(response, "Human: ")
	response = strings.TrimPrefix(response, "Customer: ")
	response = strings.Trim(response, "\"")

	return response, nil
}

// ============================================================================
// END DETECTION
// ============================================================================

// checkEndConditions checks if the simulation should end.
func (s *Service) checkEndConditions(ctx context.Context, config *AudienceConfig, state *SimulationState, session *store.AgentSession, lastResponse *ChatResponse) string {
	// 1. Check agent phase
	if lastResponse.Metadata.Phase == "resolved" || lastResponse.Metadata.Phase == "closed" {
		return "phase_closed"
	}
	if lastResponse.Metadata.Phase == "escalated" {
		return "phase_closed"
	}

	// 2. Check for closing keywords in recent messages
	if len(state.Messages) >= 2 {
		lastHuman := ""
		lastAgent := ""
		for i := len(state.Messages) - 1; i >= 0 && (lastHuman == "" || lastAgent == ""); i-- {
			if state.Messages[i].Role == "human_sim" && lastHuman == "" {
				lastHuman = strings.ToLower(state.Messages[i].Content)
			}
			if state.Messages[i].Role == "agent" && lastAgent == "" {
				lastAgent = strings.ToLower(state.Messages[i].Content)
			}
		}

		// Human closing phrases
		humanClosingPhrases := []string{
			"thank you", "thanks", "that's all", "that is all",
			"no more questions", "i'm good", "i think that's it",
			"goodbye", "bye", "have a good", "i'll call back",
		}
		for _, phrase := range humanClosingPhrases {
			if strings.Contains(lastHuman, phrase) {
				// Check if agent also indicated closing
				agentClosingPhrases := []string{
					"anything else", "help you with", "have a great day",
					"we'll be in touch", "give us a call", "take care",
				}
				for _, agentPhrase := range agentClosingPhrases {
					if strings.Contains(lastAgent, agentPhrase) {
						return "keyword_match"
					}
				}
			}
		}
	}

	// 3. LLM-based end detection for longer conversations
	if state.CurrentTurn >= 15 && state.CurrentTurn%5 == 0 {
		isComplete := s.detectConversationEnd(ctx, config, state)
		if isComplete {
			return "llm_detected"
		}
	}

	return ""
}

// detectConversationEnd uses LLM to determine if conversation is complete.
func (s *Service) detectConversationEnd(ctx context.Context, config *AudienceConfig, state *SimulationState) bool {
	model, apiKey := s.getLLMConfig(ctx, config.TenantID)
	if apiKey == "" {
		return false
	}

	// Get last 5 messages
	messages := state.Messages
	if len(messages) > 10 {
		messages = messages[len(messages)-10:]
	}

	var historyBuilder strings.Builder
	for _, msg := range messages {
		role := "Human"
		if msg.Role == "agent" {
			role = "Agent"
		}
		historyBuilder.WriteString(fmt.Sprintf("%s: %s\n\n", role, msg.Content))
	}

	prompt := fmt.Sprintf(`Analyze this conversation and determine if it has reached a natural conclusion.

A conversation is complete if:
- The customer's issue has been resolved
- The customer has indicated they have no more questions
- A clear next step has been established (callback scheduled, etc.)
- The conversation has reached a natural goodbye

Conversation:
%s

Respond with JSON only: {"is_complete": true/false, "reason": "brief explanation"}`, historyBuilder.String())

	client := openrouter.NewClient(apiKey)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage("You analyze conversations to determine if they have concluded. Respond only with JSON."),
			openrouter.UserMessage(prompt),
		},
	})
	if err != nil {
		return false
	}

	if len(resp.Choices) == 0 {
		return false
	}

	responseText := resp.Choices[0].Message.Content.Text

	// Extract JSON
	var result struct {
		IsComplete bool   `json:"is_complete"`
		Reason     string `json:"reason"`
	}

	// Try to parse JSON
	jsonPattern := regexp.MustCompile(`\{[^}]+\}`)
	jsonMatch := jsonPattern.FindString(responseText)
	if jsonMatch != "" {
		if err := json.Unmarshal([]byte(jsonMatch), &result); err == nil {
			return result.IsComplete
		}
	}

	return false
}

// ============================================================================
// TRANSCRIPT PERSISTENCE
// ============================================================================

// SaveSimulationTranscript saves the simulation transcript to the database.
func (s *Service) SaveSimulationTranscript(ctx context.Context, state *SimulationState) (*store.AgentSimulationTranscript, error) {
	// Convert messages to store format
	storeMessages := make([]store.SimulationMessage, len(state.Messages))
	for i, msg := range state.Messages {
		var metadata *store.SimulationMetadata
		if msg.Metadata != nil {
			metadata = &store.SimulationMetadata{
				Intent:  msg.Metadata.Intent,
				Phase:   msg.Metadata.Phase,
				Urgency: msg.Metadata.Urgency,
			}
		}
		storeMessages[i] = store.SimulationMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			TurnNum:   msg.TurnNum,
			Timestamp: msg.Timestamp,
			Metadata:  metadata,
		}
	}

	transcript := &store.AgentSimulationTranscript{
		ID:            state.ID,
		TenantID:      state.TenantID,
		UserID:        state.UserID,
		InitialPrompt: state.InitialPrompt,
		PersonaHint:   state.PersonaHint,
		TotalTurns:    state.CurrentTurn,
		EndReason:     state.EndReason,
		Messages:      storeMessages,
	}

	return s.store.CreateAgentSimulationTranscript(ctx, transcript)
}

// ListSimulationTranscripts lists simulation transcripts for a tenant.
func (s *Service) ListSimulationTranscripts(ctx context.Context, tenantID int32, limit, offset int) ([]*store.AgentSimulationTranscript, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.ListAgentSimulationTranscripts(ctx, &store.FindAgentSimulationTranscript{
		TenantID: &tenantID,
		Limit:    limit,
		Offset:   offset,
	})
}

// GetSimulationTranscript gets a specific simulation transcript.
func (s *Service) GetSimulationTranscript(ctx context.Context, id string) (*store.AgentSimulationTranscript, error) {
	return s.store.GetAgentSimulationTranscript(ctx, &store.FindAgentSimulationTranscript{
		ID: &id,
	})
}
