// Package ai provides the AI manager that processes natural language requests,
// interacts with LLMs, calls tools via the gateway, and stores conversation memory.
package ai

import (
	"strings"
	"sync"
	"time"
	"unicode"
)

type MemoryEntry struct {
	Query     string    `json:"query"`
	Answer    string    `json:"answer"`
	ToolsUsed []string  `json:"tools_used"`
	Timestamp time.Time `json:"timestamp"`
}

type MemoryStore struct {
	mu      sync.RWMutex
	entries []MemoryEntry
	maxSize int
}

func NewMemoryStore(maxSize int) *MemoryStore {
	return &MemoryStore{
		entries: make([]MemoryEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

func (m *MemoryStore) Save(entry MemoryEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.Timestamp = time.Now()
	m.entries = append(m.entries, entry)
	if len(m.entries) > m.maxSize {
		m.entries = m.entries[len(m.entries)-m.maxSize:]
	}
}

func (m *MemoryStore) QueryRelevant(query string, limit int) []MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type scored struct {
		entry MemoryEntry
		score int
	}
	queryWords := tokenize(query)
	var scoredEntries []scored
	for _, entry := range m.entries {
		score := 0
		entryWords := tokenize(entry.Query + " " + entry.Answer)
		for _, qw := range queryWords {
			for _, ew := range entryWords {
				if strings.EqualFold(qw, ew) {
					score++
				}
			}
		}
		if score > 0 {
			scoredEntries = append(scoredEntries, scored{entry, score})
		}
	}

	for i := 0; i < len(scoredEntries); i++ {
		for j := i + 1; j < len(scoredEntries); j++ {
			if scoredEntries[j].score > scoredEntries[i].score {
				scoredEntries[i], scoredEntries[j] = scoredEntries[j], scoredEntries[i]
			}
		}
	}

	if limit > len(scoredEntries) {
		limit = len(scoredEntries)
	}
	result := make([]MemoryEntry, limit)
	for i := 0; i < limit; i++ {
		result[i] = scoredEntries[i].entry
	}
	return result
}

func tokenize(s string) []string {
	var words []string
	var current []rune
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current = append(current, unicode.ToLower(r))
		} else if len(current) > 0 {
			if len(current) > 2 {
				words = append(words, string(current))
			}
			current = nil
		}
	}
	if len(current) > 2 {
		words = append(words, string(current))
	}
	return words
}
