package plugin

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// DependencyResolver handles plugin dependency resolution
type DependencyResolver struct {
	plugins map[string]*PluginInfo
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver(plugins map[string]*PluginInfo) *DependencyResolver {
	return &DependencyResolver{
		plugins: plugins,
	}
}

// DependencyError represents a dependency resolution error
type DependencyError struct {
	PluginID     string
	DependencyID string
	Reason       string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("plugin %s: dependency %s: %s", e.PluginID, e.DependencyID, e.Reason)
}

// ResolutionResult contains the result of dependency resolution
type ResolutionResult struct {
	// Order is the topologically sorted list of plugin IDs
	Order []string
	// Errors contains any dependency errors encountered
	Errors []*DependencyError
	// Warnings contains non-fatal issues (e.g., optional dependencies not found)
	Warnings []string
}

// Resolve resolves dependencies and returns a topologically sorted order
func (r *DependencyResolver) Resolve() *ResolutionResult {
	result := &ResolutionResult{
		Order:    make([]string, 0),
		Errors:   make([]*DependencyError, 0),
		Warnings: make([]string, 0),
	}

	// Build dependency graph
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize all plugins
	for id := range r.plugins {
		graph[id] = make([]string, 0)
		inDegree[id] = 0
	}

	// Build edges and check dependencies
	for id, info := range r.plugins {
		if info.Manifest == nil {
			continue
		}

		for _, dep := range info.Manifest.Dependencies {
			depInfo, exists := r.plugins[dep.ID]

			if !exists {
				if dep.Optional {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("plugin %s: optional dependency %s not found", id, dep.ID))
					continue
				}
				result.Errors = append(result.Errors, &DependencyError{
					PluginID:     id,
					DependencyID: dep.ID,
					Reason:       "dependency not found",
				})
				continue
			}

			// Check version constraint
			if dep.Version != "" && depInfo.Manifest != nil {
				if !r.checkVersionConstraint(depInfo.Manifest.Version, dep.Version) {
					if dep.Optional {
						result.Warnings = append(result.Warnings,
							fmt.Sprintf("plugin %s: optional dependency %s version %s does not satisfy %s",
								id, dep.ID, depInfo.Manifest.Version, dep.Version))
						continue
					}
					result.Errors = append(result.Errors, &DependencyError{
						PluginID:     id,
						DependencyID: dep.ID,
						Reason:       fmt.Sprintf("version %s does not satisfy constraint %s", depInfo.Manifest.Version, dep.Version),
					})
					continue
				}
			}

			// Add edge: dep.ID -> id (dep must be initialized before id)
			graph[dep.ID] = append(graph[dep.ID], id)
			inDegree[id]++
		}
	}

	// If there are errors, return early
	if len(result.Errors) > 0 {
		return result
	}

	// Topological sort using Kahn's algorithm
	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		result.Order = append(result.Order, current)

		// Process dependents
		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if len(result.Order) != len(r.plugins) {
		// Find plugins involved in cycle
		cyclePlugins := make([]string, 0)
		for id, degree := range inDegree {
			if degree > 0 {
				cyclePlugins = append(cyclePlugins, id)
			}
		}
		result.Errors = append(result.Errors, &DependencyError{
			PluginID:     strings.Join(cyclePlugins, ", "),
			DependencyID: "",
			Reason:       "circular dependency detected",
		})
	}

	return result
}

// checkVersionConstraint checks if a version satisfies a constraint
// Supports: exact match, ^major, ~minor, >=, <=, >, <, ranges
func (r *DependencyResolver) checkVersionConstraint(version, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	version = strings.TrimSpace(version)

	// Handle empty constraint (any version)
	if constraint == "" || constraint == "*" {
		return true
	}

	// Parse version
	v, err := parseVersion(version)
	if err != nil {
		return false
	}

	// Handle range constraints (e.g., ">=1.0.0 <2.0.0")
	if strings.Contains(constraint, " ") {
		parts := strings.Fields(constraint)
		for _, part := range parts {
			if !r.checkSingleConstraint(v, part) {
				return false
			}
		}
		return true
	}

	return r.checkSingleConstraint(v, constraint)
}

