package security

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Storage handles persistence of security data.
type Storage struct {
	basePath string
	mu       sync.RWMutex

	// In-memory caches
	events     []SecurityEvent
	blockedIPs map[string]*BlockedIP
	scanCache  *ScanCache
}

// ScanCache stores cached scan results.
type ScanCache struct {
	Items     []SecurityScanItem `json:"items"`
	Summary   ScanSummary        `json:"summary"`
	Timestamp time.Time          `json:"timestamp"`
}

// NewStorage creates a new security storage instance.
func NewStorage(basePath string) (*Storage, error) {
	s := &Storage{
		basePath:   basePath,
		events:     make([]SecurityEvent, 0),
		blockedIPs: make(map[string]*BlockedIP),
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(basePath, "events"),
		filepath.Join(basePath, "blocked"),
		filepath.Join(basePath, "scans"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Load existing data
	if err := s.loadEvents(); err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}
	if err := s.loadBlockedIPs(); err != nil {
		return nil, fmt.Errorf("failed to load blocked IPs: %w", err)
	}
	if err := s.loadScanCache(); err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to load scan cache: %v\n", err)
	}

	return s, nil
}

// eventsFile returns the file path for events on a given date.
func (s *Storage) eventsFile(t time.Time) string {
	return filepath.Join(s.basePath, "events", fmt.Sprintf("%s.jsonl", t.Format("2006-01-02")))
}

// blockedIPsFile returns the file path for blocked IPs.
func (s *Storage) blockedIPsFile() string {
	return filepath.Join(s.basePath, "blocked", "blocked_ips.json")
}

// scanCacheFile returns the file path for scan cache.
func (s *Storage) scanCacheFile() string {
	return filepath.Join(s.basePath, "scans", "last_scan.json")
}

// SaveEvent saves a security event to storage.
func (s *Storage) SaveEvent(event *SecurityEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Append to daily event file
	filePath := s.eventsFile(event.Timestamp)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create events directory: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open events file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Update in-memory cache
	s.events = append(s.events, *event)

	// Keep only last 1000 events in memory
	if len(s.events) > 1000 {
		s.events = s.events[len(s.events)-1000:]
	}

	return nil
}

// GetEvents retrieves events with optional filtering.
func (s *Storage) GetEvents(limit, offset int, eventType string) ([]SecurityEvent, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter events
	var filtered []SecurityEvent
	for _, event := range s.events {
		if eventType != "" && event.Type != eventType {
			continue
		}
		filtered = append(filtered, event)
	}

	total := len(filtered)

	// Sort by timestamp descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply pagination
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if limit <= 0 || end > len(filtered) {
		end = len(filtered)
	}
	filtered = filtered[offset:end]

	return filtered, total, nil
}

// SaveBlockedIP saves a blocked IP to storage.
func (s *Storage) SaveBlockedIP(blocked *BlockedIP) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.blockedIPs[blocked.IPAddress] = blocked
	return s.writeBlockedIPs()
}

// RemoveBlockedIP removes a blocked IP from storage.
func (s *Storage) RemoveBlockedIP(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.blockedIPs, ip)
	return s.writeBlockedIPs()
}

// GetBlockedIPs retrieves all blocked IPs.
func (s *Storage) GetBlockedIPs() map[string]*BlockedIP {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*BlockedIP)
	for k, v := range s.blockedIPs {
		result[k] = v
	}
	return result
}

// IsIPBlocked checks if an IP is blocked.
func (s *Storage) IsIPBlocked(ip string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blocked, ok := s.blockedIPs[ip]
	if !ok {
		return false
	}

	// Check if block has expired
	if !blocked.Permanent && timeutil.NowNano() > blocked.ExpiresAt.UnixNano() {
		return false
	}

	return true
}

// SaveScanCache saves scan results to cache.
func (s *Storage) SaveScanCache(items []SecurityScanItem, summary ScanSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scanCache = &ScanCache{
		Items:     items,
		Summary:   summary,
		Timestamp: timeutil.NowTime(),
	}

	filePath := s.scanCacheFile()
	data, err := json.MarshalIndent(s.scanCache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal scan cache: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write scan cache: %w", err)
	}

	return nil
}

// GetScanCache retrieves cached scan results.
func (s *Storage) GetScanCache() *ScanCache {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scanCache
}

