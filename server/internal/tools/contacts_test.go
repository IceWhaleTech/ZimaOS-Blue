package tools

import (
	"context"
	"strings"
	"testing"
)

type memoryContactsStore struct {
	contacts map[string]ContactRecord
}

func newMemoryContactsStore() *memoryContactsStore {
	return &memoryContactsStore{
		contacts: make(map[string]ContactRecord),
	}
}

func (s *memoryContactsStore) List(_ context.Context, _ string, opts ContactsQueryOptions) ([]ContactRecord, error) {
	result := make([]ContactRecord, 0, len(s.contacts))
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	for _, contact := range s.contacts {
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				contact.DisplayName,
				contact.FirstName,
				contact.LastName,
				contact.Organization,
				strings.Join(contact.EmailAddresses, " "),
				strings.Join(contact.PhoneNumbers, " "),
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		result = append(result, contact)
	}
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}
	return result, nil
}

func (s *memoryContactsStore) Get(_ context.Context, _ string, id string) (*ContactRecord, error) {
	contact, ok := s.contacts[id]
	if !ok {
		return nil, nil
	}
	return &contact, nil
}

func (s *memoryContactsStore) Create(_ context.Context, _ string, contact ContactRecord) (*ContactRecord, error) {
	s.contacts[contact.ID] = contact
	stored := s.contacts[contact.ID]
	return &stored, nil
}

func TestContactsTool_CreateSearchAndGet(t *testing.T) {
	store := newMemoryContactsStore()
	tool := NewContactsTool(store)
	ctx := WithLang(WithUserID(context.Background(), "user-a"), "en-US")

	createdRaw, err := tool.Execute(ctx, map[string]interface{}{
		"name":             "Alice Chen",
		"emails":           []interface{}{"alice@example.com"},
		"phone_numbers":    []string{"+1 555 0100"},
		"organizationName": "Zima",
	})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}

	created := createdRaw.(map[string]interface{})
	contact := created["contact"].(map[string]interface{})
	identifier, _ := contact["identifier"].(string)
	if identifier == "" {
		t.Fatalf("expected identifier in result, got %#v", contact)
	}
	if got, _ := contact["display_name"].(string); got != "Alice Chen" {
		t.Fatalf("display_name = %q, want Alice Chen", got)
	}

	searchRaw, err := tool.Execute(ctx, map[string]interface{}{
		"search": "alice@example.com",
	})
	if err != nil {
		t.Fatalf("search contact: %v", err)
	}
	search := searchRaw.(map[string]interface{})
	if got, _ := search["count"].(int); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
	found := search["contacts"].([]map[string]interface{})
	if len(found) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(found))
	}
	if got, _ := found[0]["organization_name"].(string); got != "Zima" {
		t.Fatalf("organization_name = %q, want Zima", got)
	}

	getRaw, err := tool.Execute(ctx, map[string]interface{}{
		"contact_id": identifier,
	})
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	get := getRaw.(map[string]interface{})
	gotContact := get["contact"].(map[string]interface{})
	if got, _ := gotContact["email_addresses"].([]string); len(got) != 1 || got[0] != "alice@example.com" {
		t.Fatalf("email_addresses = %#v, want alice@example.com", gotContact["email_addresses"])
	}
}

type stubContactsStore struct {
	name string
}

func (s *stubContactsStore) List(context.Context, string, ContactsQueryOptions) ([]ContactRecord, error) {
	return nil, nil
}

func (s *stubContactsStore) Get(context.Context, string, string) (*ContactRecord, error) {
	return nil, nil
}

func (s *stubContactsStore) Create(context.Context, string, ContactRecord) (*ContactRecord, error) {
	return nil, nil
}

func TestPreferredContactsStore_UsesNativeFactoryWhenAvailable(t *testing.T) {
	fallback := &stubContactsStore{name: "fallback"}
	native := &stubContactsStore{name: "native"}
	restore := setContactsNativeStoreFactoryForTest(func(ContactsStore) ContactsStore {
		return native
	})
	defer restore()

	got := PreferredContactsStore(fallback)
	if got != native {
		t.Fatalf("PreferredContactsStore() = %#v, want native %#v", got, native)
	}
}

func TestPreferredContactsStore_FallsBackToProvidedStoreWhenNativeUnavailable(t *testing.T) {
	fallback := &stubContactsStore{name: "fallback"}
	restore := setContactsNativeStoreFactoryForTest(func(ContactsStore) ContactsStore {
		return nil
	})
	defer restore()

	got := PreferredContactsStore(fallback)
	if got != fallback {
		t.Fatalf("PreferredContactsStore() = %#v, want fallback %#v", got, fallback)
	}
}
