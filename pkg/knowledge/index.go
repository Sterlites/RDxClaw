package knowledge

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	// BM25 parameters
	k1 = 1.2
	b  = 0.75

	// Chunking parameters
	chunkSize    = 1000 // characters
	chunkOverlap = 200  // characters
)

// Posting stores the TF for a term in a specific document/chunk.
type Posting struct {
	ChunkID string `json:"id"`
	TF      int    `json:"tf"`
}

// Index implements a simple BM25 search index.
type Index struct {
	Name        string               `json:"name"`
	Docs        map[string]Chunk     `json:"docs"`         // Map of ChunkID -> Chunk
	InvertedIdx map[string][]Posting `json:"inverted_idx"` // Map of Term -> []Posting
	DocLengths  map[string]int       `json:"doc_lengths"`  // Map of ChunkID -> WordCount
	DocCount    int                  `json:"doc_count"`
	SumDocLen   int                  `json:"sum_doc_len"` // Sum of all document lengths
	mu          sync.RWMutex
}

// NewIndex creates a new search index.
func NewIndex(name string) *Index {
	return &Index{
		Name:        name,
		Docs:        make(map[string]Chunk),
		InvertedIdx: make(map[string][]Posting),
		DocLengths:  make(map[string]int),
	}
}

// AddDocument chunks a document and adds it to the index.
func (idx *Index) AddDocument(doc Document) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	chunks := chunkText(doc.Content, chunkSize, chunkOverlap)
	for i, content := range chunks {
		chunkID := fmt.Sprintf("%s_chk_%d", doc.ID, i)
		chunk := Chunk{
			ID:         chunkID,
			DocumentID: doc.ID,
			Content:    content,
			Index:      i,
			Metadata:   doc.Metadata,
		}

		// Store chunk
		idx.Docs[chunkID] = chunk

		// Tokenize and calculate TF
		tokens := tokenize(content)
		docLen := len(tokens)
		idx.DocLengths[chunkID] = docLen
		idx.SumDocLen += docLen
		idx.DocCount++

		termFreqs := make(map[string]int)
		for _, token := range tokens {
			termFreqs[token]++
		}

		// Update inverted index with postings
		for term, count := range termFreqs {
			idx.InvertedIdx[term] = append(idx.InvertedIdx[term], Posting{
				ChunkID: chunkID,
				TF:      count,
			})
		}
	}

	return nil
}

// Search searches the index using BM25.
func (idx *Index) Search(query string, limit int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.DocCount == 0 {
		return []SearchResult{}, nil
	}

	queryTokens := tokenize(query)
	scores := make(map[string]float64)
	avgDocLen := float64(idx.SumDocLen) / float64(idx.DocCount)

	for _, term := range queryTokens {
		// Get postings list for term
		postings := idx.InvertedIdx[term]
		docFreq := len(postings)
		if docFreq == 0 {
			continue
		}

		// IDF (Inverse Document Frequency)
		// idf = log( (N - n + 0.5) / (n + 0.5) + 1 )
		idf := math.Log(float64(idx.DocCount-docFreq+0.5)/float64(docFreq+0.5) + 1)

		for _, posting := range postings {
			chunkID := posting.ChunkID
			tf := float64(posting.TF)

			// Use cached doc length
			docLen := float64(idx.DocLengths[chunkID])

			// BM25 Score formula
			numerator := tf * (k1 + 1)
			denominator := tf + k1*(1-b+b*(docLen/avgDocLen))
			scores[chunkID] += idf * (numerator / denominator)
		}
	}

	// Sort results
	var results []SearchResult
	for chunkID, score := range scores {
		chunk := idx.Docs[chunkID]
		results = append(results, SearchResult{
			Chunk:      chunk,
			Score:      score,
			DocumentID: chunk.DocumentID,
			Source:     fmt.Sprintf("chunk:%s", chunkID),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score // Descending
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Save persists the index to disk.
func (idx *Index) Save(dir string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s.index.json", idx.Name))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	// encoder.SetIndent("", "  ") // Save space
	return encoder.Encode(idx)
}

// Load loads an index from disk.
func LoadIndex(name, dir string) (*Index, error) {
	filename := filepath.Join(dir, fmt.Sprintf("%s.index.json", name))
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return NewIndex(name), nil
		}
		return nil, err
	}
	defer file.Close()

	var idx Index
	if err := json.NewDecoder(file).Decode(&idx); err != nil {
		return nil, err
	}
	idx.Name = name // Ensure name matches
	return &idx, nil
}

// --- Helpers ---

var tokenRegexp = regexp.MustCompile(`[a-zA-Z0-9]+`)

func tokenize(text string) []string {
	matches := tokenRegexp.FindAllString(strings.ToLower(text), -1)
	return matches
}

func chunkText(text string, size, overlap int) []string {
	if len(text) <= size {
		return []string{text}
	}

	var chunks []string
	runes := []rune(text)

	for i := 0; i < len(runes); i += (size - overlap) {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
		if end >= len(runes) {
			break
		}
	}
	return chunks
}