// CleanupExpired removes expired data.
func (s *Storage) CleanupExpired(retentionDays int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeutil.NowTime()
	cutoff := now.AddDate(0, 0, -retentionDays)

	// Cleanup old event files
	eventsDir := filepath.Join(s.basePath, "events")
	entries, err := os.ReadDir(eventsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read events directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".jsonl")
		date, err := time.Parse("2006-01-02", name)
		if err != nil {
			continue
		}
		if date.Before(cutoff) {
			filePath := filepath.Join(eventsDir, entry.Name())
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("failed to remove old event file: %w", err)
			}
		}
	}

	// Cleanup expired blocked IPs
	for ip, blocked := range s.blockedIPs {
		if !blocked.Permanent && now.After(blocked.ExpiresAt) {
			delete(s.blockedIPs, ip)
		}
	}
	if err := s.writeBlockedIPs(); err != nil {
		return fmt.Errorf("failed to update blocked IPs: %w", err)
	}

	// Cleanup in-memory events
	var filtered []SecurityEvent
	for _, event := range s.events {
		if event.Timestamp.After(cutoff) {
			filtered = append(filtered, event)
		}
	}
	s.events = filtered

	return nil
}

// Close closes the storage.
func (s *Storage) Close() error {
	return nil
}

// Helper methods

func (s *Storage) loadEvents() error {
	eventsDir := filepath.Join(s.basePath, "events")
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Load only recent events (last 7 days)
	cutoff := timeutil.NowTime().AddDate(0, 0, -7)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".jsonl")
		date, err := time.Parse("2006-01-02", name)
		if err != nil {
			continue
		}
		if date.Before(cutoff) {
			continue
		}

		filePath := filepath.Join(eventsDir, entry.Name())
		events, err := s.readEventsFromFile(filePath)
		if err != nil {
			continue
		}
		s.events = append(s.events, events...)
	}

	// Sort by timestamp
	sort.Slice(s.events, func(i, j int) bool {
		return s.events[i].Timestamp.Before(s.events[j].Timestamp)
	})

	// Keep only last 1000 events
	if len(s.events) > 1000 {
		s.events = s.events[len(s.events)-1000:]
	}

	return nil
}

func (s *Storage) readEventsFromFile(filePath string) ([]SecurityEvent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []SecurityEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event SecurityEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, scanner.Err()
}

func (s *Storage) loadBlockedIPs() error {
	filePath := s.blockedIPsFile()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var blocked map[string]*BlockedIP
	if err := json.Unmarshal(data, &blocked); err != nil {
		return err
	}

	s.blockedIPs = blocked
	return nil
}

func (s *Storage) writeBlockedIPs() error {
	filePath := s.blockedIPsFile()
	data, err := json.MarshalIndent(s.blockedIPs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func (s *Storage) loadScanCache() error {
	filePath := s.scanCacheFile()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var cache ScanCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return err
	}

	s.scanCache = &cache
	return nil
}

// EventStats represents aggregated event statistics.
type EventStats struct {
	Period     string         `json:"period"`      // "day", "week", "month"
	StartDate  time.Time      `json:"start_date"`
	EndDate    time.Time      `json:"end_date"`
	TotalCount int            `json:"total_count"`
	ByType     map[string]int `json:"by_type"`
	ByDay      []DayStats     `json:"by_day"`
}

// DayStats represents statistics for a single day.
type DayStats struct {
	Date   string         `json:"date"`
	Count  int            `json:"count"`
	ByType map[string]int `json:"by_type"`
}

// GetEventStats returns aggregated event statistics for the specified period.
func (s *Storage) GetEventStats(period string) (*EventStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := timeutil.NowTime()
	var startDate time.Time

	switch period {
	case "day":
		startDate = now.AddDate(0, 0, -1)
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	default:
		startDate = now.AddDate(0, 0, -7) // Default to week
		period = "week"
	}

	stats := &EventStats{
		Period:    period,
		StartDate: startDate,
		EndDate:   now,
		ByType:    make(map[string]int),
		ByDay:     []DayStats{},
	}

	// Group events by day
	dayMap := make(map[string]*DayStats)

	for _, event := range s.events {
		if event.Timestamp.Before(startDate) {
			continue
		}

		stats.TotalCount++
		stats.ByType[event.Type]++

		dayKey := event.Timestamp.Format("2006-01-02")
		if _, ok := dayMap[dayKey]; !ok {
			dayMap[dayKey] = &DayStats{
				Date:   dayKey,
				Count:  0,
				ByType: make(map[string]int),
			}
		}
		dayMap[dayKey].Count++
		dayMap[dayKey].ByType[event.Type]++
	}

	// Convert map to sorted slice
	days := make([]string, 0, len(dayMap))
	for day := range dayMap {
		days = append(days, day)
	}
	sort.Strings(days)

	for _, day := range days {
		stats.ByDay = append(stats.ByDay, *dayMap[day])
	}

	return stats, nil
}
