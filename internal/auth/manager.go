package auth

import (
	"fmt"
	"net/url"
	"sync"
)

// Manager coordinates domain credential caching, prompter delegation, and strategy resolution.
type Manager struct {
	store    *Store
	prompter Prompter
	mu       sync.Mutex
}

// NewManager constructs an authentication Manager with the provided Store and Prompter.
func NewManager(st *Store, prompter Prompter) *Manager {
	if prompter == nil {
		prompter = NewNonInteractivePrompter()
	}
	return &Manager{
		store:    st,
		prompter: prompter,
	}
}

// DefaultManager creates a Manager using the DefaultStore and TerminalPrompter.
func DefaultManager() (*Manager, error) {
	st, err := DefaultStore()
	if err != nil {
		return nil, err
	}
	return NewManager(st, NewTerminalPrompter()), nil
}

// Store returns the underlying persistent Store.
func (m *Manager) Store() *Store {
	return m.store
}

// ResolveDomain resolves an authentication strategy for a specific domain name.
// If the domain is not recognized in the cache, the prompter is called once,
// saved into ~/.downloader/auth, and returned for all future invocations.
func (m *Manager) ResolveDomain(domain string) (Strategy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	norm := NormalizeHostname(domain)

	// 1. Check persistent cache
	if m.store != nil {
		if entry, found := m.store.Get(norm); found {
			return ToStrategy(entry)
		}
	}

	// 2. Not found in cache; prompt caller / user
	if m.prompter != nil {
		entry, err := m.prompter.PromptAuth(norm)
		if err != nil {
			return nil, fmt.Errorf("credential prompt failed for %q: %w", norm, err)
		}

		if entry == nil {
			entry = &Entry{Method: MethodNone}
		}

		// Persist selection so subsequent runs don't prompt again
		if m.store != nil {
			if saveErr := m.store.Set(norm, entry); saveErr != nil {
				// Log or non-fatal fallback
				fmt.Printf("[Warning] Failed saving auth selection for %s: %v\n", norm, saveErr)
			}
		}

		return ToStrategy(entry)
	}

	return NewAnonymousStrategy(), nil
}

// ResolveURL extracts the domain from targetURL and resolves the authentication strategy.
func (m *Manager) ResolveURL(targetURL string) (Strategy, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	domain := parsed.Hostname()
	if domain == "" {
		domain = parsed.Host
	}

	return m.ResolveDomain(domain)
}
