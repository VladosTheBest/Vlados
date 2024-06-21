package auth

import (
	"sync"
)

// Cache godoc
type Cache struct {
	// list of permissions for each role
	roles map[string]map[string]bool
	// read write lock on data
	lock *sync.RWMutex
}

var cache *Cache

func init() {
	cache = &Cache{
		roles: make(map[string]map[string]bool),
		lock:  &sync.RWMutex{},
	}
}

// HasPerm godoc
func HasPerm(role, perm string) (hasPerm bool) {
	cache.lock.RLock()
	if role, ok := cache.roles[role]; ok {
		hasPerm = role[perm]
	}
	cache.lock.RUnlock()
	return
}

// SetAll godoc
func SetAll(roles map[string]map[string]bool) {
	cache.lock.Lock()
	cache.roles = roles
	cache.lock.Unlock()
}
