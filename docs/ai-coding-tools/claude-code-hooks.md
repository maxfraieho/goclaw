# Claude Code Hooks

Claude Code supports lifecycle hooks — shell commands that run at specific points in a session. All hooks are registered in `~/.claude/settings.json`.

## Registered hooks

```
~/.claude/hooks/
├── session_start.sh          # SessionStart: context injection
├── mempalace-save.sh         # Stop + PreCompact: diary save
├── unified-skill-hook.sh     # UserPromptSubmit: load skills
├── ctx-workflow-policy.sh    # UserPromptSubmit: context policy
└── gitnexus/
    └── gitnexus-hook.cjs     # PreToolUse + PostToolUse: graph augmentation
```

## Hook events and what runs

| Event | Hook | Purpose |
|-------|------|---------|
| `SessionStart` | `session_start.sh` | Inject MemPalace context, git state, trigger background reindex |
| `UserPromptSubmit` | `unified-skill-hook.sh` | Load relevant skills into context |
| `UserPromptSubmit` | `ctx-workflow-policy.sh` | Apply context workflow policy |
| `PreToolUse` (Grep/Glob/Bash) | `gitnexus-hook.cjs` | Augment searches with graph context |
| `PostToolUse` (Bash) | `gitnexus-hook.cjs` | Detect GitNexus index staleness after git mutations |
| `Stop` | `mempalace-save.sh` | Save session diary to MemPalace |
| `PreCompact` | `mempalace-save.sh` | Save diary before context compaction |

## `session_start.sh` — detailed walkthrough

Source: `~/projects/ai-session-bootstrap/session_start.sh`  
Installed at: `~/.claude/hooks/session_start.sh`

Runs once when a Claude Code session opens. Builds a context block injected into Claude via the `SessionStart` JSON response format:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "<markdown context string>"
  }
}
```

### What it does, step by step

**Step 1 — Git check**: Reads `CLAUDE_PROJECT_DIR` (set by Claude Code) and finds git root. Exits cleanly if not in a git repo.

**Step 2 — GitNexus background reindex**:
```bash
npx gitnexus@latest analyze --skills --skip-agents-md --skip-embeddings &
```
Runs in background to avoid blocking session start. Fast incremental update — no embeddings.

**Step 3a — MemPalace session mining** (every 10 min):
```bash
mempalace mine ~/.claude/projects --mode convos
```
Mines new Claude Code conversation files into the palace. Cooldown: `~/.mempalace/.last_mine_<wing>`.

**Step 3b — Markdown documentation mining** (once per day):
Copies all `.md` files from the repo to a temp dir, mines them with `--wing <wing>`. Does not mine code — GitNexus handles that.
Cooldown: `~/.mempalace/.last_md_mine_<wing>`.

**Step 3c — MemPalace wake-up** (synchronous):
```bash
mempalace wake-up --wing "$WING_NAME"   # falls back to global if empty
```
Returns a summarized session context injected directly into Claude's conversation.

**Step 3d — Memory search** (synchronous):
```bash
mempalace search "$PROJECT_NAME" --wing "$WING_NAME" --limit 5
```
Returns top 5 relevant memories for the current project.

**Step 3e — Palace status**:
```bash
mempalace status | grep -E "drawers|wings|rooms" | head -5
```

**Step 4 — Git context**:
- `git rev-parse --abbrev-ref HEAD` — current branch
- `git status --short | head -10` — uncommitted changes
- `git log --oneline -5` — recent commits

All sections are assembled into a single markdown block and output as JSON.

## `gitnexus-hook.cjs` — graph augmentation

Source: `~/.claude/hooks/gitnexus/gitnexus-hook.cjs`

A Node.js hook that handles both `PreToolUse` and `PostToolUse`. Only activates when `.gitnexus/` is found in the current working directory (walks up 5 levels).

**PreToolUse** — pattern extraction and augmentation:
1. Extracts a search pattern from Grep/Glob/Bash tool input
2. Calls `gitnexus augment -- <pattern>` (via hardcoded path to the CLI JS file)
3. Returns graph context as `additionalContext`

**PostToolUse** — staleness check:
1. Watches for `git commit|merge|rebase|cherry-pick|pull` in Bash commands
2. Reads `meta.json.lastCommit` and compares with `git rev-parse HEAD`
3. If different, suggests `npx gitnexus analyze`

Hardcoded CLI path: `/home/vokov/.npm-global/lib/node_modules/gitnexus/dist/cli/index.js`  
Falls back to `require.resolve` then `npx` if that path doesn't exist.

## `mempalace-save.sh`

Runs on `Stop` and `PreCompact`. Calls `mempalace diary_write` or equivalent to persist a session summary. This ensures knowledge is saved even if the session ends abruptly or context is compacted.

## Hook timeout behavior

- `PreToolUse` gitnexus-hook: 10s timeout. If exceeded, tool call proceeds without augmentation.
- `PostToolUse` gitnexus-hook: 10s timeout.
- `SessionStart` session_start.sh: no explicit timeout; mining runs in background so it returns quickly.

## Hook output protocol

Hooks communicate with Claude Code via stdout JSON:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "additionalContext": "string injected into Claude context"
  }
}
```

Exit code 0 = success. Non-zero exit = hook failed (Claude Code logs it, continues anyway).

## Troubleshooting hooks

### All Bash commands fail silently

Check the shell snapshot for dangerous set flags:

```bash
cat ~/.claude/shell-snapshots/snapshot-bash-*.sh | grep "^set -o"
```

Remove `set -o onecmd` if present — it causes bash to exit after one command, breaking all Bash tool calls.

### SessionStart hook not injecting context

Test the hook manually:

```bash
echo '{"source": "test"}' | CLAUDE_PROJECT_DIR=/path/to/repo \
  bash ~/.claude/hooks/session_start.sh
```

Should print JSON to stdout.

### gitnexus hook not augmenting

Check that `.gitnexus/` exists in the project directory and GitNexus CLI path is correct:

```bash
ls /home/vokov/.npm-global/lib/node_modules/gitnexus/dist/cli/index.js
```

Enable debug output:

```bash
GITNEXUS_DEBUG=1 echo '{"hook_event_name":"PreToolUse","tool_name":"Grep","tool_input":{"pattern":"UserService"},"cwd":"/path/to/repo"}' | \
  node ~/.claude/hooks/gitnexus/gitnexus-hook.cjs
```
