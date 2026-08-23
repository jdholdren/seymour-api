---
paths:
  - ".claude/**"
---

# Working in `.claude/`: memory and skill layout

Reference: [Claude Code memory docs](https://code.claude.com/docs/en/memory),
[Agent Skills overview](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview#how-skills-work).

## CLAUDE.md / memory naming

- Project instructions: `./CLAUDE.md` or `./.claude/CLAUDE.md` — shared via
  source control, applies to the whole team.
- Personal, project-specific overrides: `./CLAUDE.local.md` — gitignore this.
- Personal, all-projects preferences: `~/.claude/CLAUDE.md`.
- Nested `CLAUDE.md`/`CLAUDE.local.md` in subdirectories load on demand when
  Claude reads files in that subdirectory, not at launch.
- Target under ~200 lines per `CLAUDE.md`; move bulky or path-specific
  content into `.claude/rules/` instead.

## `.claude/rules/` layout

- One topic per file, descriptive kebab-case filename (`testing.md`,
  `api-design.md`), discovered recursively — subdirectories like `frontend/`
  or `backend/` are fine.
- A rule with no frontmatter (or no `paths` key) loads unconditionally, at
  the same priority as `.claude/CLAUDE.md`.
- A rule scoped with YAML frontmatter only loads when Claude reads a
  matching file:
  ```markdown
  ---
  paths:
    - "src/api/**/*.ts"
  ---
  ```
  `paths` takes glob patterns (brace expansion allowed, e.g.
  `src/**/*.{ts,tsx}`); multiple patterns are ORed.
- User-level personal rules live in `~/.claude/rules/` and load before
  project rules.
- `.claude/rules/` supports symlinks for sharing rules across projects.

## Skills layout (`.claude/skills/` or `~/.claude/skills/`)

Skills are directories, not single files — `~/.claude/skills/` (personal) or
`.claude/skills/` (project), auto-discovered.

```
my-skill/
├── SKILL.md        # required: instructions + YAML frontmatter
├── REFERENCE.md     # optional: extra docs, loaded only if referenced
└── scripts/
    └── helper.py     # optional: run via bash, output only enters context
```

`SKILL.md` frontmatter, both fields required:
- `name`: ≤64 chars, lowercase letters/numbers/hyphens only, no XML tags,
  can't contain "anthropic" or "claude".
- `description`: non-empty, ≤1024 chars, no XML tags — must state **what**
  the skill does **and when** to use it (this is what Claude matches
  requests against to decide whether to trigger it).

Loading is progressive: frontmatter (name+description) is always in
context; the SKILL.md body loads only once the skill triggers; bundled
files (other `.md`, scripts, resources) load only when referenced/run.
Keep the body itself under ~5k tokens and push detail into separate files.
