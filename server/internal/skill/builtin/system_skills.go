package builtin

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// Files is a built-in file operations skill
type Files struct {
	manifest *skill.Manifest
	baseDir  string // Restrict operations to this directory
}

// NewFiles creates a new files skill
func NewFiles(baseDir string) *Files {
	if baseDir == "" {
		baseDir = "."
	}
	return &Files{
		manifest: &skill.Manifest{
			ID:          "files",
			Name:        "Files",
			Version:     "1.0.0",
			Description: "File system operations",
			Category:    "system",
			Icon:        "files",
			Tags:        []string{"files", "filesystem", "io", "system"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: list, read, write, delete, info, search",
					Required:    true,
				},
				{
					Name:        "path",
					Type:        "string",
					Description: "File or directory path",
					Required:    false,
				},
				{
					Name:        "content",
					Type:        "string",
					Description: "Content to write (for write action)",
					Required:    false,
				},
				{
					Name:        "pattern",
					Type:        "string",
					Description: "Search pattern (for search action)",
					Required:    false,
				},
				{
					Name:        "recursive",
					Type:        "boolean",
					Description: "Recursive operation",
					Required:    false,
					Default:     false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "files",
					Type:        "array",
					Description: "List of files",
				},
				{
					Name:        "content",
					Type:        "string",
					Description: "File content",
				},
				{
					Name:        "info",
					Type:        "object",
					Description: "File information",
				},
			},
			Permissions: []string{"files.read", "files.write"},
		},
		baseDir: baseDir,
	}
}

// Manifest returns the skill manifest
func (f *Files) Manifest() *skill.Manifest {
	return f.manifest
}

// Validate validates the input parameters
func (f *Files) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"list": true, "read": true, "write": true,
		"delete": true, "info": true, "search": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	switch actionStr {
	case "read", "delete", "info":
		if _, ok := input["path"]; !ok {
			return fmt.Errorf("path is required for %s action", actionStr)
		}
	case "write":
		if _, ok := input["path"]; !ok {
			return fmt.Errorf("path is required for write action")
		}
		if _, ok := input["content"]; !ok {
			return fmt.Errorf("content is required for write action")
		}
	case "search":
		if _, ok := input["pattern"]; !ok {
			return fmt.Errorf("pattern is required for search action")
		}
	}

	return nil
}

// Execute executes the files skill
func (f *Files) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "list":
		return f.listFiles(input)
	case "read":
		return f.readFile(input)
	case "write":
		return f.writeFile(input)
	case "delete":
		return f.deleteFile(input)
	case "info":
		return f.fileInfo(input)
	case "search":
		return f.searchFiles(input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (f *Files) resolvePath(path string) (string, error) {
	if path == "" {
		return f.baseDir, nil
	}

	// Resolve to absolute path
	absPath := filepath.Join(f.baseDir, path)
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", err
	}

	// Security: ensure path is within baseDir
	baseAbs, _ := filepath.Abs(f.baseDir)
	if !strings.HasPrefix(absPath, baseAbs) {
		return "", fmt.Errorf("path outside allowed directory")
	}

	return absPath, nil
}

func (f *Files) listFiles(input map[string]any) (*skill.Result, error) {
	path := ""
	if p, ok := input["path"].(string); ok {
		path = p
	}

	absPath, err := f.resolvePath(path)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	var files []map[string]any
	for _, entry := range entries {
		info, _ := entry.Info()
		file := map[string]any{
			"name":  entry.Name(),
			"isDir": entry.IsDir(),
		}
		if info != nil {
			file["size"] = info.Size()
			file["modified"] = info.ModTime().Format("2006-01-02 15:04:05")
		}
		files = append(files, file)
	}

	return skill.NewResult(map[string]any{
		"path":  path,
		"files": files,
		"count": len(files),
	}), nil
}

func (f *Files) readFile(input map[string]any) (*skill.Result, error) {
	path := input["path"].(string)

	absPath, err := f.resolvePath(path)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"path":    path,
		"content": string(content),
		"size":    len(content),
	}), nil
}

func (f *Files) writeFile(input map[string]any) (*skill.Result, error) {
	path := input["path"].(string)
	content := input["content"].(string)

	absPath, err := f.resolvePath(path)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	// Ensure directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return skill.NewErrorResult(err), nil
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"path":    path,
		"written": len(content),
		"message": fmt.Sprintf("File '%s' written successfully", path),
	}), nil
}

