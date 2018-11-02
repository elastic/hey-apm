package compose

var f1 = []byte(`{"function":"onread","abs_path":"net.js","filename":"net.js","lineno":547,"library_frame":true,"vars":{"key":"value"},"module":"somemodule","colno":4,"context_line":"line3","pre_context":["vartrans=this.currentTransaction",""],"post_context":["ins.currentTransaction=prev","returnresult"]}`)

var f2 = []byte(`{"abs_path":"/real/file/name.py","filename":"file/name.py","function":"foo","vars":{"key":"value"},"pre_context":["line1","line2"],"context_line":"line3","library_frame":true,"lineno":3,"module":"App::MyModule","colno":4,"post_context":["line4","line5"]}`)
var f3 = []byte(`{"filename":"lib/instrumentation/index.js","lineno":102,"function":"instrumented","abs_path":"/Users/watson/code/node_modules/elastic/lib/instrumentation/index.js","vars":{"key":"value"},"pre_context":["vartrans=this.currentTransaction","","returninstrumented","","functioninstrumented(){","varprev=ins.currentTransaction","ins.currentTransaction=trans"],"context_line":"varresult=original.apply(this,arguments)","post_context":["ins.currentTransaction=prev","returnresult","}","}","","Instrumentation.prototype._recoverTransaction=function(trans){","if(this.currentTransaction===trans)return"]}`)
var f4 = []byte(`{"abs_path":"http://localhost:8000/test/../test/e2e/general-usecase/bundle.js.map","filename":"test/e2e/general-usecase/bundle.js.map","function":"<anonymous>","library_frame":true,"lineno":1,"colno":18}`)
var f5 = []byte(`{"abs_path":"http://localhost:8000/test/./e2e/general-usecase/bundle.js.map","filename":"~/test/e2e/general-usecase/bundle.js.map","function":"invokeTask","library_frame":false,"lineno":1,"colno":181}`)
var f6 = []byte(`{"abs_path":"http://localhost:8000/test/e2e/general-usecase/bundle.js.map","filename":"~/test/e2e/general-usecase/bundle.js.map","function":"runTask","lineno":1,"colno":15}`)
var f7 = []byte(`{"abs_path":"http://localhost:8000/test/e2e/general-usecase/bundle.js.map","filename":"~/test/e2e/general-usecase/bundle.js.map","function":"invoke","lineno":1,"colno":199}`)
var f8 = []byte(`{"abs_path":"http://localhost:8000/test/e2e/general-usecase/bundle.js.map","filename":"~/test/e2e/general-usecase/bundle.js.map","function":"timer","lineno":1,"colno":33}`)

var f9 = []byte(`{"function":"thisisatestfunc","abs_path":"/tmp/os/pppppp/eeeeeeee","filename":"net-apm-test.js","lineno":18976,"library_frame":false,"vars":{"ke923483948394832942849y1":"valuasdofsk1234sdffeabs123","ke923483948394832942849y2":"valuasdofsk1234sdffe7897897987","ke923483948394832942849y3":"valuasdofsk1234sdffe21168451121","ke923483948394832942849y4":"valuasdofsk1234sdffe","ke923483948394832942849y5":"valuasdofsk1234sdffepskfsdmfwfklmfkwmlsdm","ke923483948394832942849y6":"valuasdofsk1234sdffe09v0skvk90isfsokfsdo","ke923483948394832942849y11":"valasdofsk1234sdffuedofks0fksfo","ke923483948394832942849y21":"valasdofsk1234sdffuemasofs0fs","ke923483948394832942849y31":"valasdofsk1234sdffuespfkskfss","ke923483948394832942849y41":"valasdofsk1234sdffuesdfssfdsdffsfsdfs","ke923483948394832942849y51":"valasdofsk1234sdffue2253teadf3q4twagfdv3q4argzv","ke923483948394832942849y61":"valasdofsk1234sdffue2qweasddfzxfwaesfd","ke923483948394832942849y12":"valasdofsk1234sdffuewafsdfdsaf","ke923483948394832942849y23":"valasdofsk1234sdffuesdfsdfsdafsdf","ke923483948394832942849y33":"valasdofsk1234sdffue32f61s5d6f1sf","ke923483948394832942849y43":"valasdofsk1234sdffusdfsdfsfsdfsde","ke923483948394832942849y53":"valasdofsk1234sdffuesdfdsf","ke923483948394832942849y63":"valasdofsk1234sdffuesdfsd","ke923483948394832942849y73":"valasdofsk1234sdffueadsf1fds2f1sd"},"module":"kldsfkdslskdfpldskdssdflsdifoafksdksdfksfsdfsaf","colno":48,"context_line":"line31f211sdfs1","pre_context":["vartrans=this.currentTransaction","varsdfsdfd=ssdfdfsdfthisds.fsdcurrentTransaction",""],"post_context":["ins.csdfurrentTransactionsdfsfsdf=prev","insss.cdsfsdurrentTransaction=prev","returnresult","}"]}`)

var f10 = []byte(`{"function":"this is a test func","abs_path":"/tmp/os/pppppp/ui","filename":"net-apm-test.js","lineno":18976,"library_frame":false,"vars":{"key1":"valueabs123","key2":"value7897897987","key3":"value21168451121","key4":"value","key5":"valuepskfsdmfwfklmfkwmlsdm","key6":"value09v0skvk90isfsokfsdo","key11":"valuedofks0fksfo","key21":"valuemasofs0fs","key31":"valuespfkskfss","key41":"valuesdfssfdsdffsfsdfs","key51":"value2253teadf3q4twagfdv3q4argzv","key61":"value2qweasddfzxfwaesfd","key12":"valuewafsdfdsaf","key23":"valuesdfsdfsdafsdf","key33":"value32f61s5d6f1sf","key43":"valusdfsdfsfsdfsde","key53":"valuesdfdsf","key63":"valuesdfsd","key73":"valueadsf1fds2f1sd"},"module":"kldsfkdslskdfpldskdssdflafksdksdfksfsdfsaf","colno":4,"context_line":"line31f2111","pre_context":["vartrans=this.currentTransaction",""],"post_context":["ins.currentTransactionsdfsfsdf=prev"]}`)

var StacktraceFrames = [][]byte{f1, f2, f3, f4, f5, f6, f7, f8, f9, f10}
