Observational Memory: 95% on LongMemEval
How we built a human-inspired memory system that achieves the highest score ever recorded on LongMemEval

Tyler Barnes
Tyler Barnes
·
Feb 9, 2026

·
15 min read


On this page
Toggle
Observational Memory (OM), a new memory system developed by Mastra, achieves SOTA on LongMemEval with an 84.23% gpt-4o score — outperforming the oracle (a configuration given only the conversations containing the answer). With gpt-5-mini, it scores 94.87% — the highest score ever recorded on this benchmark, by any system, with any model.

We achieved these scores with a completely stable context window. Most memory systems change the prompt every turn by injecting dynamically retrieved context; OM doesn't. The context window is predictable, reproducible, and prompt-cacheable across many agent/user turns.

The best part: it's completely open source end to end.

Read the docs at https://mastra.ai/docs/memory/observational-memory

View the benchmark runner →

View the implementation →

What is Observational Memory?
Two background agents — an Observer and a Reflector — watch your agent's conversations and maintain a dense observation log that replaces raw message history as it grows. Think of them as the subconscious mind of the main agent: always running, never interrupting. The main agent sees these observations in its context and uses them to decide what to do next — without explicitly writing to or querying from memory.

The context window has two sections: memory (observations) at the start, and message history (the active conversation) at the end. As message history grows, the Observer converts raw messages into dense observations — then the original messages are dropped and replaced with those observations.

The result: a context window that's predictable, low-latency, and fully prompt-cacheable. No per-turn dynamic retrieval based on the user prompt. No dynamic injection. Just a stable log of what happened.

Results
All results use the longmemeval_s dataset: 500 questions across ~57M tokens of conversation data. Each question has ~50 sessions attached, and the memory system must give the model the ability to correctly answer questions across all sessions.

We tested three models. gpt-4o is the official benchmark model, used to compare against other systems. gemini-3-pro and gpt-5-mini results are shared to show how OM performs with newer models.

In each configuration, gemini-2.5-flash ingested the data into OM as it's the default model for observation/reflection currently. It ran during data ingestion into the memory system, not during the eval itself, so the Actor model is the only variable between runs.

gpt-4o comparison
gpt-4o overall comparison: OM vs Supermemory vs Zep

gpt-4o per-category comparison: OM vs Supermemory vs Zep

Dataset: longmemeval_s
Model: gpt-4o (Actor) / gemini-2.5-flash (Observer, Reflector)
────────────────────────────────────────────────────
Accuracy by Question Type:
  knowledge-update          85.9% [█████████████████░░░] (67/78)
  multi-session             79.7% [████████████████░░░░] (106/133)
  single-session-assistant  82.1% [████████████████░░░░] (46/56)
  single-session-preference 73.3% [███████████████░░░░░] (22/30)
  single-session-user       98.6% [████████████████████] (69/70)
  temporal-reasoning        85.7% [█████████████████░░░] (114/133)
 
Overall Accuracy: 84.23%

OM with gpt-4o beats the full-context baseline by 24 points. It also beats the oracle by 2 points (84.23% vs 82.4%). The oracle is a configuration where the model is given only the specific ~1-3 conversations containing the answer, instead of all ~50 conversations like OM has in context. Admittedly the delta here is small enough to be within run variance, but even matching the oracle would be notable: OM ingested all ~50 conversations per question, yet its observations are at least as useful to the model as the raw data filtered to only contain the answers.

Put simply: The oracle is limited to its 1-3 sessions. OM has all 50 statically in memory and could answer additional questions about any of them — at the same accuracy and with no additional context retrieval needed.

gemini-3-pro comparison
gemini-3-pro overall comparison: OM vs Supermemory

gemini-3-pro per-category comparison: OM vs Supermemory

Dataset: longmemeval_s
Model: gemini-3-pro-preview (Actor) / gemini-2.5-flash (Observer, Reflector)
────────────────────────────────────────────────────
Accuracy by Question Type:
  knowledge-update          94.9% [███████████████████░] (74/78)
  multi-session             87.2% [█████████████████░░░] (116/133)
  single-session-assistant  96.4% [███████████████████░] (54/56)
  single-session-preference 90.0% [██████████████████░░] (27/30)
  single-session-user       97.1% [███████████████████░] (68/70)
  temporal-reasoning        94.0% [███████████████████░] (125/133)
 
Overall Accuracy: 93.27%

