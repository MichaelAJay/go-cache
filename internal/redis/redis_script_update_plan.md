Redis Cache Delete Handling Plan

1. Core Problem

Current delete logic uses KEYS queries (redis.call("KEYS", ...)) → not scalable in production, risk of blocking the server.

Indexes for relationships (e.g., ownerID → entryIDs) are currently held outside Redis, creating consistency problems when multiple processes interact.

Need bi-directional indexes for efficient deletes:

ownerID → entryIDs (all cache entries for an owner).

entryID → ownerID (find owner when deleting a cache entry).

2. Corrected Design

Store all indexes inside Redis (no in-process only state).

Use Redis Sets:

owner:{ownerID}:entries → set of entryIDs for an owner.

entry:{entryID}:owner → single ownerID for a cache entry.

Deletes use these indexes:

DeleteEntry(entryID):

Lookup ownerID via entry:{entryID}:owner.

Remove entryID from owner:{ownerID}:entries.

Delete the cache entry key itself.

Delete entry:{entryID}:owner.

DeleteOwnerEntries(ownerID):

Lookup all entryIDs via owner:{ownerID}:entries.

Delete all cache entry keys + entry:{entryID}:owner entries.

Delete owner:{ownerID}:entries.

DeleteEntry.lua
-- KEYS[1] = entry:{entryID}:owner
-- KEYS[2] = owner:{ownerID}:entries
-- KEYS[3] = entry:{entryID} (actual cache entry data)

local ownerID = redis.call("GET", KEYS[1])
if not ownerID then
return 0
end

redis.call("SREM", "owner:" .. ownerID .. ":entries", ARGV[1])
redis.call("DEL", KEYS[1], KEYS[3])
return 1

DeleteOwnerEntries.lua
-- KEYS[1] = owner:{ownerID}:entries

local entries = redis.call("SMEMBERS", KEYS[1])
for \_, eid in ipairs(entries) do
redis.call("DEL", "entry:" .. eid, "entry:" .. eid .. ":owner")
end
redis.call("DEL", KEYS[1])
return #entries

Go Helper Wrappers
package cache

import (
"context"
"github.com/redis/go-redis/v9"
)

type EntryCache struct {
rdb \*redis.Client
ctx context.Context
}

// NewEntryCache constructor
func NewEntryCache(rdb *redis.Client) *EntryCache {
return &EntryCache{
rdb: rdb,
ctx: context.Background(),
}
}

// Delete a single cache entry
func (c \*EntryCache) DeleteEntry(entryID string) (bool, error) {
keys := []string{
"entry:" + entryID + ":owner", // KEYS[1]
"", // placeholder (owner set resolved in script)
"entry:" + entryID, // KEYS[3]
}
return c.rdb.EvalSha(c.ctx, deleteEntrySha, keys, entryID).Bool()
}

// Delete all cache entries for an owner
func (c \*EntryCache) DeleteOwnerEntries(ownerID string) (int64, error) {
keys := []string{
"owner:" + ownerID + ":entries", // KEYS[1]
}
return c.rdb.EvalSha(c.ctx, deleteOwnerEntriesSha, keys).Int64()
}
