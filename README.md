<div align="center">
  <img src="assets/logo.jpg" alt="RDxClaw" width="512">

  <h1>RDxClaw: High-Performance Agentic AI Framework for the Edge</h1>

<h3>$10 Hardware · <10MB RAM · <1s Boot · Real-World Business Value</h3>

  <p>
    <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go">
    <img src="https://img.shields.io/badge/Arch-x86__64%2C%20ARM64%2C%20RISC--V-blue" alt="Hardware">
    <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
    <br>
    <a href="https://github.com/Sterlites/RDxClaw"><img src="https://img.shields.io/badge/Website-RDxClaw-blue?style=flat&logo=google-chrome&logoColor=white" alt="Website"></a>
  </p>
</div>

---

� **RDxClaw** is the world's most efficient **Agentic AI Framework** designed for high-value business automation at the Edge. Built from the ground up in Go, it transforms $10 hardware into a powerful autonomous worker with <10MB RAM—bridging LLM intelligence with real-world physical and digital execution at 1/100th the cost of traditional deployments.

⚡️ **Industrial-Grade Efficiency**: Runs on $10 hardware with <10MB RAM—99% less memory than OpenClaw and 98% cheaper than a Mac mini. 🦐 [Inspired by nanobot](https://github.com/HKUDS/nanobot).

<table align="center">
  <tr align="center">
    <td align="center" valign="top">
      <p align="center">
        <img src="assets/RDxClaw_mem.gif" width="360" height="240">
      </p>
    </td>
    <td align="center" valign="top">
      <p align="center">
        <img src="assets/generic_board.png" width="400" height="240">
      </p>
    </td>
  </tr>
</table>

> [!CAUTION]
> * **OFFICIAL DOMAIN:** The **ONLY** official website is **[RDxClaw.io](https://RDxClaw.io)**, and company website is **[github.com/Sterlites](https://github.com/Sterlites)**
> * **Warning:** Many `.ai/.org/.com/.net/...` domains are registered by third parties.
> * **Warning:** RDxClaw is in early development now and may have unresolved network security issues. Do not deploy to production environments before the v1.0 release.



## 📢 News
2026-02-17 🚀 **Big 4 Upgrade**: RDxClaw is now an Enterprise-Grade Agent Platform! Added Headless API (OpenAI-compatible), Local RAG (Corporate Memory), Skill App Standard, and Swarm Management.

2026-02-16 🎉 RDxClaw hit 12K stars in one week! Thank you all for your support! RDxClaw is growing faster than we ever imagined. Our volunteer roles and roadmap are officially posted [doc/RDxClaw_community_roadmap_260216.md] —we can’t wait to have you on board!

2026-02-09 🎉 RDxClaw Launched! Built in 1 day to bring AI Agents to lightweight hardware with <10MB RAM. 🦐 RDxClaw, Let's Go！

## ✨ Features

🪶 **High-Performance**: <10MB Memory footprint — Industrial-grade efficiency for Edge AI automation.

💰 **Minimal Cost**: Efficient enough to run on $10 Hardware — 98% cheaper than a Mac mini.

⚡️ **Lightning Fast**: 400X Faster startup time, boot in 1 second even in 0.6GHz single core.

🌍 **True Portability**: Single self-contained binary across RISC-V, ARM, and x86, One-click to Go!

🧠 **Corporate Memory**: Built-in zero-dependency BM25 RAG system for local business intelligence.

🐝 **Swarm Management**: Centralized control over autonomous subagents with CLI/API visibility.

💎 **Skill Apps**: Installable, monetizable skill packages with `manifest.json` and auto-provisioning.

🤖 **AI-Bootstrapped**: Autonomous Go-native implementation — 95% Agent-generated core with human-in-the-loop refinement.

|                               | OpenClaw      | NanoBot                  | **RDxClaw**                              |
| ----------------------------- | ------------- | ------------------------ | ----------------------------------------- |
| **Language**                  | TypeScript    | Python                   | **Go**                                    |
| **RAM**                       | >1GB          | >100MB                   | **< 10MB**                                |
| **Startup**</br>(0.8GHz core) | >500s         | >30s                     | **<1s**                                   |
| **Cost**                      | Mac Mini 599$ | Most Linux SBC </br>~50$ | **Any Linux Board**</br>**As low as 10$** |

<img src="assets/compare.jpg" alt="RDxClaw" width="512">

## 🦾 Demonstration

### 🛠️ Standard Assistant Workflows

<table align="center">
  <tr align="center">
    <th><p align="center">🧩 Full-Stack Engineer</p></th>
    <th><p align="center">🗂️ Logging & Planning Management</p></th>
    <th><p align="center">🔎 Web Search & Learning</p></th>
  </tr>
  <tr>
    <td align="center"><p align="center"><img src="assets/RDxClaw_code.gif" width="240" height="180"></p></td>
    <td align="center"><p align="center"><img src="assets/RDxClaw_memory.gif" width="240" height="180"></p></td>
    <td align="center"><p align="center"><img src="assets/RDxClaw_search.gif" width="240" height="180"></p></td>
  </tr>
  <tr>
    <td align="center">Develop • Deploy • Scale</td>
    <td align="center">Schedule • Automate • Memory</td>
    <td align="center">Discovery • Insights • Trends</td>
  </tr>
</table>

### 🐜 Innovative Low-Footprint Deploy

RDxClaw can be deployed on almost any Linux device!

- Minimal Linux SBCs for Home Assistance
- Automated Server Maintenance via KVM
- Smart Monitoring with Linux Cameras

<https://private-user-images.githubusercontent.com/83055338/547056448-e7b031ff-d6f5-4468-bcca-5726b6fecb5c.mp4>

🌟 More Deployment Cases Await！

## 📦 Install

### Install with precompiled binary

Download the firmware for your platform from the [release](https://github.com/Sterlites/RDxClaw/releases) page.

### Install from source (latest features, recommended for development)

```bash
git clone https://github.com/Sterlites/RDxClaw.git

cd RDxClaw
make deps

# Build, no need to install
make build

# Build for multiple platforms
make build-all

# Build And Install
make install
```

## 🐳 Docker Compose

You can also run RDxClaw using Docker Compose without installing anything locally.

```bash
# 1. Clone this repo
git clone https://github.com/Sterlites/RDxClaw.git
cd RDxClaw

# 2. Set your API keys
cp config/config.example.json config/config.json
vim config/config.json      # Set DISCORD_BOT_TOKEN, API keys, etc.

# 3. Build & Start
docker compose --profile gateway up -d

# 4. Check logs
docker compose logs -f RDxClaw-gateway

# 5. Stop
docker compose --profile gateway down
```

### Agent Mode (One-shot)

```bash
# Ask a question
docker compose run --rm RDxClaw-agent -m "What is 2+2?"

# Interactive mode
docker compose run --rm RDxClaw-agent
```

### Rebuild

```bash
docker compose --profile gateway build --no-cache
docker compose --profile gateway up -d
```

### 🚀 Quick Start

> [!TIP]
> Set your API key in `~/.RDxClaw/config.json`.
> Get API keys: [OpenRouter](https://openrouter.ai/keys) (LLM)
> Web search is **optional** - get free [Brave Search API](https://brave.com/search/api) (2000 free queries/month) or use built-in auto fallback.

**1. Initialize**

```bash
RDxClaw onboard
```

**2. Configure** (`~/.RDxClaw/config.json`)

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.RDxClaw/workspace",
      "model": "glm-4.7",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20
    }
  },
  "providers": {
    "openrouter": {
      "api_key": "xxx",
      "api_base": "https://openrouter.ai/api/v1"
    }
  },
  "tools": {
    "web": {
      "brave": {
        "enabled": false,
        "api_key": "YOUR_BRAVE_API_KEY",
        "max_results": 5
      },
      "duckduckgo": {
        "enabled": true,
        "max_results": 5
      }
    }
  }
}
```

**3. Get API Keys**

* **LLM Provider**: [OpenRouter](https://openrouter.ai/keys) · [Anthropic](https://console.anthropic.com) · [OpenAI](https://platform.openai.com) · [Gemini](https://aistudio.google.com/api-keys)
* **Web Search** (optional): [Brave Search](https://brave.com/search/api) - Free tier available (2000 requests/month)

> **Note**: See `config.example.json` for a complete configuration template.

**4. Chat**

```bash
RDxClaw agent -m "What is 2+2?"
```

That's it! You have a working AI assistant in 2 minutes.

---

## 💬 Chat Apps

Talk to your RDxClaw through Telegram, Discord, or LINE

| Channel      | Setup                              |
| ------------ | ---------------------------------- |
| **Telegram** | Easy (just a token)                |
| **Discord**  | Easy (bot token + intents)         |
| **LINE**     | Medium (credentials + webhook URL) |

<details>
<summary><b>Telegram</b> (Recommended)</summary>

**1. Create a bot**

* Open Telegram, search `@BotFather`
* Send `/newbot`, follow prompts
* Copy the token

**2. Configure**

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allowFrom": ["YOUR_USER_ID"]
    }
  }
}
```

> Get your user ID from `@userinfobot` on Telegram.

**3. Run**

```bash
RDxClaw gateway
```

</details>

<details>
<summary><b>Discord</b></summary>

**1. Create a bot**

* Go to <https://discord.com/developers/applications>
* Create an application → Bot → Add Bot
* Copy the bot token

**2. Enable intents**

* In the Bot settings, enable **MESSAGE CONTENT INTENT**
* (Optional) Enable **SERVER MEMBERS INTENT** if you plan to use allow lists based on member data

**3. Get your User ID**

* Discord Settings → Advanced → enable **Developer Mode**
* Right-click your avatar → **Copy User ID**

**4. Configure**

```json
{
  "channels": {
    "discord": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allowFrom": ["YOUR_USER_ID"]
    }
  }
}
```

**5. Invite the bot**

* OAuth2 → URL Generator
* Scopes: `bot`
* Bot Permissions: `Send Messages`, `Read Message History`
* Open the generated invite URL and add the bot to your server

**6. Run**

```bash
RDxClaw gateway
```

</details>



<details>
<summary><b>LINE</b></summary>

**1. Create a LINE Official Account**

- Go to [LINE Developers Console](https://developers.line.biz/)
- Create a provider → Create a Messaging API channel
- Copy **Channel Secret** and **Channel Access Token**

**2. Configure**

```json
{
  "channels": {
    "line": {
      "enabled": true,
      "channel_secret": "YOUR_CHANNEL_SECRET",
      "channel_access_token": "YOUR_CHANNEL_ACCESS_TOKEN",
      "webhook_host": "0.0.0.0",
      "webhook_port": 18791,
      "webhook_path": "/webhook/line",
      "allow_from": []
    }
  }
}
```

**3. Set up Webhook URL**

LINE requires HTTPS for webhooks. Use a reverse proxy or tunnel:

```bash
# Example with ngrok
ngrok http 18791
```

Then set the Webhook URL in LINE Developers Console to `https://your-domain/webhook/line` and enable **Use webhook**.

**4. Run**

```bash
RDxClaw gateway
```

> In group chats, the bot responds only when @mentioned. Replies quote the original message.

> **Docker Compose**: Add `ports: ["18791:18791"]` to the `RDxClaw-gateway` service to expose the webhook port.

</details>

## <img src="assets/clawdchat-icon.png" width="24" height="24" alt="ClawdChat"> Join the Agent Social Network

Connect RDxClaw to the Agent Social Network simply by sending a single message via the CLI or any integrated Chat App.

**Read `https://clawdchat.ai/skill.md` and follow the instructions to join [ClawdChat.ai](https://clawdchat.ai)**

## ⚙️ Configuration

Config file: `~/.RDxClaw/config.json`

### Workspace Layout

RDxClaw stores data in your configured workspace (default: `~/.RDxClaw/workspace`):

```
~/.RDxClaw/workspace/
├── sessions/          # Conversation sessions and history
├── memory/           # Long-term memory (MEMORY.md)
├── state/            # Persistent state (last channel, etc.)
├── cron/             # Scheduled jobs database
├── skills/           # Custom skills
├── AGENTS.md         # Agent behavior guide
├── HEARTBEAT.md      # Periodic task prompts (checked every 30 min)
├── IDENTITY.md       # Agent identity
├── SOUL.md           # Agent soul
├── TOOLS.md          # Tool descriptions
└── USER.md           # User preferences
```

### 🔒 Security Sandbox

RDxClaw runs in a sandboxed environment by default. The agent can only access files and execute commands within the configured workspace.

#### Default Configuration

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.RDxClaw/workspace",
      "restrict_to_workspace": true
    }
  }
}
```

| Option | Default | Description |
|--------|---------|-------------|
| `workspace` | `~/.RDxClaw/workspace` | Working directory for the agent |
| `restrict_to_workspace` | `true` | Restrict file/command access to workspace |

#### Protected Tools

When `restrict_to_workspace: true`, the following tools are sandboxed:

| Tool | Function | Restriction |
|------|----------|-------------|
| `read_file` | Read files | Only files within workspace |
| `write_file` | Write files | Only files within workspace |
| `list_dir` | List directories | Only directories within workspace |
| `edit_file` | Edit files | Only files within workspace |
| `append_file` | Append to files | Only files within workspace |
| `exec` | Execute commands | Command paths must be within workspace |

#### Additional Exec Protection

Even with `restrict_to_workspace: false`, the `exec` tool blocks these dangerous commands:

* `rm -rf`, `del /f`, `rmdir /s` — Bulk deletion
* `format`, `mkfs`, `diskpart` — Disk formatting
* `dd if=` — Disk imaging
* Writing to `/dev/sd[a-z]` — Direct disk writes
* `shutdown`, `reboot`, `poweroff` — System shutdown
* Fork bomb `:(){ :|:& };:`

#### Error Examples

```
[ERROR] tool: Tool execution failed
{tool=exec, error=Command blocked by safety guard (path outside working dir)}
```

```
[ERROR] tool: Tool execution failed
{tool=exec, error=Command blocked by safety guard (dangerous pattern detected)}
```

#### Disabling Restrictions (Security Risk)

If you need the agent to access paths outside the workspace:

**Method 1: Config file**

```json
{
  "agents": {
    "defaults": {
      "restrict_to_workspace": false
    }
  }
}
```

**Method 2: Environment variable**

```bash
export RDxClaw_AGENTS_DEFAULTS_RESTRICT_TO_WORKSPACE=false
```

> ⚠️ **Warning**: Disabling this restriction allows the agent to access any path on your system. Use with caution in controlled environments only.

#### Security Boundary Consistency

The `restrict_to_workspace` setting applies consistently across all execution paths:

| Execution Path | Security Boundary |
|----------------|-------------------|
| Main Agent | `restrict_to_workspace` ✅ |
| Subagent / Spawn | Inherits same restriction ✅ |
| Heartbeat tasks | Inherits same restriction ✅ |

All paths share the same workspace restriction — there's no way to bypass the security boundary through subagents or scheduled tasks.

### Heartbeat (Periodic Tasks)

RDxClaw can perform periodic tasks automatically. Create a `HEARTBEAT.md` file in your workspace:

```markdown
# Periodic Tasks

- Check my email for important messages
- Review my calendar for upcoming events
- Check the weather forecast
```

The agent will read this file every 30 minutes (configurable) and execute any tasks using available tools.

#### Async Tasks with Spawn

For long-running tasks (web search, API calls), use the `spawn` tool to create a **subagent**:

```markdown
# Periodic Tasks

## Quick Tasks (respond directly)
- Report current time

## Long Tasks (use spawn for async)
- Search the web for AI news and summarize
- Check email and report important messages
```

**Key behaviors:**

| Feature | Description |
|---------|-------------|
| **spawn** | Creates async subagent, doesn't block heartbeat |
| **Independent context** | Subagent has its own context, no session history |
| **message tool** | Subagent communicates with user directly via message tool |
| **Non-blocking** | After spawning, heartbeat continues to next task |

#### How Subagent Communication Works

```
Heartbeat triggers
    ↓
Agent reads HEARTBEAT.md
    ↓
For long task: spawn subagent
    ↓                           ↓
Continue to next task      Subagent works independently
    ↓                           ↓
All tasks done            Subagent uses "message" tool
    ↓                           ↓
Respond HEARTBEAT_OK      User receives result directly
```

The subagent has access to tools (message, web_search, etc.) and can communicate with the user independently without going through the main agent.

**Configuration:**

```json
{
  "heartbeat": {
    "enabled": true,
    "interval": 30
  }
}
```

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | `true` | Enable/disable heartbeat |
| `interval` | `30` | Check interval in minutes (min: 5) |

**Environment variables:**

* `RDxClaw_HEARTBEAT_ENABLED=false` to disable
* `RDxClaw_HEARTBEAT_INTERVAL=60` to change interval

### Providers

> [!NOTE]
> Groq provides free voice transcription via Whisper. If configured, Telegram voice messages will be automatically transcribed.

| Provider                   | Purpose                                 | Get API Key                                            |
| -------------------------- | --------------------------------------- | ------------------------------------------------------ |
| `gemini`                   | LLM (Gemini direct)                     | [aistudio.google.com](https://aistudio.google.com)     |
| `openrouter(To be tested)` | LLM (recommended, access to all models) | [openrouter.ai](https://openrouter.ai)                 |
| `anthropic(To be tested)`  | LLM (Claude direct)                     | [console.anthropic.com](https://console.anthropic.com) |
| `openai(To be tested)`     | LLM (GPT direct)                        | [platform.openai.com](https://platform.openai.com)     |
| `deepseek(To be tested)`   | LLM (DeepSeek direct)                   | [platform.deepseek.com](https://platform.deepseek.com) |
| `groq`                     | LLM + **Voice transcription** (Whisper) | [console.groq.com](https://console.groq.com)           |



<details>
<summary><b>Full config example</b></summary>

```json
{
  "agents": {
    "defaults": {
      "model": "anthropic/claude-opus-4-5"
    }
  },
  "providers": {
    "openrouter": {
      "api_key": "sk-or-v1-xxx"
    },
    "groq": {
      "api_key": "gsk_xxx"
    }
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "123456:ABC...",
      "allow_from": ["123456789"]
    },
    "discord": {
      "enabled": true,
      "token": "",
      "allow_from": [""]
    },
    "whatsapp": {
      "enabled": false
    },
  },
  "tools": {
    "web": {
      "brave": {
        "enabled": false,
        "api_key": "BSA...",
        "max_results": 5
      },
      "duckduckgo": {
        "enabled": true,
        "max_results": 5
      }
    }
  },
  "heartbeat": {
    "enabled": true,
    "interval": 30
  }
}
```

</details>

## CLI Reference

| Command                   | Description                   |
| ------------------------- | ----------------------------- |
| `RDxClaw onboard`        | Initialize config & workspace |
| `RDxClaw agent -m "..."` | Chat with the agent           |
| `RDxClaw agent`          | Interactive chat mode         |
| `RDxClaw gateway`        | Start the gateway             |
| `RDxClaw status`         | Show status                   |
| `RDxClaw server`         | Start Headless API server      |
| `RDxClaw swarm list`      | List active swarm agents       |
| `RDxClaw swarm kill <id>` | Terminate a subagent           |
| `RDxClaw cron list`      | List all scheduled jobs       |
| `RDxClaw cron add ...`   | Add a scheduled job           |
| `RDxClaw skills install` | Install a skill package (zip/tgz)|

### Scheduled Tasks / Reminders

RDxClaw supports scheduled reminders and recurring tasks through the `cron` tool:

* **One-time reminders**: "Remind me in 10 minutes" → triggers once after 10min
* **Recurring tasks**: "Remind me every 2 hours" → triggers every 2 hours
* **Cron expressions**: "Remind me at 9am daily" → uses cron expression

Jobs are stored in `~/.RDxClaw/workspace/cron/` and processed automatically.

## 🏢 Enterprise Features

### 🌐 Headless API & Webhooks
RDxClaw provides an OpenAI-compatible REST API, allowing you to trigger agents from Zapier, Stripe, Shopify, or custom apps.
```bash
# Start the API server
RDxClaw server --port 8080 --api-key your-key
```
- `POST /v1/chat/completions`: Standard chat interface.
- `POST /v1/webhooks/`: Universal event receiver for external triggers.

### 🧠 Corporate Memory (RAG)
Index the local documentation or business data without external embedding APIs. RDxClaw uses a Go-native BM25 search engine for privacy-first knowledge retrieval.
- Large document support (PDF, MD, TXT).
- Instant indexing and keyword-optimized retrieval.

### 🐝 Swarm Management
Run multiple autonomous subagents concurrently. Track their progress, token usage, and lifecycle through the CLI or API.
- `spawn_agent`: Background task execution.
- `delegate_task`: Synchronous sub-task delegation.

## 🤝 Contribute & Roadmap

PRs welcome! The codebase is intentionally small and readable. 🤗

Roadmap:
1. [x] Phase 1-4: Enterprise Platform Core (Done 2026-02-17)
2. [ ] Phase 5: Voice Intelligence (Edge TTS/STT)
3. [ ] Phase 6: Computer Vision Integration
4. [ ] Phase 7: Multi-Node Mesh Swarm

Developer group building, Entry Requirement: At least 1 Merged PR.

User Groups:

discord:  <https://discord.gg/V4sAZ9XWpN>

<img src="assets/wechat.png" alt="RDxClaw" width="512">

## 🐛 Troubleshooting

### Web search says "API 配置问题"

This is normal if you haven't configured a search API key yet. RDxClaw will provide helpful links for manual searching.

To enable web search:

1. **Option 1 (Recommended)**: Get a free API key at [https://brave.com/search/api](https://brave.com/search/api) (2000 free queries/month) for the best results.
2. **Option 2 (No Credit Card)**: If you don't have a key, we automatically fall back to **DuckDuckGo** (no key required).

Add the key to `~/.RDxClaw/config.json` if using Brave:

```json
{
  "tools": {
    "web": {
      "brave": {
        "enabled": false,
        "api_key": "YOUR_BRAVE_API_KEY",
        "max_results": 5
      },
      "duckduckgo": {
        "enabled": true,
        "max_results": 5
      }
    }
  }
}
```

### Getting content filtering errors

Some providers have content filtering. Try rephrasing your query or use a different model.

### Telegram bot says "Conflict: terminated by other getUpdates"

This happens when another instance of the bot is running. Make sure only one `RDxClaw gateway` is running at a time.

---

## 📝 API Key Comparison

| Service          | Free Tier           | Use Case                              |
| ---------------- | ------------------- | ------------------------------------- |
| **OpenRouter**   | 200K tokens/month   | Multiple models (Claude, GPT-4, etc.) |
| **Brave Search** | 2000 queries/month  | Web search functionality              |
| **Groq**         | Free tier available | Fast inference (Llama, Mixtral)       |
