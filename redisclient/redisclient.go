package redisclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

var (
	cache   = map[string]*clientEntry{}
	rw      sync.RWMutex
	started bool
	// idle timeout after which unused clients are closed
	idleTimeout = 5 * time.Minute
	// cleanup interval
	cleanupInterval = 1 * time.Minute
	ctx             = context.Background()
)

type clientEntry struct {
	client   *redis.Client
	lastUsed time.Time
}

func keyFor(addr, pass string) string {
	return addr + "|" + pass
}

// GetClient returns a shared *redis.Client for the given addr and password.
// It creates and pings a new client if needed, and updates lastUsed timestamp.
func GetClient(addr, pass string) (*redis.Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("empty addr")
	}
	k := keyFor(addr, pass)
	// start cleanup goroutine once
	rw.Lock()
	if !started {
		started = true
		go cleanupLoop()
	}
	// check existing (use read lock and verify client is alive)
	if e, ok := func() (*clientEntry, bool) {
		rw.RLock()
		defer rw.RUnlock()
		ent, ok := cache[k]
		return ent, ok
	}(); ok && e != nil {
		// ping to ensure client not closed (use short timeout so callers don't block long)
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := e.client.Ping(pingCtx).Err(); err == nil {
			// update lastUsed and return
			rw.Lock()
			e.lastUsed = time.Now()
			rw.Unlock()
			logrus.Debugf("redisclient: reusing client for %s", addr)
			return e.client, nil
		} else {
			// remove closed or unresponsive client
			logrus.Warnf("redisclient: cached client for %s appears closed/unresponsive: %v, recreating", addr, err)
			rw.Lock()
			delete(cache, k)
			rw.Unlock()
			_ = e.client.Close()
		}
	}

	// create new client with network timeouts to avoid long blocking on dial/read
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     pass,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	// ping with short timeout
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		// ensure closed
		_ = rdb.Close()
		return nil, err
	}
	entry := &clientEntry{client: rdb, lastUsed: time.Now()}

	rw.Lock()
	cache[k] = entry
	rw.Unlock()
	logrus.Infof("redisclient: created new client for %s", addr)
	return rdb, nil
}

func cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rw.Lock()
		for k, e := range cache {
			if now.Sub(e.lastUsed) > idleTimeout {
				logrus.Infof("redisclient: closing idle client %s", k)
				_ = e.client.Close()
				delete(cache, k)
			}
		}
		rw.Unlock()
	}
}

// CloseAll closes all cached clients (used in shutdown)
func CloseAll() {
	rw.Lock()
	for k, e := range cache {
		_ = e.client.Close()
		delete(cache, k)
	}
	rw.Unlock()
}
