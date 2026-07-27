# RAG vs OM — When to Use What

**Purpose:** Clarify the relationship between RAG (Retrieval-Augmented Generation) and OM (Observational Memory) in bchat, and when to use each.

---

## The Two Systems

| | OM (Observational Memory) | RAG (Retrieval-Augmented Generation) |
|---|---|---|
| **Source** | Conversation history (sessions) | Uploaded documents (KB.MD, POLICY.MD) |
| **Question answered** | "What did the user say before?" | "What services do we offer?" |
| **Type of knowledge** | Temporal/conversational | Factual/documentary |
| **Storage** | Observation logs (compressed) | Vector embeddings (semantic search) |
| **Retrieval method** | Observer compresses sessions into summary | Hybrid search (vector + BM25) over document chunks |
| **Tested by** | LongMemEval benchmark | RAG pipeline tests |
| **Config** | `OM_ENABLED=true` | `RAG_PIPELINE_ENABLED=true` |

---

## When to Use OM Only

**Scenario:** Tenant with simple FAQ bot, no uploaded knowledge base.

- User asks about past conversations
- Agent recalls preferences, history, context
- No documents to retrieve

```bash
OM_ENABLED=true
RAG_PIPELINE_ENABLED=false
```

**Example use cases:**
- Personal assistant that remembers user preferences
- Customer service agent that recalls past issues
- Booking agent that remembers previous appointments

---

## When to Use RAG Only

**Scenario:** Tenant with static knowledge base, no conversation memory.

- User asks "what are your prices?"
- Agent retrieves from KB.MD
- No need to remember past sessions

```bash
OM_ENABLED=false
RAG_PIPELINE_ENABLED=true
```

**Example use cases:**
- Product FAQ bot
- Documentation assistant
- Policy lookup agent

---

## When to Use Both

**Scenario:** Full-featured agent with both conversation memory and knowledge base.

User asks: "Can you remind me about the water extraction service we discussed?"

**OM provides:**
- Discussed on July 15
- User had flooding in basement
- Agent recommended emergency response

**RAG provides:**
- Service description from KB.MD
- Pricing from KB.MD
- Coverage area from KB.MD
- Safety protocols from KB.MD

```bash
OM_ENABLED=true
RAG_PIPELINE_ENABLED=true
```

**Example use cases:**
- Enterprise customer service (history + product docs)
- Healthcare assistant (patient history + medical knowledge)
- Financial advisor (client history + market data)

---

## How They Complement Each Other

```
User: "Can you tell me about the service we discussed last week?"

        ┌─────────────────────────────────────────────────┐
        │                  LLM Answer                     │
        └─────────────────────────────────────────────────┘
                           ▲
                           │
            ┌──────────────┴──────────────┐
            │                             │
    ┌───────┴───────┐             ┌───────┴───────┐
    │  OM Context   │             │  RAG Context  │
    │  (Memory)     │             │  (Knowledge)  │
    └───────┬───────┘             └───────┬───────┘
            │                             │
    ┌───────┴───────┐             ┌───────┴───────┐
    │ Observation   │             │ Vector Search │
    │ Log           │             │ over KB/POLICY│
    │ (compressed   │             │ (chunked,     │
    │  sessions)    │             │  embedded)    │
    └───────────────┘             └───────────────┘
```

---

## Configuration

### OM Only

```bash
OM_ENABLED=true
OM_SCOPE=resource
OM_OBSERVER_TOKEN_THRESHOLD=30000
OM_TOKEN_THRESHOLD=2000
RAG_PIPELINE_ENABLED=false
```

### RAG Only

```bash
OM_ENABLED=false
RAG_PIPELINE_ENABLED=true
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
LANCEDB_STORAGE_PROVIDER=local        # default: memory (volatile) — use local for persistence
LANCEDB_LOCAL_PATH=build/data/lancedb
```

### Both (Recommended for Production)

```bash
OM_ENABLED=true
OM_SCOPE=resource
RAG_PIPELINE_ENABLED=true
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
LANCEDB_STORAGE_PROVIDER=local        # default: memory (volatile) — use local for persistence
LANCEDB_LOCAL_PATH=build/data/lancedb
```

---

## System Prompt Integration

When both are enabled, the system prompt combines both sources:

```
## MEMORY (Observation Log)
{OM observation log — compressed conversation history}

## KNOWLEDGE BASE
{RAG retrieved chunks — relevant document sections}

## QUESTION
{User's question}
```

The LLM uses both to construct a response that leverages:
- Conversational context from OM
- Factual knowledge from RAG

---

## Evaluation

| System | Benchmark | What It Tests |
|--------|-----------|---------------|
| OM | LongMemEval | Cross-session memory, factual recall, preference inference |
| RAG | RAG pipeline tests | Document retrieval accuracy, chunk quality |
| Both | Combined evaluation | End-to-end agent accuracy with both sources |

**Current status:**
- OM: LongMemEval benchmark implemented (SingleHop 92.2%, KnowledgeUpdate 81.4%, Preference 46.7%)
- RAG: Pipeline implemented, no formal benchmark yet
- Both: No combined benchmark yet

---

## Key Insight

**OM and RAG solve different problems.** They are not competing approaches — they are complementary:

- OM answers: "What happened in past conversations?"
- RAG answers: "What do we know from our documents?"
- Together they answer: "Given what we discussed AND what we know, what's the best response?"

A production agent benefits from both. The LongMemEval benchmark validates the OM half. A RAG benchmark would validate the other half. A combined benchmark would validate the full system.
