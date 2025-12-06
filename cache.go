package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheEntry represents cached season data
type CacheEntry struct {
	Season      int       `json:"season"`
	Source      string    `json:"source"`
	FetchedAt   time.Time `json:"fetched_at"`
	EndDate     string    `json:"end_date"`
	Games       []Game    `json:"games"`
}

// Cache handles local storage of season data
type Cache struct {
	dir string
}

// NewCache creates a cache in the user's cache directory
func NewCache() (*Cache, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		// Fallback to current directory
		cacheDir = "."
	}

	dir := filepath.Join(cacheDir, "ncaa-bayes-elo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{dir: dir}, nil
}

// cacheFile returns the path to the cache file for a season/source
func (c *Cache) cacheFile(season int, source string) string {
	return filepath.Join(c.dir, fmt.Sprintf("%s_%d.json", source, season))
}

// Get retrieves cached games if available and not stale
func (c *Cache) Get(season int, source string) ([]Game, bool) {
	path := c.cacheFile(season, source)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Check if cache is still valid
	// For completed seasons (before current date), cache indefinitely
	// For current season, cache is valid if fetched today
	now := time.Now()
	seasonEnd := time.Date(season, time.April, 15, 0, 0, 0, 0, time.UTC)

	if now.After(seasonEnd) {
		// Season is complete, cache is always valid
		fmt.Printf("Using cached data for %d season (%d games)\n", season, len(entry.Games))
		return entry.Games, true
	}

	// Current season - check if cache was fetched today
	if entry.FetchedAt.Year() == now.Year() &&
	   entry.FetchedAt.YearDay() == now.YearDay() {
		fmt.Printf("Using today's cached data for %d season (%d games)\n", season, len(entry.Games))
		return entry.Games, true
	}

	// Cache is stale
	return nil, false
}

// Put stores games in the cache
func (c *Cache) Put(season int, source string, games []Game) error {
	entry := CacheEntry{
		Season:    season,
		Source:    source,
		FetchedAt: time.Now(),
		EndDate:   time.Now().Format("2006-01-02"),
		Games:     games,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	path := c.cacheFile(season, source)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	fmt.Printf("Cached %d games for %d season\n", len(games), season)
	return nil
}

// Clear removes cached data for a season
func (c *Cache) Clear(season int, source string) error {
	path := c.cacheFile(season, source)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ClearAll removes all cached data
func (c *Cache) ClearAll() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			os.Remove(filepath.Join(c.dir, entry.Name()))
		}
	}
	return nil
}
