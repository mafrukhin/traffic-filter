local http = require "resty.http"
local redis = require "resty.redis"

local args = ngx.req.get_uri_args()

local direct = args["dl"] or os.getenv("DIRECT_LINK_DEFAULT")
local block = args["bl"] or os.getenv("BLOCK_LINK_DEFAULT")

local ip = ngx.var.remote_addr
local ua = ngx.var.http_user_agent

-- fallback safety
if not direct then direct = "https://google.com" end
if not block then block = "https://example.com" end

-- BASIC BOT
if ua == nil then
    return ngx.redirect(block)
end

if string.find(string.lower(ua), "bot") then
    return ngx.redirect(block)
end

-- RATE LIMIT (Redis)
local red = redis:new()
red:set_timeout(1000)

local ok, err = red:connect("redis", 6379)

if ok then
    local key = "ip:" .. ip
    local reqs = red:incr(key)

    if reqs == 1 then
        red:expire(key, 10)
    end

    if reqs > 5 then
        return ngx.redirect(block)
    end
end

-- CALL BOT ENGINE
local httpc = http.new()

local res = httpc:request_uri("http://botengine:8080/check", {
    method = "POST",
    body = "ip="..ip.."&ua="..(ua or "")
})

if res and res.body == "block" then
    return ngx.redirect(block)
end

return ngx.redirect(direct)