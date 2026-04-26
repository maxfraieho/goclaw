# MemPalace — Persistent Memory System

MemPalace 3.3.3 provides long-term semantic memory for AI agents. It stores text fragments ("drawers") in a ChromaDB vector database with SQLite as the persistent backing store.

## Installation

```bash
pipx install mempalace
# binary lands at: ~/.local/bin/mempalace
# venv at:         ~/.local/share/pipx/venvs/mempalace/
```

Initialize the palace (one-time):

```bash
mempalace init /home/vokov/projects/goclaw --yes
```

## Storage layout

```
~/.mempalace/
├── config.json          # palace config: palace_path, collection_name
├── palace/              # ChromaDB raw files
│   ├── chroma.sqlite3   # canonical truth — always intact
│   └── <uuid>/          # HNSW binary indices (rebuilt from SQLite if deleted)
└── .last_mine_<wing>    # cooldown markers for mining
```

`collection_name` is `mempalace_drawers`. The SQLite file is the authoritative store; HNSW directories are caches.

## Wings (project namespaces)

Wings are isolated namespaces within the palace. Current state:

| Wing | Drawers | Contents |
|------|---------|----------|
| `goclaw` | ~1357 | GoClaw codebase docs, session history |
| `agent_onboarding_kit` | ~858 | Agent onboarding project |
| `welcome_page_creator` | ~102 | Welcome page project |
| `welcome_page_pro` | ~792 | Pro welcome page project |

Wing name derivation (used in session_start.sh and ai_project_init.sh):

```bash
WING_NAME="$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | tr ' .-' '___')"
# "goclaw" → "goclaw"
# "My-Project" → "my_project"
```

## Mining workflow

Mining converts documents into palace drawers. Two types run automatically via the SessionStart hook:

### Session history mining (every 10 min)

Mines Claude Code conversation files from `~/.claude/projects`:

```bash
mempalace mine ~/.claude/projects --mode convos
```

Cooldown marker: `~/.mempalace/.last_mine_<wing>`

### Markdown documentation mining (once per day)

Only `.md` files are mined (GitNexus handles code). Uses a temp-dir approach because `mempalace mine` has no `--include` filter:

```bash
TMP_MD=$(mktemp -d)
find "$GIT_ROOT" -name "*.md" -not -path "*/.git/*" | while IFS= read -r f; do
  rel="${f#$GIT_ROOT/}"
  mkdir -p "$TMP_MD/$(dirname "$rel")"
  cp "$f" "$TMP_MD/$rel"
done
mempalace mine "$TMP_MD" --wing "$WING_NAME"
rm -rf "$TMP_MD"
```

Cooldown marker: `~/.mempalace/.last_md_mine_<wing>`

### Manual mining

```bash
# Mine entire project (all file types)
mempalace mine /path/to/project --wing goclaw

# Mine into specific wing
mempalace mine /path/to/docs --wing goclaw

# Mine Claude sessions
mempalace mine ~/.claude/projects --mode convos

# Force re-mine (ignore deduplication)
mempalace mine /path --wing goclaw --force
```

**Important:** never run multiple `mempalace mine` processes concurrently against the same palace. Parallel writes corrupt the HNSW binary indices. See Troubleshooting.

## CLI commands

```bash
mempalace status               # overview: wings, drawer count, MCP status
mempalace search "query"       # semantic search across all wings
mempalace search "q" --wing goclaw --limit 5
mempalace wake-up              # summarized context for session start
mempalace wake-up --wing goclaw
mempalace list-wings           # list all wings
```

## MCP server

The MCP server exposes 29 tools to Claude and other agents.

### Starting the server

```bash
/home/vokov/.local/share/pipx/venvs/mempalace/bin/python -m mempalace.mcp_server
```

Recommended env vars to suppress telemetry noise:

```
ANONYMIZED_TELEMETRY=false
CHROMA_ANONYMIZED_TELEMETRY=false
CHROMA_TELEMETRY=false
PYTHONWARNINGS=ignore
PYTHONUNBUFFERED=1
```

### Claude Code registration (`~/.claude.json`)

MemPalace is not currently registered as a direct MCP server in `~/.claude.json` for Claude Code — it is accessed via the SessionStart hook and via GoClaw's bridge (see llm-integration.md). Claude Code accesses MemPalace output injected into context by `session_start.sh`.

### Key MCP tools (29 total, prefix `mempalace_`)

| Tool | Purpose |
|------|---------|
| `mempalace_status` | Palace overview + protocol |
| `mempalace_search` | Semantic search with optional `--wing` |
| `mempalace_wake_up` | Session context summary |
| `mempalace_add_drawer` | Store a new memory fragment |
| `mempalace_list_agents` | List specialist agents |
| `mempalace_kg_add` | Add node/edge to knowledge graph |
| `mempalace_kg_query` | Query knowledge graph |
| `mempalace_diary_write` | End-of-session diary entry |
| `mempalace_diary_read` | Read diary entries |
| `mempalace_check_duplicate` | Dedup check before adding |

In GoClaw agents, tool names are prefixed: `mcp_mp___mempalace_search` (see [`llm-integration.md`](llm-integration.md)).

## Session start protocol

At the start of every Claude Code session, `session_start.sh` calls:

```bash
mempalace wake-up --wing "$WING_NAME"   # or global if wing empty
mempalace search "$PROJECT_NAME" --wing "$WING_NAME" --limit 5
mempalace status | grep -E "drawers|wings|rooms"
```

Results are injected into Claude's context as `additionalContext` via the SessionStart hook JSON response.

## Troubleshooting

### ChromaDB HNSW corruption

Symptom: `mempalace status` segfaults (exit 139) or returns `InternalError: Failed to apply logs to hnsw segment writer`.

Cause: parallel `mempalace mine` processes writing to the same palace simultaneously.

Fix:

```bash
# 1. Kill any running MCP server
pkill -f "mempalace.mcp_server" 2>/dev/null

# 2. Delete HNSW binary indices (SQLite is the truth, they rebuild automatically)
ls ~/.mempalace/palace/
rm -rf ~/.mempalace/palace/<corrupted-uuid-dir>/

# 3. Verify SQLite integrity
python3 -c "
import sqlite3
conn = sqlite3.connect('/home/vokov/.mempalace/palace/chroma.sqlite3')
result = conn.execute('PRAGMA integrity_check').fetchone()
print(result)
conn.close()
"

# 4. Test recovery
mempalace status
```

ChromaDB rebuilds HNSW from SQLite on next access automatically.

### MCP server holds database open

If `mempalace mine` fails with a lock error while the MCP server is running:

```bash
pkill -f "mempalace.mcp_server"
# wait 2s, then retry mine
```

### "No drawers found" after mining

Check that the mine actually ran (not just the background process marker):

```bash
mempalace status
mempalace search "test" --limit 1
```

If SQLite has rows but search returns nothing, HNSW index was not built. Delete UUID dirs and run `mempalace status` to trigger rebuild.
