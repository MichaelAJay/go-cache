# Cache + Secondary Indexing: Design & Implementation Summary

## Conversation Summary

We explored the design of a cache layer that manages not only primary record storage (e.g., sessions) but also secondary indexes to enable efficient queries and bulk operations. Initially, the concern was whether consumers would need to manage indexes themselves, but we established that Redis Lua scripts can ensure atomic updates to both the primary data and all relevant indexes. This guarantees consistency and minimizes consumer responsibility.

## Key Concepts

- **Primary record:** A session object keyed by `session:{sessionID}`.
- **Forward index:** `subject:{subjectID}:sessions` → set of sessionIDs.
- **Reverse index:** `session:{sessionID}:subject` → subjectID for quick lookups.
- **Atomicity:** All operations (add/remove session) update primary record and indexes in a single Lua script to prevent inconsistencies.
- **Provider responsibility:** Cache provider manages both data and indexes; consumers call high-level helpers without worrying about index maintenance.

## Code Components

### Lua Scripts

1. **AddSession**

   - Stores session under `session:{sessionID}`.
   - Adds `sessionID` to `subject:{subjectID}:sessions` set.
   - Maps `session:{sessionID}:subject` → `subjectID`.

2. **DeleteSession**

   - Removes session object.
   - Looks up subjectID from reverse index.
   - Removes `sessionID` from `subject:{subjectID}:sessions`.
   - Removes reverse mapping.

3. **DeleteSubjectSessions**

   - Fetches all sessionIDs from `subject:{subjectID}:sessions`.
   - Deletes each session and reverse mapping.
   - Clears the forward index set.

### Go Helper Wrappers

- Helpers construct the KEYS/ARGV arrays and call the scripts.
- Expose clean functions:

  - `AddSession(ctx, client, sessionID, subjectID, data)`
  - `DeleteSession(ctx, client, sessionID)`
  - `DeleteSubjectSessions(ctx, client, subjectID)`

### Guarantees

- Consistent maintenance of both forward and reverse indexes.
- Atomic operations ensure no dangling references.
- Consumer does not need to manage index updates explicitly.

## Consumer Responsibilities

- Use provided helpers (`AddSession`, `DeleteSession`, `DeleteSubjectSessions`).
- Provide session data and IDs; the provider handles the rest.
- No manual index updates needed.

## Benefits

- Centralized management of indexes by cache provider.
- Reduced consumer complexity.
- Consistent, atomic, and scalable session/index handling.

---

This design ensures **clean separation of concerns**: the consumer focuses on business logic, while the cache provider guarantees integrity of both primary records and secondary indexes.
