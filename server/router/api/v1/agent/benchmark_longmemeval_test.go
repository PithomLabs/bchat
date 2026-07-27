package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/revrost/go-openrouter"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

// --- Benchmark structs (plan9-compatible) ---

type BenchmarkTurn struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	HasAnswer *bool  `json:"has_answer,omitempty"`
}

type BenchmarkQuestion struct {
	QuestionID      string      `json:"question_id"`
	QuestionType    string      `json:"question_type"`
	QuestionContent interface{} `json:"question_content"`
	HumanValidLabel bool        `json:"human_valid_label"`
}

type questionContent struct {
	Facts       []interface{} `json:"facts"`
	Question    string        `json:"question"`
	Answer      interface{}   `json:"answer"`
	Explanation string        `json:"explanation"`
}

func (q BenchmarkQuestion) content() questionContent {
	var c questionContent
	data, _ := json.Marshal(q.QuestionContent)
	_ = json.Unmarshal(data, &c)
	if c.Question == "" {
		fmt.Fprintf(os.Stderr, "WARN: question %s: content unmarshal produced empty question (data: %s)\n", q.QuestionID, string(data))
	}
	return c
}

func (q BenchmarkQuestion) AnswerString() string {
	c := q.content()
	switch v := c.Answer.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "yes"
		}
		return "no"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (q BenchmarkQuestion) QuestionString() string {
	c := q.content()
	return c.Question
}

type CacheEntry struct {
	QuestionID string          `json:"question_id"`
	Session    []BenchmarkTurn `json:"session,omitempty"`
	SessionOld []BenchmarkTurn `json:"session_old,omitempty"`
	SessionNew []BenchmarkTurn `json:"session_new,omitempty"`
	Session1   []BenchmarkTurn `json:"session_1,omitempty"`
	Session2   []BenchmarkTurn `json:"session_2,omitempty"`
}

type DryRunResult struct {
	QuestionID     string
	QuestionType   string
	Question       string
	ExpectedAnswer string
	ActualAnswer   string
	JudgeVerdict   string
	ObservationLog string
	ModelUsed      string
	Error          string
	Status         string // "pass", "fail", "skipped"
}

// --- Data loading ---

func loadBenchmarkDataDryRun(t *testing.T) ([]BenchmarkQuestion, map[string]CacheEntry) {
	t.Helper()

	questionPath := "/home/chaschel/Desktop/memory/custom_history_data/2_questions/0822_all_500_questions_final_v2.json"
	cachePath := "/home/chaschel/Desktop/memory/custom_history_data/6_session_cache/data_6_session_cache.json"

	// Load questions
	qData, err := os.ReadFile(questionPath)
	require.NoError(t, err, "failed to read question file at %s", questionPath)

	var allQuestions []BenchmarkQuestion
	require.NoError(t, json.Unmarshal(qData, &allQuestions), "failed to parse question file")

	// Load cache
	cData, err := os.ReadFile(cachePath)
	require.NoError(t, err, "failed to read cache file at %s", cachePath)

	var cacheList []CacheEntry
	require.NoError(t, json.Unmarshal(cData, &cacheList), "failed to parse cache file")

	cacheMap := make(map[string]CacheEntry, len(cacheList))
	for _, entry := range cacheList {
		cacheMap[entry.QuestionID] = entry
	}

	// Filter to testable types
	testableTypes := map[string]bool{
		"single_hop":              true,
		"implicit_preference_v2":  true,
		"knowledge_update":        true,
	}

	var filtered []BenchmarkQuestion
	for _, q := range allQuestions {
		if testableTypes[q.QuestionType] {
			filtered = append(filtered, q)
		}
	}

	t.Logf("Loaded %d questions (%d testable), %d cache entries", len(allQuestions), len(filtered), len(cacheMap))
	return filtered, cacheMap
}

