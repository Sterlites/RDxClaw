package knowledge

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Store manages multiple knowledge collections (indexes).
type Store struct {
	baseDir string
	indexes map[string]*Index
	mu      sync.RWMutex
}

// NewStore initializes a new knowledge store in the given directory.
func NewStore(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create knowledge store directory: %w", err)
	}

	return &Store{
		baseDir: baseDir,
		indexes: make(map[string]*Index),
	}, nil
}

// GetIndex returns an index by name, loading it from disk if necessary.
func (s *Store) GetIndex(name string) (*Index, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Normalize name
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return nil, fmt.Errorf("index name cannot be empty")
	}

	// Check memory cache
	if idx, ok := s.indexes[name]; ok {
		return idx, nil
	}

	// Try loading from disk
	idx, err := LoadIndex(name, s.baseDir)
	if err == nil {
		s.indexes[name] = idx
		return idx, nil
	}

	// Create new index
	idx = NewIndex(name)
	s.indexes[name] = idx

	// Save immediately to ensure file exists
	if err := idx.Save(s.baseDir); err != nil {
		return nil, fmt.Errorf("failed to save new index '%s': %w", name, err)
	}

	return idx, nil
}

// AddDocument adds a document to a specific collection.
func (s *Store) AddDocument(collection string, doc Document) error {
	idx, err := s.GetIndex(collection)
	if err != nil {
		return err
	}

	// Ensure document has ID and timestamps
	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc_%d", time.Now().UnixNano())
	}
	now := time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	if err := idx.AddDocument(doc); err != nil {
		return err
	}

	// Persist index after modification
	return idx.Save(s.baseDir)
}

// Search searches a specific collection.
func (s *Store) Search(collection, query string, limit int) ([]SearchResult, error) {
	idx, err := s.GetIndex(collection)
	if err != nil {
		return nil, err
	}

	return idx.Search(query, limit)
}

// ListCollections returns a list of available collections.
func (s *Store) ListCollections() ([]Collection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var collections []Collection

	// Read directory for index files
	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".index.json") {
			name := strings.TrimSuffix(file.Name(), ".index.json")

			// Load index stats (lightweight load if we optimize LoadIndex later, currently full load)
			// Since we want stats, we might need to load it. For now, let's just peek if already loaded or create minimal info.

			// Check if loaded
			var docCount, chunkCount int
			if idx, ok := s.indexes[name]; ok {
				docCount = idx.DocCount
				chunkCount = len(idx.Docs)
			} else {
				// Load temporarily to get stats (this might be slow for many indexes)
				// Optimization: store stats in separate metadata file.
				// For now (MVP), just load it.
				idx, err := LoadIndex(name, s.baseDir)
				if err == nil {
					docCount = idx.DocCount
					chunkCount = len(idx.Docs)
					// Cache it
					s.indexes[name] = idx
				}
			}

			collections = append(collections, Collection{
				Name:      name,
				Documents: docCount,
				Chunks:    chunkCount,
			})
		}
	}

	return collections, nil
}
