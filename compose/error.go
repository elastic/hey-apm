package compose

var SingleError = []byte(`
{"context":
	{"custom":{},
	"request":{"body":null,
				"cookies":{},
				"env":{"REMOTE_ADDR":"127.0.0.1","SERVER_NAME":"1.0.0.127.in-addr.arpa","SERVER_PORT":"8000"},
				"headers":{"accept":"*/*","accept-encoding":"gzip, deflate, br","accept-language":"en-US,en;q=0.9","connection":"keep-alive","content-length":"","content-type":"text/plain","host":"localhost:8000","referer":"http://localhost:8000/dashboard","user-agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36"},
				"method":"GET",
				"socket":{"encrypted":false,"remote_address":"127.0.0.1"},
				"url":{"full":"http://localhost:8000/api/stats","hostname":"localhost","pathname":"/api/stats","port":"8000","protocol":"http:"}
				},
	"user":{"id":null,"is_authenticated":false,"username":""}
	},
"culprit":"opbeans.views.stats",
"exception":
	{"message":"ConnectionError: Error 61 connecting to localhost:6379. Connection refused.",
		"module":"redis.exceptions",
		"stacktrace":[],
 		"type":"ConnectionError",
		"handled":false
	},

"id":"e99fd5d7-516f-422d-a6fe-3550a49283e0",
"transaction":{"id":"87d45146-e0ce-4a04-877c-a672921df059"}}
`)
