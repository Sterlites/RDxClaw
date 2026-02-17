package knowledge

import "time"

// Document represents a source document (file, web page, etc.)
type Document struct {
	ID        string                 `json:"id"`
	Source    string                 `json:"source"` // file path or URL
	Type      string                 `json:"type"`   // "markdown", "text", "pdf", etc.
	Title     string                 `json:"title,omitempty"`
	Content   string                 `json:"content"` // Full content (optional to store)
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// Chunk represents a segment of a document for indexing
type Chunk struct {
	ID         string                 `json:"id"`
	DocumentID string                 `json:"document_id"`
	Content    string                 `json:"content"`
	Index      int                    `json:"index"` // Order in document
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResult represents a matched chunk with score
type SearchResult struct {
	Chunk      Chunk   `json:"chunk"`
	Score      float64 `json:"score"`
	DocumentID string  `json:"document_id"`
	Source     string  `json:"source"`
}

// Collection represents a grouping of documents (e.g., "codebase", "notes")
type Collection struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Documents int       `json:"documents_count"`
	Chunks    int       `json:"chunks_count"`
}
