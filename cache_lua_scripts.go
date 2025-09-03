package cache

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// initLuaScripts initializes Lua scripts for atomic operations using vetted scripts
func (c *RedisCache[T]) initLuaScripts() {
	// Get script
	c.getScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]

		-- Get the value
		local value = redis.call('GET', dataKey)
		if not value then
			return {nil, '0'}  -- not found
		end

		-- Update metadata atomically
		local now = redis.call('TIME')
		local ts = now[1]
		redis.call('HINCRBY', metaKey, 'access_count', 1)
		redis.call('HSET', metaKey, 'last_accessed', ts)

		return {value, '1'}  -- found
	`)

	// Simple SET script (no indexing)
	c.setScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local serializedVal = ARGV[1]
		local ttl = tonumber(ARGV[2])

		-- Set value
		if ttl and ttl > 0 then
			redis.call('SETEX', dataKey, ttl, serializedVal)
		else
			redis.call('SET', dataKey, serializedVal)
		end

		-- Set metadata
		local now = redis.call('TIME')
		local ts = now[1]
		redis.call('HSET', metaKey,
			'created_at', ts,
			'last_accessed', ts,
			'access_count', '1',
			'ttl', tostring(ttl or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end

		return 'OK'
	`)

	// SET script with indexing support
	c.setWithIndexScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local indexKey = KEYS[3]
		local reverseKey = KEYS[4]
		local serializedVal = ARGV[1]
		local ttl = tonumber(ARGV[2])
		local entryKey = ARGV[3]
		local ownerKey = ARGV[4]

		-- Set value
		redis.call('SET', dataKey, serializedVal)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', dataKey, ttl)
		end

		-- Set metadata
		local now = redis.call('TIME')
		local ts = now[1]
		redis.call('HSET', metaKey,
			'created_at', ts,
			'last_accessed', ts,
			'access_count', '1',
			'ttl', tostring(ttl or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end

		-- Update forward index (owner -> entry keys)
		redis.call('SADD', indexKey, entryKey)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', indexKey, ttl)
		end

		-- Update reverse index (entry -> owner key)
		redis.call('SET', reverseKey, ownerKey)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', reverseKey, ttl)
		end

		return 'OK'
	`)

	// GetOrSet script
	c.getOrSetScript = redis.NewScript(`
		local lockKey       = KEYS[1]
		local dataKey       = KEYS[2]
		local metaKey       = KEYS[3]
		local lockValue     = ARGV[1]
		local ttl           = tonumber(ARGV[2])
		local serializedVal = ARGV[3]
		local lockTimeout   = tonumber(ARGV[4])

		-- Fast path: already present
		local existing = redis.call('GET', dataKey)
		if existing then
			return {existing, '0'}  -- found, no write
		end

		-- Acquire lock
		local ok = redis.call('SET', lockKey, lockValue, 'EX', lockTimeout, 'NX')
		if not ok then
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
			return {nil, '2'}  -- Signal: miss + no data available
		end

		-- Set value
		if ttl and ttl > 0 then
			redis.call('SETEX', dataKey, ttl, serializedVal)
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
			'ttl', tostring(ttl or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end

		-- Release lock if we own it
		if redis.call('GET', lockKey) == lockValue then
			redis.call('DEL', lockKey)
		end

		return {serializedVal, '0'}
	`)

	// Update script
	c.updateScript = redis.NewScript(`
		local lockKey   = KEYS[1]
		local dataKey   = KEYS[2]
		local metaKey   = KEYS[3]
		local lockValue = ARGV[1]
		local ttl       = tonumber(ARGV[2])
		local newVal    = ARGV[3]
		local lockTout  = tonumber(ARGV[4])

		local ok = redis.call('SET', lockKey, lockValue, 'EX', lockTout, 'NX')
		if not ok then
			return {false, '1'} -- signal retry
		end

		local oldVal = redis.call('GET', dataKey)
		local existed = oldVal and '1' or '0'

		if ttl and ttl > 0 then
			redis.call('SETEX', dataKey, ttl, newVal)
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
			'ttl', tostring(ttl or 0),
			'size', tostring(string.len(newVal))
		)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end

		-- Rlease only if we still own it
		if redis.call('GET', lockKey) == lockValue then
			redis.call('DEL', lockKey)
		end

		return {oldVal, existed, newVal}
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
	// Delete with index cleanup script
	c.deleteByEntryScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local reverseKey = KEYS[3]
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

		local totalDeleted = 0

		-- Delete each entry's data, metadata, and reverse index atomically
		for i = 1, #entryKeys do
			local entryKey = entryKeys[i]
			local dataKey = dataPrefix .. entryKey
			local metaKey = metaPrefix .. entryKey
			local reverseKey = reversePrefix .. entryKey
			
			totalDeleted = totalDeleted + redis.call('DEL', dataKey)
			totalDeleted = totalDeleted + redis.call('DEL', metaKey)
			totalDeleted = totalDeleted + redis.call('DEL', reverseKey)
		end

		-- Delete the owner index itself
		totalDeleted = totalDeleted + redis.call('DEL', indexKey)

		return totalDeleted
	`)

	// SetIfExists script - atomic conditional SET that only sets if key exists
	c.setIfExistsScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local serializedVal = ARGV[1]
		local ttl = tonumber(ARGV[2])

		-- Try to set only if key already exists
		local setArgs = { "SET", dataKey, serializedVal, "XX" }
		if ttl and ttl > 0 then
			table.insert(setArgs, "EX")
			table.insert(setArgs, ttl)
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
			'ttl', tostring(ttl or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end

		return 1  -- SET succeeded
	`)

	// SetIfNotExists script - atomic conditional SET that only sets if key doesn't exist
	c.setIfNotExistsScript = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local serializedVal = ARGV[1]
		local ttl = tonumber(ARGV[2])

		-- Try to set only if key does not already exist
		local setArgs = { "SET", dataKey, serializedVal, "NX" }
		if ttl and ttl > 0 then
			table.insert(setArgs, "EX")
			table.insert(setArgs, ttl)
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
			'ttl', tostring(ttl or 0),
			'size', tostring(string.len(serializedVal))
		)
		if ttl and ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end

		return 1  -- SET succeeded
	`)

	// Unified conditional SET script - single script that handles both cases
	c.setIfExistsOrNot = redis.NewScript(`
		local dataKey = KEYS[1]
		local metaKey = KEYS[2]
		local serializedVal = ARGV[1]
		local ttl = tonumber(ARGV[2])
		local condition = ARGV[3]  -- 'EXISTS' or 'NOT_EXISTS'

		-- Build SET command based on condition
		local setArgs = { "SET", dataKey, serializedVal }
		if condition == "EXISTS" then
			table.insert(setArgs, "XX")
		elseif condition == "NOT_EXISTS" then
			table.insert(setArgs, "NX")
		end
		if ttl and ttl > 0 then
			table.insert(setArgs, "EX")
			table.insert(setArgs, ttl)
		end

		local ok = redis.call(unpack(setArgs))
		if not ok then
			return 0  -- Condition not met, SET failed
		end

		-- Handle metadata based on whether this is create or update
		local now = redis.call('TIME')
		local ts = now[1]

		if condition == "NOT_EXISTS" then
			-- New entry - set initial metadata
			redis.call('HSET', metaKey,
				'created_at', ts,
				'last_accessed', ts,
				'access_count', '1',
				'ttl', tostring(ttl or 0),
				'size', tostring(string.len(serializedVal))
			)
		else
			-- Existing entry - update metadata
			local acc = redis.call('HGET', metaKey, 'access_count') or '0'
			acc = tostring((tonumber(acc) or 0) + 1)
			redis.call('HSET', metaKey,
				'last_accessed', ts,
				'access_count', acc,
				'ttl', tostring(ttl or 0),
				'size', tostring(string.len(serializedVal))
			)
		end

		if ttl and ttl > 0 then
			redis.call('EXPIRE', metaKey, ttl)
		end

		return 1  -- SET succeeded
	`)

	if c.options.WarmLuaScripts {
		c.warmLuaScripts(context.Background())
	}
}

func (c *RedisCache[T]) warmLuaScripts(ctx context.Context) error {
	scripts := []*redis.Script{
		c.getScript, c.setScript, c.setWithIndexScript,
		c.getOrSetScript, c.updateScript, c.deleteByIndexScript,
		c.deleteByEntryScript, c.getByOwnerScript, c.deleteByOwnerScript,
		c.setIfExistsScript, c.setIfNotExistsScript, c.setIfExistsOrNot,
	}

	for _, script := range scripts {
		if err := script.Load(ctx, c.client).Err(); err != nil {
			return fmt.Errorf("failed to warm script :%w", err)
		}
	}
	return nil
}
