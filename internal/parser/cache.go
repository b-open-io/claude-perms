package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/b-open-io/claude-perms/internal/types"
)

// CacheEntry represents cached permission stats for a session
type CacheEntry struct {
	FileHash string                  `json:"hash"` // mtime:size as cache key
	Stats    []types.PermissionStats `json:"stats"`
}

// AgentMappingEntry caches agentId->agentType mappings extracted from a session file
type AgentMappingEntry struct {
	FileHash string            `json:"hash"`
	Mappings map[string]string `json:"mappings"` // agentId -> agentType
}

// AgentSessionEntry caches parsed tool_use stats from an agent file
type AgentSessionEntry struct {
	FileHash string                  `json:"hash"`
	Perms    []types.PermissionStats `json:"perms"`
	LastSeen time.Time               `json:"lastSeen"`
}

// PermsCache holds all cached data for the permission analyzer
type PermsCache struct {
	Version       int                          `json:"version"`
	Sessions      map[string]CacheEntry        `json:"sessions"`      // session path -> permission stats
	AgentMappings map[string]AgentMappingEntry  `json:"agentMappings"` // session path -> agentId mappings
	AgentSessions map[string]AgentSessionEntry  `json:"agentSessions"` // agent file path -> parsed stats
}

const cacheVersion = 3

// cachePath returns the path to the cache file
func cachePath() string {
	return filepath.Join(claudeDir(), "perms-cache.json")
}

// loadCache loads the unified cache from disk
func loadCache() *PermsCache {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return newCache()
	}

	var cache PermsCache
	if err := json.Unmarshal(data, &cache); err != nil || cache.Version != cacheVersion {
		return newCache()
	}

	// Ensure maps are initialized
	if cache.Sessions == nil {
		cache.Sessions = make(map[string]CacheEntry)
	}
	if cache.AgentMappings == nil {
		cache.AgentMappings = make(map[string]AgentMappingEntry)
	}
	if cache.AgentSessions == nil {
		cache.AgentSessions = make(map[string]AgentSessionEntry)
	}

	return &cache
}

func newCache() *PermsCache {
	return &PermsCache{
		Version:       cacheVersion,
		Sessions:      make(map[string]CacheEntry),
		AgentMappings: make(map[string]AgentMappingEntry),
		AgentSessions: make(map[string]AgentSessionEntry),
	}
}

// saveCache writes the cache to disk
func saveCache(cache *PermsCache) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(), data, 0644)
}

// fileHash generates a hash from file metadata (mtime + size)
// This is an OS-level trick that doesn't read file contents
func fileHash(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	// Combine mtime and size as cache key
	key := fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size())
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:8]), nil // Use first 8 bytes (16 hex chars)
}

// getCachedStats returns cached permission stats if the file hasn't changed
func getCachedStats(cache *PermsCache, path string) ([]types.PermissionStats, bool) {
	hash, err := fileHash(path)
	if err != nil {
		return nil, false
	}

	entry, exists := cache.Sessions[path]
	if !exists || entry.FileHash != hash {
		return nil, false
	}

	return entry.Stats, true
}

// setCachedStats stores permission stats in the cache
func setCachedStats(cache *PermsCache, path string, stats []types.PermissionStats) {
	hash, err := fileHash(path)
	if err != nil {
		return
	}

	cache.Sessions[path] = CacheEntry{
		FileHash: hash,
		Stats:    stats,
	}
}

// getCachedAgentMappings returns cached agentId->agentType mappings if the file hasn't changed
func getCachedAgentMappings(cache *PermsCache, path string) (map[string]string, bool) {
	hash, err := fileHash(path)
	if err != nil {
		return nil, false
	}

	entry, exists := cache.AgentMappings[path]
	if !exists || entry.FileHash != hash {
		return nil, false
	}

	return entry.Mappings, true
}

// setCachedAgentMappings stores agentId mappings in the cache
func setCachedAgentMappings(cache *PermsCache, path string, mappings map[string]string) {
	hash, err := fileHash(path)
	if err != nil {
		return
	}

	cache.AgentMappings[path] = AgentMappingEntry{
		FileHash: hash,
		Mappings: mappings,
	}
}

