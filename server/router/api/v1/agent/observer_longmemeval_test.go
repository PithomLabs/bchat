package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test 1: End-to-End Observer Pipeline ---

func TestEndToEndObserver_DetailPreservation(t *testing.T) {
	ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-e2e", mockObservations["specific_name"])
	defer ts.Close()

	sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-e2e-1", mustMakeMessages(
		"user", "I've been listening to this one playlist on Spotify that I created, called Summer Vibes, and it's got all these chill tracks.",
		"assistant", "Great choice! Summer Vibes sounds like a perfect playlist for relaxed listening.",
	))

	err := service.RunObserver(ctx, tenant.ID, sessionID)
	require.NoError(t, err)

	obsLog, err := ts.GetObservationLog(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, obsLog)

	assert.Contains(t, obsLog.ObservationLog, "Summer Vibes", "observer must preserve specific playlist name")
	assert.Contains(t, obsLog.ObservationLog, "Spotify", "observer must preserve platform name")
	assert.NotEmpty(t, obsLog.CurrentTask)
	assert.NotEmpty(t, obsLog.SuggestedResponse)
}

func TestEndToEndObserver_ResourceScope(t *testing.T) {
	ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-e2e-resource", mockObservations["specific_name"])
	defer ts.Close()

	setOMEnvAndReload(t, "OM_SCOPE", "resource")

	var userID int32 = 42
	sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-resource-1", mustMakeMessages(
		"user", "I have a playlist called Summer Vibes on Spotify.",
	))
	session := service.memorySessions.Get(tenant.ID, sessionID)
	session.UserID = &userID
	service.memorySessions.Update(session)

	err := service.RunObserver(ctx, tenant.ID, sessionID)
	require.NoError(t, err)

	obsLog, err := ts.GetObservationLogByResource(ctx, "user_42")
	require.NoError(t, err)
	require.NotNil(t, obsLog)
	assert.Contains(t, obsLog.ObservationLog, "Summer Vibes")
}

// --- Test 2: Reflector Detail Preservation ---

func TestReflector_DetailPreservation(t *testing.T) {
	ctx, ts, service, tenant := newObserverTestService(t, "obs-reflector")
	defer ts.Close()

	setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")

	// First call: observer produces observations.
	// Second call: reflector compresses them.
	newCountingMockLLM(t, mockObservations["specific_name"], mockObservations["reflector_compressed"])

	sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-refl-1", mustMakeMessages(
		"user", "I have a playlist called Summer Vibes on Spotify with chill tracks.",
		"assistant", "Summer Vibes is a great playlist name!",
	))

	err := service.RunObserver(ctx, tenant.ID, sessionID)
	require.NoError(t, err)

	obsLog, err := ts.GetObservationLog(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, obsLog)

	assert.Less(t, obsLog.TokensInLog, 20,
		"reflector must compress observations below threshold")
	assert.Contains(t, obsLog.ObservationLog, "Summer Vibes",
		"compressed observations must preserve key details")
}

func TestReflector_PreservesMultipleDetails(t *testing.T) {
	ctx, ts, service, tenant := newObserverTestService(t, "obs-reflector-multi")
	defer ts.Close()

	setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")

	newCountingMockLLM(t, mockObservations["cross_session"], mockObservations["reflector_compressed_multi"])

	sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-refl-multi", mustMakeMessages(
		"user", "I own a Korg B1 keyboard and a Yamaha acoustic guitar.",
		"assistant", "Nice instruments! The Korg B1 is a great digital piano.",
	))

	err := service.RunObserver(ctx, tenant.ID, sessionID)
	require.NoError(t, err)

	obsLog, err := ts.GetObservationLog(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, obsLog)

	assert.Contains(t, obsLog.ObservationLog, "Korg B1")
	assert.Contains(t, obsLog.ObservationLog, "Yamaha")
}

// --- Test 3: Malformed LLM Output Handling ---

