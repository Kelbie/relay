#!lua name=credits

-- external redis variables
local KEY_INDEX = 'keyIndex'
local NODE = 'node:'
local ADDITION_TIMESTAMP = 'added_TS'
local WALKS_VISITING = 'walksVisiting:'

-- internal redis variables
local BUCKET = 'creditBucket:'
local TOKENS = 'tokens'
local LAST_MODIFIED = 'last_modified'
local SUCCESS = 'successful deduction'
local FAILED = 'failed deduction'

local function parse(args)
    local pubkey = tostring(args[1])
    if not pubkey or pubkey == "" then
        return nil, redis.error_reply('pubkey not provided')
    end

    local cost = tonumber(args[2])
    if not cost then
        return nil, redis.error_reply('missing or invalid cost')
    end

    local refill_amount = tonumber(args[3])
    if not refill_amount then
        return nil, redis.error_reply('missing or invalid refill_amount')
    end

    local refill_interval = tonumber(args[4])
    if not refill_interval then
        return nil, redis.error_reply('missing or invalid refill_interval')
    end

    local refill_walk_threshold = tonumber(args[5])
    if not refill_walk_threshold then
        return nil, redis.error_reply('missing or invalid refill_walk_threshold')
    end

    return {
        pubkey = pubkey,
        cost = cost,
        refill_amount = refill_amount,
        refill_interval = refill_interval,
        refill_walk_threshold = refill_walk_threshold
    }
end

-- automatic_refill the bucket of `pubkey` when conditions are met, and returns the number of tokens it holds at the end.
local function automatic_refill(params)
    local pubkey                = params.pubkey
    local refill_amount         = params.refill_amount
    local refill_interval       = params.refill_interval
    local refill_walk_threshold = params.refill_walk_threshold

    local tokens = tonumber(redis.call('HGET', BUCKET .. pubkey, TOKENS)) or 0
    if tokens >= refill_amount then
        return tokens
    end

    local now = tonumber(redis.call('TIME')[1])
    local last_modified = tonumber(redis.call('HGET', BUCKET .. pubkey, LAST_MODIFIED)) or 0
    if now - last_modified < refill_interval then
        return tokens
    end

    local id = redis.call('HGET', KEY_INDEX, pubkey)
    if not id then
        -- if the pubkey is not found in the keyIndex, 
        -- it's a low-reputation key and we don't refill
        return tokens
    end

    local added = tonumber(redis.call('HGET', NODE .. id, ADDITION_TIMESTAMP)) or 0
    if now - added < refill_interval then
        -- if the pubkey is not old enough, no refill
        return tokens
    end

    local walks = tonumber(redis.call('SCARD', WALKS_VISITING .. id))
    if walks < refill_walk_threshold then
        return tokens
    end

    redis.call('HSET', BUCKET .. pubkey, TOKENS, refill_amount, LAST_MODIFIED, now)
    return refill_amount
end

local function deduct(_, args)
    local params, err = parse(args)
    if err then 
        return err
    end

    local tokens = automatic_refill(params)
    local cost = params.cost

    if cost <= 0 then
        return SUCCESS
    end

    if tokens < cost then
        return FAILED
    end

    local now = tonumber(redis.call('TIME')[1])
    redis.call('HSET', BUCKET .. params.pubkey, TOKENS, tokens - cost, LAST_MODIFIED, now)
    return SUCCESS
end

redis.register_function('deduct', deduct)
  