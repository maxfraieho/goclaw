# LLM Integration — MemPalace + GitNexus Across Agents

The same MemPalace palace and GitNexus code graphs are shared across all agents. Each agent type has its own way of connecting to these tools.

## Overview

| Agent | MemPalace access | GitNexus access |
|-------|-----------------|-----------------|
| **Claude Code** | Via SessionStart hook (context injection) | MCP stdio + PreToolUse hook |
| **OpenCode** | MCP stdio (direct, registered in opencode.json) | MCP stdio (registered in opencode.json) |
| **Codex** | Not yet wired | Skills files in `~/.agents/skills/` |
| **GoClaw agents** | MCP stdio via GoClaw bridge (`mp_` prefix) ✅ | MCP stdio via GoClaw bridge (`gn_` prefix) ✅ |
| **Any MCP-compatible LLM** | Start MCP server, register as stdio server | Start `gitnexus mcp`, register as stdio server |

## Claude Code

Claude Code uses hooks rather than a direct MCP connection for MemPalace. This is intentional: the SessionStart hook injects a summarized wake-up context before the conversation begins.

**MemPalace**: provided via `session_start.sh` (wake-up + search results in context). If `mcp__mempalace__*` tools appear in the session, Claude should call `mempalace_status`, `mempalace_search`, `mempalace_kg_query` at session start.

**GitNexus**: registered as MCP in `~/.claude.json`:

```json
{
  "mcpServers": {
    "gitnexus": {
      "type": "stdio",
      "command": "/home/vokov/.npm-global/bin/gitnexus",
      "args": ["mcp"]
    }
  }
}
```

Additionally the `gitnexus-hook.cjs` augments every Grep/Glob/Bash search in real time.

## OpenCode

Config: `~/.config/opencode/opencode.json`

Both MemPalace and GitNexus are registered as MCP stdio servers:

```json
{
  "mcp": {
    "mempalace": {
      "type": "local",
      "command": "/home/vokov/.local/share/pipx/venvs/mempalace/bin/python",
      "args": ["-m", "mempalace.mcp_server"],
      "env": {
        "ANONYMIZED_TELEMETRY": "false",
        "CHROMA_ANONYMIZED_TELEMETRY": "false",
        "CHROMA_TELEMETRY": "false",
        "PYTHONWARNINGS": "ignore"
      }
    },
    "gitnexus": {
      "type": "local",
      "command": "/home/vokov/.npm-global/bin/gitnexus",
      "args": ["mcp"]
    }
  }
}
```

GitNexus skills are installed at `~/.config/opencode/skill/`.

## Codex (OpenAI)

Codex does not use MCP. It uses a skills-file mechanism. GitNexus skills are installed at `~/.agents/skills/`. MemPalace is not yet wired to Codex.

## GoClaw agents

GoClaw uses a built-in MCP bridge. Both MemPalace and GitNexus are registered as global MCP servers and granted to all 4 agents.

### Registered MCP servers

| Server | ID | Transport | Prefix | Agents |
|--------|----|-----------|--------|--------|
| `mempalace` | `019dc3bf-3962-7260-8b3d-38ad34e605c8` | stdio | `mp_` | all 4 |
| `gitnexus` | `019dc7c1-0903-753c-908c-26d42a556935` | stdio | `gn_` | all 4 |
| `notebooklm` | `019dc7f0-197a-793a-993e-91b42be005fc` | http | `nb_` | all 4 |

Full tool names inside agents use double-underscore: `mcp_mp___mempalace_search`, `mcp_gn___search`, `mcp_nb___...`

### Session startup sequence (agent-driven)

Unlike Claude Code (which has an automatic `SessionStart` shell hook), GoClaw agents execute the startup sequence via **frontmatter instructions**. GoClaw has no hook equivalent — the agent runs the tools voluntarily at the start of every conversation.

**Mandatory startup (all GoClaw agents):**

```
1. mcp_mp___mempalace_status()                          — palace overview
2. mcp_mp___mempalace_search(query="<task>", limit=5)   — relevant memories
3. mcp_mp___mempalace_diary_read(date="today")          — today's session notes
4. mcp_gn___list_repos()                                — what's indexed
```

**Before any code search (replaces gitnexus-hook.cjs):**
```
mcp_gn___search(query="<symbol>")
mcp_gn___context(symbol="<fn>")
```

**Session end / before long pause:**
```
mcp_mp___mempalace_diary_write(content="<session summary>")
```

### GoClaw's own memory mechanisms

