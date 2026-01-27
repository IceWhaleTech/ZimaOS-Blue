package builtin

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
)

// Contact represents a contact entry
type Contact struct {
	ID           string            `json:"id"`
	FirstName    string            `json:"first_name"`
	LastName     string            `json:"last_name"`
	DisplayName  string            `json:"display_name"`
	Email        []string          `json:"email,omitempty"`
	Phone        []string          `json:"phone,omitempty"`
	Address      string            `json:"address,omitempty"`
	Organization string            `json:"organization,omitempty"`
	Title        string            `json:"title,omitempty"`
	Birthday     string            `json:"birthday,omitempty"`
	Notes        string            `json:"notes,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Custom       map[string]string `json:"custom,omitempty"`
	Created      time.Time         `json:"created"`
	Updated      time.Time         `json:"updated"`
}

// ContactsConfig holds contacts configuration
type ContactsConfig struct {
	CardDAVURL      string `json:"carddav_url"`
	CardDAVUser     string `json:"carddav_user"`
	CardDAVPassword string `json:"carddav_password"`
}

// Contacts is a built-in contacts skill
type Contacts struct {
	manifest *skill.Manifest
	config   *ContactsConfig
	contacts map[string]*Contact
	mu       sync.RWMutex
	counter  int
}

// NewContacts creates a new contacts skill
func NewContacts(config *ContactsConfig) *Contacts {
	return &Contacts{
		manifest: &skill.Manifest{
			ID:          "contacts",
			Name:        "Contacts",
			Version:     "1.0.0",
			Description: "Contact management with CardDAV support",
			Category:    "communication",
			Tags:        []string{"contacts", "address book", "carddav"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: create, read, update, delete, list, search",
					Required:    true,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Contact ID",
					Required:    false,
				},
				{
					Name:        "first_name",
					Type:        "string",
					Description: "First name",
					Required:    false,
				},
				{
					Name:        "last_name",
					Type:        "string",
					Description: "Last name",
					Required:    false,
				},
				{
					Name:        "email",
					Type:        "array",
					Description: "Email addresses",
					Required:    false,
				},
				{
					Name:        "phone",
					Type:        "array",
					Description: "Phone numbers",
					Required:    false,
				},
				{
					Name:        "organization",
					Type:        "string",
					Description: "Organization/Company",
					Required:    false,
				},
				{
					Name:        "query",
					Type:        "string",
					Description: "Search query",
					Required:    false,
				},
				{
					Name:        "tag",
					Type:        "string",
					Description: "Filter by tag",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "contact",
					Type:        "object",
					Description: "Contact details",
				},
				{
					Name:        "contacts",
					Type:        "array",
					Description: "List of contacts",
				},
			},
			Permissions: []string{"contacts.read", "contacts.write"},
		},
		config:   config,
		contacts: make(map[string]*Contact),
	}
}

// Manifest returns the skill manifest
func (c *Contacts) Manifest() *skill.Manifest {
	return c.manifest
}

// Validate validates the input parameters
func (c *Contacts) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"create": true, "read": true, "update": true,
		"delete": true, "list": true, "search": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	switch actionStr {
	case "create":
		if _, ok := input["first_name"]; !ok {
			if _, ok := input["last_name"]; !ok {
				return fmt.Errorf("first_name or last_name is required for create action")
			}
		}
	case "read", "delete":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s action", actionStr)
		}
	case "update":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for update action")
		}
	case "search":
		if _, ok := input["query"]; !ok {
			return fmt.Errorf("query is required for search action")
		}
	}

	return nil
}

// Execute executes the contacts skill
func (c *Contacts) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "create":
		return c.createContact(input)
	case "read":
		return c.readContact(input)
	case "update":
		return c.updateContact(input)
	case "delete":
		return c.deleteContact(input)
	case "list":
		return c.listContacts(input)
	case "search":
		return c.searchContacts(input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (c *Contacts) createContact(input map[string]any) (*skill.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.counter++
	id := fmt.Sprintf("contact-%d", c.counter)
	now := time.Now()

	contact := &Contact{
		ID:      id,
		Created: now,
		Updated: now,
	}

	if firstName, ok := input["first_name"].(string); ok {
		contact.FirstName = firstName
	}
	if lastName, ok := input["last_name"].(string); ok {
		contact.LastName = lastName
	}

	// Build display name
	contact.DisplayName = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
	if contact.DisplayName == "" {
		contact.DisplayName = "Unknown"
	}

	// Parse email array
	if emails, ok := input["email"].([]interface{}); ok {
		for _, e := range emails {
			if email, ok := e.(string); ok {
				contact.Email = append(contact.Email, email)
			}
		}
	}

	// Parse phone array
	if phones, ok := input["phone"].([]interface{}); ok {
		for _, p := range phones {
			if phone, ok := p.(string); ok {
				contact.Phone = append(contact.Phone, phone)
			}
		}
	}

	if org, ok := input["organization"].(string); ok {
		contact.Organization = org
	}
	if title, ok := input["title"].(string); ok {
		contact.Title = title
	}
	if address, ok := input["address"].(string); ok {
		contact.Address = address
	}
	if birthday, ok := input["birthday"].(string); ok {
		contact.Birthday = birthday
	}
	if notes, ok := input["notes"].(string); ok {
		contact.Notes = notes
	}

	// Parse tags
	if tags, ok := input["tags"].([]interface{}); ok {
		for _, t := range tags {
			if tag, ok := t.(string); ok {
				contact.Tags = append(contact.Tags, tag)
			}
		}
	}

	c.contacts[id] = contact

	return skill.NewResult(map[string]any{
		"contact": contact,
		"message": fmt.Sprintf("Contact '%s' created", contact.DisplayName),
	}), nil
}

func (c *Contacts) readContact(input map[string]any) (*skill.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	id := input["id"].(string)

	if contact, ok := c.contacts[id]; ok {
		return skill.NewResult(map[string]any{
			"contact": contact,
		}), nil
	}

	return skill.NewResult(map[string]any{
		"contact": nil,
		"message": fmt.Sprintf("Contact '%s' not found", id),
	}), nil
}

func (c *Contacts) updateContact(input map[string]any) (*skill.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := input["id"].(string)

	contact, ok := c.contacts[id]
	if !ok {
		return skill.NewResult(map[string]any{
			"updated": false,
			"message": fmt.Sprintf("Contact '%s' not found", id),
		}), nil
	}

	if firstName, ok := input["first_name"].(string); ok {
		contact.FirstName = firstName
	}
	if lastName, ok := input["last_name"].(string); ok {
		contact.LastName = lastName
	}

	// Update display name
	contact.DisplayName = strings.TrimSpace(contact.FirstName + " " + contact.LastName)

	if emails, ok := input["email"].([]interface{}); ok {
		contact.Email = nil
		for _, e := range emails {
			if email, ok := e.(string); ok {
				contact.Email = append(contact.Email, email)
			}
		}
	}

	if phones, ok := input["phone"].([]interface{}); ok {
		contact.Phone = nil
		for _, p := range phones {
			if phone, ok := p.(string); ok {
				contact.Phone = append(contact.Phone, phone)
			}
		}
	}

	if org, ok := input["organization"].(string); ok {
		contact.Organization = org
	}
	if title, ok := input["title"].(string); ok {
		contact.Title = title
	}
	if address, ok := input["address"].(string); ok {
		contact.Address = address
	}
	if birthday, ok := input["birthday"].(string); ok {
		contact.Birthday = birthday
	}
	if notes, ok := input["notes"].(string); ok {
		contact.Notes = notes
	}

	contact.Updated = time.Now()

	return skill.NewResult(map[string]any{
		"updated": true,
		"contact": contact,
		"message": fmt.Sprintf("Contact '%s' updated", id),
	}), nil
}

func (c *Contacts) deleteContact(input map[string]any) (*skill.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := input["id"].(string)

	if contact, ok := c.contacts[id]; ok {
		delete(c.contacts, id)
		return skill.NewResult(map[string]any{
			"deleted": true,
			"contact": contact,
			"message": fmt.Sprintf("Contact '%s' deleted", id),
		}), nil
	}

	return skill.NewResult(map[string]any{
		"deleted": false,
		"message": fmt.Sprintf("Contact '%s' not found", id),
	}), nil
}

func (c *Contacts) listContacts(input map[string]any) (*skill.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tagFilter := ""
	if tag, ok := input["tag"].(string); ok {
		tagFilter = tag
	}

	var contacts []*Contact
	for _, contact := range c.contacts {
		if tagFilter != "" {
			hasTag := false
			for _, t := range contact.Tags {
				if t == tagFilter {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		contacts = append(contacts, contact)
	}

	// Sort by display name
	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].DisplayName < contacts[j].DisplayName
	})

	return skill.NewResult(map[string]any{
		"contacts": contacts,
		"count":    len(contacts),
	}), nil
}

func (c *Contacts) searchContacts(input map[string]any) (*skill.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	query := strings.ToLower(input["query"].(string))

	var results []*Contact
	for _, contact := range c.contacts {
		// Search in various fields
		if strings.Contains(strings.ToLower(contact.DisplayName), query) ||
			strings.Contains(strings.ToLower(contact.FirstName), query) ||
			strings.Contains(strings.ToLower(contact.LastName), query) ||
			strings.Contains(strings.ToLower(contact.Organization), query) ||
			strings.Contains(strings.ToLower(contact.Notes), query) {
			results = append(results, contact)
			continue
		}

		// Search in emails
		for _, email := range contact.Email {
			if strings.Contains(strings.ToLower(email), query) {
				results = append(results, contact)
				break
			}
		}

		// Search in phones
		for _, phone := range contact.Phone {
			if strings.Contains(phone, query) {
				results = append(results, contact)
				break
			}
		}
	}

	// Sort by display name
	sort.Slice(results, func(i, j int) bool {
		return results[i].DisplayName < results[j].DisplayName
	})

	return skill.NewResult(map[string]any{
		"contacts": results,
		"count":    len(results),
		"query":    query,
		"message":  fmt.Sprintf("Found %d contacts matching '%s'", len(results), query),
	}), nil
}
