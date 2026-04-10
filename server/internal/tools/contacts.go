package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultContactsUnavailableMessage = "CONTACTS_NATIVE_UNAVAILABLE: native contacts are not available on this platform"

// ContactRecord represents a native contact exposed through the contacts tool.
type ContactRecord struct {
	ID             string    `json:"id"`
	FirstName      string    `json:"first_name,omitempty"`
	LastName       string    `json:"last_name,omitempty"`
	DisplayName    string    `json:"display_name"`
	Organization   string    `json:"organization,omitempty"`
	Title          string    `json:"title,omitempty"`
	Address        string    `json:"address,omitempty"`
	Birthday       string    `json:"birthday,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	PhoneNumbers   []string  `json:"phone_numbers,omitempty"`
	EmailAddresses []string  `json:"email_addresses,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// ContactsQueryOptions captures list/search filters.
type ContactsQueryOptions struct {
	Query string
	Limit int
}

// ContactsStore is the persistence interface used by ContactsTool.
type ContactsStore interface {
	List(ctx context.Context, ownerID string, opts ContactsQueryOptions) ([]ContactRecord, error)
	Get(ctx context.Context, ownerID, id string) (*ContactRecord, error)
	Create(ctx context.Context, ownerID string, contact ContactRecord) (*ContactRecord, error)
}

type unavailableContactsStore struct {
	reason string
}

// NewUnavailableContactsStore returns a runtime backend that reports native
// contacts as unavailable instead of silently falling back to app-local data.
func NewUnavailableContactsStore(reason string) ContactsStore {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = defaultContactsUnavailableMessage
	}
	return &unavailableContactsStore{reason: reason}
}

func (s *unavailableContactsStore) List(context.Context, string, ContactsQueryOptions) ([]ContactRecord, error) {
	return nil, errors.New(s.reason)
}

func (s *unavailableContactsStore) Get(context.Context, string, string) (*ContactRecord, error) {
	return nil, errors.New(s.reason)
}

func (s *unavailableContactsStore) Create(context.Context, string, ContactRecord) (*ContactRecord, error) {
	return nil, errors.New(s.reason)
}

// ContactsTool provides native contacts lookup and creation workflows.
type ContactsTool struct {
	service ContactsStore
	now     func() time.Time
}

// NewContactsTool creates a native contacts tool.
func NewContactsTool(service ContactsStore) *ContactsTool {
	return &ContactsTool{
		service: service,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// SetNowFunc overrides the clock, mainly for tests.
func (t *ContactsTool) SetNowFunc(fn func() time.Time) {
	if t != nil && fn != nil {
		t.now = fn
	}
}

// Definition returns the tool schema.
func (t *ContactsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "contacts",
		Description: "Access native contacts when the platform supports them. List contacts, open a contact by identifier, search by name/email/phone, and create new contacts in the system address book.",
		Icon:        "contacts",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"list", "get", "search", "create"},
					"description": "Contacts action. Defaults to list, get, search, or create based on provided arguments.",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Contact identifier for get.",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query across display name, organization, phone numbers, and email addresses.",
				},
				"display_name": map[string]interface{}{
					"type":        "string",
					"description": "Display name for create.",
				},
				"given_name": map[string]interface{}{
					"type":        "string",
					"description": "Given/first name for create.",
				},
				"family_name": map[string]interface{}{
					"type":        "string",
					"description": "Family/last name for create.",
				},
				"organization_name": map[string]interface{}{
					"type":        "string",
					"description": "Organization/company name for create.",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Job title for create.",
				},
				"address": map[string]interface{}{
					"type":        "string",
					"description": "Street/postal address for create when supported by the native store.",
				},
				"birthday": map[string]interface{}{
					"type":        "string",
					"description": "Birthday in YYYY-MM-DD format for create when supported by the native store.",
				},
				"notes": map[string]interface{}{
					"type":        "string",
					"description": "Optional notes for create when supported by the native store.",
				},
				"phone_numbers": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Phone numbers for create.",
				},
				"email_addresses": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Email addresses for create.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum contacts to return (default 25).",
				},
			},
		},
	}
}