func (f *Files) deleteFile(input map[string]any) (*skill.Result, error) {
	path := input["path"].(string)

	absPath, err := f.resolvePath(path)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	if err := os.Remove(absPath); err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"path":    path,
		"deleted": true,
		"message": fmt.Sprintf("File '%s' deleted successfully", path),
	}), nil
}

func (f *Files) fileInfo(input map[string]any) (*skill.Result, error) {
	path := input["path"].(string)

	absPath, err := f.resolvePath(path)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"path":     path,
		"name":     info.Name(),
		"size":     info.Size(),
		"isDir":    info.IsDir(),
		"mode":     info.Mode().String(),
		"modified": info.ModTime().Format("2006-01-02 15:04:05"),
	}), nil
}

func (f *Files) searchFiles(input map[string]any) (*skill.Result, error) {
	pattern := input["pattern"].(string)
	path := ""
	if p, ok := input["path"].(string); ok {
		path = p
	}

	absPath, err := f.resolvePath(path)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	var matches []string
	err = filepath.WalkDir(absPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if matched, _ := filepath.Match(pattern, d.Name()); matched {
			relPath, _ := filepath.Rel(absPath, p)
			matches = append(matches, relPath)
		}
		return nil
	})

	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"pattern": pattern,
		"path":    path,
		"matches": matches,
		"count":   len(matches),
	}), nil
}

// Network is a built-in network diagnostics skill
type Network struct {
	manifest *skill.Manifest
}

// NewNetwork creates a new network skill
func NewNetwork() *Network {
	return &Network{
		manifest: &skill.Manifest{
			ID:          "network",
			Name:        "Network",
			Version:     "1.0.0",
			Description: "Network diagnostics and information",
			Category:    "system",
			Icon:        "network",
			Tags:        []string{"network", "diagnostics", "connectivity", "system"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: info, check",
					Required:    true,
				},
				{
					Name:        "host",
					Type:        "string",
					Description: "Host to check (for check action)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "info",
					Type:        "object",
					Description: "Network information",
				},
				{
					Name:        "reachable",
					Type:        "boolean",
					Description: "Whether host is reachable",
				},
			},
		},
	}
}

func (n *Network) Manifest() *skill.Manifest { return n.manifest }

func (n *Network) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}
	if actionStr == "check" {
		if _, ok := input["host"]; !ok {
			return fmt.Errorf("host is required for check action")
		}
	}
	return nil
}

func (n *Network) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)
	switch action {
	case "info":
		hostname, _ := os.Hostname()
		return skill.NewResult(map[string]any{
			"hostname": hostname,
			"message":  "Network information retrieved",
		}), nil
	case "check":
		host := input["host"].(string)
		return skill.NewResult(map[string]any{
			"host":      host,
			"reachable": true,
			"message":   fmt.Sprintf("Connectivity check for %s (placeholder)", host),
		}), nil
	}
	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// Processes is a built-in process management skill
type Processes struct {
	manifest *skill.Manifest
}

// NewProcesses creates a new processes skill
func NewProcesses() *Processes {
	return &Processes{
		manifest: &skill.Manifest{
			ID:          "processes",
			Name:        "Processes",
			Version:     "1.0.0",
			Description: "Process information and management",
			Category:    "system",
			Icon:        "processes",
			Tags:        []string{"processes", "system", "monitoring"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: list, info",
					Required:    true,
				},
				{
					Name:        "pid",
					Type:        "number",
					Description: "Process ID (for info action)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "processes",
					Type:        "array",
					Description: "List of processes",
				},
				{
					Name:        "process",
					Type:        "object",
					Description: "Process information",
				},
			},
			Permissions: []string{"system.processes"},
		},
	}
}

func (p *Processes) Manifest() *skill.Manifest { return p.manifest }

func (p *Processes) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}
	validActions := map[string]bool{"list": true, "info": true}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}
	return nil
}

func (p *Processes) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)
	switch action {
	case "list":
		return skill.NewResult(map[string]any{
			"processes": []map[string]any{
				{"pid": os.Getpid(), "name": "zimaos-blue"},
			},
			"message": "Process listing (limited implementation)",
		}), nil
	case "info":
		pid := os.Getpid()
		if p, ok := input["pid"].(float64); ok {
			pid = int(p)
		}
		return skill.NewResult(map[string]any{
			"pid":     pid,
			"message": fmt.Sprintf("Process info for PID %d (placeholder)", pid),
		}), nil
	}
	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

