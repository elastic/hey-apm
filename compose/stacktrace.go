package compose

var StacktraceFrame = []byte(`
{"abs_path":"/opbeans/lib/python3.6/site-packages/redis/client.py","context_line":"            connection.send_command(*args)","filename":"redis/client.py","function":"execute_command","library_frame":true,"lineno":673,"module":"redis.client","post_context":["            return self.parse_response(connection, command_name, **options)","        finally:"],"pre_context":["            connection.disconnect()","            if not connection.retry_on_timeout and isinstance(e, TimeoutError):","                raise"],"vars":{"args":["GET","cache:1:shop-stats"],"command_name":"GET","connection":"Connection\u003chost=localhost,port=6379,db=1\u003e","options":{},"pool":"ConnectionPool\u003cConnection\u003chost=localhost,port=6379,db=1\u003e\u003e","self":"StrictRedis\u003cConnectionPool\u003cConnection\u003chost=localhost,port=6379,db=1\u003e\u003e\u003e"}}
`)
