# AI Coding Tools — Overview

This directory documents the persistent-memory and code-graph tooling stack used across all AI agents in this environment.

## Stack at a glance

| Tool | Role | Transport |
|------|------|-----------|
| **MemPalace 3.3.3** | Long-term semantic memory (ChromaDB + SQLite) | MCP stdio |
| **GitNexus 1.6.3** | Code knowledge graph (KuzuDB) | MCP stdio / CLI |
| **Claude Code hooks** | Lifecycle automation (session start, tool use, save) | Shell / Node.js |

## Documents

| File | Contents |
|------|----------|
| [`mempalace.md`](mempalace.md) | Palace structure, wings, mining workflow, MCP tools, troubleshooting |
| [`gitnexus.md`](gitnexus.md) | Code-graph indexing, MCP tools, skills system |
| [`claude-code-hooks.md`](claude-code-hooks.md) | All 7 hooks — what they do and how they're wired |
| [`llm-integration.md`](llm-integration.md) | OpenCode, Codex, GoClaw agents, adding a new LLM |

## How the pieces fit together

```
Session starts
    │
    ▼
[Claude Code] session_start.sh (SessionStart hook)
    ├── MemPalace mine: session history (~/.claude/projects) every 10 min
    ├── MemPalace mine: project .md files                  once per day
    ├── GitNexus analyze --skills (background reindex)
    ├── mempalace wake-up  → injected into Claude context
    └── git branch / status / log  → injected into Claude context

[GoClaw agents] agent-driven startup (frontmatter + context_files)
    ├── STARTUP.md injected into every session via agent_context_files
    ├── mcp_mp___mempalace_status()    → palace overview
    ├── mcp_mp___mempalace_search()    → relevant memories for task
    ├── mcp_mp___mempalace_diary_read() → today's notes
    └── mcp_gn___list_repos()          → indexed code graphs
    (GoClaw's own pgvector memory runs automatically in background)

While working
    │
    ├── [Claude Code] PreToolUse (Grep/Glob/Bash) → gitnexus-hook.cjs augments with graph context
    ├── [GoClaw agents] mcp_gn___search() before grep — agent calls explicitly
    ├── PostToolUse (Bash git commit) → staleness check, notify to reindex
    └── UserPromptSubmit              → unified-skill-hook, ctx-workflow-policy

Session ends / context compacts
    │
    ├── [Claude Code] Stop / PreCompact hooks → mempalace-save.sh persists diary entry
    └── [GoClaw agents] mcp_mp___mempalace_diary_write() — agent calls explicitly
```

## Current state (as of 2026-04-26)

### GoClaw MCP servers (all registered + granted to all 4 agents)

| Server | Transport | Prefix | Status |
|--------|-----------|--------|--------|
| `mempalace` | stdio | `mp_` | ✅ active |
| `gitnexus` | stdio | `gn_` | ✅ active |
| `notebooklm` | http → `192.168.3.234:8002/mcp` | `nb_` | ✅ active |

### GoClaw agents

| Agent | Model | Workspace | STARTUP.md |
|-------|-------|-----------|-----------|
| `bloom-backend` | Claude Sonnet | `agent-onboarding-kit` | ✅ |
| `bloom-backend-ds` | DeepSeek V4 Flash | `agent-onboarding-kit` | ✅ |
| `bloom-frontend` | Claude Sonnet | `welcome-page-pro` | ✅ |
| `bloom-frontend-ds` | DeepSeek V4 Flash | `welcome-page-pro` | ✅ |

### GitNexus indexed repos (global registry, shared across all agents)

| Repo | Path | Files | Nodes | Status |
|------|------|-------|-------|--------|
| `goclaw` | `~/projects/goclaw` | 3163 | 66853 | ✅ |
| `agent-onboarding-kit` | `~/.goclaw/workspace/agent-onboarding-kit` | 278 | 5824 | ✅ |
| `claude_n_codex_api_proxy` | `~/projects/claude_n_codex_api_proxy` | 28 | 522 | ✅ |
| `welcome-page-pro` | `~/.goclaw/workspace/welcome-page-pro` | — | — | ⏳ indexing |

### Difference: Claude Code hooks vs GoClaw startup

| Aspect | Claude Code | GoClaw agents |
|--------|-------------|---------------|
| Trigger | Automatic `SessionStart` shell hook | Agent reads `STARTUP.md` via `agent_context_files` |
| MemPalace wake-up | Auto-injected into context | Agent calls `mcp_mp___mempalace_status()` explicitly |
| GitNexus reindex | Background `gitnexus analyze` on every session | No auto-reindex; agent uses `gn_search` on demand |
| Git state | Auto-injected (branch, status, log) | Agent reads workspace dir via exec tool |
| Own memory | None | GoClaw pgvector (episodic + semantic) runs automatically |
| Session save | Auto via `Stop`/`PreCompact` hooks | Agent calls `mcp_mp___mempalace_diary_write()` |

## Key paths

| Path | Purpose |
|------|---------|
| `~/.mempalace/` | Palace home (config, wings, ChromaDB files) |
| `~/.claude/settings.json` | Claude Code hook registrations |
| `~/.claude/hooks/` | Hook scripts |
| `~/.npm-global/bin/gitnexus` | GitNexus CLI |
| `/home/vokov/.local/share/pipx/venvs/mempalace/` | MemPalace virtualenv |
| `<repo>/.gitnexus/` | Per-repo code graph database |
| `~/projects/ai-session-bootstrap/` | Deployment scripts and hook sources |
| `~/.goclaw/workspace/bloom-backend/STARTUP.md` | GoClaw backend agent context file |
| `~/.goclaw/workspace/bloom-frontend/STARTUP.md` | GoClaw frontend agent context file |
