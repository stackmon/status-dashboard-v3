package auth

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalStorage_StoreAndGet(t *testing.T) {
	s := newInternalStorage()
	token := TokenRepr{AccessToken: "at", RefreshToken: "rt"}

	s.Store("key1", token)

	got, ok := s.Get("key1")
	require.True(t, ok)
	assert.Equal(t, "at", got.AccessToken)
	assert.Equal(t, "rt", got.RefreshToken)
}

func TestInternalStorage_GetMissing(t *testing.T) {
	s := newInternalStorage()
	_, ok := s.Get("nonexistent")
	assert.False(t, ok)
}

func TestInternalStorage_Delete(t *testing.T) {
	s := newInternalStorage()
	s.Store("key1", TokenRepr{AccessToken: "at"})

	s.Delete("key1")

	_, ok := s.Get("key1")
	assert.False(t, ok)
}

func TestInternalStorage_DeleteNonexistent(t *testing.T) {
	s := newInternalStorage()
	assert.NotPanics(t, func() { s.Delete("nope") })
}

func TestInternalStorage_Overwrite(t *testing.T) {
	s := newInternalStorage()
	s.Store("key1", TokenRepr{AccessToken: "old"})
	s.Store("key1", TokenRepr{AccessToken: "new"})

	got, ok := s.Get("key1")
	require.True(t, ok)
	assert.Equal(t, "new", got.AccessToken)
}

func TestInternalStorage_ConcurrentAccess(_ *testing.T) {
	s := newInternalStorage()
	var wg sync.WaitGroup
	const n = 100

	// Concurrent writes
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := "key"
			s.Store(key, TokenRepr{AccessToken: "at"})
		}()
	}

	// Concurrent reads
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Get("key")
		}()
	}

	// Concurrent deletes
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Delete("key")
		}()
	}

	wg.Wait()
}
