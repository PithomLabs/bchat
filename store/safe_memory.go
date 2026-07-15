package store

import (
	"log/slog"
	"sync"

	"github.com/PithomLabs/memstate"
)

// SafeMemory wraps memstate.Memory with a mutex for safe concurrent access.
// Used for in-memory belief-revision facts per chat session.
type SafeMemory struct {
	mu  sync.Mutex
	mem *memstate.Memory
}

// NewSafeMemory creates a new SafeMemory. Accepts optional memstate.Config
// for threshold tuning (e.g. SupersedeThreshold).
func NewSafeMemory(cfg ...memstate.Config) *SafeMemory {
	return &SafeMemory{mem: memstate.New(cfg...)}
}

// Add inserts a fact. Panics are recovered and logged.
func (s *SafeMemory) Add(text string) {
	if s == nil || s.mem == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("memstate panicked on Add", "panic", r, "text", text)
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem.Add(text)
}

// Prompt compiles current facts into an LLM-ready string. Panics are recovered.
func (s *SafeMemory) Prompt(query string, budget int) string {
	if s == nil || s.mem == nil {
		return ""
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("memstate panicked on Prompt", "panic", r)
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mem.Prompt(query, budget)
}

// Facts returns a deep copy of all facts. Panics are recovered.
// Deep copy avoids pointer-safety issues after the lock is released.
func (s *SafeMemory) Facts(includePrivate bool) []*memstate.Fact {
	if s == nil || s.mem == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("memstate panicked on Facts", "panic", r)
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	raw := s.mem.Facts(includePrivate)
	result := make([]*memstate.Fact, len(raw))
	for i, f := range raw {
		cp := *f
		if f.SupersededBy != nil {
			v := *f.SupersededBy
			cp.SupersededBy = &v
		}
		if f.Tokens != nil {
			cp.Tokens = make(map[string]struct{}, len(f.Tokens))
			for k, v := range f.Tokens {
				cp.Tokens[k] = v
			}
		}
		result[i] = &cp
	}
	return result
}
