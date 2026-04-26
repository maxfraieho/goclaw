# GitNexus — Code Knowledge Graph

GitNexus 1.6.3 builds a structural knowledge graph of a codebase using KuzuDB. It indexes functions, classes, imports, and call relationships, then exposes them via MCP and a CLI. The graph is stored locally in each repo as `.gitnexus/`.

## Installation

```bash
npm config set prefix ~/.npm-global          # one-time: avoid sudo for globals
npm install -g gitnexus
# binary: ~/.npm-global/bin/gitnexus
# package: ~/.npm-global/lib/node_modules/gitnexus/
```

Verify:

```bash
gitnexus --version     # 1.6.3
```

## Project indexing

### First-time (use ai_project_init.sh)

```bash
cd /path/to/repo
bash ~/projects/ai-session-bootstrap/ai_project_init.sh
```

This runs `gitnexus analyze --force --skills --embeddings` and creates `.gitnexus/` in the repo root.

### Manual indexing

```bash
# Full index with embeddings and Claude skills
npx gitnexus@latest analyze --skills --embeddings

# Incremental update (no embeddings = faster)
npx gitnexus@latest analyze --skills --skip-embeddings

# Skip rewriting CLAUDE.md (preserve manual edits)
npx gitnexus@latest analyze --skills --skip-agents-md
```

### Index contents

```
.gitnexus/
├── kuzu/              # KuzuDB graph files
├── skills/            # Generated skill files for Claude / OpenCode / Codex
│   ├── claude/        # CLAUDE.md snippets
│   ├── opencode/      # OpenCode skill .md files
│   └── codex/         # Codex .md skill files
├── meta.json          # lastCommit, stats (nodes, edges, embeddings)
└── .last_indexed      # ISO timestamp of last successful analyze
```

## CLI tools

```bash
gitnexus search "UserService"          # semantic search in graph
gitnexus context "functionName"        # full context: code + deps
gitnexus callers "functionName"        # who calls this function
gitnexus callees "functionName"        # what this function calls
gitnexus augment -- "pattern"          # enriched context for a pattern (used by hook)
gitnexus list-repos                    # all indexed repos
gitnexus mcp                           # start MCP server
```

## MCP server

### Registration in `~/.claude.json`

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

### Key MCP tools

| Tool | Purpose |
|------|---------|
| `search` | Semantic search across codebase |
| `context` | Full source + dependency context for a symbol |
| `callers` | Reverse call graph — who uses a function |
| `callees` | Forward call graph — what a function uses |
| `list_repos` | All indexed repositories |
| `augment` | Enrich a search pattern with graph context |

## Skills system

GitNexus generates skills — structured prompts that teach agents how to use the graph — during `analyze --skills`. Skills are written to `.gitnexus/skills/` and also installed to agent-specific directories.

### Claude Code skills

Skills are appended to `CLAUDE.md` (or injected at session start). The hook `unified-skill-hook.sh` loads them dynamically via `UserPromptSubmit`.

### OpenCode skills

```
~/.config/opencode/skill/
├── gitnexus-cli.md
├── gitnexus-debugging.md
├── gitnexus-exploring.md
├── gitnexus-guide.md
├── gitnexus-impact-analysis.md
├── gitnexus-pr-review.md
└── gitnexus-refactoring.md
```

Referenced in OpenCode sessions automatically when the MCP server is configured.

### Codex skills

```
~/.agents/skills/
├── gitnexus-cli.md
├── gitnexus-debugging.md
├── gitnexus-exploring.md
├── gitnexus-guide.md
├── gitnexus-impact-analysis.md
├── gitnexus-pr-review.md
├── gitnexus-refactoring.md
└── superpowers.md
```

Codex loads skills from `~/.agents/skills/` at agent startup.

## Claude Code hook integration

Two hooks use `gitnexus-hook.cjs` (`~/.claude/hooks/gitnexus/gitnexus-hook.cjs`):

### PreToolUse — graph augmentation

Triggers on: `Grep`, `Glob`, `Bash` (when command contains `rg` or `grep`).

Extracts the search pattern from the tool input, calls `gitnexus augment -- <pattern>`, and injects the graph context into Claude's context via `additionalContext`. This gives Claude call-graph awareness before it reads raw search results.

Pattern extraction rules:
- `Grep`: uses `pattern` field directly
- `Glob`: extracts word from path pattern
- `Bash`: finds the first non-flag argument after `rg`/`grep`

Only activates if `.gitnexus/` exists in the current working directory (up to 5 levels up).

### PostToolUse — staleness detection

Triggers on: `Bash` commands matching `git commit|merge|rebase|cherry-pick|pull`.

Compares `git rev-parse HEAD` against `meta.json.lastCommit`. If they differ, notifies Claude to run `npx gitnexus analyze`.

### SessionStart background reindex

`session_start.sh` runs `npx gitnexus@latest analyze --skills --skip-agents-md --skip-embeddings` in background on every session start (fast incremental update).

## Indexed repos in this environment

| Repo | Wing | Notes |
|------|------|-------|
| `~/projects/goclaw` | `goclaw` | Primary project |
| `~/projects/agent-onboarding-kit` | `agent_onboarding_kit` | |
| `~/projects/welcome-page-creator` | `welcome_page_creator` | |
| `~/projects/welcome-page-pro` | `welcome_page_pro` | |

Run `ai_project_init.sh` in any new repo to set up GitNexus + MemPalace for it.
