Redis Lua Script & Indexing Review
Observations on Provided Lua Scripts

Locking

Current approach (SET NX EX + unconditional DEL) is unsafe.

Risk: a process can delete another’s lock.

Fix: only DEL if GET lockKey == lockValue.

Delete by Pattern Script

Uses KEYS, which is O(N) across entire Redis keyspace and blocks Redis.

Fix: avoid KEYS. Replace with index-driven deletes.

Delete by Index Script

Uses SMEMBERS.

Safe if sets are small, but O(N) blocking if sets are large.

Fix: if sets may grow large, move iteration to client with SSCAN + batched deletes.

Metadata

HMSET is deprecated, replace with HSET.

Counter updates can reset if metadata is missing. Validate expected behavior.

General

Watch for nil values (serializedValue, newSerializedValue).

Prefix math in deleteByPattern can target unintended keys.

Indexing Strategy

Primary keys: Redis keys holding actual data (e.g. data:session:<sessionID>).

Secondary indexes: maintained explicitly inside Redis.

index:subject:<subjectID> → set of sessionIDs.

index:session:<sessionID> → subjectID (reverse mapping).

Why Reverse Mapping?

Needed to support deletes when starting from sessionID.

Example: “Given a sessionID, delete all sessions for that subject” requires first resolving subjectID from sessionID.

This enables traversals in both directions: subject → sessions and session → subject.

Delete Use Cases

Delete by subjectID

SMEMBERS index:subject:<subjectID> → sessionIDs.

Delete all data:session:<sessionID>.

Delete all index:session:<sessionID>.

Delete index:subject:<subjectID>.

Delete by sessionID

GET index:session:<sessionID> → subjectID.

Remove sessionID from index:subject:<subjectID>.

Delete data:session:<sessionID>.

Delete index:session:<sessionID>.

Delete by pattern

🚫 Do not use KEYS.

✅ Instead, operate on index keys (e.g. SCAN index:subject:\* if bulk deletion is required).

Deletes remain bounded and predictable.

Actionable Next Steps

Rewrite lock release to check value before DEL.

Replace deleteByPattern with index-driven deletes.

For deleteByIndex, decide:

If sets always small → Lua loop OK.

If sets can grow large → use client-driven SSCAN batching.

Replace HMSET with HSET.

Validate nil handling and metadata semantics.

Standardize index maintenance:

Every write = update data:session:<sessionID>, index:subject:<subjectID>, and index:session:<sessionID>.

Every delete = remove all three consistently.
