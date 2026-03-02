# RDxClaw Architectural Map

This diagram visualizes the flow of information and command execution within the RDxClaw framework.

```mermaid
graph TD
    %% External Interfaces
    User([User / External System]) --> |Message| Channels
    
    subgraph Channels_Layer [Channels & Interfaces]
        Channels{Channel Manager}
        Channels --> Discord[Discord Connector]
        Channels --> Slack[Slack Connector]
        Channels --> Telegram[Telegram Connector]
        Channels --> REST[Headless REST API]
    end

    %% Internal Communication
    Channels_Layer --> |InboundMessage| Bus[Internal Message Bus]
    Bus --> Agent_Brain

    subgraph Agent_Brain [The Agent Brain - pkg/agent]
        Loop[Agent Loop - loop.go]
        Context[Context Builder - context.go]
        State[State Manager]
        
        Loop <--> Context
        Loop <--> State
    end

    %% Intelligence Layer
    Agent_Brain --> |Prompt| LLM_Gateway{LLM Provider Layer}
    
    subgraph Providers [LLM Providers - pkg/providers]
        LLM_Gateway --> OpenAI[OpenAI / GPT-4o]
        LLM_Gateway --> Claude[Anthropic / Claude 3.5]
        LLM_Gateway --> HTTP[Custom HTTP Endpoint]
    end
    
    Providers --> |JSON Response / Tool Call| Agent_Brain

    %% Execution Layer
    Agent_Brain --> |Execute| Registry{Tool Registry}

    subgraph Tools [Function Capabilities - pkg/tools]
        Registry --> FS[Filesystem - read/write]
        Registry --> Web[Web - search/fetch]
        Registry --> Hardware[Hardware - I2C/SPI]
        Registry --> Knowledge[Knowledge - RAG]
        Registry --> Swarm[Spawn - Sub-agents]
    end

    %% Memory & Storage
    subgraph Data_Storage [Workspace & Memory]
        Knowledge <--> BM25[Local BM25 Index]
        BM25 <--> Disk[(workspace/memory/)]
        FS <--> Workspace[(workspace/)]
        Loop <--> Sessions[(workspace/sessions/)]
    end

    %% Specialized Modules
    subgraph Modules [Specialized Services]
        Swarm --> Swarm_Mgr[Swarm Manager]
        Swarm_Mgr --> SubAgent[Sub-agent Instance]
        Skills[Skills Manager] --> Registry
    end

    %% Feedback Loop
    Tools --> |Result| Agent_Brain
    Agent_Brain -- Iteration --> LLM_Gateway
    Agent_Brain --> |Final Answer| Bus
    Bus --> Channels_Layer
    Channels_Layer --> |Response| User

    %% Styling
    classDef brain fill:#6366f1,stroke:#fff,stroke-width:2px,color:#fff
    classDef logic fill:#f59e0b,stroke:#fff,stroke-width:2px
    classDef storage fill:#10b981,stroke:#fff,stroke-width:2px,color:#fff
    classDef interface fill:#3b82f6,stroke:#fff,stroke-width:2px,color:#fff
    
    class Loop,Context,Agent_Brain brain
    class Registry,LLM_Gateway,Channels logic
    class BM25,Disk,Workspace,Sessions storage
    class User,Channels_Layer interface
```

---

## Interactive Flow Explanation

### 1. The Inbound Flow
A request enters through the **Channel Manager**. Whether it's a Slack message or a REST API call, it is normalized into an `InboundMessage` and sent over the internal **Message Bus**.

### 2. The Thought Process (Loop)
The **Agent Loop** picks up the message and:
1.  Loads the persistent **Session** history.
2.  Identifies the current **Workspace** state.
3.  Builds a context-rich prompt using the **Context Builder**.
4.  Queries the **LLM Provider**.

### 3. Tool Execution & RAG
If the LLM decides to take an action (e.g., "Read the sensors"), the agent routes the request through the **Tool Registry**. 
- For hardware tasks, it uses the **I2C/SPI** tools.
- For information retrieval, it queries the **Knowledge** tool, which searches the local **BM25 Index**.

### 4. The Iteration Loop
The result of any tool call is fed back into the **Agent Brain**. The brain then updates the context and asks the LLM, "With this new data, what's our next step?" This continues until a final answer is generated.

### 5. Swarms & Skills
- **Skills** are pre-packaged logic that can register new tools dynamically.
- **Swarms** allow the main agent to delegate long-running tasks to background sub-agents, keeping the main loop responsive.