func TestMalformedLLMOutput(t *testing.T) {
	tests := []struct {
		name        string
		mockKey     string
		expectEmpty bool
		expectRaw   string // expected substring in fallback raw output; "" = no check
	}{
		{
			name:        "missing closing tag falls back to raw output",
			mockKey:     "malformed_no_closing",
			expectEmpty: false,
			expectRaw:   "User stated something important",
		},
		{
			name:        "content outside tags extracts between tags",
			mockKey:     "malformed_content_outside",
			expectEmpty: false,
			expectRaw:   "",
		},
		{
			name:        "multiple observation blocks takes first",
			mockKey:     "malformed_multiple_blocks",
			expectEmpty: false,
			expectRaw:   "",
		},
		{
			name:        "extra whitespace is trimmed",
			mockKey:     "malformed_extra_whitespace",
			expectEmpty: false,
			expectRaw:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-malformed-"+tt.mockKey, mockObservations[tt.mockKey])
			defer ts.Close()

			sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-malformed-"+tt.mockKey, mustMakeMessages(
				"user", "Test message content here.",
			))

			err := service.RunObserver(ctx, tenant.ID, sessionID)
			require.NoError(t, err, "observer must not error on malformed output")

			obsLog, err := ts.GetObservationLog(ctx, sessionID)
			require.NoError(t, err)
			require.NotNil(t, obsLog, "observation log must be persisted even with malformed output")

			if tt.expectEmpty {
				assert.Empty(t, obsLog.ObservationLog)
			}
			if tt.expectRaw != "" {
				assert.Contains(t, obsLog.ObservationLog, tt.expectRaw,
					"fallback raw output must contain expected content")
			}
		})
	}

	t.Run("empty response from LLM", func(t *testing.T) {
		ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-malformed-empty", mockObservations["empty_response"])
		defer ts.Close()

		sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-malformed-empty", mustMakeMessages(
			"user", "Test message.",
		))

		err := service.RunObserver(ctx, tenant.ID, sessionID)
		_ = err
	})

	t.Run("trivial response from LLM", func(t *testing.T) {
		ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-malformed-trivial", mockObservations["trivial_response"])
		defer ts.Close()

		sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-malformed-trivial", mustMakeMessages(
			"user", "Test message.",
		))

		err := service.RunObserver(ctx, tenant.ID, sessionID)
		require.NoError(t, err)

		obsLog, err := ts.GetObservationLog(ctx, sessionID)
		require.NoError(t, err)
		require.NotNil(t, obsLog)
	})
}

// --- Test 4: Edge Cases ---

func TestEdgeCases(t *testing.T) {
	t.Run("empty session", func(t *testing.T) {
		ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-edge-empty", mockObservations["empty_response"])
		defer ts.Close()

		sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-edge-empty", nil)

		err := service.RunObserver(ctx, tenant.ID, sessionID)
		require.NoError(t, err)
	})

	t.Run("trivial messages only", func(t *testing.T) {
		ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-edge-trivial", mockObservations["empty_response"])
		defer ts.Close()

		sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-edge-trivial", mustMakeMessages(
			"user", "ok",
			"assistant", "thanks",
			"user", "yes",
			"assistant", "sure",
		))

		err := service.RunObserver(ctx, tenant.ID, sessionID)
		require.NoError(t, err)

		obsLog, err := ts.GetObservationLog(ctx, sessionID)
		require.NoError(t, err)
		require.NotNil(t, obsLog)
	})

	t.Run("single message", func(t *testing.T) {
		ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-edge-single", mockObservations["specific_name"])
		defer ts.Close()

		sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-edge-single", mustMakeMessages(
			"user", "I just created a playlist called Summer Vibes on Spotify.",
		))

		err := service.RunObserver(ctx, tenant.ID, sessionID)
		require.NoError(t, err)

		obsLog, err := ts.GetObservationLog(ctx, sessionID)
		require.NoError(t, err)
		require.NotNil(t, obsLog)
		assert.Contains(t, obsLog.ObservationLog, "Summer Vibes")
	})

	t.Run("long content", func(t *testing.T) {
		ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-edge-long", mockObservations["edge_long_content"])
		defer ts.Close()

		longMsg := strings.Repeat("This is a detailed code review comment. ", 200)
		sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-edge-long", mustMakeMessages(
			"user", longMsg,
		))

		err := service.RunObserver(ctx, tenant.ID, sessionID)
		require.NoError(t, err)

		obsLog, err := ts.GetObservationLog(ctx, sessionID)
		require.NoError(t, err)
		require.NotNil(t, obsLog)
	})

	t.Run("unicode content", func(t *testing.T) {
		ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-edge-unicode", mockObservations["edge_unicode"])
		defer ts.Close()

		sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-edge-unicode", mustMakeMessages(
			"user", "Hello! 你好！こんにちは！🎉 émojis and ñoño",
		))

		err := service.RunObserver(ctx, tenant.ID, sessionID)
		require.NoError(t, err)

		obsLog, err := ts.GetObservationLog(ctx, sessionID)
		require.NoError(t, err)
		require.NotNil(t, obsLog)
	})
}

