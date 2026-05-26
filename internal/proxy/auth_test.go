package proxy

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestAccountStoreAcceptsPlainPassword(t *testing.T) {
	store := newStore(t, []string{"alice:secret"}, "", "")

	if !store.Valid("alice", "secret") {
		t.Fatal("valid credentials rejected")
	}
	if store.Valid("alice", "wrong") {
		t.Fatal("invalid password accepted")
	}
}

func TestAccountStoreAcceptsBcryptHash(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("hunter2"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	store := newStore(t, []string{"carol:" + string(hash)}, "", "")
	if !store.Valid("carol", "hunter2") {
		t.Fatal("bcrypt credentials rejected")
	}
}

func TestAccountStoreDefaultAccount(t *testing.T) {
	store := newStore(t, nil, "karen", "password")
	if !store.Valid("karen", "password") {
		t.Fatal("default credentials rejected")
	}
}

func TestAccountStoreLockout(t *testing.T) {
	store, err := NewAccountStore([]string{"bob:secret"}, "", "", 2, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new account store: %v", err)
	}

	_ = store.Valid("bob", "wrong")
	_ = store.Valid("bob", "wrong")
	if store.Valid("bob", "secret") {
		t.Fatal("locked account accepted")
	}

	time.Sleep(15 * time.Millisecond)
	if !store.Valid("bob", "secret") {
		t.Fatal("credentials rejected after lockout expired")
	}
}

func TestUnknownUsersDoNotGrowFailureState(t *testing.T) {
	store := newStore(t, []string{"alice:secret"}, "", "")

	for i := 0; i < 3; i++ {
		_ = store.Valid("unknown", "wrong")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.failures) != 0 {
		t.Fatalf("failures length = %d, want 0", len(store.failures))
	}
}

func TestExpiredFailuresAreCleaned(t *testing.T) {
	store, err := NewAccountStore([]string{"alice:secret"}, "", "", 3, time.Millisecond)
	if err != nil {
		t.Fatalf("new account store: %v", err)
	}

	_ = store.Valid("alice", "wrong")
	time.Sleep(2 * time.Millisecond)
	_ = store.Valid("alice", "wrong-again")

	store.mu.Lock()
	defer store.mu.Unlock()
	if got := store.failures["alice"].count; got != 1 {
		t.Fatalf("failure count = %d, want 1", got)
	}
}

func TestAccountParsingRejectsBadEntries(t *testing.T) {
	tests := []string{
		"missing-separator",
		":secret",
		"alice:",
		"alice:secret",
		"alice:other",
	}

	if _, err := NewAccountStore(tests, "", "", 3, time.Minute); err == nil {
		t.Fatal("expected duplicate user error")
	}
}

func newStore(t *testing.T, accounts []string, user, password string) *AccountStore {
	t.Helper()

	store, err := NewAccountStore(accounts, user, password, 3, time.Minute)
	if err != nil {
		t.Fatalf("new account store: %v", err)
	}
	return store
}
