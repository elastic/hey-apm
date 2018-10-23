package compose

var SingleTransaction = []byte(`{
"id": "4340a8e0df1906ecbfa9", 
"trace_id": "0acd456789abcdef0123456789abcdef", 
"name": "GET /api/types",
"type": "request",
"duration": 32.592981,
"result": "success", 
"timestamp": 1496170407154000, 
"sampled": true, 
"span_count": {"started": 17},
"context":{
	"request": 
		{"socket": {"remote_address": "12.53.12.1","encrypted": true},
		"http_version": "1.1",
		"method": "POST",
		"url": {"protocol": "https:",
		"full": "https://www.example.com/p/a/t/h?query=string#hash",
		"hostname": "www.example.com",
		"port": "8080",
		"pathname": "/p/a/t/h",
		"search": "?query=string",
		"hash": "#hash",
		"raw": "/p/a/t/h?query=string#hash"},
	"headers": {"user-agent": "Mozilla Chrome Edge","content-type": "text/html","cookie": "c1=v1; c2=v2","some-other-header": "foo","array": ["foo","bar","baz"]},
	"cookies": {"c1": "v1","c2": "v2"},
	"env": {"SERVER_SOFTWARE": "nginx","GATEWAY_INTERFACE": "CGI/1.1"},
	"body": {"str": "hello world","additional": { "foo": {},"bar": 123,"req": "additional information"}}},
	"response":{
		"status_code": 200,
		"headers": {"content-type": "application/json"},
		"headers_sent": true,
		"finished": true}, 
	"user": {"id": "99","username": "foo","email": "foo@example.com"},
	"tags": {"organization_uuid": "9f0e9d64-c185-4d21-a6f4-4673ed561ec8"},
	"custom": {"my_key": 1,"some_other_value": "foo bar","and_objects": {"foo": ["bar","baz"]},"(": "not a valid regex and that is fine"}}}`)