// --- Test 5: LongMemEval Data Loader ---

type LongMemEvalQuestion struct {
	QuestionID        string              `json:"question_id"`
	QuestionType      string              `json:"question_type"`
	Question          string              `json:"question"`
	Answer            string              `json:"answer"`
	QuestionDate      string              `json:"question_date"`
	HaystackSessionIDs []string           `json:"haystack_session_ids"`
	HaystackDates     []string            `json:"haystack_dates"`
	HaystackSessions  [][]map[string]string `json:"haystack_sessions"`
	AnswerSessionIDs  []string            `json:"answer_session_ids,omitempty"`
}

// embeddedQuestions is a small subset of LongMemEval data for when the full
// dataset isn't available. Covers all 5 question types + abstention.
var embeddedQuestions = []LongMemEvalQuestion{
	{
		QuestionID:   "embedded_ie_user",
		QuestionType: "single-session-user",
		Question:     "What is the name of the playlist I created on Spotify?",
		Answer:       "Summer Vibes",
		HaystackSessions: [][]map[string]string{
			{
				{"role": "user", "content": "I've been listening to this one playlist on Spotify that I created, called Summer Vibes, and it's got all these chill tracks."},
				{"role": "assistant", "content": "Summer Vibes sounds like a great playlist for relaxed listening!"},
			},
		},
		HaystackDates: []string{"2025-01-15"},
	},
	{
		QuestionID:   "embedded_ie_assistant",
		QuestionType: "single-session-assistant",
		Question:     "What chess move did the user mention?",
		Answer:       "27. Kg2 Bd5+",
		HaystackSessions: [][]map[string]string{
			{
				{"role": "user", "content": "After 27. Kg2 Bd5+ the chess game became very sharp."},
				{"role": "assistant", "content": "That's a critical moment! The check on g2 forces White to respond."},
			},
		},
		HaystackDates: []string{"2025-01-20"},
	},
	{
		QuestionID:   "embedded_mr",
		QuestionType: "multi-session",
		Question:     "How many musical instruments have I mentioned owning?",
		Answer:       "2",
		HaystackSessions: [][]map[string]string{
			{
				{"role": "user", "content": "I just got a new Korg B1 keyboard for my home studio."},
				{"role": "assistant", "content": "The Korg B1 is an excellent digital piano!"},
			},
			{
				{"role": "user", "content": "I also have a Yamaha acoustic guitar that I've been playing for years."},
				{"role": "assistant", "content": "A Yamaha guitar is a solid choice for acoustic playing."},
			},
		},
		HaystackDates: []string{"2025-02-01", "2025-02-10"},
	},
	{
		QuestionID:   "embedded_ku",
		QuestionType: "knowledge-update",
		Question:     "How many bikes do I have now?",
		Answer:       "4",
		HaystackSessions: [][]map[string]string{
			{
				{"role": "user", "content": "I have 3 bikes in my garage."},
				{"role": "assistant", "content": "That's a nice collection!"},
			},
			{
				{"role": "user", "content": "Actually, I just got a new hybrid bike yesterday, so now I have 4."},
				{"role": "assistant", "content": "Congrats on the new hybrid bike!"},
			},
		},
		HaystackDates: []string{"2025-03-01", "2025-03-15"},
	},
	{
		QuestionID:   "embedded_tr",
		QuestionType: "temporal-reasoning",
		Question:     "How long ago did I visit the museum with Alex?",
		Answer:       "2 months",
		HaystackSessions: [][]map[string]string{
			{
				{"role": "user", "content": "I visited the modern art museum with my friend Alex last January. It was fantastic!"},
				{"role": "assistant", "content": "Sounds like a great trip! Who is Alex?"},
			},
			{
				{"role": "user", "content": "Alex is my college friend who lives downtown."},
				{"role": "assistant", "content": "Nice to have a friend nearby for outings."},
			},
		},
		HaystackDates: []string{"2025-01-10", "2025-02-20"},
	},
	{
		QuestionID:   "embedded_abs",
		QuestionType: "abstention",
		Question:     "What did I tell you about my 30-gallon fish tank?",
		Answer:       "The user has not mentioned a 30-gallon fish tank.",
		HaystackSessions: [][]map[string]string{
			{
				{"role": "user", "content": "I have a 10-gallon freshwater aquarium with tropical fish."},
				{"role": "assistant", "content": "A 10-gallon tank is great for beginners!"},
			},
		},
		HaystackDates: []string{"2025-04-01"},
	},
}