// Execute dispatches the selected contacts action.
func (t *ContactsTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("contacts service not available")
	}
	action := normalizeContactsAction(firstCompatString(args, "action", "op", "operation", "command"), args)
	ownerID := normalizeProductivityOwnerID(GetUserID(ctx))
	lang := GetLang(ctx)

	switch action {
	case "create":
		contact, err := contactRecordFromArgs(args, t.now())
		if err != nil {
			return nil, err
		}
		created, err := t.service.Create(ctx, ownerID, contact)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":  "success",
			"message": contactsLocalized(lang, fmt.Sprintf("Created contact: %s", created.DisplayName), fmt.Sprintf("已创建联系人：%s", created.DisplayName)),
			"contact": contactRecordForResult(*created),
		}, nil
	case "list", "search":
		contacts, err := t.service.List(ctx, ownerID, ContactsQueryOptions{
			Query: strings.TrimSpace(firstCompatString(args, "query", "q", "search")),
			Limit: contactsLimitFromArgs(args),
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":   "success",
			"message":  contactsLocalized(lang, fmt.Sprintf("Contacts results: %d contact(s).", len(contacts)), fmt.Sprintf("联系人结果：%d 条。", len(contacts))),
			"count":    len(contacts),
			"contacts": contactRecordsForResult(contacts),
		}, nil
	case "get":
		id := strings.TrimSpace(firstCompatString(args, "id", "contact_id", "contactId", "identifier"))
		if id == "" {
			return nil, errors.New("id is required")
		}
		contact, err := t.service.Get(ctx, ownerID, id)
		if err != nil {
			return nil, err
		}
		if contact == nil {
			return nil, errors.New("contact not found")
		}
		return map[string]interface{}{
			"status":  "success",
			"message": contactsLocalized(lang, fmt.Sprintf("Opened contact: %s", contact.DisplayName), fmt.Sprintf("已打开联系人：%s", contact.DisplayName)),
			"contact": contactRecordForResult(*contact),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported contacts action %q", action)
	}
}

func RegisterContactsTool(registry *Registry, service ContactsStore) *ContactsTool {
	if registry == nil || service == nil {
		return nil
	}
	tool := NewContactsTool(service)
	registry.Register(tool)
	return tool
}

func GetContactsTool(registry *Registry) *ContactsTool {
	if registry == nil {
		return nil
	}
	tool := registry.Get("contacts")
	if tool == nil {
		return nil
	}
	contactsTool, _ := tool.(*ContactsTool)
	return contactsTool
}

func normalizeContactsAction(raw string, args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case "add", "new", "create_contact":
		return "create"
	case "open", "show", "detail":
		return "get"
	case "find", "lookup", "query":
		return "search"
	}
	switch action {
	case "", "create", "list", "get", "search":
	default:
		return action
	}
	if action != "" {
		return action
	}
	if strings.TrimSpace(firstCompatString(args, "id", "contact_id", "contactId", "identifier")) != "" {
		return "get"
	}
	if strings.TrimSpace(firstCompatString(args, "query", "q", "search")) != "" {
		return "search"
	}
	if contactsHasCreateArgs(args) {
		return "create"
	}
	return "list"
}

func contactsHasCreateArgs(args map[string]interface{}) bool {
	if strings.TrimSpace(firstCompatString(args,
		"display_name", "displayName", "name",
		"given_name", "givenName", "first_name", "firstName",
		"family_name", "familyName", "last_name", "lastName",
		"organization_name", "organizationName", "organization",
		"title", "address", "birthday", "notes",
	)) != "" {
		return true
	}
	return len(stringSliceArg(args, "phone_numbers", "phones", "phone", "phoneNumbers")) > 0 ||
		len(stringSliceArg(args, "email_addresses", "emails", "email", "emailAddresses")) > 0
}

