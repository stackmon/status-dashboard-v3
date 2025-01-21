package auth

import "sync"

// TODO: think about TTL for tokens
type internalStorage struct {
	mu sync.RWMutex
	m  map[string]TokenRepr
}

func newInternalStorage() *internalStorage {
	return &internalStorage{
		m: make(map[string]TokenRepr),
	}
}

// Store sets the value for a key.
func (cm *internalStorage) Store(key string, value TokenRepr) {
	cm.mu.Lock()
	cm.m[key] = value
	cm.mu.Unlock()
}

// Get retrieves the value for a key.
func (cm *internalStorage) Get(key string) (TokenRepr, bool) {
	cm.mu.RLock()
	value, ok := cm.m[key]
	cm.mu.RUnlock()
	return value, ok
}

// Delete removes the value for a key.
func (cm *internalStorage) Delete(key string) {
	cm.mu.Lock()
	delete(cm.m, key)
	cm.mu.Unlock()
}