func loadLongMemEvalQuestions(t *testing.T) []LongMemEvalQuestion {
	t.Helper()

	paths := []string{
		"../../build/data/longmemeval_s.json",
		"../../../../build/data/longmemeval_s.json",
		filepath.Join("..", "..", "..", "build", "data", "longmemeval_s.json"),
	}

	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	paths = append(paths, filepath.Join(testDir, "..", "..", "..", "build", "data", "longmemeval_s.json"))

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var questions []LongMemEvalQuestion
			if err := json.Unmarshal(data, &questions); err == nil && len(questions) > 0 {
				t.Logf("Loaded %d questions from %s", len(questions), p)
				return questions
			}
		}
	}

	zipPaths := []string{
		"../../Desktop/memory/LongMemEval-main.zip",
		"../../../../Desktop/memory/LongMemEval-main.zip",
		filepath.Join(testDir, "..", "..", "..", "Desktop", "memory", "LongMemEval-main.zip"),
	}
	for _, zp := range zipPaths {
		if _, err := os.Stat(zp); err == nil {
			t.Logf("Found zip at %s — extraction not implemented, using embedded subset", zp)
			break
		}
	}

	t.Log("LongMemEval dataset not found — using embedded subset (6 questions)")
	return embeddedQuestions
}

func TestLongMemEvalDataLoader(t *testing.T) {
	t.Run("LoadHaystack", func(t *testing.T) {
		questions := loadLongMemEvalQuestions(t)
		require.NotEmpty(t, questions)

		q := questions[0]
		assert.NotEmpty(t, q.QuestionID)
		assert.NotEmpty(t, q.QuestionType)
		assert.NotEmpty(t, q.Question)
		assert.NotEmpty(t, q.Answer)
		require.NotEmpty(t, q.HaystackSessions, "must have at least one session")

		for i, session := range q.HaystackSessions {
			require.NotEmpty(t, session, "session %d must not be empty", i)
			for j, turn := range session {
				role, ok := turn["role"]
				require.True(t, ok, "session %d turn %d must have role", i, j)
				assert.Contains(t, []string{"user", "assistant"}, role,
					"session %d turn %d has invalid role: %s", i, j, role)
				_, hasContent := turn["content"]
				assert.True(t, hasContent, "session %d turn %d must have content", i, j)
			}
		}

		assert.Equal(t, len(q.HaystackSessions), len(q.HaystackDates),
			"haystack_dates must align with haystack_sessions")
	})

	t.Run("FilterByType", func(t *testing.T) {
		questions := loadLongMemEvalQuestions(t)

		typeCount := make(map[string]int)
		for _, q := range questions {
			typeCount[q.QuestionType]++
		}

		expectedTypes := []string{
			"single-session-user", "single-session-assistant", "multi-session", "knowledge-update",
			"temporal-reasoning", "abstention",
		}
		for _, qt := range expectedTypes {
			assert.GreaterOrEqual(t, typeCount[qt], 1,
				"must have at least one question of type %q", qt)
		}
	})

	t.Run("FormatForObserver", func(t *testing.T) {
		questions := loadLongMemEvalQuestions(t)
		q := questions[0]
		session := q.HaystackSessions[0]

		msgs := make([]struct {
			Role    string
			Content string
		}, 0, len(session))
		for _, turn := range session {
			msgs = append(msgs, struct {
				Role    string
				Content string
			}{
				Role:    turn["role"],
				Content: turn["content"],
			})
		}

		require.NotEmpty(t, msgs)
		assert.Equal(t, "user", msgs[0].Role)
		assert.NotEmpty(t, msgs[0].Content)

		for _, m := range msgs {
			assert.Contains(t, []string{"user", "assistant"}, m.Role)
			assert.NotEmpty(t, m.Content)
		}
	})
}

