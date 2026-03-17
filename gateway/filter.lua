local args = ngx.req.get_uri_args()

-- redirect ke fingerprint dulu
return ngx.redirect("/fp.html?" .. (ngx.var.query_string or ""))