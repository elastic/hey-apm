package compose

var SingleTransaction = []byte(`
{"spans":[],
"context":
	{"request":
		{"body":null,
		"cookies":{},
		"env":{"REMOTE_ADDR":"127.0.0.1","SERVER_NAME":"1.0.0.127.in-addr.arpa","SERVER_PORT":"8000"},
		"headers":{"accept":"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8","accept-encoding":"gzip, deflate, br","accept-language":"en-US,en;q=0.9","connection":"keep-alive","content-length":"","content-type":"text/plain","host":"localhost:8000","upgrade-insecure-requests":"1","user-agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36"},
		"method":"GET",
		"socket":{"encrypted":false,"remote_address":"127.0.0.1"},
		"url":{"full":"http://localhost:8000/","hostname":"localhost","pathname":"/","port":"8000","protocol":"http:"}
	},
	"response":{"headers":{"content-length":"365","content-type":"text/html; charset=utf-8","x-frame-options":"SAMEORIGIN"},
				"status_code":200},
	"tags":{},
	"user":{"id":null,"is_authenticated":false,"username":""}
},
"timestamp":"2018-01-09T03:35:37.604813Z",
"type":"request",
"duration":25.555133819580078,
"id":"9eb1899c-f767-4f40-85af-e2de18aaaf0c",
"name":"GET django.views.generic.base.TemplateView",
"result":"HTTP 2xx",
"sampled":true
}
`)
