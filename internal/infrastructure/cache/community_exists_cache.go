package cache

import (
	"context"
	"sync"
	"time"

	"github.com/joacominatel/pulse/internal/domain"
)

// CommunityExistsCache is a simple in-memory cache for community existence checks.
// avoids hitting the database on every event ingestion request.
// uses a simple TTL-based expiration strategy.
type CommunityExistsCache struct {
	entries map[string]*communityEntry
	mu      sync.RWMutex
	ttl     time.Duration
	repo    domain.CommunityRepository
}

type communityEntry struct {
	exists    bool
	isActive  bool
	expiresAt time.Time
}

// NewCommunityExistsCache creates a new community existence cache.
func NewCommunityExistsCache(repo domain.CommunityRepository, ttl time.Duration) *CommunityExistsCache {
	return &CommunityExistsCache{
		entries: make(map[string]*communityEntry),
		ttl:     ttl,
		repo:    repo,
	}
}

// CheckActive checks if a community exists and is active.
// returns (exists, isActive, error).
// uses cache if available, otherwise queries the database.
func (c *CommunityExistsCache) CheckActive(ctx context.Context, id domain.CommunityID) (exists, isActive bool, err error) {
	idStr := id.String()

	// fast path: check cache
	c.mu.RLock()
	entry, ok := c.entries[idStr]
	if ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.exists, entry.isActive, nil
	}
	c.mu.RUnlock()

	// slow path: query database
	community, err := c.repo.FindByID(ctx, id)
	if err != nil {
		if err == domain.ErrNotFound {
			// cache negative result
			c.mu.Lock()
			c.entries[idStr] = &communityEntry{
				exists:    false,
				isActive:  false,
				expiresAt: time.Now().Add(c.ttl),
			}
			c.mu.Unlock()
			return false, false, nil
		}
		return false, false, err
	}

	// cache positive result
	c.mu.Lock()
	c.entries[idStr] = &communityEntry{
		exists:    true,
		isActive:  community.IsActive(),
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return true, community.IsActive(), nil
}

// Invalidate removes a community from the cache.
// call this when a community is created or its status changes.
func (c *CommunityExistsCache) Invalidate(id domain.CommunityID) {
	c.mu.Lock()
	delete(c.entries, id.String())
	c.mu.Unlock()
}

// Size returns the current number of cached entries.
func (c *CommunityExistsCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Cleanup removes expired entries.
// call this periodically to prevent memory growth.
func (c *CommunityExistsCache) Cleanup() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	for id, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, id)
		}
	}
}
