# Plan: Real E2E 20 Backlinks Production-Ready

**Created:** 2026-05-05
**Status:** in-progress
**Branch:** dev
**Decisions locked:**

- **D1** AI content: Full article 800-1500 từ Vietnamese, title + meta + anchor rotation
- **D2** Architecture: User-owned WP sites (current arch). Source_mode=custom default
- **D3** Worker: Embedded Go goroutine in api server (MVP), separate Fly machine deferred
- **D4** Verification: HTTP HEAD + anchor presence check. GSC index defer Phase 8

## Phases

| # | Phase | Status | Effort | Owner |
|---|---|---|---|---|
| 7.04 | AI content generation (Claude+OpenAI fallback) | in-progress | 5-8h | fullstack-developer |
| 7.05 | Embedded Go worker (poll/claim/execute/retry/DLQ) | pending | 10-15h | fullstack-developer |
| 7.06 | Frontend site multi-select + auto-enqueue + progress polling | pending | 3-5h | fullstack-developer + ui-ux-designer |
| 7.07 | Post-publish verification (HTTP HEAD + anchor) | pending | 3-5h | fullstack-developer + qa-engineer |
| 7.08 | E2E smoke test 20 backlinks + runbook | pending | 2-3h | qa-engineer |

**Total ETA:** 25-35h sequential auto-chain.

## Auto-chain rules

- Sequential 7.04 → 7.05 → 7.06 → 7.07 → 7.08
- Auto-proceed when CI green per phase
- 3 retry max on failure → halt blocker report
- Worktree principle: stash WIP before each phase, restore after
- Conventional commit prefix: `feat(ai)`, `feat(worker)`, `feat(landing)`, `feat(verification)`, `docs(runbook)`
- Red-team patches preserved (F1/F4/F7/F9/F12/F13/F20/F21)

## Cross-phase dependencies

- 7.04 → 7.05: worker reads `jobs.content_body` populated by 7.04
- 7.05 → 7.06: frontend progress polling shows worker-driven status transitions
- 7.04+7.05 → 7.07: verification runs after worker publish
- 7.04+7.05+7.06+7.07 → 7.08: full E2E smoke

## Files

Each phase has its own `phase-7-XX.md` with detailed file lists, code samples, success criteria, and risk assessment.

## Final deliverables

- 5 commits + Beta tags v1.0.0-beta.42 → 46
- 20/20 backlinks live in smoke test
- AI cost <$0.05/article (Claude Haiku Sonnet pricing target)
- Worker p95 latency <30s/job
- Vercel preview URL with new campaign UI
- Memory journal updated
