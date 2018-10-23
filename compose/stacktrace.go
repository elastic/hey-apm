package compose

var StacktraceFrame = []byte(`{"function": "onread", "abs_path": "net.js", "filename": "net.js", "lineno": 547, "library_frame": true, "vars": {"key": "value"}, "module": "some module", "colno": 4, "context_line": "line3", "pre_context": [ "  var trans = this.currentTransaction", "" ], "post_context": [ "    ins.currentTransaction = prev", "    return result"]}`)