#!lua name=rate

-- external redis variable names
local KEY_INDEX = 'keyIndex'
local WALKS_VISITING = 'walksVisiting:'

local BUCKET = 'creditBucket:'
local TOKENS = 'tokens'
local LAST_MODIFIED = 'last_modified'
local NOT_PAID = 'unable to pay'
local PAID = 'paid'

-- automatic_refill() refills the bucket of `pubkey` when conditions are met, and returns the number of tokens it holds at the end.
local function automatic_refill(pubkey, refill_tokens, refill_interval, max_tokens_before_refill, walks_threshold)
    local tokens = tonumber(redis.call('HGET', BUCKET .. pubkey, TOKENS)) or 0
    if tokens > max_tokens_before_refill then
        return tokens
    end

    local now = tonumber(redis.call('TIME')[1])
    local last_request = tonumber(redis.call('HGET', BUCKET .. pubkey, LAST_MODIFIED)) or 0
    if now - last_request < refill_interval then
        return tokens
    end

    local nodeID = redis.call('HGET', KEY_INDEX, pubkey)
    if not nodeID then
        -- if the pubkey is not found in the keyIndex, then we assume it's a low-reputation key and we don't refill
        return tokens
    end

    local walks = redis.call('SCARD', WALKS_VISITING .. nodeID)
    if tonumber(walks) < walks_threshold then
        return tokens
    end

    redis.call('HINCRBY', BUCKET .. pubkey , TOKENS, refill_tokens)
    redis.call('HSET', BUCKET .. pubkey, LAST_MODIFIED, tonumber(redis.call('TIME')[1]))
    return tokens + refill_tokens
end

local function pay(_, args)
    local pubkey = tostring(args[1])
    if not pubkey then
        return redis.error_reply('pubkey not provided')
    end

    local cost = tonumber(args[2])
    if not cost then
        return redis.error_reply('missing or invalid cost')
    end

    local refill_tokens = tonumber(args[3])
    if not refill_tokens then
        return redis.error_reply('missing or invalid refill_tokens')
    end

    local refill_interval = tonumber(args[4])
    if not refill_interval then
        return redis.error_reply('missing or invalid refill_interval')
    end

    local max_tokens_before_refill = tonumber(args[5])
    if not max_tokens_before_refill then
        return redis.error_reply('missing or invalid max_tokens_before_refill')
    end

    local walks_threshold = tonumber(args[6])
    if not walks_threshold then
        return redis.error_reply('missing or invalid walks_threshold')
    end

    local tokens = automatic_refill(pubkey, refill_tokens, refill_interval, max_tokens_before_refill, walks_threshold)
    if tokens < cost then
        return NOT_PAID
    end

    redis.call('HINCRBY', BUCKET .. pubkey, TOKENS, -cost)
    redis.call('HSET', BUCKET .. pubkey, LAST_MODIFIED, tonumber(redis.call('TIME')[1]))
    return PAID
end

redis.register_function('pay', pay)
  