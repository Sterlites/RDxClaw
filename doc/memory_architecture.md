# rdxclaw Memory Architecture

This document describes the memory and state management architecture of the rdxclaw agent.

## Overview

rdxclaw operates on a file-based memory system designed for transparency and ease of manipulation. It differentiates between **Structured State** (technical, machine-readable) and **Unstructured Memory** (cognitive, LLM-readable).

## Architecture Diagram

```mermaid
graph TD
    subgraph "Execution Context"
        Agent["Agent Loop"]
        CB["ContextBuilder (pkg/agent/context.go)"]
        LLM["LLM Inference"]
    end

    subgraph "Memory Systems (Cognitive)"
        MS["MemoryStore (pkg/agent/memory.go)"]
        
        LTM[("Long Term Memory<br/>workspace/memory/MEMORY.md")]
        STM[("Daily Notes<br/>workspace/memory/YYYYMM/DD.md")]
        Core[("Core Identity<br/>workspace/SOUL.md<br/>workspace/AGENTS.md")]
    end

    subgraph "State Systems (Technical)"
        SM["StateManager (pkg/state/state.go)"]
        CS["CronService (pkg/cron/service.go)"]
        
        StateDB[("Session State<br/>workspace/state/state.json")]
        CronDB[("Scheduled Jobs<br/>workspace/cron.json")]
    end

    %% Flows
    Agent --> CB
    CB -- "Builds System Prompt" --> LLM
    
    %% Memory Access
    CB -- "ReadLongTerm()" --> MS
    CB -- "GetRecentDailyNotes(3)" --> MS
    CB -- "LoadBootstrapFiles()" --> Core
    
    MS <-- "Read/Write Markdown" --> LTM
    MS <-- "Append/Read Context" --> STM
    
    %% State Access
    Agent -- "Update Last Channel" --> SM
    SM <-- "Atomic Write JSON" --> StateDB
    
    Agent -- "Check Schedule" --> CS
    CS <-- "Sync Jobs JSON" --> CronDB
    
    %% Interactions
    LLM -- "Tool Call (write_file)" --> MS
    LLM -- "Tool Call (cron)" --> CS

    classDef memory fill:#e1f5fe,stroke:#01579b,stroke-width:2px;
    classDef state fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;
    classDef process fill:#fff3e0,stroke:#ff6f00,stroke-width:2px;
    
    class LTM,STM,Core,StateDB,CronDB memory;
    class Agent,CB,LLM,MS,SM,CS process;
```

## Component Details

### 1. MemoryStore (`pkg/agent/memory.go`)
Manages the agent's cognitive memory stored in Markdown files.
- **Long-Term Memory**: Stores persistent facts and user preferences in `memory/MEMORY.md`.
- **Daily Notes**: Stores episodic memory in `memory/YYYYMM/YYYYMMDD.md`.
- **Access Pattern**: The ContextBuilder injects `MEMORY.md` and the last 3 days of daily notes into every prompt.

### 2. StateManager (`pkg/state/state.go`)
Manages technical session state using JSON.
- **Session State**: Tracks the last active channel and chat ID to support seamless conversation resumption.
- **Consistency**: Uses atomic file writes (write to temp + rename) to prevent corruption.

### 3. CronService (`pkg/cron/service.go`)
Manages the scheduling of tasks.
- **Persistence**: Stores job definitions and next run times in a JSON file within the workspace.
- **Resolution**: Handles `at`, `every`, and `cron` schedule types.

### 4. Component Bootstrapping (`pkg/agent/context.go`)
Loads the agent's core identity files (`SOUL.md`, `AGENTS.md`) directly into the system prompt to define personality and capabilities.