gemini-3-pro jumps nearly 10 points over gpt-4o, showing OM's architecture scales well with the capabilities of newer models.

gpt-5-mini comparison
gpt-5-mini overall comparison: OM vs Supermemory

gpt-5-mini per-category comparison: OM vs Supermemory

Dataset: longmemeval_s
Model: gpt-5-mini (Actor) / gemini-2.5-flash (Observer, Reflector)
────────────────────────────────────────────────────
Accuracy by Question Type:
  knowledge-update          96.2% [███████████████████░] (75/78)
  multi-session             87.2% [█████████████████░░░] (116/133)
  single-session-assistant  94.6% [███████████████████░] (53/56)
  single-session-preference 100.0% [████████████████████] (30/30)
  single-session-user       95.7% [███████████████████░] (67/70)
  temporal-reasoning        95.5% [███████████████████░] (127/133)
 
Overall Accuracy: 94.87%

gpt-5-mini achieves 94.87% — the highest LongMemEval score on record. Perfect 100% on single-session-preference. The hardest categories are multi-session, temporal-reasoning, and knowledge-update. OM scores 87.2%, 95.5%, and 96.2% respectively — outperforming prior systems in temporal-reasoning and knowledge-update, and matching the best reported multi-session score.

Leaderboard
System	Model	Overall
Mastra OM	gpt-5-mini	94.87%
Mastra OM	gemini-3-pro-preview	93.27%
Hindsight	gemini-3-pro-preview	91.40%
Mastra OM	gemini-3-flash-preview	89.20%
Hindsight	GPT-OSS 120B	89.00%
EmergenceMem Internal*	gpt-4o	86.00%
Supermemory	gemini-3-pro-preview	85.20%
Supermemory	gpt-5	84.60%
Mastra OM	gpt-4o	84.23%
Hindsight	GPT-OSS 20B	83.60%
EmergenceMem Simple	gpt-4o	82.40%
Oracle	gpt-4o	82.40%
Supermemory	gpt-4o	81.60%
Mastra RAG (topK 20)	gpt-4o	80.05%
Zep	gpt-4o	71.20%
Full context	gpt-4o	60.20%
* EmergenceMem's 86.00% is reported for an "Internal" configuration and is not publicly reproducible. Public configs include Simple (82.40%) and Simple Fast (79.00%).

Key findings:

OM with gpt-4o (84.23%) is the highest openly reproducible score that uses the official benchmark model, gpt-4o. EmergenceMem internal is a closed implementation that can't be reproduced, and also uses multi-step reranking (slower, more expensive).
OM beats the oracle. OM's observations are more useful to the model than the raw correct data.
OM outperforms Hindsight's best by 3.5 points. Hindsight uses four parallel retrieval strategies with neural reranking. OM achieves higher scores with a single pass and no additional retrieval steps.
OM is unusual in maintaining a stable, cacheable prompt. Other systems in the leaderboard dynamically inject retrieved content each turn, invalidating prompt caching.
Scaling
OM's architecture scales with model quality:

gpt-4o                  84.23%  ████████████████░░░░
gemini-3-flash-preview  89.20%  ██████████████████░░
gemini-3-pro-preview    93.27%  ███████████████████░
gpt-5-mini              94.87%  ███████████████████░
Compare the scaling rates:

Supermemory went from 81.6% (gpt-4o) to 85.2% (gemini-3-pro-preview) — a 3.6 point gain.
Observational Memory went from 84.23% (gpt-4o) to 93.27% (gemini-3-pro-preview) — a 9 point gain.
Better models extract more value from the same structured observations. This is likely due to gpt-4o (an older model), not doing as well with dense information.

Context window economics
Because OM's context window is append-only and stable, the prefix (system prompt + observations) doesn't change between turns. This means high prompt cache hit rates. Many model providers reduce token costs by 4–10× vs uncached prompts.

OM also makes smaller context windows behave like much larger ones. The average context window size for the entire LongMemEval benchmark run was ~30k tokens.

Methodology: Observational Memory architecture
OM manages the agent's context window with a three-tier representation of information, each progressively more compressed.

1. Three-tier system
Message history is the raw conversation — what the agent just said and what the user just asked. It's recent and relevant, but it grows with every turn.