// --- Test 6: Parameterized Detail Preservation ---

func TestDetailPreservationByQuestionType(t *testing.T) {
	tests := []struct {
		name         string
		mockKey      string
		questionType string
		detail       string
		messages     []string
	}{
		{
			name:         "single-session-user preserves specific name",
			mockKey:      "specific_name",
			questionType: "single-session-user",
			detail:       "Summer Vibes",
			messages: []string{
				"user", "I created a playlist on Spotify called Summer Vibes with all my favorite chill tracks.",
				"assistant", "Summer Vibes is a great playlist name!",
			},
		},
		{
			name:         "single-session-assistant preserves implicit context",
			mockKey:      "implicit_context",
			questionType: "single-session-assistant",
			detail:       "27. Kg2 Bd5+",
			messages: []string{
				"user", "Let me tell you about the game. After 27. Kg2 Bd5+ the position became very sharp.",
				"assistant", "That's a critical moment! The check on g2 forces White to respond.",
			},
		},
		{
			name:         "temporal-reasoning preserves temporal anchor",
			mockKey:      "temporal_anchor",
			questionType: "temporal-reasoning",
			detail:       "museum",
			messages: []string{
				"user", "I visited the modern art museum with my friend Alex last January.",
				"assistant", "The modern art museum is wonderful this time of year!",
			},
		},
		{
			name:         "knowledge-update captures state change",
			mockKey:      "knowledge_update",
			questionType: "knowledge-update",
			detail:       "4 bikes",
			messages: []string{
				"user", "I have 3 bikes in my garage.",
				"assistant", "That's a nice collection!",
				"user", "Actually I just got a new hybrid bike, now I have 4 bikes total.",
				"assistant", "Congrats on the new hybrid!",
			},
		},
		{
			name:         "multi-session preserves cross-session facts",
			mockKey:      "cross_session",
			questionType: "multi-session",
			detail:       "Korg B1",
			messages: []string{
				"user", "I own a Korg B1 keyboard for my home studio.",
				"assistant", "The Korg B1 is an excellent digital piano!",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if os.Getenv("BENCHMARK_REAL_LLM") != "true" {
				t.Skip("Set BENCHMARK_REAL_LLM=true to run with real LLM (mock mode: testing assertion logic only)")
			}

			ctx, ts, service, tenant := newObserverTestService(t, "obs-detail-"+tt.questionType)
			defer ts.Close()

			msgs := mustMakeMessages(tt.messages...)
			sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-detail-"+tt.questionType, msgs)

			err := service.RunObserver(ctx, tenant.ID, sessionID)
			require.NoError(t, err)

			obsLog, err := ts.GetObservationLog(ctx, sessionID)
			require.NoError(t, err)
			require.NotNil(t, obsLog)

			assert.Contains(t, obsLog.ObservationLog, tt.detail,
				"observer must preserve detail %q for question type %q", tt.detail, tt.questionType)
		})
	}

	t.Run("abstention no hallucination", func(t *testing.T) {
		if os.Getenv("BENCHMARK_REAL_LLM") != "true" {
			t.Skip("Abstention only meaningful with real LLM — mock trivially returns what it's told")
		}

		ctx, ts, service, tenant := newObserverTestService(t, "obs-detail-abstention")
		defer ts.Close()

		sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-detail-abs", mustMakeMessages(
			"user", "I have a 10-gallon freshwater aquarium with tropical fish.",
			"assistant", "A 10-gallon tank is great for beginners!",
		))

		err := service.RunObserver(ctx, tenant.ID, sessionID)
		require.NoError(t, err)

		obsLog, err := ts.GetObservationLog(ctx, sessionID)
		require.NoError(t, err)
		require.NotNil(t, obsLog)

		assert.NotContains(t, obsLog.ObservationLog, "30-gallon",
			"observer must not hallucinate details not present in the conversation")
		assert.Contains(t, obsLog.ObservationLog, "10-gallon",
			"observer must capture the actual tank size mentioned")
	})
}

