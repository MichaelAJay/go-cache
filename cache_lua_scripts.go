package cache

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// initLuaScripts initializes Lua scripts for atomic operations using vetted scripts
func (c *RedisCache[T]) initLuaScripts() {
	// Get script with LRU tracking
	c.getScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local lruTrackerKey = KEYS[3]
		local entryKey = ARGV[1]

		-- Get the value
		local value = redis.call('GET', dataKey)
		if not value then
			return {nil, '0'}  -- not found
		end

		-- Update metadata atomically using microsecond precision
		local now = redis.call('TIME')
		local ts = tonumber(now[1]) * 1000000 + tonumber(now[2])
		redis.call('HINCRBY', metaKey, 'access_count', 1)
		redis.call('HSET', metaKey, 'last_accessed', ts)

		-- Update LRU tracker with new access time
		redis.call('ZADD', lruTrackerKey, ts, entryKey)

		return {value, '1'}  -- found
	`)

	// Unified SET script with conditional indexing and LRU eviction
	c.setScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local indexKey = KEYS[3]
		local reverseKey = KEYS[4]
		local lruTrackerKey = KEYS[5]
		local serializedVal = ARGV[1]
		local ttlMs = tonumber(ARGV[2])
		local entryKey = ARGV[3]
		local ownerKey = ARGV[4]
		local indexingEnabled = ARGV[5] == "true"
		local maxEntries = tonumber(ARGV[6])
		local dataPrefix = ARGV[7]
		local metaPrefix = ARGV[8]

		-- Set value with millisecond precision using modern SET syntax
		if ttlMs and ttlMs > 0 then
			redis.call('SET', dataKey, serializedVal, 'PX', ttlMs)
		else
			redis.call('SET', dataKey, serializedVal)
		end

		-- Set metadata using microsecond precision
		local now = redis.call('TIME')
		local ts = tonumber(now[1]) * 1000000 + tonumber(now[2])
		redis.call('HSET', metaKey,
			'created_at', ts,
			'last_accessed', ts,
			'access_count', '1',
			'ttl', tostring(ttlMs or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttlMs and ttlMs > 0 then
			redis.call('PEXPIRE', metaKey, ttlMs)
		end

		-- LRU tracking and eviction logic
		if maxEntries and maxEntries > 0 then
			-- Update LRU tracker with current timestamp as score
			redis.call('ZADD', lruTrackerKey, ts, entryKey)
			
			-- Check if we need to evict entries
			local currentCount = redis.call('ZCARD', lruTrackerKey)
			if currentCount > maxEntries then
				-- Get oldest entries that need to be evicted
				local toEvict = currentCount - maxEntries
				local oldestEntries = redis.call('ZRANGE', lruTrackerKey, 0, toEvict - 1)
				
				-- Remove oldest entries from cache and LRU tracker
				for i = 1, #oldestEntries do
					local oldEntryKey = oldestEntries[i]
					local oldDataKey = dataPrefix .. oldEntryKey
					local oldMetaKey = metaPrefix .. oldEntryKey
					
					-- Delete the actual cache data
					redis.call('DEL', oldDataKey)
					redis.call('DEL', oldMetaKey)
					
					-- Remove from LRU tracker
					redis.call('ZREM', lruTrackerKey, oldEntryKey)
					
					-- If indexing is enabled, clean up reverse index for evicted entry
					if indexingEnabled then
						local oldReverseKey = 'cache:reverse:' .. oldEntryKey
						local oldOwner = redis.call('GET', oldReverseKey)
						if oldOwner then
							local oldIndexKey = 'cache:index:owner:' .. oldOwner
							redis.call('SREM', oldIndexKey, oldEntryKey)
							redis.call('DEL', oldReverseKey)
						end
					end
				end
			end
		end

		-- Early return if indexing is disabled
		if not indexingEnabled then
			return 'OK'  -- SET succeeded, no indexing
		end

		-- Get old owner for potential cleanup
		local oldOwner = redis.call('GET', reverseKey)
		
		-- Handle owner change - remove from old index if owner changed
		if oldOwner and oldOwner ~= ownerKey then
			-- Build old index key using the same pattern as buildIndexKey
			local oldIndexKey = 'cache:index:owner:' .. oldOwner
			redis.call('SREM', oldIndexKey, entryKey)
		end

		-- Update forward index (owner -> entry keys)
		redis.call('SADD', indexKey, entryKey)
		if ttlMs and ttlMs > 0 then
			redis.call('PEXPIRE', indexKey, ttlMs)
		end

		-- Update reverse index (entry -> owner key)
		if ttlMs and ttlMs > 0 then
			redis.call('SET', reverseKey, ownerKey, 'PX', ttlMs)
		else
			redis.call('SET', reverseKey, ownerKey)
		end

		return 'OK'
	`)

	// GetOrSet script
	c.getOrSetScript = redis.NewScript(`
		local lockKey        = KEYS[1]
		local dataKey        = KEYS[2]
		local metaKey        = KEYS[3]
		local lockValue      = ARGV[1]
		local ttlMs          = tonumber(ARGV[2])
		local serializedVal  = ARGV[3]
		local lockTimeoutMs  = tonumber(ARGV[4])

		-- Fast path: already present
		local existing = redis.call('GET', dataKey)
		if existing then
			return {existing, '0'}  -- found, no write
		end

		-- Acquire lock with millisecond precision
		local ok = redis.call('SET', lockKey, lockValue, 'PX', lockTimeoutMs, 'NX')
		-- SET with NX returns empty table on success, false on failure
		if ok == false then
			-- Someone else is loading; check again
			existing = redis.call('GET', dataKey)
			if existing then
				return {existing, '0'}
			end
			return {false, '1'} -- signal retry
		end

		-- Validate we have actual data to set
		if serializedVal == nil or serializedVal == "" then
			-- Release lock and return miss signal - no data to set
			if redis.call('GET', lockKey) == lockValue then
				redis.call('DEL', lockKey)
			end
			return {false, '2'}  -- Signal: miss + no data available (use false instead of nil)
		end

		-- Set value with millisecond precision using modern SET syntax
		if ttlMs and ttlMs > 0 then
			redis.call('SET', dataKey, serializedVal, 'PX', ttlMs)
		else
			redis.call('SET', dataKey, serializedVal)
		end

		-- Metadata
		local now = redis.call('TIME')
		local ts  = now[1]
		redis.call('HSET', metaKey,
			'created_at', ts,
			'last_accessed', ts,
			'access_count', '1',
			'ttl', tostring(ttlMs or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttlMs and ttlMs > 0 then
			redis.call('PEXPIRE', metaKey, ttlMs)
		end

		-- Release lock if we own it
		if redis.call('GET', lockKey) == lockValue then
			redis.call('DEL', lockKey)
		end

		return {serializedVal, '0'}
	`)

	// Update script
	c.updateScript = redis.NewScript(`
		local lockKey     = KEYS[1]
		local dataKey     = KEYS[2]
		local metaKey     = KEYS[3]
		local lockValue   = ARGV[1]
		local ttlMs       = tonumber(ARGV[2])
		local newVal      = ARGV[3]
		local lockToutMs  = tonumber(ARGV[4])

		local ok = redis.call('SET', lockKey, lockValue, 'PX', lockToutMs, 'NX')
		-- SET with NX returns empty table on success, false on failure
		if ok == false then
			return {false, '1'} -- signal retry
		end

		local oldVal = redis.call('GET', dataKey)
		local existed = oldVal and '1' or '0'

		-- Only set value if newVal is not empty (not the read-only call)
		if newVal and newVal ~= "" then
			-- Set value with millisecond precision using modern SET syntax
			if ttlMs and ttlMs > 0 then
				redis.call('SET', dataKey, newVal, 'PX', ttlMs)
			else
				redis.call('SET', dataKey, newVal)
			end

			local now = redis.call('TIME')
			local ts  = now[1]
			local acc = redis.call('HGET', metaKey, 'access_count') or '0'
			acc = tostring((tonumber(acc) or 0) + 1)

			redis.call('HSET', metaKey,
				'last_accessed', ts,
				'access_count', acc,
				'ttl', tostring(ttlMs or 0),
				'size', tostring(string.len(newVal))
			)
			if ttlMs and ttlMs > 0 then
				redis.call('PEXPIRE', metaKey, ttlMs)
			end
		end

		-- Always release lock if we still own it
		if redis.call('GET', lockKey) == lockValue then
			redis.call('DEL', lockKey)
		end

		return {oldVal or false, existed, newVal}
	`)

	// @TODO change name from sessIdxPref
	// Delete by index script
	c.deleteByIndexScript = redis.NewScript(`
		local subjIdxKey   = KEYS[1]
		local dataPref     = ARGV[1]
		local metaPref     = ARGV[2]
		local sessIdxPref  = ARGV[3]

		local sids = redis.call('SMEMBERS', subjIdxKey)
		local total = 0

		for i = 1, #sids do
			local sid = sids[i]
			total = total + redis.call('DEL', dataPref .. sid)
			total = total + redis.call('DEL', metaPref .. sid)
			total = total + redis.call('DEL', sessIdxPref .. sid)
		end

		total = total + redis.call('DEL', subjIdxKey)
		return total
	`)

	// @TODO think about a rename. Also think about whether there needs to be any manipulation of index values
	// e.g. reverseKey -> does it GET the OwnerKey, or the OwnerKey less the prefix. One requires some string manipulation, the other requires more data stored
	// Delete with index cleanup and LRU tracking script
	c.deleteByEntryScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local reverseKey = KEYS[3]
		local lruTrackerKey = KEYS[4]
		local entryKey = ARGV[1]
		local indexPrefix = ARGV[2]

		local removed = 0

		-- Get the owner key from reverse index
		local ownerKey = redis.call('GET', reverseKey)
		
		if ownerKey then
			-- Remove entry from forward index
			local forwardIndexKey = indexPrefix .. 'owner:' .. ownerKey
			redis.call('SREM', forwardIndexKey, entryKey)
		end

		-- Delete main keys
		removed = removed + redis.call('DEL', dataKey)
		removed = removed + redis.call('DEL', metaKey)
		removed = removed + redis.call('DEL', reverseKey)

		-- Remove from LRU tracker
		redis.call('ZREM', lruTrackerKey, entryKey)

		return removed
	`)

	// GetByOwner script - atomic fetch of all entries for an owner
	c.getByOwnerScript = redis.NewScript(`
		local indexKey = KEYS[1]
		local dataPrefix = ARGV[1]
		local metaPrefix = ARGV[2]

		-- Get all entry keys for this owner
		local entryKeys = redis.call('SMEMBERS', indexKey)
		if #entryKeys == 0 then
			return {}
		end

		local results = {}
		local now = redis.call('TIME')
		local ts = now[1]

		-- Fetch each entry's data and update access metadata atomically
		for i = 1, #entryKeys do
			local entryKey = entryKeys[i]
			local dataKey = dataPrefix .. entryKey
			local metaKey = metaPrefix .. entryKey
			
			local value = redis.call('GET', dataKey)
			if value then
				-- Update access metadata atomically
				redis.call('HINCRBY', metaKey, 'access_count', 1)
				redis.call('HSET', metaKey, 'last_accessed', ts)
				
				-- Include both key and value in results
				table.insert(results, {entryKey, value})
			end
		end

		return results
	`)

	// DeleteByOwner script - atomic deletion of all entries for an owner
	// Returns the number of sessions/entries actually deleted, counting successful deletions
	c.deleteByOwnerScript = redis.NewScript(`
		local indexKey = KEYS[1]
		local dataPrefix = ARGV[1]
		local metaPrefix = ARGV[2]
		local reversePrefix = ARGV[3]

		-- Get all entry keys for this owner
		local entryKeys = redis.call('SMEMBERS', indexKey)
		if #entryKeys == 0 then
			return 0
		end

		local sessionsDeleted = 0

		-- Delete each entry's data, metadata, and reverse index atomically
		-- Count sessions as deleted only if the main data key existed and was deleted
		for i = 1, #entryKeys do
			local entryKey = entryKeys[i]
			local dataKey = dataPrefix .. entryKey
			local metaKey = metaPrefix .. entryKey
			local reverseKey = reversePrefix .. entryKey
			
			-- Only count as deleted if the main data key actually existed
			local dataDeleted = redis.call('DEL', dataKey)
			if dataDeleted > 0 then
				sessionsDeleted = sessionsDeleted + 1
			end
			
			-- Always clean up metadata and reverse indexes regardless
			redis.call('DEL', metaKey)
			redis.call('DEL', reverseKey)
		end

		-- Delete the owner index itself
		redis.call('DEL', indexKey)

		-- Return count of sessions actually deleted (based on data key existence)
		return sessionsDeleted
	`)

	c.getCountByOwnerScript = redis.NewScript(`
		local indexKey = KEYS[1]
		
		-- Count all entry keys for this owner using set cardinality
		-- Returns 0 if the index key doesn't exist (no entries for this owner)
		local count = redis.call('SCARD', indexKey)
		return count
	`)

	// SetIfExists script - atomic conditional SET that only sets if key exists
	c.setIfExistsScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local indexKey = KEYS[3]
		local reverseKey = KEYS[4]
		local serializedVal = ARGV[1]
		local ttlMs = tonumber(ARGV[2])
		local entryKey = ARGV[3]
		local ownerKey = ARGV[4]
		local indexingEnabled = ARGV[5] == "true"

		-- Try to set only if key already exists, using millisecond precision
		local setArgs = { "SET", dataKey, serializedVal, "XX" }
		if ttlMs and ttlMs > 0 then
			table.insert(setArgs, "PX")
			table.insert(setArgs, ttlMs)
		end

		local ok = redis.call(unpack(setArgs))
		if not ok then
			return 0  -- Key doesn't exist, SET failed
		end

		-- Update metadata
		local now = redis.call('TIME')
		local ts = now[1]
		local acc = redis.call('HGET', metaKey, 'access_count') or '0'
		acc = tostring((tonumber(acc) or 0) + 1)

		redis.call('HSET', metaKey,
			'last_accessed', ts,
			'access_count', acc,
			'ttl', tostring(ttlMs or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttlMs and ttlMs > 0 then
			redis.call('PEXPIRE', metaKey, ttlMs)
		end

		-- Early return if indexing is disabled
		if not indexingEnabled then
			return 1  -- SET succeeded, no indexing
		end

		-- Get old owner for potential cleanup
		local oldOwner = redis.call('GET', reverseKey)

		-- Handle owner change - remove from old index if owner changed
		if oldOwner and oldOwner ~= ownerKey then
			local oldIndexKey = string.gsub(indexKey, ownerKey, oldOwner)
			redis.call('SREM', oldIndexKey, entryKey)
		end

		-- Update forward index (owner -> entry keys)
		redis.call('SADD', indexKey, entryKey)
		if ttlMs and ttlMs > 0 then
			redis.call('PEXPIRE', indexKey, ttlMs)
		end

		-- Update reverse index (entry -> owner key)
		if ttlMs and ttlMs > 0 then
			redis.call('SET', reverseKey, ownerKey, 'PX', ttlMs)
		else
			redis.call('SET', reverseKey, ownerKey)
		end

		return 1  -- SET succeeded
	`)

	// SetIfNotExists script - atomic conditional SET that only sets if key doesn't exist
	c.setIfNotExistsScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local indexKey = KEYS[3]
		local reverseKey = KEYS[4]
		local serializedVal = ARGV[1]
		local ttlMs = tonumber(ARGV[2])
		local entryKey = ARGV[3]
		local ownerKey = ARGV[4]
		local indexingEnabled = ARGV[5] == "true"

		-- Try to set only if key does not already exist, using millisecond precision
		local setArgs = { "SET", dataKey, serializedVal, "NX" }
		if ttlMs and ttlMs > 0 then
			table.insert(setArgs, "PX")
			table.insert(setArgs, ttlMs)
		end

		local ok = redis.call(unpack(setArgs))
		if not ok then
			return 0  -- Key exists, SET failed
		end

		-- Set initial metadata
		local now = redis.call('TIME')
		local ts = now[1]
		redis.call('HSET', metaKey,
			'created_at', ts,
			'last_accessed', ts,
			'access_count', '1',
			'ttl', tostring(ttlMs or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttlMs and ttlMs > 0 then
			redis.call('PEXPIRE', metaKey, ttlMs)
		end

		-- Early return if indexing is disabled
		if not indexingEnabled then
			return 1  -- SET succeeded, no indexing
		end

		-- Update forward index (owner -> entry keys)
		redis.call('SADD', indexKey, entryKey)
		if ttlMs and ttlMs > 0 then
			redis.call('PEXPIRE', indexKey, ttlMs)
		end

		-- Update reverse index (entry -> owner key)
		if ttlMs and ttlMs > 0 then
			redis.call('SET', reverseKey, ownerKey, 'PX', ttlMs)
		else
			redis.call('SET', reverseKey, ownerKey)
		end

		return 1  -- SET succeeded
	`)

	// GetMany metadata update script for atomic metadata updates after batch retrieval
	c.getManyMetadataUpdateScript = redis.NewScript(`
		local metaKeys = KEYS
		if #metaKeys == 0 then
			return 0
		end
		
		local now = redis.call('TIME')
		local ts = tonumber(now[1]) * 1000000 + tonumber(now[2])
		
		for i = 1, #metaKeys do
			redis.call('HINCRBY', metaKeys[i], 'access_count', 1)
			redis.call('HSET', metaKeys[i], 'last_accessed', ts)
		end
		
		return #metaKeys
	`)

	// Orphaned metadata cleanup script - atomic cleanup when data key is missing
	c.cleanupOrphanedMetadataScript = redis.NewScript(`
		local metaKey = KEYS[1]
		local reverseKey = KEYS[2]
		local lruTrackerKey = KEYS[3]
		local entryKey = ARGV[1]
		local indexPrefix = ARGV[2]

		-- Check if metadata actually exists
		local metadataExists = redis.call('EXISTS', metaKey)
		if metadataExists == 0 then
			return 0  -- No metadata to clean
		end

		local cleaned = 0

		-- Clean up metadata
		cleaned = cleaned + redis.call('DEL', metaKey)

		-- Clean up reverse index if it exists
		local ownerKey = redis.call('GET', reverseKey)
		if ownerKey then
			-- Remove from forward index
			local forwardIndexKey = indexPrefix .. 'owner:' .. ownerKey
			redis.call('SREM', forwardIndexKey, entryKey)
			-- Remove reverse index
			cleaned = cleaned + redis.call('DEL', reverseKey)
		end

		-- Remove from LRU tracker
		redis.call('ZREM', lruTrackerKey, entryKey)

		return cleaned
	`)

	// DeleteMany script - atomic batch deletion with proper index and LRU cleanup
	c.deleteManyScript = redis.NewScript(`
		local lruTrackerKey = KEYS[1]
		local indexPrefix = ARGV[1]
		local indexingEnabled = ARGV[2] == "true"
		local reversePrefix = ARGV[3]
		-- Keys are provided in groups of 3: dataKey, metaKey, reverseKey
		-- Starting from ARGV[4] onwards
		
		local totalDeleted = 0
		local argIndex = 4
		
		-- Process each group of keys (dataKey, metaKey, reverseKey)
		while argIndex <= #ARGV do
			local dataKey = ARGV[argIndex]
			local metaKey = ARGV[argIndex + 1]
			local reverseKey = ARGV[argIndex + 2]
			
			-- Extract entry key from reverseKey by removing the reverse prefix
			-- reverseKey format: "cache:reverse:session:batch-count-1"
			-- reversePrefix:    "cache:reverse:"
			-- We need to extract: "session:batch-count-1"
			local entryKey = string.gsub(reverseKey, "^" .. reversePrefix:gsub("([%-%^%$%(%)%%%.%[%]%*%+%?])", "%%%1"), "")
			
			-- Index cleanup if indexing is enabled (do this BEFORE deleting keys)
			if indexingEnabled then
				-- Get owner from reverse index for forward index cleanup
				local ownerKey = redis.call('GET', reverseKey)
				if ownerKey then
					-- Remove entry from forward index (owner -> entry keys)
					local forwardIndexKey = indexPrefix .. 'owner:' .. ownerKey
					redis.call('SREM', forwardIndexKey, entryKey)
				end
				-- Delete reverse index (entry -> owner key)
				totalDeleted = totalDeleted + redis.call('DEL', reverseKey)
			end
			
			-- Delete main keys and count successful deletions
			local dataDeleted = redis.call('DEL', dataKey)
			local metaDeleted = redis.call('DEL', metaKey)
			totalDeleted = totalDeleted + dataDeleted + metaDeleted
			
			-- Remove from LRU tracker
			redis.call('ZREM', lruTrackerKey, entryKey)
			
			argIndex = argIndex + 3
		end
		
		return totalDeleted
	`)

	if c.options.WarmLuaScripts {
		c.warmLuaScripts(context.Background())
	}
}

func (c *RedisCache[T]) warmLuaScripts(ctx context.Context) error {
	scripts := []*redis.Script{
		c.getScript, c.setScript,
		c.getOrSetScript, c.updateScript, c.deleteByIndexScript,
		c.deleteByEntryScript, c.getByOwnerScript, c.deleteByOwnerScript, c.getCountByOwnerScript,
		c.setIfExistsScript, c.setIfNotExistsScript,
		c.getManyMetadataUpdateScript, c.cleanupOrphanedMetadataScript,
		c.deleteManyScript,
	}

	for _, script := range scripts {
		if err := script.Load(ctx, c.client).Err(); err != nil {
			return fmt.Errorf("failed to warm script :%w", err)
		}
	}
	return nil
}