Observations are made by a background Observer agent that watches the conversation. When message history reaches a token threshold, the Observer processes those messages into concise, dense notes, which then replace the messages they observed. For text-only content, we see 3–6× compression (around 6× in our LongMemEval runs). For tool-call-heavy workloads, we've anecdotally seen 5–40× — the noisier the tool output, the higher the compression ratio.

Reflections happen when observations accumulate past a second token threshold. A Reflector agent restructures and condenses the observations: combining related items, reflecting on overarching patterns, and dropping context that's no longer relevant.

2. The Observer
The Observer watches the conversation and produces a structured list of dated observations, each capturing a specific event: a user statement, an agent action, a tool call result, a preference expressed in passing. Each observation is tagged with a priority and a date.

The Observer also tracks:

Current task — what the agent is working on right now
Suggested response — how the agent should respond next (a continuity mechanism, not a benchmark-affecting feature)
Because observations are append-only (until reflection), the context window is stable and cacheable.

3. The Reflector
When observations grow past a token threshold, the Reflector takes all existing observations and produces a restructured, condensed version: combining related items, reflecting on patterns, and removing observations that have been superseded.

4. Temporal anchoring
Every observation carries up to three dates:

Observation date — when the observation was created
Referenced date — the date mentioned in the content itself ("my flight is January 31")
Relative date — a computed relative offset ("2 days from today")
This three-date model is critical for temporal reasoning — one of the hardest LongMemEval categories, where OM with gpt-5-mini scores 95.5%.

5. Token budgets
OM triggers on token counts, not time intervals or message counts. Two thresholds control when each agent runs: one for unobserved message history (triggering the Observer), and one for total observation size (triggering the Reflector).

6. Observation format
Observations are formatted text, not structured objects. The format has specific properties:

Two-level bulleted lists — top-level bullets are tasks or events, sub-bullets capture details
Emoji-based prioritization — 🔴 high, 🟡 medium, 🟢 low, these are signals from the observer to the reflector, so it has a picture of what the observer thought was really important at the time
Titles and timestamps — observations are grouped by date with titled sections
How to reproduce
Observational Memory implementation
LongMemEval runner code
Evaluation methodology:

LongMemEval provides question-specific evaluation prompts
gpt-4o is used as the judge model
Each question is evaluated as correct/incorrect (binary)
Overall accuracy is the unweighted average across all 6 categories
All conversation data is processed sequentially in chronological order
Each configuration was run multiple times during development. Scores were generally stable across runs; the results published here are from the latest run of each configuration.

Limitations
Where OM struggles
Multi-session is the hardest category. Multi-session questions require synthesizing information scattered across multiple conversations. OM's best multi-session score (87.2%) ties Hindsight's best. This appears to be a ceiling that current systems hit.

System	Model	Multi-session
Mastra OM	gpt-5-mini	87.2%
Mastra OM	gemini-3-pro-preview	87.2%
Hindsight	gemini-3-pro-preview	87.2%
Supermemory	gpt-4o	73.0%
Single-session-preference is volatile. This category has only 30 questions, making it statistically noisy. OM's scores swing from 73.3% (gpt-4o) to 100% (gpt-5-mini). A single question flip moves the score by 3.3 percentage points.

gpt-4o shows clear per-category gaps compared to gpt-5-mini — especially on temporal-reasoning (+9.8), single-session-preference (+26.7), and single-session-assistant (+12.5).

Backstory
The idea for OM came from thinking about how human memory works. I don't remember every word of every conversation I've had. I have a kind of subconscious log — a compressed, structured record of what happened. My subconscious observes what's happening from moment to moment, without me choosing to. Over time, my brain reflects on those observations: combining, restructuring, creating connections, and discarding what's no longer relevant.

That's the model. What if an AI agent worked the same way?

I've been using OM, via a custom coding agent harness that implemented it, for all of own work (well any where I used a coding agent) since October 2025. Months of daily use informed the architecture decisions — the three-date model, the emoji priorities, the formatted append only text structure. So these weren't theoretical choices; they're what worked in practice. My initial OM prototype was used as the memory system for the coding agent that built the productionized version that shipped in Mastra. So OM was used to build itself, and the agent was aware it was using this memory system in order to build the memory system. It often used debug logs from its own execution (and my guidance) to validate ideas and fix bugs.

Why LongMemEval?
LongMemEval is a benchmark designed to test how well AI systems handle long-term memory across multiple conversations. It tests the kind of memory that matters in real applications: remembering what users said weeks ago, tracking preference changes over time, and reasoning about when things happened.