func (r *DependencyResolver) checkSingleConstraint(v *semver, constraint string) bool {
	// Handle caret (^) - compatible with major version
	if strings.HasPrefix(constraint, "^") {
		cv, err := parseVersion(strings.TrimPrefix(constraint, "^"))
		if err != nil {
			return false
		}
		// ^1.2.3 means >=1.2.3 <2.0.0
		if v.major != cv.major {
			return false
		}
		return v.compare(cv) >= 0
	}

	// Handle tilde (~) - compatible with minor version
	if strings.HasPrefix(constraint, "~") {
		cv, err := parseVersion(strings.TrimPrefix(constraint, "~"))
		if err != nil {
			return false
		}
		// ~1.2.3 means >=1.2.3 <1.3.0
		if v.major != cv.major || v.minor != cv.minor {
			return false
		}
		return v.compare(cv) >= 0
	}

	// Handle comparison operators
	if strings.HasPrefix(constraint, ">=") {
		cv, err := parseVersion(strings.TrimPrefix(constraint, ">="))
		if err != nil {
			return false
		}
		return v.compare(cv) >= 0
	}

	if strings.HasPrefix(constraint, "<=") {
		cv, err := parseVersion(strings.TrimPrefix(constraint, "<="))
		if err != nil {
			return false
		}
		return v.compare(cv) <= 0
	}

	if strings.HasPrefix(constraint, ">") {
		cv, err := parseVersion(strings.TrimPrefix(constraint, ">"))
		if err != nil {
			return false
		}
		return v.compare(cv) > 0
	}

	if strings.HasPrefix(constraint, "<") {
		cv, err := parseVersion(strings.TrimPrefix(constraint, "<"))
		if err != nil {
			return false
		}
		return v.compare(cv) < 0
	}

	// Handle exact match (with optional = prefix)
	constraint = strings.TrimPrefix(constraint, "=")
	cv, err := parseVersion(constraint)
	if err != nil {
		return false
	}
	return v.compare(cv) == 0
}

// semver represents a semantic version
type semver struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func (v *semver) compare(other *semver) int {
	if v.major != other.major {
		return v.major - other.major
	}
	if v.minor != other.minor {
		return v.minor - other.minor
	}
	if v.patch != other.patch {
		return v.patch - other.patch
	}
	// Prerelease versions have lower precedence
	if v.prerelease == "" && other.prerelease != "" {
		return 1
	}
	if v.prerelease != "" && other.prerelease == "" {
		return -1
	}
	return strings.Compare(v.prerelease, other.prerelease)
}

var (
	semverRegexOnce sync.Once
	semverRegex     *regexp.Regexp
)

func ensureSemverRegex() {
	semverRegexOnce.Do(func() {
		semverRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([a-zA-Z0-9.-]+))?(?:\+[a-zA-Z0-9.-]+)?$`)
	})
}

func parseVersion(version string) (*semver, error) {
	ensureSemverRegex()

	matches := semverRegex.FindStringSubmatch(version)
	if matches == nil {
		return nil, fmt.Errorf("invalid version: %s", version)
	}

	v := &semver{}

	v.major, _ = strconv.Atoi(matches[1])
	if matches[2] != "" {
		v.minor, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		v.patch, _ = strconv.Atoi(matches[3])
	}
	if matches[4] != "" {
		v.prerelease = matches[4]
	}

	return v, nil
}

// GetDependencies returns the dependencies of a plugin
func (r *DependencyResolver) GetDependencies(pluginID string) []string {
	info, exists := r.plugins[pluginID]
	if !exists || info.Manifest == nil {
		return nil
	}

	deps := make([]string, 0, len(info.Manifest.Dependencies))
	for _, dep := range info.Manifest.Dependencies {
		deps = append(deps, dep.ID)
	}
	return deps
}

// GetDependents returns plugins that depend on the given plugin
func (r *DependencyResolver) GetDependents(pluginID string) []string {
	dependents := make([]string, 0)

	for id, info := range r.plugins {
		if info.Manifest == nil {
			continue
		}
		for _, dep := range info.Manifest.Dependencies {
			if dep.ID == pluginID {
				dependents = append(dependents, id)
				break
			}
		}
	}

	return dependents
}

// CheckDependenciesSatisfied checks if all dependencies of a plugin are satisfied
func (r *DependencyResolver) CheckDependenciesSatisfied(pluginID string) (bool, []*DependencyError) {
	info, exists := r.plugins[pluginID]
	if !exists || info.Manifest == nil {
		return true, nil
	}

	errors := make([]*DependencyError, 0)

	for _, dep := range info.Manifest.Dependencies {
		depInfo, exists := r.plugins[dep.ID]

		if !exists {
			if !dep.Optional {
				errors = append(errors, &DependencyError{
					PluginID:     pluginID,
					DependencyID: dep.ID,
					Reason:       "dependency not found",
				})
			}
			continue
		}

		// Check if dependency is in error state
		if depInfo.Status == StatusError {
			if !dep.Optional {
				errors = append(errors, &DependencyError{
					PluginID:     pluginID,
					DependencyID: dep.ID,
					Reason:       "dependency is in error state",
				})
			}
			continue
		}

		// Check version constraint
		if dep.Version != "" && depInfo.Manifest != nil {
			if !r.checkVersionConstraint(depInfo.Manifest.Version, dep.Version) {
				if !dep.Optional {
					errors = append(errors, &DependencyError{
						PluginID:     pluginID,
						DependencyID: dep.ID,
						Reason:       fmt.Sprintf("version %s does not satisfy constraint %s", depInfo.Manifest.Version, dep.Version),
					})
				}
			}
		}
	}

	return len(errors) == 0, errors
}
