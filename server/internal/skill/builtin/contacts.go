package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// ContactsExecutor is the interface for the native contacts backend.
type ContactsExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Contacts is a built-in skill wrapper over the native contacts tool.
type Contacts struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	executor ContactsExecutor
}

// NewContacts creates a new contacts skill.
func NewContacts() *Contacts {
	return &Contacts{
		manifest: &skill.Manifest{
			ID:          "contacts",
			Name:        "Contacts",
			Version:     "1.0.0",
			Description: "Native contacts skill. Lists and searches contacts by name, phone, or email, opens a contact by identifier, and creates new contacts in supported system address books.",
			Category:    "system",
			Icon:        "contacts",
			Tags:        []string{"contacts", "address-book", "people", "productivity"},
			Inputs: []skill.Parameter{
				{Name: "action", Type: "string", Description: "create, list, get, search"},
				{Name: "id", Type: "string", Description: "Contact identifier for get"},
				{Name: "query", Type: "string", Description: "Search query for list/search"},
				{Name: "display_name", Type: "string", Description: "Display name for create"},
				{Name: "given_name", Type: "string", Description: "Given/first name for create"},
				{Name: "family_name", Type: "string", Description: "Family/last name for create"},
				{Name: "organization_name", Type: "string", Description: "Organization name for create"},
				{Name: "title", Type: "string", Description: "Job title for create"},
				{Name: "address", Type: "string", Description: "Postal address for create"},
				{Name: "birthday", Type: "string", Description: "Birthday for create"},
				{Name: "phone_numbers", Type: "array", Description: "Phone numbers for create"},
				{Name: "email_addresses", Type: "array", Description: "Email addresses for create"},
				{Name: "limit", Type: "integer", Description: "Maximum results to return"},
			},
			Outputs: []skill.Parameter{
				{Name: "contact", Type: "object", Description: "Single contact"},
				{Name: "contacts", Type: "array", Description: "Contact list"},
			},
		},
	}
}

// SetExecutor injects the native contacts backend.
func (c *Contacts) SetExecutor(exec ContactsExecutor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.executor = exec
}

func (c *Contacts) Manifest() *skill.Manifest { return c.manifest }

func (c *Contacts) Validate(input map[string]any) error {
	normalizeContactsSkillInput(input)

	if input == nil {
		return fmt.Errorf("input is required")
	}
	action, _ := input["action"].(string)
	switch action {
	case "create":
		if !contactsSkillHasCreateArgs(input) {
			return fmt.Errorf("create requires a name, organization, phone number, or email address")
		}
	case "get":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for get")
		}
	}
	return nil
}

func normalizeContactsSkillInput(input map[string]any) {
	normalizeStringAlias(input, "action", "op", "operation", "command")
	normalizeStringAlias(input, "display_name", "displayName", "name")
	normalizeStringAlias(input, "given_name", "givenName", "first_name", "firstName")
	normalizeStringAlias(input, "family_name", "familyName", "last_name", "lastName")
	normalizeStringAlias(input, "organization_name", "organizationName", "organization")
	normalizeStringAlias(input, "title", "job_title", "jobTitle")
	normalizeStringAlias(input, "id", "contact_id", "contactId", "identifier")
	normalizeStringAlias(input, "query", "q", "search")
	normalizeValueAlias(input, "email_addresses", "emails", "email", "emailAddresses")
	normalizeValueAlias(input, "phone_numbers", "phones", "phone", "phoneNumbers")

	action := normalizeContactsSkillAction(firstTrimmedStringValue(input, "action"), input)
	if action != "" {
		input["action"] = action
	}
}

func normalizeContactsSkillAction(raw string, input map[string]any) string {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case "add", "new", "create_contact":
		return "create"
	case "open", "show", "detail":
		return "get"
	case "find", "lookup", "query":
		return "search"
	case "", "create", "list", "get", "search":
	default:
		return action
	}
	if action != "" {
		return action
	}
	if firstTrimmedStringValue(input, "id") != "" {
		return "get"
	}
	if firstTrimmedStringValue(input, "query") != "" {
		return "search"
	}
	if contactsSkillHasCreateArgs(input) {
		return "create"
	}
	return "list"
}

func contactsSkillHasCreateArgs(input map[string]any) bool {
	if firstTrimmedStringValue(input, "display_name", "given_name", "family_name", "organization_name", "title", "address", "birthday", "notes") != "" {
		return true
	}
	_, hasEmails := input["email_addresses"]
	_, hasPhones := input["phone_numbers"]
	return hasEmails || hasPhones
}

func (c *Contacts) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := c.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	c.mu.RLock()
	exec := c.executor
	c.mu.RUnlock()
	if exec == nil {
		return skill.NewErrorResult(fmt.Errorf("contacts backend not available")), nil
	}

	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
	}
	result, err := exec.Execute(ctx, args)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}
	switch typed := result.(type) {
	case string:
		var parsed map[string]any
		if json.Unmarshal([]byte(typed), &parsed) == nil {
			return skill.NewResult(parsed), nil
		}
		return skill.NewResult(map[string]any{"result": typed}), nil
	case map[string]interface{}:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = v
		}
		return skill.NewResult(out), nil
	default:
		b, _ := json.Marshal(typed)
		return skill.NewResult(map[string]any{"result": string(b)}), nil
	}
}
