package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	sessionsActive bool
)

// sessionsCmd represents the sessions command
var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Session management",
	Long: `Manage conversation sessions.

Subcommands:
  list              List all sessions
  show <id>         Show session details
  delete <id>       Delete a session
  clear             Clear all sessions`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Run:   runSessionsList,
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show session details",
	Args:  cobra.ExactArgs(1),
	Run:   runSessionsShow,
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	Run:   runSessionsDelete,
}

var sessionsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all sessions",
	Run:   runSessionsClear,
}

func init() {
	sessionsListCmd.Flags().BoolVar(&sessionsActive, "active", false, "show only active sessions")

	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsClearCmd)

	rootCmd.AddCommand(sessionsCmd)
}

// SessionInfo represents session information
type SessionInfo struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
	TokenCount   int       `json:"token_count,omitempty"`
	Status       string    `json:"status"`
}

type sessionListEntry struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

type sessionListResponse struct {
	Conversations []sessionListEntry `json:"conversations"`
}

func decodeSessionsListResponse(body io.Reader) (sessionListResponse, error) {
	payload, err := io.ReadAll(body)
	if err != nil {
		return sessionListResponse{}, err
	}

	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return sessionListResponse{}, nil
	}

	switch trimmed[0] {
	case '[':
		var conversations []sessionListEntry
		if err := json.Unmarshal(trimmed, &conversations); err != nil {
			return sessionListResponse{}, err
		}
		return sessionListResponse{Conversations: conversations}, nil
	default:
		var wrapped sessionListResponse
		if err := json.Unmarshal(trimmed, &wrapped); err != nil {
			return sessionListResponse{}, err
		}
		if wrapped.Conversations == nil {
			wrapped.Conversations = []sessionListEntry{}
		}
		return wrapped, nil
	}
}

func runSessionsList(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	baseURL := getServiceBaseURL()

	url := baseURL + "/api/v1/conversations"
	if sessionsActive {
		url += "?active=true"
	}

	resp, err := client.Get(url)
	if err != nil {
		printSessionsError("Failed to fetch sessions", err)
		return
	}
	defer resp.Body.Close()

	result, err := decodeSessionsListResponse(resp.Body)
	if err != nil {
		printSessionsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		if len(result.Conversations) == 0 {
			fmt.Println("No sessions found")
			return
		}

		fmt.Println("Sessions:")
		for _, s := range result.Conversations {
			title := s.Title
			if title == "" {
				title = "(untitled)"
			}
			if len(title) > 40 {
				title = title[:37] + "..."
			}

			if noColor {
				fmt.Printf("  %s  %s  (%d messages)\n", s.ID[:8], title, s.MessageCount)
			} else {
				fmt.Printf("  \033[36m%s\033[0m  %s  (%d messages)\n", s.ID[:8], title, s.MessageCount)
			}
		}
		fmt.Printf("\nTotal: %d sessions\n", len(result.Conversations))
	}
}

func runSessionsShow(cmd *cobra.Command, args []string) {
	sessionID := args[0]
	client := &http.Client{Timeout: 10 * time.Second}

	baseURL := getServiceBaseURL()

	resp, err := client.Get(baseURL + "/api/v1/conversations/" + sessionID)
	if err != nil {
		printSessionsError("Failed to fetch session", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"error": "Session not found",
				"id":    sessionID,
			})
		} else {
			fmt.Printf("Session not found: %s\n", sessionID)
		}
		os.Exit(1)
	}

	var session struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		printSessionsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(session)
	} else {
		fmt.Printf("Session: %s\n", session.ID)
		fmt.Printf("Title: %s\n", session.Title)
		fmt.Printf("Created: %s\n", session.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Updated: %s\n", session.UpdatedAt.Format(time.RFC3339))
		fmt.Printf("Messages: %d\n", len(session.Messages))

		if verbose && len(session.Messages) > 0 {
			fmt.Println("\nRecent messages:")
			start := 0
			if len(session.Messages) > 5 {
				start = len(session.Messages) - 5
			}
			for _, msg := range session.Messages[start:] {
				content := msg.Content
				if len(content) > 100 {
					content = content[:97] + "..."
				}
				if noColor {
					fmt.Printf("  [%s] %s\n", msg.Role, content)
				} else {
					roleColor := "\033[32m" // Green for assistant
					if msg.Role == "user" {
						roleColor = "\033[34m" // Blue for user
					}
					fmt.Printf("  %s[%s]\033[0m %s\n", roleColor, msg.Role, content)
				}
			}
		}
	}
}

func runSessionsDelete(cmd *cobra.Command, args []string) {
	sessionID := args[0]
	client := &http.Client{Timeout: 10 * time.Second}

	baseURL := getServiceBaseURL()

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/conversations/"+sessionID, nil)
	if err != nil {
		printSessionsError("Failed to create request", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		printSessionsError("Failed to delete session", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"success": false,
				"error":   "Session not found",
				"id":      sessionID,
			})
		} else {
			fmt.Printf("Session not found: %s\n", sessionID)
		}
		os.Exit(1)
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": true,
			"id":      sessionID,
		})
	} else {
		if noColor {
			fmt.Printf("Deleted session: %s\n", sessionID)
		} else {
			fmt.Printf("\033[32m✓\033[0m Deleted session: %s\n", sessionID)
		}
	}
}

func runSessionsClear(cmd *cobra.Command, args []string) {
	// Confirm before clearing
	if !jsonOutput {
		fmt.Print("Are you sure you want to clear all sessions? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}

	baseURL := getServiceBaseURL()

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/conversations", nil)
	if err != nil {
		printSessionsError("Failed to create request", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		printSessionsError("Failed to clear sessions", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Deleted int `json:"deleted"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": true,
			"deleted": result.Deleted,
		})
	} else {
		if noColor {
			fmt.Printf("Cleared %d sessions\n", result.Deleted)
		} else {
			fmt.Printf("\033[32m✓\033[0m Cleared %d sessions\n", result.Deleted)
		}
	}
}

func printSessionsError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": false,
			"error":   msg,
			"details": err.Error(),
		})
	} else {
		if noColor {
			fmt.Printf("Error: %s\n", msg)
		} else {
			fmt.Printf("\033[31mError:\033[0m %s\n", msg)
		}
		if verbose && err != nil {
			fmt.Printf("Details: %v\n", err)
		}
	}
	os.Exit(1)
}
