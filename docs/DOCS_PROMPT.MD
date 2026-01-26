# System Prompt Guide for KB Formatting

**Date:** 2026-01-27
**Purpose:** How to write system prompts that format KB content for optimal RAG performance without annotations

---

## Overview

bchat's RAG pipeline uses heading-based chunking (`ChunkMarkdownContent()`) that works with plain markdown - no `<!-- @section -->` annotations required. This guide provides system prompts you can use with any LLM to transform raw content into optimal markdown structure.

**Goal:** Convert unstructured text into well-structured markdown that produces semantically coherent chunks for vector search.

---

## How the Chunking Algorithm Works

### Splitting Priority

The `ChunkMarkdownContent()` function splits content in this order:

1. **H2 Headers (`## `)** - Primary split points, creates main chunks
2. **H3 Headers (`### `)** - Secondary splits if H2 sections are too large
3. **Paragraphs (`\n\n`)** - Tertiary splits for oversized subsections
4. **Sentences (`. ! ?`)** - Final fallback for very long paragraphs

### Chunk Size Limits

| Embedding Provider | Max Tokens | Recommended Chunk Size |
|-------------------|-----------|------------------------|
| **Cybertron** | 512 | 150 tokens (~600 chars) |
| **OpenRouter** | 8191 | 2000 tokens (~8000 chars) |
| **Local** | 512 | 150 tokens (~600 chars) |

### Merging Behavior

- Chunks smaller than `min_chunk_size` (default: 100 tokens) are merged with adjacent chunks
- Merged chunks cannot exceed `max_chunk_size` (default: 800 tokens)
- This prevents tiny, low-context chunks

---

## Optimal Markdown Structure

### The Ideal Format

```markdown
# Company Knowledge Base

## Service Category 1
Brief overview of this service category.

### Service 1.1: Water Damage Restoration
Detailed description of the service. Include what's covered,
response times, and any important notes.

This is a second paragraph providing additional context.
Paragraphs are separated by blank lines.

### Service 1.2: Fire Damage Restoration
Another service description with similar structure.

## Service Category 2
Overview of second category.

### Service 2.1: Mold Remediation
Service details here.

## Frequently Asked Questions

### How do I schedule a service?
Answer to the FAQ. Keep answers concise but complete.

### What are your hours of operation?
Another FAQ answer.

## Coverage Areas

### Included Areas
- City 1
- City 2
- County 1

### Excluded Areas
- Remote Location 1
- Remote Location 2
```

### Why This Structure Works

1. **H2 creates semantic boundaries** - Each `## Section` becomes a potential chunk
2. **H3 provides granularity** - If H2 sections are large, H3 headers create sub-chunks
3. **Paragraphs preserve context** - Content within a paragraph stays together
4. **Lists are preserved** - Markdown lists stay intact within chunks

---

## System Prompt Template

Use this prompt with any LLM (Claude, GPT-4, etc.) to format raw content:

### Basic Formatting Prompt

```
You are a document formatter. Transform the following raw text into well-structured markdown optimized for semantic search and retrieval.

FORMATTING RULES:
1. Use ## (H2) for main topics/categories (services, FAQs, policies, etc)
2. Use ### (H3) for individual items within categories
3. Separate paragraphs with blank lines
4. Keep each section focused on ONE topic
5. Aim for 100-500 words per H3 section
6. Use bullet lists for enumerations
7. Remove redundant or repetitive content
8. Fix spelling and grammar issues
9. Do NOT add <!-- annotations --> - just use plain markdown

STRUCTURE TEMPLATE:
## [Category Name]
Brief category overview.

### [Item Name]
Detailed description of this item.

Additional paragraph if needed.

RAW CONTENT TO FORMAT:
---
{paste your raw content here}
---

Output the formatted markdown only, no explanations.
```

### Advanced Formatting Prompt (with extraction)

```
You are a knowledge base formatter. Transform raw content into structured markdown for a RAG (Retrieval-Augmented Generation) system.

GOALS:
1. Create semantically coherent sections that can stand alone
2. Each section should answer a specific question or describe a specific topic
3. Preserve all factual information
4. Improve clarity without changing meaning

STRUCTURE REQUIREMENTS:
- H1 (#): Document title only
- H2 (##): Major categories (Services, FAQs, Policies, Coverage, Safety, Placeholder)
- H3 (###): Individual items with descriptive titles
- Paragraphs: 2-5 sentences, separated by blank lines
- Lists: Use for enumerations (coverage areas, steps, features)

SECTION GUIDELINES:
- Services: Include description, response time, what's covered
- FAQs: Question as H3 title, answer as content
- Locations: Question as H3 title, locations as content
- Policies: Rule name as H3, explanation as content
- Coverage: Geographic areas, included/excluded lists
- Safety: Emergency protocols, step-by-step instructions
- Placeholder: Any section title that is fitting based on context

CHUNK SIZE TARGET:
- Ideal: 150-300 words per H3 section
- Maximum: 500 words (will be split if larger)
- Minimum: 50 words (will be merged if smaller)

RAW CONTENT:
---
{paste your raw content here}
---

Output clean markdown. No explanations or meta-commentary.
```