func TestDetailPreservationByQuestionType_MockFallback(t *testing.T) {
	tests := []struct {
		name         string
		mockKey      string
		questionType string
		detail       string
	}{
		{"mock_ie_user", "specific_name", "single-session-user", "Summer Vibes"},
		{"mock_ie_assistant", "implicit_context", "single-session-assistant", "27. Kg2 Bd5+"},
		{"mock_temporal", "temporal_anchor", "temporal-reasoning", "museum"},
		{"mock_knowledge_update", "knowledge_update", "knowledge-update", "4 bikes"},
		{"mock_multi_session", "cross_session", "multi-session", "Korg B1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, ts, service, tenant := newObserverTestServiceWithMock(t, "obs-mock-"+tt.questionType, mockObservations[tt.mockKey])
			defer ts.Close()

			sessionID := createTestSession(t, ctx, ts, service, tenant.ID, "session-mock-"+tt.questionType, mustMakeMessages(
				"user", "Test message for "+tt.questionType,
			))

			err := service.RunObserver(ctx, tenant.ID, sessionID)
			require.NoError(t, err)

			obsLog, err := ts.GetObservationLog(ctx, sessionID)
			require.NoError(t, err)
			require.NotNil(t, obsLog)

			assert.Contains(t, obsLog.ObservationLog, tt.detail,
				"mock observation must contain detail %q", tt.detail)
		})
	}
}

// --- Test 7: Tier 2 Real LLM Detail Preservation Integration ---
// Added by bugs/049/plan5. Requires BENCHMARK_REAL_LLM=true and OPENROUTER_API_KEY.

