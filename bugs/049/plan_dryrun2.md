# Plan Dry Run 2: Add Answer Generation + Judge

**Date:** 2026-07-28
**Depends on:** plan_dryrun_assess.md, plan_dryrun.md
**Status:** PLAN (ready to implement)

---

## Dry Run 1 Results (Baseline)

```
Date: 2026-07-28 00:43:50
Duration: 63.1s
Questions: 4
```

| # | ID | Type | Observer | Answer | Judge | Status |
|---|-----|------|----------|--------|-------|--------|
| 1 | 6a1eabeb | knowledge_update | ✅ | ❌ empty | ❌ empty | pending |
| 2 | e47becba | single_hop | ✅ | ❌ empty | ❌ empty | pending |
| 3 | 8a2466db | implicit_preference_v2 | ✅ | ❌ empty | ❌ empty | pending |
| 4 | 0862e8bf_abs | single_hop (abstention) | ✅ | ❌ empty | ❌ empty | pending |

### Observer quality verified:

**Q1 (knowledge_update):** "User recently set a personal best time in a 5K charity run" — ✅ captured
**Q2 (single_hop):** "User graduated with a degree in Business Administration" — ✅ captured
**Q3 (implicit_preference_v2):** "User enjoys Adobe Premiere Pro, prefers Lumetri Color Panel" — ✅ captured
**Q4 (abstention):** Cat info only, no hamster hallucination — ✅ correct

### What's missing:
- Answer generation: call `openrouter/free` with obs log + question → answer
- Judge: call `openai/gpt-4o` with question + expected + actual → yes/no

**Gold baseline excluded** per assessment review (N=1 is noise-dominated with `openrouter/free` variance). Full 10-question baseline in plan9's `TestBenchmarkLongMemEval` serves as the real gate.

---

## Implementation Plan

### 1. Add `generateAnswerDryRun` function

Calls `openrouter/free` with observation log + question → returns answer string.

```go
func generateAnswerDryRun(t *testing.T, ctx context.Context, apiKey, obsLog, question string) (string, string, error) {
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
```

### 2. Add `judgeAnswerDryRun` function

Calls `openai/gpt-4o` with question + expected + actual → "yes" or "no".

Three prompt variants (from plan8.md):
- **Standard:** simple correctness
- **Preference:** rubric-based (for `implicit_preference_v2`)
- **Abstention:** check refused-to-answer (for `_abs` suffix)

```go
func judgeAnswerDryRun(t *testing.T, ctx context.Context, apiKey, question, expected, actual, questionID, questionType string) (string, error) {
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
    if strings.Contains(verdict, "yes") {
        return "yes", nil
    }
    return "no", nil
}
```

### 3. Update `TestBenchmarkLongMemEvalDryRun`

After observer step, add:
1. Call `generateAnswerDryRun` → populate `ActualAnswer` and `ModelUsed`
2. Call `judgeAnswerDryRun` → populate `JudgeVerdict`
3. Set `Status` to "pass" or "fail" based on verdict
4. Handle failures gracefully (log, set status="skipped", continue)

### 4. Updated verbose output

Add to `printDryRunResult`:
```
Generated Answer: <actual>
Expected Answer:  <expected>
Judge Verdict:    <verdict>
Model Used:       <model>
Status:           pass/fail/skipped
```

### 5. Updated report

The report file already handles all fields — no changes needed.

---

## API Calls

| Call | Model | Count | Cost |
|------|-------|-------|------|
| Observer | openrouter/free | 4 | $0 |
| Answer | openrouter/free | 4 | $0 |
| Judge | openai/gpt-4o | 4 | ~$0.008 |
| **Total** | | 12 | **~$0.008** |

---

## Files Changed

| File | Change | Lines |
|------|--------|-------|
| `server/.../agent/benchmark_longmemeval_test.go` | Add `generateAnswerDryRun`, `judgeAnswerDryRun`, update test loop | +60 |

---

## Expected Results

Based on observer quality from dry run 1:

| # | Type | Observer | Expected |
|---|------|----------|----------|
| 1 | knowledge_update | ✅ 5K time captured | Pass — answer should be "25:50" |
| 2 | single_hop | ✅ degree captured | Pass — answer should be "Business Administration" |
| 3 | implicit_preference_v2 | ✅ Premiere Pro prefs | Pass — should recommend Premiere Pro resources |
| 4 | abstention | ✅ no hamster hallucination | Pass — should refuse to answer (judge omits expected answer for abstention) |

Target: 4/4 pass (100%) on this small sample.

---

## Run Command

```bash
cd /home/chaschel/Documents/go/bchat && \
BENCHMARK_LONGMEMEVAL=true OPENROUTER_API_KEY=sk-or-v1-... \
go test ./server/router/api/v1/agent/ -run TestBenchmarkLongMemEvalDryRun \
-v -count=1 -timeout=10m
```

Estimated time: ~2-3 min for answer + judge only (observer already verified in dry run 1). If full re-run including observers: ~3-4 min total (observers are rate-limited at ~16s/call based on dry run 1 measurement).
