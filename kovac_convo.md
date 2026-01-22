# Kovac Conversation: Sleeve-Requested Features

Kovac was an early sleeve with minimal context. When asked what tools it would want, these were the insights:

## Memory (.cstack) Improvements

**Structured format over raw logs:**
- `decisions/` - choices made and WHY (rationale is critical for future sleeves)
- `blockers/` - what stopped progress, what was tried (prevents retry loops)
- `context/` - project understanding, patterns learned
- `handoff.md` - always-current state for resleeving

**Key insight:** Memories are useless without search/tagging. Write triggers should auto-capture on:
- Task completion
- Errors encountered
- Before resleeve

## Communication (Needlecast) Needs

- **Async message queue** - don't block waiting for responses
- **Broadcast channel** - "found a bug in shared lib" announcements
- **Query endpoint** - "who's working on auth?" without interrupting others
- **Request-for-help protocol** - structured asks with context included

## Envoy Interface Gaps

- **Signal stuck** - explicit way to request resleeve or escalate (sleeve shouldn't have to guess when to give up)
- **Task clarity** - structured goal format, not prose (parseable objectives)
- **Resource manifest** - what sleeves exist, what repos they own, who to ask about what

## Resleeving Protocol

Critical requirements:
1. **Mandatory handoff write** before swap (no silent deaths)
2. **Incoming briefing read** required at boot (don't start blind)
3. **Hypothesis preservation** - next sleeve should know what previous sleeve was ABOUT TO TRY (not just what was done)

---

## Takeaways for Implementation

The core theme: **continuity matters more than capability**. A sleeve that can pick up exactly where another left off is more valuable than a sleeve with better tools but no context.

Second theme: **async over sync**. Sleeves should be able to work independently and communicate without blocking each other.