// getCachedAgentSession returns cached agent session stats if the file hasn't changed
func getCachedAgentSession(cache *PermsCache, path string) ([]types.PermissionStats, time.Time, bool) {
	hash, err := fileHash(path)
	if err != nil {
		return nil, time.Time{}, false
	}

	entry, exists := cache.AgentSessions[path]
	if !exists || entry.FileHash != hash {
		return nil, time.Time{}, false
	}

	return entry.Perms, entry.LastSeen, true
}

// setCachedAgentSession stores agent session stats in the cache
func setCachedAgentSession(cache *PermsCache, path string, perms []types.PermissionStats, lastSeen time.Time) {
	hash, err := fileHash(path)
	if err != nil {
		return
	}

	cache.AgentSessions[path] = AgentSessionEntry{
		FileHash: hash,
		Perms:    perms,
		LastSeen: lastSeen,
	}
}

// LoadAllPermissionStatsWithCache loads stats with caching support
func LoadAllPermissionStatsWithCache(progress chan<- string) ([]types.PermissionStats, error) {
	projectsDir := filepath.Join(claudeDir(), "projects")
	return loadPermissionStatsWithCache(projectsDir, progress)
}

func loadPermissionStatsWithCache(projectsDir string, progress chan<- string) ([]types.PermissionStats, error) {
	cache := loadCache()
	cacheHits := 0
	cacheMisses := 0

	// Map to aggregate stats by permission
	statsMap := make(map[string]*types.PermissionStats)
	projectsMap := make(map[string]map[string]bool)

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(projectsDir, entry.Name())
		projectName := decodeProjectPath(entry.Name())

		if progress != nil {
			progress <- projectName
		}

		indexPath := filepath.Join(projectPath, "sessions-index.json")
		sessions, err := loadSessionsIndex(indexPath)
		if err != nil {
			continue
		}

		for _, session := range sessions {
			// Send session ID update (prefix with "session:" for parsing)
			if progress != nil {
				progress <- "session:" + session.SessionID[:12] + "..."
			}

			sessionPath := filepath.Join(projectPath, session.SessionID+".jsonl")

			var sessionTime time.Time
			if session.Modified != "" {
				sessionTime, _ = time.Parse(time.RFC3339, session.Modified)
			}
			if sessionTime.IsZero() {
				sessionTime = time.Unix(session.FileMtime/1000, 0)
			}

			// Try cache first
			var perms []types.PermissionStats
			if cached, hit := getCachedStats(cache, sessionPath); hit {
				perms = cached
				cacheHits++
			} else {
				// Parse and cache
				var err error
				perms, err = parseSessionLog(sessionPath, sessionTime)
				if err != nil {
					continue
				}
				setCachedStats(cache, sessionPath, perms)
				cacheMisses++
			}

			// Aggregate stats
			for _, p := range perms {
				key := PermissionKey(p.Permission)

				if _, exists := statsMap[key]; !exists {
					statsMap[key] = &types.PermissionStats{
						Permission: p.Permission,
						Count:      0,
						LastSeen:   time.Time{},
						Projects:   nil,
					}
					projectsMap[key] = make(map[string]bool)
				}

				statsMap[key].Count += p.Count
				statsMap[key].Approved += p.Approved
				statsMap[key].Denied += p.Denied
				if p.LastSeen.After(statsMap[key].LastSeen) {
					statsMap[key].LastSeen = p.LastSeen
				}
				projectsMap[key][projectName] = true
			}
		}
	}

	// Save cache
	if cacheMisses > 0 {
		_ = saveCache(cache)
	}

	if progress != nil {
		progress <- fmt.Sprintf("Cache: %d hits, %d misses", cacheHits, cacheMisses)
	}

	// Convert and sort
	stats := make([]types.PermissionStats, 0, len(statsMap))
	for key, s := range statsMap {
		projects := make([]string, 0, len(projectsMap[key]))
		for proj := range projectsMap[key] {
			projects = append(projects, proj)
		}
		s.Projects = projects
		stats = append(stats, *s)
	}

	// Sort by count descending
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].Count > stats[i].Count {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}

	return stats, nil
}
