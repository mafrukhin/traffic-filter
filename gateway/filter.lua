local args = ngx.req.get_uri_args()

local direct = args["dl"] or os.getenv("DIRECT_LINK_DEFAULT")
local block = args["bl"] or os.getenv("BLOCK_LINK_DEFAULT")

local ua = ngx.var.http_user_agent

if not direct then direct = "https://google.com" end
if not block then block = "https://example.com" end

if ua == nil then
    return ngx.redirect(block)
end

if string.find(string.lower(ua), "bot") then
    return ngx.redirect(block)
end

return ngx.redirect(direct)