GoClaw also has a built-in 3-tier memory system (independent of MemPalace):

| Tier | Storage | Trigger |
|------|---------|---------|
| Working | Conversation history (DB) | Every message |
| Episodic | Session summaries (pgvector) | Auto on session end |
| Semantic | Knowledge graph (pgvector) | Consolidation workers |

This runs **automatically** — agents don't need to call it explicitly. MemPalace supplements it with cross-session long-term memory (ChromaDB) that persists across restarts and is shared between Claude Code, OpenCode, and GoClaw agents.

### Agent workspace mapping

| Agent | Workspace | GitNexus repo |
|-------|-----------|---------------|
| `bloom-backend` | `~/.goclaw/workspace/agent-onboarding-kit` | `agent-onboarding-kit` |
| `bloom-backend-ds` | `~/.goclaw/workspace/agent-onboarding-kit` | `agent-onboarding-kit` |
| `bloom-frontend` | `~/.goclaw/workspace/welcome-page-pro` | `welcome-page-pro` (indexing) |
| `bloom-frontend-ds` | `~/.goclaw/workspace/welcome-page-pro` | `welcome-page-pro` (indexing) |

### Re-registering (if needed)

```bash
# mempalace
curl -s -X POST http://localhost:18790/v1/mcp/servers \
  -H "Authorization: Bearer $GOCLAW_GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mempalace",
    "transport": "stdio",
    "command": "/home/vokov/.local/share/pipx/venvs/mempalace/bin/python",
    "args": ["-m", "mempalace.mcp_server"],
    "tool_prefix": "mp_",
    "enabled": true,
    "env": {
      "ANONYMIZED_TELEMETRY": "false",
      "CHROMA_ANONYMIZED_TELEMETRY": "false",
      "CHROMA_TELEMETRY": "false",
      "PYTHONWARNINGS": "ignore",
      "PYTHONUNBUFFERED": "1"
    }
  }'

# gitnexus
curl -s -X POST http://localhost:18790/v1/mcp/servers \
  -H "Authorization: Bearer $GOCLAW_GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "gitnexus",
    "transport": "stdio",
    "command": "/usr/bin/node",
    "args": ["/home/vokov/.npm-global/lib/node_modules/gitnexus/dist/cli/index.js", "mcp"],
    "tool_prefix": "gn_",
    "enabled": true,
    "env": {
      "PATH": "/home/vokov/.npm-global/bin:/usr/bin:/bin",
      "HOME": "/home/vokov"
    }
  }'

# Grant to agent
curl -s -X POST http://localhost:18790/v1/mcp/servers/<server-id>/grants/agent \
  -H "Authorization: Bearer $GOCLAW_GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"agent_id": "<agent-uuid>"}'
```

## Adding a new LLM / agent

Any LLM that supports MCP stdio can use MemPalace and GitNexus.

### MemPalace

1. The LLM's config needs a stdio MCP entry pointing to:
   ```
   command: /home/vokov/.local/share/pipx/venvs/mempalace/bin/python
   args: [-m, mempalace.mcp_server]
   ```
2. Optionally set env vars to suppress telemetry.
3. The LLM will see 29 tools prefixed `mempalace_`.
4. Instruct the LLM to call `mempalace_status` at session start.

### GitNexus

1. The LLM's config needs a stdio MCP entry pointing to:
   ```
   command: /home/vokov/.npm-global/bin/gitnexus
   args: [mcp]
   ```
2. The LLM will see tools: `search`, `context`, `callers`, `callees`, `list_repos`, `augment`.
3. For non-MCP agents: copy skills from `.gitnexus/skills/<agent-type>/` to the agent's skills directory.

### New project onboarding

For any new git repo:

```bash
cd /path/to/new-repo
bash ~/projects/ai-session-bootstrap/ai_project_init.sh
```

This:
1. Creates `.gitnexus/` with full code graph
2. Generates skills for Claude, OpenCode, Codex
3. Mines project files into MemPalace under the project's wing
4. Mines existing Claude sessions
5. Updates `CLAUDE.md` with MemPalace and GitNexus instructions

Subsequent sessions: `session_start.sh` runs automatically and keeps both systems up-to-date.

## MCP connection debugging

Test MemPalace MCP manually:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | \
  /home/vokov/.local/share/pipx/venvs/mempalace/bin/python -m mempalace.mcp_server
```

Test GitNexus MCP:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | \
  /home/vokov/.npm-global/bin/gitnexus mcp
```

Both should return a JSON list of tools.