func contactRecordFromArgs(args map[string]interface{}, now time.Time) (ContactRecord, error) {
	contact := ContactRecord{
		ID:             strings.TrimSpace(firstCompatString(args, "id", "contact_id", "contactId", "identifier")),
		DisplayName:    strings.TrimSpace(firstCompatString(args, "display_name", "displayName", "name")),
		FirstName:      strings.TrimSpace(firstCompatString(args, "given_name", "givenName", "first_name", "firstName")),
		LastName:       strings.TrimSpace(firstCompatString(args, "family_name", "familyName", "last_name", "lastName")),
		Organization:   strings.TrimSpace(firstCompatString(args, "organization_name", "organizationName", "organization")),
		Title:          strings.TrimSpace(firstCompatString(args, "title", "job_title", "jobTitle")),
		Address:        strings.TrimSpace(firstCompatString(args, "address")),
		Birthday:       strings.TrimSpace(firstCompatString(args, "birthday")),
		Notes:          strings.TrimSpace(firstCompatString(args, "notes", "description")),
		PhoneNumbers:   normalizeContactStringList(stringSliceArg(args, "phone_numbers", "phones", "phone", "phoneNumbers"), false),
		EmailAddresses: normalizeContactStringList(stringSliceArg(args, "email_addresses", "emails", "email", "emailAddresses"), true),
		CreatedAt:      now.UTC(),
		UpdatedAt:      now.UTC(),
	}
	if strings.TrimSpace(contact.ID) == "" {
		contact.ID = uuid.NewString()
	}
	contact.DisplayName = buildContactDisplayName(contact)
	if contact.DisplayName == "" && len(contact.PhoneNumbers) == 0 && len(contact.EmailAddresses) == 0 {
		return ContactRecord{}, errors.New("contact requires a name, organization, phone number, or email address")
	}
	return contact, nil
}

func contactsLimitFromArgs(args map[string]interface{}) int {
	limit := compatInt(args, "limit", "max_results", "maxResults")
	if limit <= 0 {
		return 25
	}
	return limit
}

func normalizeContactStringList(values []string, lower bool) []string {
	result := make([]string, 0, len(values))
	for _, value := range normalizeStringList(values) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if lower {
			value = strings.ToLower(value)
		}
		duplicate := false
		for _, existing := range result {
			if strings.EqualFold(existing, value) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, value)
		}
	}
	return result
}

func buildContactDisplayName(contact ContactRecord) string {
	if displayName := strings.TrimSpace(contact.DisplayName); displayName != "" {
		return displayName
	}
	name := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(contact.FirstName),
		strings.TrimSpace(contact.LastName),
	}, " "))
	if name != "" {
		return name
	}
	return strings.TrimSpace(contact.Organization)
}

func contactsMatchQuery(contact ContactRecord, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		contact.DisplayName,
		contact.FirstName,
		contact.LastName,
		contact.Organization,
		contact.Title,
		contact.Address,
		contact.Birthday,
		contact.Notes,
		strings.Join(contact.PhoneNumbers, " "),
		strings.Join(contact.EmailAddresses, " "),
	}, " "))
	return strings.Contains(haystack, query)
}

func sortContactRecords(records []ContactRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		left := strings.TrimSpace(records[i].DisplayName)
		right := strings.TrimSpace(records[j].DisplayName)
		if left == right {
			return records[i].ID < records[j].ID
		}
		return left < right
	})
}

func contactRecordForResult(contact ContactRecord) map[string]interface{} {
	result := map[string]interface{}{
		"id":                contact.ID,
		"identifier":        contact.ID,
		"display_name":      contact.DisplayName,
		"displayName":       contact.DisplayName,
		"first_name":        contact.FirstName,
		"given_name":        contact.FirstName,
		"givenName":         contact.FirstName,
		"last_name":         contact.LastName,
		"family_name":       contact.LastName,
		"familyName":        contact.LastName,
		"organization":      contact.Organization,
		"organization_name": contact.Organization,
		"organizationName":  contact.Organization,
		"title":             contact.Title,
		"address":           contact.Address,
		"birthday":          contact.Birthday,
		"notes":             contact.Notes,
		"phone_numbers":     append([]string(nil), contact.PhoneNumbers...),
		"phoneNumbers":      append([]string(nil), contact.PhoneNumbers...),
		"email_addresses":   append([]string(nil), contact.EmailAddresses...),
		"emailAddresses":    append([]string(nil), contact.EmailAddresses...),
	}
	if !contact.CreatedAt.IsZero() {
		result["created_at"] = contact.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !contact.UpdatedAt.IsZero() {
		result["updated_at"] = contact.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return result
}

func contactRecordsForResult(contacts []ContactRecord) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(contacts))
	for _, contact := range contacts {
		result = append(result, contactRecordForResult(contact))
	}
	return result
}

func contactsLocalized(lang, en, zh string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		return zh
	}
	return en
}
