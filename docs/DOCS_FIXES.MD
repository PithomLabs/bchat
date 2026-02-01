# Fixes Documentation

This document tracks notable bug fixes for future reference.

---

## 2026-01-31: View Transcript Modal Shows Blank Fields and Invalid Date

### Problem

In the View Transcript modal (AgentAdmin page), the following fields displayed incorrectly:
- **Session:** blank
- **Audience:** blank
- **Messages:** blank (count shows 0 or undefined)
- **Started:** shows "Invalid Date"

### Root Cause

**JSON field naming mismatch** between Go backend (snake_case) and TypeScript frontend (camelCase).

Go struct serialized to JSON with snake_case: `{"session_id": "abc", "started_at": "2026-01-31T12:00:00Z"}`

TypeScript tried to access camelCase: `transcript.sessionId` → `undefined`

### Solution

Updated `AgentTranscript` struct JSON tags in `store/agent.go` from snake_case to camelCase:

| Field | Before | After |
|-------|--------|-------|
| TenantID | `tenant_id` | `tenantId` |
| SessionID | `session_id` | `sessionId` |
| AudienceType | `audience_type` | `audienceType` |
| MessageCount | `message_count` | `messageCount` |
| ClientIP | `client_ip` | `clientIp` |
| UserAgent | `user_agent` | `userAgent` |
| CustomerName | `customer_name` | `customerName` |
| CustomerPhone | `customer_phone` | `customerPhone` |
| CustomerEmail | `customer_email` | `customerEmail` |
| CustomerLocation | `customer_location` | `customerLocation` |
| DetectedIntent | `detected_intent` | `detectedIntent` |
| StartedAt | `started_at` | `startedAt` |
| EndedAt | `ended_at` | `endedAt` |
| LastMessageAt | `last_message_at` | `lastMessageAt` |
| IsCompleted | `is_completed` | `isCompleted` |
| CompletionReason | `completion_reason` | `completionReason` |

### Files Modified

- `store/agent.go` - Updated JSON tags in `AgentTranscript` struct (lines 556-575)

### Notes

- No database migration needed (JSON tags only affect serialization, not storage)
- Existing transcripts in database are unaffected
- `AgentMessage` struct already used matching field names (no changes needed)