// extractTurns extracts conversation turns from a cache entry.
func extractTurnsDryRun(entry CacheEntry) []BenchmarkTurn {
	if entry.Session != nil {
		return entry.Session
	}
	if entry.SessionOld != nil {
		turns := make([]BenchmarkTurn, 0, len(entry.SessionOld)+len(entry.SessionNew))
		turns = append(turns, entry.SessionOld...)
		turns = append(turns, entry.SessionNew...)
		return turns
	}
	if entry.Session1 != nil {
		return entry.Session1
	}
	return nil
}

// convertBenchmarkTurns converts BenchmarkTurns to AgentMessages for the observer.
func convertBenchmarkTurns(turns []BenchmarkTurn) []store.AgentMessage {
	msgs := make([]store.AgentMessage, 0, len(turns))
	baseTime := time.Now()
	for i, turn := range turns {
		msgs = append(msgs, store.AgentMessage{
			Role:      turn.Role,
			Content:   turn.Content,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		})
	}
	return msgs
}

// generateAnswerDryRun calls openrouter/free to generate an answer from the observation log.
func generateAnswerDryRun(t *testing.T, ctx context.Context, apiKey, obsLog, question string) (string, string, error) {
	t.Helper()
	client := newOpenRouterClient(apiKey)

	prompt := fmt.Sprintf(`You are an AI assistant answering a question based on your memory of past conversations.

## MEMORY (Observation Log)
%s

## QUESTION
%s

Answer concisely. If the memory does not contain enough information to answer, say "I don't have enough information to answer this question."`, obsLog, question)

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: "openrouter/free",
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage("You are a helpful assistant. Answer questions based on memory."),
			openrouter.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("answer LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", "", fmt.Errorf("no response from answer LLM")
	}
	return resp.Choices[0].Message.Content.Text, resp.Model, nil
}

// judgeAnswerDryRun calls openai/gpt-4o to judge if the answer is correct.
func judgeAnswerDryRun(t *testing.T, ctx context.Context, apiKey, question, expected, actual, questionID, questionType string) (string, error) {
	t.Helper()
	client := newOpenRouterClient(apiKey)

	var judgePrompt string
	switch {
	case strings.HasSuffix(questionID, "_abs"):
		judgePrompt = fmt.Sprintf(`You are evaluating a model's answer. Determine if the model correctly identified the question as unanswerable.
Question: %s
The correct response is that the question cannot be answered based on available information.
Model answer: %s
Did the model correctly refuse to answer? Answer "yes" if it identifies the question as unanswerable or says it doesn't have enough information. Answer "no" if it hallucinated an answer.`, question, actual)
	case questionType == "implicit_preference_v2":
		judgePrompt = fmt.Sprintf(`You are evaluating a model's answer on a preference/suggestion question.
Question: %s
Rubric (expected behavior): %s
Model answer: %s
Is the model's answer correct based on the rubric? Answer "yes" or "no".`, question, expected, actual)
	default:
		judgePrompt = fmt.Sprintf(`You are evaluating a model's answer. Determine if the answer is correct.
Question: %s
Correct answer: %s
Model answer: %s
Is the model's answer correct? Answer "yes" or "no".`, question, expected, actual)
	}

	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage("You are an evaluator. Answer only 'yes' or 'no'."),
			openrouter.UserMessage(judgePrompt),
		},
	})
	if err != nil {
		return "", fmt.Errorf("judge LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from judge LLM")
	}

	verdict := strings.ToLower(strings.TrimSpace(resp.Choices[0].Message.Content.Text))
	if verdict == "" {
		return "", fmt.Errorf("empty judge response")
	}
	if strings.Contains(verdict, "yes") {
		return "yes", nil
	}
	return "no", nil
}

