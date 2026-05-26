package proxy

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const dummyBcryptHash = "$2a$10$ddddddddddddddddddddddO8j/KBGxKZ5O3qkMBF9Lx9a9a9a9a9a"

type AccountStore struct {
	hashes      map[string][]byte
	failures    map[string]authFailure
	mu          sync.Mutex
	maxFailures int
	lockout     time.Duration
}

type authFailure struct {
	count       int
	lockedUntil time.Time
	lastFailed  time.Time
}

func NewAccountStore(accounts []string, user, password string, maxFailures int, lockout time.Duration) (*AccountStore, error) {
	hashes, err := loadAccounts(accounts, user, password)
	if err != nil {
		return nil, err
	}
	return &AccountStore{
		hashes:      hashes,
		failures:    make(map[string]authFailure),
		maxFailures: maxFailures,
		lockout:     lockout,
	}, nil
}

func (s *AccountStore) Valid(user, password string) bool {
	now := time.Now()

	hash, locked, known := s.snapshot(user, now)
	if locked || !known {
		compareDummyPassword(password)
		return false
	}

	if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
		s.recordFailure(user, time.Now())
		return false
	}

	s.resetFailures(user)
	return true
}

func (s *AccountStore) snapshot(user string, now time.Time) ([]byte, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanup(now)
	failure, hasFailure := s.failures[user]
	locked := hasFailure && now.Before(failure.lockedUntil)
	hash, known := s.hashes[user]
	return hash, locked, known
}

func (s *AccountStore) recordFailure(user string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	failure := s.failures[user]
	failure.count++
	failure.lastFailed = now
	if failure.count >= s.maxFailures {
		failure.count = 0
		failure.lockedUntil = now.Add(s.lockout)
	}
	s.failures[user] = failure
}

func (s *AccountStore) resetFailures(user string) {
	s.mu.Lock()
	delete(s.failures, user)
	s.mu.Unlock()
}

func (s *AccountStore) cleanup(now time.Time) {
	for user, failure := range s.failures {
		if !failure.lockedUntil.IsZero() && now.After(failure.lockedUntil) {
			delete(s.failures, user)
			continue
		}
		if failure.lockedUntil.IsZero() && !failure.lastFailed.IsZero() && now.Sub(failure.lastFailed) > s.lockout {
			delete(s.failures, user)
		}
	}
}

func loadAccounts(accounts []string, user, password string) (map[string][]byte, error) {
	hashes := make(map[string][]byte)

	if len(accounts) == 0 {
		hash, err := hashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("hash password for %q: %w", user, err)
		}
		hashes[user] = hash
		return hashes, nil
	}

	for _, account := range accounts {
		if strings.TrimSpace(account) == "" {
			continue
		}
		name, hash, err := parseAccount(account)
		if err != nil {
			return nil, err
		}
		if _, exists := hashes[name]; exists {
			return nil, fmt.Errorf("duplicate PROXY_ACCOUNTS user %q", name)
		}
		hashes[name] = hash
	}
	if len(hashes) == 0 {
		return nil, fmt.Errorf("PROXY_ACCOUNTS does not contain any valid accounts")
	}
	return hashes, nil
}

func parseAccount(account string) (string, []byte, error) {
	user, secret, ok := strings.Cut(account, ":")
	if !ok {
		return "", nil, fmt.Errorf("invalid PROXY_ACCOUNTS entry %q, expected user:password-or-bcrypt-hash", account)
	}

	user = strings.TrimSpace(user)
	secret = strings.TrimSpace(secret)
	if user == "" || secret == "" {
		return "", nil, fmt.Errorf("invalid PROXY_ACCOUNTS entry %q", account)
	}
	if isBcryptHash(secret) {
		return user, []byte(secret), nil
	}

	hash, err := hashPassword(secret)
	if err != nil {
		return "", nil, fmt.Errorf("hash password for %q: %w", user, err)
	}
	return user, hash, nil
}

func isBcryptHash(value string) bool {
	return strings.HasPrefix(value, "$2a$") ||
		strings.HasPrefix(value, "$2b$") ||
		strings.HasPrefix(value, "$2y$")
}

func hashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func compareDummyPassword(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(password))
}
