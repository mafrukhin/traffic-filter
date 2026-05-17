-- Only redirect if not already on fp.html
if ngx.var.uri ~= "/fp.html" then
  local args = ngx.req.get_uri_args()
  return ngx.redirect("/fp.html?" .. (ngx.var.query_string or ""))
end
-- Let fp.html serve normally
return nil