### FAQ Extraction Prompt

```
Extract FAQs from the following content and format as markdown.

OUTPUT FORMAT:
## Frequently Asked Questions

### [Question in natural language]?
[Clear, concise answer. 1-3 sentences preferred.]

### [Next question]?
[Answer]

RULES:
1. Phrase questions as customers would ask them
2. Keep answers direct and factual
3. One topic per FAQ
4. If content implies a question, make it explicit

CONTENT:
---
{paste content}
---
```

### Service Catalog Prompt

```
Extract services from the following content and format as a service catalog.

OUTPUT FORMAT:
## Services

### [Service Name]
[2-3 sentence description of what the service includes]

**Response Time:** [If mentioned]
**Coverage:** [If mentioned]

RULES:
1. One service per ### section
2. Include all relevant details
3. Use consistent formatting
4. Preserve pricing/timing information if present

CONTENT:
---
{paste content}
---
```

---

## Examples

### Example 1: Raw Text to Structured KB

**Before (Raw):**
```
Our company offers water damage restoration services. We respond within 2 hours for emergencies. We also do fire damage restoration and mold removal. For water damage we extract water, dry the area, and restore damaged materials. Fire damage includes smoke removal, cleaning, and reconstruction. We serve the greater Austin area including Round Rock, Cedar Park, and Georgetown. We don't serve areas outside Travis and Williamson counties. Call us 24/7 at 555-1234.
```

**After (Formatted):**
```markdown
## Services

### Water Damage Restoration
Complete water damage restoration including water extraction, structural drying, and restoration of damaged materials.

**Response Time:** 2 hours for emergencies
**Availability:** 24/7

### Fire Damage Restoration
Comprehensive fire damage services including smoke removal, cleaning, and reconstruction of affected areas.

### Mold Remediation
Professional mold removal and remediation services.

## Coverage Areas

### Included Areas
- Austin (greater area)
- Round Rock
- Cedar Park
- Georgetown
- Travis County
- Williamson County

### Excluded Areas
- Areas outside Travis and Williamson counties

## Contact

### Emergency Line
Available 24/7 at 555-1234.
```

### Example 2: Chunking Visualization

**Input:**
```markdown
## Water Services

### Flood Cleanup
We provide emergency flood cleanup...
(150 words)

### Leak Repair
We fix all types of leaks...
(120 words)

## Fire Services

### Smoke Damage
Smoke damage restoration...
(200 words)
```

**Resulting Chunks:**

| Chunk # | Content | Tokens | Title |
|---------|---------|--------|-------|
| 1 | "## Water Services\n\n### Flood Cleanup\nWe provide..." | ~160 | Water Services > Flood Cleanup |
| 2 | "### Leak Repair\nWe fix all types..." | ~125 | Water Services > Leak Repair |
| 3 | "## Fire Services\n\n### Smoke Damage\nSmoke damage..." | ~210 | Fire Services > Smoke Damage |

---

## Configuration by Use Case

### For Cybertron (Development)

```
Max tokens: 150 (~600 characters)

RECOMMENDATION:
- Keep H3 sections to 100-150 words
- Use more H3 headers for granularity
- Prefer bullet lists over long paragraphs
```

### For OpenRouter (Production)

```
Max tokens: 2000 (~8000 characters)

RECOMMENDATION:
- H3 sections can be 200-500 words
- More context per chunk = better retrieval
- Can include full paragraphs without splitting
```

---

## Best Practices

### DO:
- Use descriptive H2/H3 headers that summarize the content
- Keep one topic per section
- Separate distinct ideas into separate paragraphs
- Include relevant keywords naturally in content
- Use consistent header hierarchy (H1 > H2 > H3)

### DON'T:
- Create very short sections (< 50 words) - they'll be merged
- Create very long sections (> 500 words for Cybertron) - they'll be split awkwardly
- Use H4+ headers - the chunker only recognizes H2/H3
- Mix multiple topics in one section
- Use annotations (`<!-- @section -->`) - they're not needed with fallback chunking

### Tips for Better Retrieval:

1. **Front-load important information** - Put key facts at the beginning of sections
2. **Use natural language** - Write how users would search
3. **Include synonyms** - "water damage" and "flood cleanup" in same section
4. **Be specific** - "2-hour response time" not "fast response"
5. **Answer implicit questions** - Structure content to answer "What/How/When/Where"

---

## Workflow Summary

1. **Gather raw content** (PDFs, docs, web pages, notes)
2. **Use system prompt** to format with an LLM
3. **Review and edit** the formatted markdown
4. **Upload to bchat** as KB.MD
5. **Verify chunking** - Check logs for "markdown-based chunking" message
6. **Test queries** - Ensure relevant content is retrieved

---

## Related Documentation

- `DOCS_LANCE_EMBEDDING.MD` - Fallback chunking implementation
- `DOCS_LANCEDB_CHATGPT.MD` - RAG comparison analysis
- `DOCS_RAG_PIPELINE.MD` - Full RAG pipeline architecture
