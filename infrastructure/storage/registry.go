package storage

import (
	"context"
	"fmt"
	"strconv"
)

// Registry holds all configured BlobStores and routes domains to the right one.
type Registry struct {
	stores       map[string]BlobStore
	defaultStore string
	domainStores map[string]string // domainID (string) → storeID
}

// NewRegistry builds a Registry from a Config, constructing each store.
func NewRegistry(cfg *Config) (*Registry, error) {
	r := &Registry{
		stores:       make(map[string]BlobStore),
		defaultStore: cfg.DefaultStore,
		domainStores: cfg.DomainStores,
	}
	if r.domainStores == nil {
		r.domainStores = map[string]string{}
	}

	for id, sc := range cfg.Stores {
		store, err := buildStore(id, sc)
		if err != nil {
			return nil, fmt.Errorf("registry: store %q: %w", id, err)
		}
		r.stores[id] = store
	}

	if r.defaultStore != "" {
		if _, ok := r.stores[r.defaultStore]; !ok {
			return nil, fmt.Errorf("registry: default_store %q not defined in stores", r.defaultStore)
		}
	}
	return r, nil
}

// ForDomain returns the BlobStore assigned to a domain ID.
// Falls back to the default store if no explicit mapping exists.
func (r *Registry) ForDomain(domainID uint) (BlobStore, error) {
	key := strconv.FormatUint(uint64(domainID), 10)
	if storeID, ok := r.domainStores[key]; ok {
		if s, ok := r.stores[storeID]; ok {
			return s, nil
		}
		return nil, fmt.Errorf("registry: domain %d mapped to unknown store %q", domainID, storeID)
	}
	if r.defaultStore == "" {
		return nil, fmt.Errorf("registry: no store configured")
	}
	s, ok := r.stores[r.defaultStore]
	if !ok {
		return nil, fmt.Errorf("registry: default store %q not found", r.defaultStore)
	}
	return s, nil
}

// Get returns a store by ID.
func (r *Registry) Get(id string) (BlobStore, error) {
	s, ok := r.stores[id]
	if !ok {
		return nil, fmt.Errorf("registry: store %q not found", id)
	}
	return s, nil
}

// All returns all registered stores.
func (r *Registry) All() map[string]BlobStore { return r.stores }

// EnsureBuckets calls EnsureBucket on all S3 stores.
func (r *Registry) EnsureBuckets(ctx context.Context) error {
	for id, s := range r.stores {
		if s3, ok := s.(*S3Store); ok {
			if err := s3.EnsureBucket(ctx); err != nil {
				return fmt.Errorf("ensure bucket for store %q: %w", id, err)
			}
		}
	}
	return nil
}

func buildStore(id string, sc StoreConfig) (BlobStore, error) {
	switch sc.Type {
	case "local":
		if sc.Path == "" {
			sc.Path = "files"
		}
		return NewLocalStore(id, sc.Path), nil
	case "s3":
		if sc.S3 == nil {
			return nil, fmt.Errorf("s3 config missing for store %q", id)
		}
		return NewS3Store(id, *sc.S3)
	default:
		return nil, fmt.Errorf("unknown store type %q", sc.Type)
	}
}
