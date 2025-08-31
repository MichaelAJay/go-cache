Key schema (recommended):

data:entry:<entryID>

meta:entry:<entryID>

index:entry:<entryID> → <ownerID> (string)

index:owner:<ownerID> → Set<entryID>

Writes:

On create/update: set data:_, meta:_, SET index:entry:<entryID> <ownerID>, SADD index:owner:<ownerID> <entryID>.

Deletes:

By entry: run deleteByEntryIDScript.

By owner (small sets): deleteAllEntriesForOwnerScript.

By owner (large sets): loop deleteEntriesForOwnerChunkedScript.
