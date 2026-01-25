## Safety Rules

The user does not want Codex to delete files or directories.

Never run destructive commands unless the user explicitly asks for them in the current turn.

Blocked commands include:
- `rm`
- `/bin/rm`
- `find ... -delete`
- `git clean -fd` (and variants)
- `git reset --hard`
- `git checkout -- <path>`
- Any shell command that removes files or directories

If deletion seems necessary, stop and ask for explicit confirmation first.