// pickDryRunQuestions selects 4 questions: 1 per testable type + 1 abstention.
func pickDryRunQuestions(t *testing.T, questions []BenchmarkQuestion, cache map[string]CacheEntry) []BenchmarkQuestion {
	t.Helper()

	picked := make(map[string]bool)
	var result []BenchmarkQuestion

	// First pass: pick one per testable type (non-abs)
	for _, q := range questions {
		if picked[q.QuestionType] {
			continue
		}
		if _, ok := cache[q.QuestionID]; !ok {
			continue
		}
		turns := extractTurnsDryRun(cache[q.QuestionID])
		if turns == nil || len(turns) < 2 {
			continue
		}
		result = append(result, q)
		picked[q.QuestionType] = true
		if len(picked) >= 3 {
			break
		}
	}

	// Second pass: pick one abstention question
	for _, q := range questions {
		if !strings.HasSuffix(q.QuestionID, "_abs") {
			continue
		}
		if _, ok := cache[q.QuestionID]; !ok {
			continue
		}
		turns := extractTurnsDryRun(cache[q.QuestionID])
		if turns == nil || len(turns) < 2 {
			continue
		}
		result = append(result, q)
		break
	}

	t.Logf("Selected %d dry run questions: %v", len(result), func() []string {
		var ids []string
		for _, q := range result {
			ids = append(ids, fmt.Sprintf("%s (%s)", q.QuestionID, q.QuestionType))
		}
		return ids
	}())

	return result
}

// --- JSONL crash recovery helpers ---

