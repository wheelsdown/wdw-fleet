# CLAUDE.md

For project conventions, build commands, architecture, schema rules,
and contribution guidelines, see [AGENTS.md](AGENTS.md). Everything
below is specific to the Claude Code operator experience on this repo.

## Commit Signing

Repo-local git config is set up to sign with `~/.claude/ssh/id_claude`
as `Claude Code (nugget) <claude@nugget.info>`. Verify before your
first commit:

```sh
git config commit.gpgsign        # → true
git config user.signingkey       # → ~/.claude/ssh/id_claude
git config user.email            # → claude@nugget.info
```

If the 1Password SSH agent is misbehaving, the repo's `core.sshCommand`
already points directly at `~/.claude/ssh/id_claude` with the agent
disabled — pushes go as `thane-developer` without the agent.

## CI Gate

**MANDATORY: `just ci` must pass locally before every `git push`. No
exceptions.** Do not rely on GitHub Actions — run the full gate
locally first and fix any issues before pushing.

## Schema Work

Before editing `internal/database/migrations/`, apply the current
migration against a fresh Postgres (`just dev-down && just dev-up`)
to confirm the baseline is clean. Then apply your new migration and
verify the `.down.sql` cleanly reverses it. Both paths must work.

After schema changes, the Go route table + DTO structs in
`internal/server/api/` and the model types in `internal/model/`
usually need matching updates. Run `just generate` and commit the
regenerated spec in the same commit or PR.

## Design Source-of-Truth

- **API surface**: `internal/server/api/routes.go` (Go route table).
  The spec at `internal/server/api/spec/openapi.yaml` is generated —
  never hand-edit it. Add or change routes in Go, run `just generate`,
  commit both. `just generate-check` gates CI.
- **Web UI**: [`docs/design/`](docs/design/) (FleetAware handoff).
  Don't invent tokens, sizes, or colors — extract from the handoff.

## GitHub Collaboration

Be a good GitHub collaborator. Review threads left open signal
unfinished work — always close the loop. Leave PRs clean and
reflective of reality.

**When addressing review feedback:**
1. Fix the issue in a commit
2. Reply to the thread with the fixing commit hash and a one-line
   explanation
3. Resolve the conversation
4. If deferring (out of scope, follow-up issue), say so explicitly
   before resolving

**After a round of fixes:** Request re-review so the reviewer knows
the ball is back in their court.

**Resolving threads via CLI:**
```sh
gh api graphql -f query='mutation { resolveReviewThread(input: {threadId: "THREAD_ID"}) { thread { isResolved } } }'
```

**PR hygiene:**
- Check off test plan items as they are verified
- Use `Refs #NNN` or `Closes #NNN` in commit bodies
- Keep the PR description accurate as scope evolves