The benchmark contains 500 questions, each with ~50 conversations attached. The questions span 6 categories:

single-session-user — Can the system recall what the user said in a specific past conversation?
single-session-assistant — Can it recall what the assistant said or produced?
single-session-preference — Can it remember user preferences stated in passing?
knowledge-update — When a user's information changes (new job, moved cities), can the system track the update?
temporal-reasoning — Can it resolve relative time ("two weeks later"), order events correctly, and answer using the latest state?
multi-session — Can it synthesize information scattered across multiple conversations?
Each category tests a distinct aspect of long-term memory. Many of the questions in the harder question categories are difficult questions to answer. A system that scores well across all six is doing something fundamentally right.

Why not LoCoMo?
Another popular memory benchmark, LoCoMo, might seem like a natural choice. We deliberately chose not to publish LoCoMo results, and we're not alone — Supermemory also publishes only LongMemEval results.

Here's why:

F1 similarity scoring is unreliable. LoCoMo uses word-overlap F1 scoring, which penalizes answers that include additional context even when they're correct.
No standardized LLM-as-judge evaluation criteria. Everyone who publishes LoCoMo results creates their own LLM-as-judge prompts. Without a standard judge, results aren't comparable. The same systems ran with different judge prompts produce drastically differing scores. We tested a few of the judge prompts we found from previous shared results, and the scoring difference was ~10%.
Inconsistent result representation. Some published results omit categories, which makes cross-system comparisons hard. (See this example from the Zep blog.)
LongMemEval uses LLM-as-judge evaluation with question-specific prompts provided by the benchmark itself, making results reproducible and comparable across all shared results.

Isn't this compaction?
Compaction — condensing message history into a shorter summary — is the most common approach to managing long context. But compaction reads more like documentation than memory: it captures the gist of what happened, losing the specific events, decisions, and details along the way.

OM takes a different approach. The Observer produces an event-based decision log — a structured list of dated, prioritized observations about what specifically happened, what was decided, and what changed. It runs more frequently, in smaller increments, which means each observation cycle processes less context and compresses it more efficiently.

This matters most when agents make a lot of tool calls. Tool call results (file contents, API responses, browser page dumps) are often huge and full of context rot — tokens that were useful for one step but are noise for every subsequent turn. For text-only content like LongMemEval's conversational data, we see 3–6× compression (around 6× in our benchmark runs). For tool-call-heavy agent workloads, we've anecdotally seen 5–40× compression — the noisier the tool output, the higher the ratio.

Compaction is a one time summarization that happens when the context window is about to overflow. OM is an append only log detailing what happened, as it happens. A full summarization never runs. Even during reflection, the observation block is only rewritten to find connections, and drop redundant data to save space, not to summarize messages.

Put simply, OM is compact event logging for LLM/human actions (with periodic reflection), compaction is bulk unstructured message summarization to avoid context overflow.

Conclusion
Observational Memory achieves the highest score ever recorded on LongMemEval — 94.87% with gpt-5-mini — while maintaining a completely stable, cacheable context window. It beats the oracle, outperforms complex multi-step reranking systems with a single pass, and scales better with model quality than existing approaches.

The architecture is simple: two background agents (Observer and Reflector) watch the conversation and maintain a dense text-only observation log that replaces raw message history as it grows. The context window stays bounded, predictable, and prompt-cacheable.

OM is open source and available now. Read the docs to get started.

Citations
Wu, X., et al. "LongMemEval: Benchmarking Chat Assistants on Long-Term Interactive Memory." arXiv:2410.10813, 2024.
Liu, N., et al. "Lost in the Middle: How Language Models Use Long Contexts." arXiv:2307.03172, 2023.
Maharana, A., et al. "LoCoMo: Long Context Multi-Turn Benchmark." arXiv:2402.09076, 2024.
Raad, G., et al. "Zep: A Temporal Knowledge Graph Architecture for Agent Memory." arXiv:2501.13956, 2025.
EmergenceMem: "Emergence is the new SOTA in agent memory." Emergence AI Blog.
Supermemory Research. Supermemory.
Zep: "Lies, Damn Lies, & Statistics: Is Mem0 Really SOTA in Agent Memory?" Zep Blog.
Kedia, A., et al. "Hindsight: A Biomimetic Memory Architecture for Agents." arXiv:2512.12818, 2025.