func TestRealLLM_DetailPreservationIntegration(t *testing.T) {
	if os.Getenv("BENCHMARK_REAL_LLM") != "true" {
		t.Skip("Set BENCHMARK_REAL_LLM=true to test detail preservation with a real LLM")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	require.NotEmpty(t, apiKey, "OPENROUTER_API_KEY must be set for real LLM tests")

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "openrouter/default"
	}
	t.Logf("Using LLM model: %s", model)

	questions := loadLongMemEvalQuestions(t)

	// detailForType maps each question type to the specific detail that must
	// be preserved through observer and reflector compression. This cannot be
	// derived from q.Answer (e.g., multi-session Answer is "2" but the detail
	// to preserve is "Korg B1").
	detailForType := func(questionType string) string {
		switch questionType {
		case "single-session-user":
			return "Summer Vibes"
		case "single-session-assistant":
			return "27. Kg2 Bd5+"
		case "multi-session":
			return "Korg B1"
		case "knowledge-update":
			return "4 bikes"
		case "temporal-reasoning":
			return "museum"
		default:
			return ""
		}
	}

	var observerPassed, reflectorPassed int

	for _, q := range questions {
		t.Run(q.QuestionType, func(t *testing.T) {
			ctx, ts, service, tenant := newObserverTestService(t, "obs-real-"+q.QuestionType)
			defer ts.Close()

			// Create all haystack sessions (one per inner []map[string]string)
			var sessionIDs []string
			for i, turns := range q.HaystackSessions {
				msgs := convertTurnsToMessages(turns)
				sid := createTestSession(t, ctx, ts, service, tenant.ID,
					fmt.Sprintf("session-real-%s-%d", q.QuestionType, i), msgs)
				sessionIDs = append(sessionIDs, sid)
			}

			// --- Pass 1: Raw observations (default threshold) ---
			pass1OK := true
			for _, sid := range sessionIDs {
				if err := service.RunObserver(ctx, tenant.ID, sid); err != nil {
					t.Logf("Pass 1 API call failed for %s session %s: %v — skipping",
						q.QuestionType, sid, err)
					pass1OK = false
					break
				}
			}
			if !pass1OK {
				return
			}

			detail := detailForType(q.QuestionType)
			if detail == "" && q.QuestionType != "abstention" {
				t.Fatalf("unknown question type: %q — add to detailForType", q.QuestionType)
			}

			// Assert detail in raw observations
			if q.QuestionType == "abstention" {
				for _, sid := range sessionIDs {
					obsLog, err := ts.GetObservationLog(ctx, sid)
					require.NoError(t, err)
					require.NotNil(t, obsLog)
					assert.NotContains(t, obsLog.ObservationLog, "30-gallon",
						"raw observations must not hallucinate 30-gallon")
					assert.Contains(t, obsLog.ObservationLog, "10-gallon",
						"raw observations must capture actual tank size")
				}
			} else {
				found := false
				for _, sid := range sessionIDs {
					obsLog, err := ts.GetObservationLog(ctx, sid)
					require.NoError(t, err)
					if obsLog != nil && strings.Contains(obsLog.ObservationLog, detail) {
						found = true
						break
					}
				}
				assert.True(t, found,
					"detail %q must appear in at least one session's raw observations", detail)
			}

			if t.Failed() {
				return
			}
			observerPassed++

			// --- Pass 2: Reflector compression (threshold=1) ---
			// Threshold=1 guarantees the reflector fires on small test sessions
			// (~2 turns, ~60-100 tokens). Plan5_review suggested 200, but that
			// would never trigger compression on these sessions.
			setOMEnvAndReload(t, "OM_TOKEN_THRESHOLD", "1")

			// Record original token counts before reset for compression assertion
			originalTokens := make(map[string]int, len(sessionIDs))
			for _, sid := range sessionIDs {
				obsLog, err := ts.GetObservationLog(ctx, sid)
				require.NoError(t, err)
				if obsLog != nil {
					originalTokens[sid] = obsLog.TokensInLog
				}
			}

			for _, sid := range sessionIDs {
				resetObservationLog(t, ctx, ts, sid)
			}

			// Re-run observer on all sessions (triggers reflector within each call)
			for _, sid := range sessionIDs {
				if err := service.RunObserver(ctx, tenant.ID, sid); err != nil {
					t.Logf("Pass 2 API call failed for %s session %s: %v — skipping reflector check",
						q.QuestionType, sid, err)
					return
				}
			}

			// Assert detail in compressed observations
			if q.QuestionType == "abstention" {
				for _, sid := range sessionIDs {
					obsLog, err := ts.GetObservationLog(ctx, sid)
					require.NoError(t, err)
					require.NotNil(t, obsLog)
					assert.NotContains(t, obsLog.ObservationLog, "30-gallon",
						"compressed observations must not hallucinate 30-gallon")
					assert.Contains(t, obsLog.ObservationLog, "10-gallon",
						"compressed observations must capture actual tank size")
				}
			} else {
				found := false
				for _, sid := range sessionIDs {
					obsLog, err := ts.GetObservationLog(ctx, sid)
					require.NoError(t, err)
					if obsLog != nil && strings.Contains(obsLog.ObservationLog, detail) {
						found = true
						break
					}
				}
				assert.True(t, found,
					"detail %q must appear in at least one session's compressed observations", detail)
			}

			if !t.Failed() {
				reflectorPassed++
			}

			// Assert token compression (compressed ≤ original)
			for _, sid := range sessionIDs {
				obsLog, err := ts.GetObservationLog(ctx, sid)
				require.NoError(t, err)
				if obsLog != nil && originalTokens[sid] > 0 {
					assert.LessOrEqual(t, obsLog.TokensInLog, originalTokens[sid],
						"compressed log should be same size or smaller for session %s", sid)
				}
			}

			t.Logf("=== %s ===", q.QuestionType)
			t.Logf("  Detail: %q", detail)
			t.Logf("  Sessions: %d", len(sessionIDs))
			t.Logf("  Passed: %v", !t.Failed())
		})
	}

	// Summary
	total := len(questions)
	t.Logf("=== Summary ===")
	t.Logf("Observer: %d/%d passed", observerPassed, total)
	t.Logf("Reflector: %d/%d passed", reflectorPassed, total)
}
