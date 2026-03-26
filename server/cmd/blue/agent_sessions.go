package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	agentSessionsLimit   int
	agentSessionsRuntime string
	agentSessionsProfile string
	agentSessionsName    string
	agentSessionsCWD     string
	agentSessionsMessage string
)

var agentSessionsCmd = &cobra.Command{
	Use:   "agent-sessions",
	Short: "Manage external ACP/A2A agent sessions",
}

var agentSessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List external agent sessions",
	Run:   runAgentSessionsList,
}

var agentSessionsNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new external agent session",
	Run:   runAgentSessionsNew,
}

var agentSessionsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show external agent session details",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentSessionsShow,
}

var agentSessionsHistoryCmd = &cobra.Command{
	Use:   "history <id>",
	Short: "Show external agent session history",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentSessionsHistory,
}

var agentSessionsSendCmd = &cobra.Command{
	Use:   "send <id>",
	Short: "Send a message to an external agent session",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentSessionsSend,
}

var agentSessionsCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel the active run for an external agent session",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentSessionsCancel,
}

var agentSessionsCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close an external agent session",
	Args:  cobra.ExactArgs(1),
	Run:   runAgentSessionsClose,
}

var agentSessionsProfilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List external agent profiles",
	Run:   runAgentSessionsProfiles,
}

func init() {
	agentSessionsListCmd.Flags().IntVar(&agentSessionsLimit, "limit", 20, "maximum number of sessions to return")
	agentSessionsListCmd.Flags().StringVar(&agentSessionsRuntime, "runtime", "", "filter by runtime (acp or a2a)")

	agentSessionsNewCmd.Flags().StringVar(&agentSessionsProfile, "profile", "", "external agent profile ID")
	agentSessionsNewCmd.Flags().StringVar(&agentSessionsName, "name", "", "session name")
	agentSessionsNewCmd.Flags().StringVar(&agentSessionsCWD, "cwd", "", "working directory for ACP sessions")
	agentSessionsNewCmd.Flags().StringVar(&agentSessionsMessage, "message", "", "optional initial message")
	_ = agentSessionsNewCmd.MarkFlagRequired("profile")

	agentSessionsHistoryCmd.Flags().IntVar(&agentSessionsLimit, "limit", 50, "maximum history events to return")
	agentSessionsSendCmd.Flags().StringVar(&agentSessionsMessage, "message", "", "message content")
	_ = agentSessionsSendCmd.MarkFlagRequired("message")

	agentSessionsCmd.AddCommand(
		agentSessionsListCmd,
		agentSessionsNewCmd,
		agentSessionsShowCmd,
		agentSessionsHistoryCmd,
		agentSessionsSendCmd,
		agentSessionsCancelCmd,
		agentSessionsCloseCmd,
		agentSessionsProfilesCmd,
	)
	rootCmd.AddCommand(agentSessionsCmd)
}

func getAgentSessionsBaseURL() string {
	return getServiceAPIBaseURL("/api/v1/agent-sessions")
}

func agentSessionsHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func doAgentSessionsRequest(method, endpoint string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return agentSessionsHTTPClient().Do(req)
}

func decodeAgentSessionsJSON(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var errBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			if msg, ok := errBody["error"].(string); ok {
				return fmt.Errorf("%s", msg)
			}
		}
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func runAgentSessionsList(cmd *cobra.Command, args []string) {
	endpoint := getAgentSessionsBaseURL() + "/sessions?limit=" + url.QueryEscape(fmt.Sprintf("%d", agentSessionsLimit))
	if strings.TrimSpace(agentSessionsRuntime) != "" {
		endpoint += "&runtime=" + url.QueryEscape(strings.TrimSpace(agentSessionsRuntime))
	}
	resp, err := doAgentSessionsRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		printAgentSessionsError("Failed to list agent sessions", err)
		return
	}
	var result struct {
		Sessions []map[string]interface{} `json:"sessions"`
	}
	if err := decodeAgentSessionsJSON(resp, &result); err != nil {
		printAgentSessionsError("Failed to decode agent sessions", err)
		return
	}
	if jsonOutput {
		printJSON(result)
		return
	}
	if len(result.Sessions) == 0 {
		fmt.Println("No external agent sessions found")
		return
	}
	for _, session := range result.Sessions {
		fmt.Printf("%s  %s  [%s]\n", asCLIString(session["id"]), asCLIString(session["name"]), asCLIString(session["status"]))
	}
}

func runAgentSessionsNew(cmd *cobra.Command, args []string) {
	resp, err := doAgentSessionsRequest(http.MethodPost, getAgentSessionsBaseURL()+"/sessions", map[string]interface{}{
		"profile_id":      agentSessionsProfile,
		"name":            agentSessionsName,
		"cwd":             agentSessionsCWD,
		"initial_message": agentSessionsMessage,
	})
	if err != nil {
		printAgentSessionsError("Failed to create agent session", err)
		return
	}
	var detail map[string]interface{}
	if err := decodeAgentSessionsJSON(resp, &detail); err != nil {
		printAgentSessionsError("Failed to decode session creation response", err)
		return
	}
	if jsonOutput {
		printJSON(detail)
		return
	}
	session, _ := detail["session"].(map[string]interface{})
	fmt.Printf("Created external agent session %s\n", asCLIString(session["id"]))
}