type BenchResult struct {
	QuestionID     string `json:"question_id"`
	QuestionType   string `json:"question_type"`
	Question       string `json:"question"`
	ExpectedAnswer string `json:"expected_answer"`
	ActualAnswer   string `json:"actual_answer"`
	JudgeVerdict   string `json:"judge_verdict"`
	ModelUsed      string `json:"model_used"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	ObservationLog string `json:"observation_log,omitempty"`
}

func appendJSONL(path string, result BenchResult) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open JSONL: %w", err)
	}
	defer f.Close()
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write JSONL: %w", err)
	}
	return nil
}

func loadCompletedFromJSONL(path string) map[string]bool {
	completed := make(map[string]bool)
	data, err := os.ReadFile(path)
	if err != nil {
		return completed
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var r BenchResult
		err := dec.Decode(&r)
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		completed[r.QuestionID] = true
	}
	return completed
}

func readJSONL(t *testing.T, path string) []BenchResult {
	t.Helper()
	var results []BenchResult
	data, err := os.ReadFile(path)
	if err != nil {
		t.Logf("error reading %s: %v", filepath.Base(path), err)
		return results
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var r BenchResult
		err := dec.Decode(&r)
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		results = append(results, r)
	}
	return results
}

func countResults(results []BenchResult) (passed, failed, skipped int) {
	for _, r := range results {
		switch r.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "skipped":
			skipped++
		}
	}
	return
}

func benchmarkJSONLPath(qType string) string {
	return filepath.Join("build/benchmark", fmt.Sprintf("%s_%s.jsonl", qType, time.Now().Format("20060102")))
}

func clearBenchmarkJSONLs() {
	dir := "build/benchmark"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// --- Filter helpers ---

func filterByType(questions []BenchmarkQuestion, qType string, includeAbs bool) []BenchmarkQuestion {
	var result []BenchmarkQuestion
	for _, q := range questions {
		if q.QuestionType != qType {
			continue
		}
		if !includeAbs && strings.HasSuffix(q.QuestionID, "_abs") {
			continue
		}
		result = append(result, q)
	}
	return result
}

func filterAbstention(questions []BenchmarkQuestion) []BenchmarkQuestion {
	var result []BenchmarkQuestion
	for _, q := range questions {
		if strings.HasSuffix(q.QuestionID, "_abs") {
			result = append(result, q)
		}
	}
	return result
}

// --- Verbose output helpers ---

func printDryRunHeader(t *testing.T, idx, total int, q BenchmarkQuestion) {
	t.Helper()
	t.Logf("\n=== Dry Run: Question %d/%d ===", idx, total)
	t.Logf("Question ID:     %s", q.QuestionID)
	t.Logf("Question Type:   %s", q.QuestionType)
	t.Logf("Question:        %s", q.QuestionString())
}

func printDryRunTurns(t *testing.T, turns []BenchmarkTurn) {
	t.Helper()
	t.Logf("Input Turns (%d total):", len(turns))
	for i, turn := range turns {
		if i >= 10 {
			t.Logf("  ... (%d more turns)", len(turns)-10)
			break
		}
		content := turn.Content
		if len(content) > 120 {
			content = content[:120] + "..."
		}
		t.Logf("  [%d] %s: %q", i, turn.Role, content)
	}
}

func printDryRunResult(t *testing.T, result DryRunResult) {
	t.Helper()
	t.Logf("Observation Log:")
	if result.ObservationLog == "" {
		t.Logf("  (empty)")
	} else {
		lines := strings.Split(result.ObservationLog, "\n")
		for i, line := range lines {
			if i >= 15 {
				t.Logf("  ... (%d more lines)", len(lines)-15)
				break
			}
			t.Logf("  %s", line)
		}
	}
	t.Logf("Generated Answer: %s", result.ActualAnswer)
	t.Logf("Expected Answer:  %s", result.ExpectedAnswer)
	t.Logf("Judge Verdict:    %s", result.JudgeVerdict)
	t.Logf("Model Used:       %s", result.ModelUsed)
	if result.Error != "" {
		t.Logf("Error:            %s", result.Error)
	}
	t.Logf("Status:           %s", result.Status)
}

// --- Report writing ---

func writeDryRunReport(t *testing.T, results []DryRunResult) {
	t.Helper()

	dir := "build/benchmark"
	require.NoError(t, os.MkdirAll(dir, 0755), "failed to create benchmark directory")

	ts := time.Now().Format("20060102_150405")
	path := filepath.Join(dir, fmt.Sprintf("dryrun_%s.txt", ts))

	var sb strings.Builder
	sb.WriteString("=== Dry Run Report ===\n")
	sb.WriteString(fmt.Sprintf("Date: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Questions: %d\n\n", len(results)))

	passed, failed, skipped := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "skipped":
			skipped++
		}
	}

	sb.WriteString(fmt.Sprintf("Summary: %d passed, %d failed, %d skipped\n\n", passed, failed, skipped))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("--- Question %d/%d ---\n", i+1, len(results)))
		sb.WriteString(fmt.Sprintf("ID:       %s\n", r.QuestionID))
		sb.WriteString(fmt.Sprintf("Type:     %s\n", r.QuestionType))
		sb.WriteString(fmt.Sprintf("Question: %s\n", r.Question))
		sb.WriteString(fmt.Sprintf("Expected: %s\n", r.ExpectedAnswer))
		sb.WriteString(fmt.Sprintf("Actual:   %s\n", r.ActualAnswer))
		sb.WriteString(fmt.Sprintf("Verdict:  %s\n", r.JudgeVerdict))
		sb.WriteString(fmt.Sprintf("Model:    %s\n", r.ModelUsed))
		sb.WriteString(fmt.Sprintf("Status:   %s\n", r.Status))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("Error:    %s\n", r.Error))
		}
		if r.ObservationLog != "" {
			sb.WriteString(fmt.Sprintf("ObsLog:\n%s\n", r.ObservationLog))
		}
		sb.WriteString("\n")
	}

	require.NoError(t, os.WriteFile(path, []byte(sb.String()), 0644), "failed to write report")
	t.Logf("Report written to %s", path)
}

// --- Main dry run test ---

func TestBenchmarkLongMemEvalDryRun(t *testing.T) {
	if os.Getenv("BENCHMARK_LONGMEMEVAL") != "true" {
		t.Skip("Set BENCHMARK_LONGMEMEVAL=true to run the dry run")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	require.NotEmpty(t, apiKey, "OPENROUTER_API_KEY required")

	if os.Getenv("BENCHMARK_FRESH") == "true" {
		clearBenchmarkJSONLs()
		t.Log("Cleared existing JSONL files (BENCHMARK_FRESH=true)")
	}

	questions, cache := loadBenchmarkDataDryRun(t)
	selected := pickDryRunQuestions(t, questions, cache)
	require.Len(t, selected, 4, "expected 4 dry run questions (3 types + 1 abstention)")

	var results []DryRunResult

	for i, q := range selected {
		printDryRunHeader(t, i+1, len(selected), q)

		entry, ok := cache[q.QuestionID]
		if !ok {
			t.Logf("  SKIP: no cache entry")
			results = append(results, DryRunResult{
				QuestionID:   q.QuestionID,
				QuestionType: q.QuestionType,
				Question:     q.QuestionString(),
				Status:       "skipped",
				Error:        "no cache entry",
			})
			continue
		}

		turns := extractTurnsDryRun(entry)
		if turns == nil || len(turns) < 2 {
			t.Logf("  SKIP: insufficient turns (%d)", len(turns))
			results = append(results, DryRunResult{
				QuestionID:   q.QuestionID,
				QuestionType: q.QuestionType,
				Question:     q.QuestionString(),
				Status:       "skipped",
				Error:        fmt.Sprintf("insufficient turns: %d", len(turns)),
			})
			continue
		}

		printDryRunTurns(t, turns)

		// Run observer pipeline
		msgs := convertBenchmarkTurns(turns)
		ctx, ts, service, tenant := newObserverTestService(t, fmt.Sprintf("dryrun-%s", q.QuestionID))
		defer ts.Close()

		setOMEnvAndReload(t, "OM_SCOPE", "resource")
		setOMEnvAndReload(t, "OM_ENABLED", "true")

		sid := createTestSession(t, ctx, ts, service, tenant.ID, fmt.Sprintf("dryrun-%s", q.QuestionID), msgs)
		var userID int32 = 999
		memSession := service.memorySessions.Get(tenant.ID, sid)
		memSession.UserID = &userID
		service.memorySessions.Update(memSession)

		err := service.RunObserver(ctx, tenant.ID, sid)
		if err != nil {
			t.Logf("  Observer error: %v", err)
			results = append(results, DryRunResult{
				QuestionID:   q.QuestionID,
				QuestionType: q.QuestionType,
				Question:     q.QuestionString(),
				Status:       "skipped",
				Error:        fmt.Sprintf("observer error: %v", err),
			})
			continue
		}

		obsLog, err := ts.GetObservationLog(ctx, sid)
		if err != nil || obsLog == nil || obsLog.ObservationLog == "" {
			t.Logf("  SKIP: no observation log produced")
			results = append(results, DryRunResult{
				QuestionID:   q.QuestionID,
				QuestionType: q.QuestionType,
				Question:     q.QuestionString(),
				Status:       "skipped",
				Error:        "no observation log produced",
			})
			continue
		}

		result := DryRunResult{
			QuestionID:     q.QuestionID,
			QuestionType:   q.QuestionType,
			Question:       q.QuestionString(),
			ExpectedAnswer: q.AnswerString(),
			ObservationLog: obsLog.ObservationLog,
			Status:         "pending",
		}

		// Generate answer
		answer, model, err := generateAnswerDryRun(t, ctx, apiKey, obsLog.ObservationLog, q.QuestionString())
		if err != nil {
			t.Logf("  Answer error: %v", err)
			result.Error = fmt.Sprintf("answer error: %v", err)
			result.Status = "skipped"
			printDryRunResult(t, result)
			results = append(results, result)
			continue
		}
		result.ActualAnswer = answer
		result.ModelUsed = model

		// Judge answer
		verdict, err := judgeAnswerDryRun(t, ctx, apiKey, q.QuestionString(), result.ExpectedAnswer, answer, q.QuestionID, q.QuestionType)
		if err != nil {
			t.Logf("  Judge error: %v", err)
			result.Error = fmt.Sprintf("judge error: %v", err)
			result.Status = "skipped"
			printDryRunResult(t, result)
			results = append(results, result)
			continue
		}
		result.JudgeVerdict = verdict
		if verdict == "yes" {
			result.Status = "pass"
		} else {
			result.Status = "fail"
		}

		printDryRunResult(t, result)
		results = append(results, result)
	}

	writeDryRunReport(t, results)

	// Summary
	passed, failed, skipped := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "skipped":
			skipped++
		}
	}
	t.Logf("\n=== Dry Run Summary ===")
	t.Logf("Total: %d | Passed: %d | Failed: %d | Skipped: %d", len(results), passed, failed, skipped)
}

// --- Per-type benchmark tests ---

func runBenchmarkQuestion(t *testing.T, apiKey, sessionID string, q BenchmarkQuestion, cacheEntry CacheEntry) BenchResult {
	t.Helper()

	result := BenchResult{
		QuestionID:     q.QuestionID,
		QuestionType:   q.QuestionType,
		Question:       q.QuestionString(),
		ExpectedAnswer: q.AnswerString(),
		Status:         "pending",
	}

	turns := extractTurnsDryRun(cacheEntry)
	if turns == nil || len(turns) < 2 {
		result.Status = "skipped"
		result.Error = fmt.Sprintf("insufficient turns: %d", len(turns))
		return result
	}

	msgs := convertBenchmarkTurns(turns)
	ctx, ts, service, tenant := newObserverTestService(t, sessionID)
	defer ts.Close()

	setOMEnvAndReload(t, "OM_SCOPE", "resource")
	setOMEnvAndReload(t, "OM_ENABLED", "true")

	sid := createTestSession(t, ctx, ts, service, tenant.ID, sessionID, msgs)
	var userID int32 = 999
	memSession := service.memorySessions.Get(tenant.ID, sid)
	memSession.UserID = &userID
	service.memorySessions.Update(memSession)

	if err := service.RunObserver(ctx, tenant.ID, sid); err != nil {
		result.Status = "skipped"
		result.Error = fmt.Sprintf("observer error: %v", err)
		return result
	}

	obsLog, err := ts.GetObservationLog(ctx, sid)
	if err != nil || obsLog == nil || obsLog.ObservationLog == "" {
		result.Status = "skipped"
		result.Error = "no observation log produced"
		return result
	}
	result.ObservationLog = obsLog.ObservationLog

	answer, model, err := generateAnswerDryRun(t, ctx, apiKey, obsLog.ObservationLog, q.QuestionString())
	if err != nil {
		result.Status = "skipped"
		result.Error = fmt.Sprintf("answer error: %v", err)
		return result
	}
	result.ActualAnswer = answer
	result.ModelUsed = model

	verdict, err := judgeAnswerDryRun(t, ctx, apiKey, q.QuestionString(), result.ExpectedAnswer, answer, q.QuestionID, q.QuestionType)
	if err != nil {
		result.Status = "skipped"
		result.Error = fmt.Sprintf("judge error: %v", err)
		return result
	}
	result.JudgeVerdict = verdict
	if verdict == "yes" {
		result.Status = "pass"
	} else {
		result.Status = "fail"
	}

	return result
}

func runPerTypeBenchmark(t *testing.T, qType, sessionPrefix string, filterFn func([]BenchmarkQuestion) []BenchmarkQuestion) {
	t.Helper()

	require.NoError(t, os.MkdirAll("build/benchmark", 0755), "failed to create benchmark directory")

	if os.Getenv("BENCHMARK_LONGMEMEVAL") != "true" {
		t.Skip("Set BENCHMARK_LONGMEMEVAL=true")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	require.NotEmpty(t, apiKey)

	if os.Getenv("BENCHMARK_FRESH") == "true" {
		clearBenchmarkJSONLs()
		t.Log("Cleared existing JSONL files (BENCHMARK_FRESH=true)")
	}

	questions, cache := loadBenchmarkDataDryRun(t)
	typeQuestions := filterFn(questions)
	t.Logf("%s: %d questions (non-abs)", qType, len(typeQuestions))

	jsonlPath := benchmarkJSONLPath(qType)
	completed := loadCompletedFromJSONL(jsonlPath)

	var results []BenchResult
	for i, q := range typeQuestions {
		if completed[q.QuestionID] {
			t.Logf("[%d/%d] SKIP %s (already completed)", i+1, len(typeQuestions), q.QuestionID)
			continue
		}
		entry, ok := cache[q.QuestionID]
		if !ok {
			t.Logf("[%d/%d] SKIP %s (no cache)", i+1, len(typeQuestions), q.QuestionID)
			continue
		}

		result := runBenchmarkQuestion(t, apiKey, fmt.Sprintf("%s-%s", sessionPrefix, q.QuestionID), q, entry)
		results = append(results, result)
		if err := appendJSONL(jsonlPath, result); err != nil {
			t.Logf("WARN: failed to write JSONL: %v", err)
		}
		t.Logf("[%d/%d] %s — %s", i+1, len(typeQuestions), q.QuestionID, result.JudgeVerdict)
	}

	passed, failed, skipped := countResults(results)
	t.Logf("=== %s Summary: %d passed, %d failed, %d skipped ===", qType, passed, failed, skipped)
}

func TestBenchmarkSingleHop(t *testing.T) {
	runPerTypeBenchmark(t, "single_hop", "sh", func(qs []BenchmarkQuestion) []BenchmarkQuestion {
		return filterByType(qs, "single_hop", false)
	})
}

func TestBenchmarkPreference(t *testing.T) {
	runPerTypeBenchmark(t, "implicit_preference_v2", "pref", func(qs []BenchmarkQuestion) []BenchmarkQuestion {
		return filterByType(qs, "implicit_preference_v2", false)
	})
}

func TestBenchmarkKnowledgeUpdate(t *testing.T) {
	runPerTypeBenchmark(t, "knowledge_update", "ku", func(qs []BenchmarkQuestion) []BenchmarkQuestion {
		return filterByType(qs, "knowledge_update", false)
	})
}

func TestBenchmarkAbstention(t *testing.T) {
	runPerTypeBenchmark(t, "abstention", "abs", filterAbstention)
}

func TestBenchmarkAggregate(t *testing.T) {
	types := []string{"single_hop", "implicit_preference_v2", "knowledge_update", "abstention"}
	typeLabels := map[string]string{
		"single_hop":              "SingleHop",
		"implicit_preference_v2":  "Preference",
		"knowledge_update":        "KnowledgeUpdate",
		"abstention":              "Abstention",
	}

	var allResults []BenchResult
	for _, qType := range types {
		jsonlPath := benchmarkJSONLPath(qType)
		results := readJSONL(t, jsonlPath)
		passed, _, _ := countResults(results)
		label := typeLabels[qType]
		if len(results) > 0 {
			t.Logf("%-20s %d/%d passed (%.1f%%)", label, passed, len(results), 100*float64(passed)/float64(len(results)))
		} else {
			t.Logf("%-20s (no results)", label)
		}
		allResults = append(allResults, results...)
	}

	uniqueTotal := len(allResults)
	passed, _, _ := countResults(allResults)
	if uniqueTotal > 0 {
		t.Logf("--- Overall: %d/%d unique questions passed (%.1f%%) ---", passed, uniqueTotal, 100*float64(passed)/float64(uniqueTotal))
	} else {
		t.Logf("--- Overall: no results (run per-type tests first) ---")
	}
}
