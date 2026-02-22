package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Sterlites/RDxClaw/pkg/knowledge"
)

// KnowledgeTool provides access to the corporate memory/knowledge base.
type KnowledgeTool struct {
	store *knowledge.Store
}

// NewKnowledgeTool creates a new knowledge tool instance.
func NewKnowledgeTool(store *knowledge.Store) *KnowledgeTool {
	return &KnowledgeTool{
		store: store,
	}
}

func (t *KnowledgeTool) Name() string {
	return "knowledge"
}

func (t *KnowledgeTool) Description() string {
	return `Search, retrieve, and manage knowledge in the corporate memory (RAG).
Use this to find existing information or save new knowledge for future recall.
Capabilities:
- search: Find relevant information using keywords (BM25)
- add: Save text snippets or summaries
- ingest: Read and index a file (markdown, text, etc.)
- list: List available knowledge collections`
}

func (t *KnowledgeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"search", "add", "ingest", "list"},
				"description": "The action to perform: 'search', 'add', 'ingest', or 'list'",
			},
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "The knowledge collection name (default: 'general')",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query keywords (for action='search')",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Text content to save (for action='add')",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Title of the document (for action='add')",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Absolute path to file to ingest (for action='ingest')",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max number of results to return (default: 5)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *KnowledgeTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	action, _ := args["action"].(string)
	collection, _ := args["collection"].(string)
	if collection == "" {
		collection = "general"
	}

	switch action {
	case "search":
		return t.handleSearch(args, collection)
	case "add":
		return t.handleAdd(args, collection)
	case "ingest":
		return t.handleIngest(args, collection)
	case "list":
		return t.handleList()
	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action))
	}
}

func (t *KnowledgeTool) handleSearch(args map[string]interface{}, collection string) *ToolResult {
	query, _ := args["query"].(string)
	if query == "" {
		return ErrorResult("query is required for search action")
	}

	limit := 5
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	results, err := t.store.Search(collection, query, limit)
	if err != nil {
		return ErrorResult(fmt.Sprintf("search failed: %v", err))
	}

	if len(results) == 0 {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("No results found for '%s' in collection '%s'.", query, collection),
			ForUser: fmt.Sprintf("🔍 Searched '%s' in '%s': No matches found.", query, collection),
		}
	}

	// Format results for LLM
	var llmOutput string
	for i, res := range results {
		llmOutput += fmt.Sprintf("Result %d (Score: %.2f)\nSource: %s\nContent:\n%s\n\n---\n\n",
			i+1, res.Score, res.Source, res.Chunk.Content)
	}

	// Simplified summary for user
	userOutput := fmt.Sprintf("🔍 Found %d results for '%s' in '%s':\n", len(results), query, collection)
	for i, res := range results {
		if i >= 3 {
			break // Show max 3 to user
		}
		preview := res.Chunk.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		title := res.Chunk.Metadata["title"]
		if title == nil {
			title = "Untitled"
		}
		userOutput += fmt.Sprintf("- **%s**: %s\n", title, preview)
	}

	return &ToolResult{
		ForLLM:  llmOutput,
		ForUser: userOutput,
	}
}

func (t *KnowledgeTool) handleAdd(args map[string]interface{}, collection string) *ToolResult {
	content, _ := args["content"].(string)
	if content == "" {
		return ErrorResult("content is required for add action")
	}
	title, _ := args["title"].(string)
	if title == "" {
		title = "Untitled Note"
	}

	doc := knowledge.Document{
		Title:   title,
		Content: content,
		Source:  "manual",
		Type:    "text",
		Metadata: map[string]interface{}{
			"title": title,
		},
	}

	if err := t.store.AddDocument(collection, doc); err != nil {
		return ErrorResult(fmt.Sprintf("failed to adding document: %v", err))
	}

	return &ToolResult{
		ForLLM:  fmt.Sprintf("Successfully added document '%s' to collection '%s'.", title, collection),
		ForUser: fmt.Sprintf("📝 Added '%s' to knowledge base '%s'.", title, collection),
	}
}

func (t *KnowledgeTool) handleIngest(args map[string]interface{}, collection string) *ToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		return ErrorResult("path is required for ingest action")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to read file: %v", err))
	}

	filename := filepath.Base(path)
	ext := filepath.Ext(filename)

	doc := knowledge.Document{
		Title:   filename,
		Content: string(content),
		Source:  path,
		Type:    ext,
		Metadata: map[string]interface{}{
			"title":    filename,
			"filename": filename,
			"path":     path,
		},
	}

	if err := t.store.AddDocument(collection, doc); err != nil {
		return ErrorResult(fmt.Sprintf("failed to ingest document: %v", err))
	}

	return &ToolResult{
		ForLLM:  fmt.Sprintf("Successfully ingested file '%s' into collection '%s'.", filename, collection),
		ForUser: fmt.Sprintf("📥 Ingested '%s' into knowledge base '%s'.", filename, collection),
	}
}

func (t *KnowledgeTool) handleList() *ToolResult {
	collections, err := t.store.ListCollections()
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to list collections: %v", err))
	}

	if len(collections) == 0 {
		return &ToolResult{
			ForLLM:  "No knowledge collections found.",
			ForUser: "📚 No knowledge collections found.",
		}
	}

	// Sort by name
	sort.Slice(collections, func(i, j int) bool {
		return collections[i].Name < collections[j].Name
	})

	output := "Available Knowledge Collections:\n"
	for _, c := range collections {
		output += fmt.Sprintf("- **%s**: %d documents, %d chunks\n", c.Name, c.Documents, c.Chunks)
	}

	return &ToolResult{
		ForLLM:  output,
		ForUser: output,
	}
}