func runAgentSessionsShow(cmd *cobra.Command, args []string) {
	resp, err := doAgentSessionsRequest(http.MethodGet, getAgentSessionsBaseURL()+"/sessions/"+url.PathEscape(args[0]), nil)
	if err != nil {
		printAgentSessionsError("Failed to fetch agent session", err)
		return
	}
	var detail map[string]interface{}
	if err := decodeAgentSessionsJSON(resp, &detail); err != nil {
		printAgentSessionsError("Failed to decode agent session", err)
		return
	}
	if jsonOutput {
		printJSON(detail)
		return
	}
	session, _ := detail["session"].(map[string]interface{})
	fmt.Printf("Session: %s\n", asCLIString(session["id"]))
	fmt.Printf("Name: %s\n", asCLIString(session["name"]))
	fmt.Printf("Status: %s\n", asCLIString(session["status"]))
}

func runAgentSessionsHistory(cmd *cobra.Command, args []string) {
	endpoint := getAgentSessionsBaseURL() + "/sessions/" + url.PathEscape(args[0]) + "/history?limit=" + url.QueryEscape(fmt.Sprintf("%d", agentSessionsLimit))
	resp, err := doAgentSessionsRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		printAgentSessionsError("Failed to fetch agent session history", err)
		return
	}
	var detail map[string]interface{}
	if err := decodeAgentSessionsJSON(resp, &detail); err != nil {
		printAgentSessionsError("Failed to decode agent session history", err)
		return
	}
	if jsonOutput {
		printJSON(detail)
		return
	}
	history, _ := detail["history"].([]interface{})
	for _, item := range history {
		record, _ := item.(map[string]interface{})
		role := asCLIString(record["role"])
		if role == "" {
			role = asCLIString(record["type"])
		}
		fmt.Printf("[%s] %s\n", role, asCLIString(record["content"]))
	}
}

func runAgentSessionsSend(cmd *cobra.Command, args []string) {
	resp, err := doAgentSessionsRequest(http.MethodPost, getAgentSessionsBaseURL()+"/sessions/"+url.PathEscape(args[0])+"/messages", map[string]interface{}{
		"message": agentSessionsMessage,
	})
	if err != nil {
		printAgentSessionsError("Failed to send agent session message", err)
		return
	}
	var run map[string]interface{}
	if err := decodeAgentSessionsJSON(resp, &run); err != nil {
		printAgentSessionsError("Failed to decode send response", err)
		return
	}
	if jsonOutput {
		printJSON(run)
		return
	}
	fmt.Printf("Queued run %s\n", asCLIString(run["id"]))
}

func runAgentSessionsCancel(cmd *cobra.Command, args []string) {
	resp, err := doAgentSessionsRequest(http.MethodPost, getAgentSessionsBaseURL()+"/sessions/"+url.PathEscape(args[0])+"/cancel", nil)
	if err != nil {
		printAgentSessionsError("Failed to cancel agent session", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		printAgentSessionsError("Failed to cancel agent session", fmt.Errorf("request failed with status %d", resp.StatusCode))
		return
	}
	if jsonOutput {
		printJSON(map[string]bool{"cancelled": true})
		return
	}
	fmt.Printf("Cancelled session %s\n", args[0])
}

func runAgentSessionsClose(cmd *cobra.Command, args []string) {
	resp, err := doAgentSessionsRequest(http.MethodPost, getAgentSessionsBaseURL()+"/sessions/"+url.PathEscape(args[0])+"/close", nil)
	if err != nil {
		printAgentSessionsError("Failed to close agent session", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		printAgentSessionsError("Failed to close agent session", fmt.Errorf("request failed with status %d", resp.StatusCode))
		return
	}
	if jsonOutput {
		printJSON(map[string]bool{"closed": true})
		return
	}
	fmt.Printf("Closed session %s\n", args[0])
}

func runAgentSessionsProfiles(cmd *cobra.Command, args []string) {
	resp, err := doAgentSessionsRequest(http.MethodGet, getAgentSessionsBaseURL()+"/profiles", nil)
	if err != nil {
		printAgentSessionsError("Failed to list agent profiles", err)
		return
	}
	var result struct {
		Profiles []map[string]interface{} `json:"profiles"`
	}
	if err := decodeAgentSessionsJSON(resp, &result); err != nil {
		printAgentSessionsError("Failed to decode agent profiles", err)
		return
	}
	if jsonOutput {
		printJSON(result)
		return
	}
	for _, profile := range result.Profiles {
		fmt.Printf("%s  [%s]  %s\n", asCLIString(profile["id"]), asCLIString(profile["protocol"]), asCLIString(profile["title"]))
	}
}

func printAgentSessionsError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": false,
			"error":   msg,
			"details": err.Error(),
		})
	} else {
		fmt.Printf("Error: %s\n", msg)
		if err != nil {
			fmt.Printf("Details: %v\n", err)
		}
	}
	os.Exit(1)
}

func asCLIString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

