# RDxClaw: Industrial-Grade AI Agent Architecture

This document provides a comprehensive overview of the **RDxClaw** architecture. It is designed to help developers understand the internal workings and easily tweak or extend any functionality.

---

## 1. Core Architecture Philosophy

RDxClaw is built with three main goals:
1.  **Industrial-Grade Efficiency**: Minimal resource footprint (<10MB RAM, sub-second startup).
2.  **Business-First Execution**: Focus on real-world actions, not just conversation.
3.  **Edge-Ready Design**: Deployable on low-cost hardware ($10 SBCs) as a single Go binary.

---

## 2. High-Level Component Map

The project is organized into several key packages within `pkg/`:

| Package | Responsibility | Key Files |
| :--- | :--- | :--- |
| **`agent`** | Core agent loop, message processing, and tool orchestration. | `loop.go`, `context.go` |
| **`providers`** | Abstraction layer for LLM APIs (Claude, OpenAI, HTTP). | `types.go`, `http_provider.go` |
| **`tools`** | Individual capabilities (Filesystem, Web Search, Hardware Control). | `base.go`, `registry.go` |
| **`knowledge`** | RAG (Retrieval Augmented Generation) logic using local BM25. | `index.go`, `store.go`, `types.go` |
| **`session`** | Persistent chat history and context management. | `manager.go` |
| **`channels`** | External interfaces (Discord, Telegram, Slack, etc.). | `manager.go`, `base.go` |
| **`swarm`** | Multi-agent coordination and sub-agent spawning. | `manager.go`, `tools.go` |
| **`skills`** | Modular, installable capability packages. | `loader.go`, `manifest.go`, `installer.go` |

---

## 3. The Agent Brain: `pkg/agent`

The central "brain" of RDxClaw lives in `pkg/agent/loop.go`. It manages the relationship between the user, the LLM, and the tools.

### Key Workflows:
- **`runAgentLoop`**: The entry point for any incoming message. It builds the context (system prompt + history), calls the LLM, handles tool results, and manages iterations.
- **Iteration Management**: If the LLM requests a tool call, the agent executes the tool, feeds the result back to the LLM, and repeats until a final answer is reached or `maxIterations` is hit.
- **Context Management**: It keeps track of token usage and automatically compresses history (`forceCompression`) or summarizes older messages (`maybeSummarize`) to stay within the model's context window.

---

## 4. Extending the Agent

### A. Adding a New Tool
To add a new capability, creating a tool is the standard approach:
1.  Create a new file in `pkg/tools/` (e.g., `pkg/tools/weather.go`).
2.  Implement the `Tool` interface:
    ```go
    type Tool interface {
        Name() string
        Description() string
        Parameters() map[string]interface{}
        Execute(ctx context.Context, args map[string]interface{}) *ToolResult
    }
    ```
3.  Register the tool in `pkg/tools/registry.go` (within `createToolRegistry` or similar).

### B. Tweaking the System Prompt
The core persona of the agent is defined in `pkg/agent/context.go`. You can find the base system instructions there and adjust the agent's behavior, tone, or constraints.

### C. Adding a New LLM Provider
If you want to use a provider not already supported:
1.  Implement the `LLMProvider` interface in `pkg/providers/types.go` (defined in `providers/types.go`).
2.  Add your implementation in a new file like `pkg/providers/new_provider.go`.
3.  Ensure it handles different message roles (System, User, Assistant, Tool).

---

## 5. Industrial Capabilities: Hardware & RAG

### Hardware Control (`pkg/tools/i2c.go`, `pkg/tools/spi.go`)
RDxClaw is unique in its ability to talk directly to hardware. These tools allow the LLM to read sensors or control actuators on Linux-based edge devices via standard protocols.

### Local RAG (`pkg/knowledge`)
Instead of heavy vector databases, RDxClaw uses a highly efficient **BM25 algorithm** for local search for speed and memory efficiency.
- **Indexing**: Documents are processed and indexed for BM25-based keyword and semantic matching.
- **Retrieval**: When the agent detects it needs local context, it queries the `knowledge` tool to retrieve relevant snippets from the local store.
- **Key Implementation**: The logic is primarily located in `pkg/knowledge/index.go` (retrieval/search) and `pkg/knowledge/store.go` (storage/persistence).

---

## 6. Communication & Swarms

### Multi-Channel Support
The `pkg/channels` package allows the same agent instance to listen on multiple platforms simultaneously.
- **Gateway**: The REST API provider allows external apps to interact with the agent as if it were a standard OpenAI endpoint.
- **Connectors**: Discord, Slack, etc., are implemented as pluggable channels that translate platform-specific messages into the agent's internal message bus format.

### Swarm Orchestration (`pkg/swarm`)
The `spawn` tool allows the primary agent to create sub-agents for specific tasks. These sub-agents can run in the background, report results asynchronously, and manage their own specialized toolsets.

---

## 7. Project Structure Quick Reference

```text
/
├── cmd/rdxclaw/        # Entry point for the CLI and Server
├── pkg/
│   ├── agent/          # Brain & Execution Loop
│   ├── providers/      # LLM API Connectors
│   ├── tools/          # Functional Capabilities
│   ├── knowledge/      # Local RAG & Vector-lite memory
│   ├── session/        # Chat History & State
│   ├── channels/       # Discord, Slack, API integrations
│   └── swarm/          # Multi-agent coordination
├── workspace/          # Default data directory
│   ├── memory/         # RAG index
│   ├── sessions/       # Persistence
│   └── skills/         # Modular skill apps
└── Makefile            # Build and Test automation
```

---

## 8. Development & Debugging Tips

- **Verbose Logging**: Use `rdxclaw agent --verbose` to see the full interaction between the agent and tools.
- **Tool Testing**: Each tool should have a corresponding `_test.go` file in `pkg/tools/` for isolated validation.
- **Session Debugging**: Check `workspace/sessions/` to see the raw JSON state of active conversations.

---

> [!TIP]
> **Modifying Iterations**: If the agent is "giving up" too early or caught in a loop, check `maxIterations` in `pkg/agent/loop.go`. The default is usually set to balance cost and efficacy.
