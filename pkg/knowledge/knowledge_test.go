package knowledge

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBM25(t *testing.T) {
	idx := NewIndex("test")

	// Document 1: "The quick brown fox jumps over the lazy dog"
	doc1 := Document{
		ID:      "doc1",
		Content: "The quick brown fox jumps over the lazy dog",
		Metadata: map[string]interface{}{
			"title": "Fox",
		},
	}
	err := idx.AddDocument(doc1)
	require.NoError(t, err)

	// Document 2: "The quick red fox jumps over the lazy dog"
	doc2 := Document{
		ID:      "doc2",
		Content: "The quick red fox jumps over the lazy dog",
		Metadata: map[string]interface{}{
			"title": "Red Fox",
		},
	}
	err = idx.AddDocument(doc2)
	require.NoError(t, err)

	// Document 3: "The lazy dog sleeps"
	doc3 := Document{
		ID:      "doc3",
		Content: "The lazy dog sleeps",
		Metadata: map[string]interface{}{
			"title": "Dog",
		},
	}
	err = idx.AddDocument(doc3)
	require.NoError(t, err)

	// Search for "brown fox"
	results, err := idx.Search("brown fox", 10)
	require.NoError(t, err)
	require.Len(t, results, 2) // doc1 and doc2 both contain "fox", doc3 doesn't

	// doc1 contains "brown", doc2 doesn't. doc1 should score higher.
	assert.Equal(t, "doc1", results[0].DocumentID)
	assert.Greater(t, results[0].Score, results[1].Score)

	// Search for "red"
	results, err = idx.Search("red", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "doc2", results[0].DocumentID)

	// Search for "lazy dog"
	results, err = idx.Search("lazy dog", 10)
	require.NoError(t, err)
	require.Len(t, results, 3) // all docs contain "lazy dog"

	// doc3 is shortest, so "lazy dog" matches more densely? No, doc3 is shorter, so score might be higher.
	// But doc1 and doc2 contain "quick" "jumps" etc.
	// Let's print scores just to debug if needed, but assertion should be flexible.
	// Just ensure we get results.
}

func TestStorePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)

	// Add doc to "notes"
	doc := Document{
		ID:      "note1",
		Content: "Persistent knowledge is valuable.",
		Title:   "Value",
	}
	err = store.AddDocument("notes", doc)
	require.NoError(t, err)

	// Search in same instance
	results, err := store.Search("notes", "knowledge", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "note1", results[0].DocumentID)

	// Create new store instance (simulate restart)
	store2, err := NewStore(tmpDir)
	require.NoError(t, err)

	// Search should still work (loaded from disk)
	results, err = store2.Search("notes", "valuable", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "note1", results[0].DocumentID)

	// Check collection list
	collections, err := store2.ListCollections()
	require.NoError(t, err)
	require.Len(t, collections, 1)
	assert.Equal(t, "notes", collections[0].Name)
	assert.Equal(t, 1, collections[0].Documents)
}

func TestLargeDocChunking(t *testing.T) {
	idx := NewIndex("test")
	
	// Create large content
	largeContent := "word "
	for i := 0; i < 2000; i++ {
		largeContent += "word "
	}
	// Total length > 10000 chars

	doc := Document{
		ID:      "large1",
		Content: largeContent,
	}
	err := idx.AddDocument(doc)
	require.NoError(t, err)

	// Should be chunked
	assert.Greater(t, len(idx.Docs), 5) // at least several chunks
	
	// Verify all chunks belong to doc
	for _, chunk := range idx.Docs {
		assert.Equal(t, "large1", chunk.DocumentID)
	}
}
