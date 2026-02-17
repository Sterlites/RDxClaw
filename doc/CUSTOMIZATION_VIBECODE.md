# 🦾 RDxClaw Customization Guide for Vibecoders
Welcome, Vibecoder! **RDxClaw** is a high-performance Agentic AI Framework designed for the Edge, but you don't need to know a single line of **Go** to make it your own. This project is built to be customized through **Markdown** and **JSON**—the universal languages of vibes.

This guide will show you how to swap the "brain," change the "personality," and give RDxClaw new "skills" to create real-world value.

---

## 🛠️ Where are the files?

rdxclaw keeps its customization in two main places. In your terminal, look for a folder named `.rdxclaw` in your home directory (usually `C:\Users\YourName\.rdxclaw` on Windows or `~/.rdxclaw` on Mac/Linux).

1.  **`config.json`**: The "Heart." This is where you put your API keys and connect to Telegram, Discord, etc.
2.  **`workspace/`**: The "Mind." This folder contains the Markdown files that define who the AI is and what it can do.

---

## 🧠 Level 1: The Soul (Identity & Personality)

Want to change rdxclaw's name? Make it more sarcastic? Give it a specific job like "Content Strategist"?

Go to your `workspace/` folder and open these files:

*   **`IDENTITY.md`**: Define the name, version, and primary purpose.
*   **`SOUL.md`**: Define the personality traits (e.g., "Helpful," "Concise," "Slightly obsessed with RDxs").
*   **`AGENT.md`**: The specific instructions for how the AI should behave (e.g., "Always use emojis," "Never apologize").

**Vibecoder Tip:** Just tell your AI assistant: *"Rewrite SOUL.md to make rdxclaw sound like a cyberpunk hacker who is always in a hurry."*

---

## 🔌 Level 2: The Plumbing (API Keys & Channels)

To actually make rdxclaw talk to the world, you need to edit `config.json`.

### Connecting an LLM (The Brain)
Open `config.json` and find the `providers` section. Paste your API key there:
```json
"providers": {
  "openai": {
    "api_key": "sk-proj-XXXXXXX"
  },
  "anthropic": {
    "api_key": "sk-ant-XXXXXXX"
  }
}
```

### Connecting to Chat Apps
Find the `channels` section. Want rdxclaw on **Telegram**?
1.  Set `"enabled": true`.
2.  Paste your `"token"` from @BotFather.
3.  Add your user ID to `"allow_from"` so only you can talk to it.

---

## ⚡ Level 3: The Skills (Adding New Powers)

A "Skill" in rdxclaw is just a Markdown file that explains how to use a tool or a command.

### How to add a new skill:
1.  Create a new folder in `workspace/skills/` (e.g., `workspace/skills/my-new-skill/`).
2.  Create a file named `SKILL.md` inside that folder.
3.  Write the skill in Markdown.

**Template for `SKILL.md`:**
```markdown
---
name: my-skill
description: Describe what this does for the AI
---

# My Skill

Explain to the AI how to use this. Provide backticks with commands.
Example:
```bash
curl -s "https://api.example.com/data"
```
```

**Vibecoder Tip:** You don't need to write the code! Just find a cool API or a terminal command and tell your AI: *"Create a new rdxclaw skill for fetching the latest Bitcoin price using the CoinGecko API."*

---

## ⏲️ Level 4: The Cron (Automated Tasks)

rdxclaw can do things on a schedule (e.g., "Send me a weather report every morning at 8 AM").

You can manage this via the CLI:
`rdxclaw cron add --name "Morning Weather" --every 86400 --message "What is the weather like today?" --deliver --channel telegram --to "YOUR_ID"`

Or by editing the `workspace/cron/jobs.json` file directly if you're feeling adventurous.

---

## 🐳 Level 5: Running with Docker (Zero Setup)

If you have Docker installed, you don't even need to install Go or compile anything.

1.  Place your `config.json` in a folder named `config`.
2.  Run the bot (Gateway mode):
    ```bash
    docker compose up rdxclaw-gateway
    ```
3.  Talk to it directly (Agent mode):
    ```bash
    docker compose run --rm rdxclaw-agent -m "Hello, who are you?"
    ```

---

## 📦 Level 6: Installing Community Skills

You can grab new skills from the community without writing any Markdown yourself!

Find a skill on GitHub and run:
`rdxclaw skills install Sterlites/RDxClaw-skills/weather`

To see what you have:
`rdxclaw skills list`

---

## 🚀 Get Started (The Vibecoder Workflow)

1.  **Clone it**: Download the project.
2.  **Onboard**: Run `rdxclaw onboard`.
3.  **Vibe**: Open the `workspace/*.md` files and tell your AI to "rebrand" rdxclaw.
4.  **Launch**: Run `rdxclaw gateway` (or use Docker) and start chatting on Telegram!

---

## 🦾 Why this is better for Vibecoders:
- **No Compiling**: Just edit a text file and the changes are instant.
- **AI Friendly**: LLMs are amazing at writing Markdown and JSON.
- **Lightweight**: It runs on almost anything, including $10 micro-boards.

**Now go experiment! If it breaks, just ask your AI to fix the JSON syntax.**
