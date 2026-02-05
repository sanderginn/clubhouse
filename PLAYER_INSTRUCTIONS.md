# Player Rendering Refactoring - Agent Instructions

This document provides instructions for an orchestrating agent to process the player rendering refactoring tasks.

## Prerequisites

Before starting, read and understand:
1. **AGENTS.md** - Development guidelines, code conventions, testing requirements
2. **DESIGN.md** - System architecture and design decisions

## Task Structure

Tasks are organized in phase directories under `player-rendering/`:

```
player-rendering/
├── phase-1/           # Frontend URL Parsers (YouTube/Spotify)
│   ├── step-01-url-detection-youtube-parser.md
│   ├── step-02-spotify-parser.md
│   └── step-03-integrate-parsers-postcard.md
├── phase-2/           # Frontend SoundCloud oEmbed
│   ├── step-01-soundcloud-oembed-fetcher.md
│   └── step-02-integrate-soundcloud-postcard.md
├── phase-3/           # Async Backend Metadata Fetching
│   ├── step-01-metadata-queue.md
│   ├── step-02-metadata-worker.md
│   ├── step-03-post-service-enqueue.md
│   ├── step-04-websocket-backend.md
│   ├── step-05-websocket-frontend.md
│   └── step-06-start-worker.md
└── phase-4/           # Bandcamp JSON-LD Extraction
    └── step-01-bandcamp-jsonld.md
```

## Execution Rules

### Sequential Processing

**IMPORTANT: Process ONE task at a time.** Spawn only ONE subagent at any given moment.

1. Start with Phase 1, Step 1
2. Wait for the subagent to complete before proceeding
3. Move completed step to the `done/` subdirectory
4. Proceed to the next step in order

### Step Completion Workflow

For each step:

1. **Read the step markdown file** to understand the task
2. **Spawn a subagent** using the `$subagent` skill with the step instructions
3. **Wait for completion** - the subagent will create a PR or report completion
4. **Verify the PR passes CI** before considering the step done
5. **Move the step file** to the `done/` subdirectory:
   ```bash
   mkdir -p player-rendering/phase-X/done
   mv player-rendering/phase-X/step-XX-name.md player-rendering/phase-X/done/
   ```
6. **Commit the moved step file** to track progress:
   ```bash
   git add player-rendering/
   git commit -m "chore: mark phase-X step-XX as done"
   ```
7. **Proceed to the next step**

### Subagent Instructions

When spawning a subagent, provide it with:

1. The full contents of the step markdown file
2. Instruction to read AGENTS.md and DESIGN.md first
3. Instruction to create a PR when done
4. The step file path so it can reference it

Example subagent prompt:
```
Read AGENTS.md and DESIGN.md first to understand the project conventions.

Then implement the following task:

[PASTE STEP MARKDOWN CONTENTS HERE]

When the implementation is complete:
1. Run `task ci:prek` to ensure all checks pass
2. Commit your changes with a descriptive message
3. Create a PR for review
```

### Phase Dependencies

Phases can be executed in order, but some have dependencies:

| Phase | Dependencies | Notes |
|-------|--------------|-------|
| Phase 1 | None | Frontend-only, safe to start |
| Phase 2 | Phase 1 complete | Uses urlParsers.ts from Phase 1 |
| Phase 3 | None (but Phase 1-2 recommended first) | Backend changes, independent |
| Phase 4 | None | Independent Bandcamp improvement |

**Recommended order:** Phase 1 → Phase 2 → Phase 3 → Phase 4

### Handling Failures

If a subagent fails or a step cannot be completed:

1. **Do not skip the step** - investigate the failure
2. **Check CI logs** for specific errors
3. **Provide additional context** to the subagent if needed
4. **Report blocking issues** to the user if unresolvable

### Done Directory Rules

- **Never pick tasks from `done/` directories** - these are completed
- Always check if a step file still exists before processing
- If a phase directory only has a `done/` subdirectory, the phase is complete

## Progress Tracking

To check progress:

```bash
# See remaining tasks
find player-rendering -name "step-*.md" -not -path "*/done/*" | sort

# See completed tasks
find player-rendering -path "*/done/*" -name "step-*.md" | sort
```

## Completion Criteria

The refactoring is complete when:

1. All step files have been moved to their respective `done/` directories
2. All PRs have been merged to main
3. The following is working:
   - YouTube/Spotify embeds render instantly from frontend URL parsing
   - SoundCloud embeds load via frontend oEmbed
   - Backend metadata fetching is async via Redis queue
   - WebSocket events update posts when metadata arrives
   - Bandcamp extraction uses JSON-LD with fallback

## Quick Reference

```bash
# Start dev environment
task dev:up

# Run all checks before committing
task ci:prek

# Run backend tests
task backend:test

# Run frontend tests
task frontend:test

# Check TypeScript
cd frontend && npm run check
```

## Notes

- Each step is self-contained with detailed description, implementation guidance, and test cases
- Steps should be completed in order within each phase
- PRs should be small and focused on a single step
- Always run `task ci:prek` before creating a commit
- **Commit after every step** - both the subagent's implementation work and the orchestrator's progress tracking (moving files to `done/`)
