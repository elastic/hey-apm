package compose

var Spans = [][]byte{
	[]byte(`{"trace_id":"fdedef0123456789abcdef9876543210","parent_id":"abcdef0123456789","id":"abcdef01234567","transaction_id":"01af25874dec69dd","stacktrace":[],"name":"get/api/types","type":"request","start":null,"duration":141.581,"timestamp":1532976822281000}`),
	[]byte(`{"trace_id":"abcdef0123456789abcdef9876543210","parent_id":"0000000011111111","id":"1234abcdef567895","transaction_id":"ab45781d265894fe","stacktrace":[],"name":"get/api/types","type":"request","start":22,"duration":32.592981,"timestamp":1532976822281000}
`),
[]byte(`{"trace_id":"abcdef0123456789abcdef9876543210","parent_id":"abcdefabcdef7890","id":"0123456a89012345","transaction_id":"ab23456a89012345","name":"get/api/types","type":"request","start":1.845,"duration":3.5642981,"stacktrace":[],"context":{"tags":{"tag1":"value1"}}}
`),
[]byte(`{"trace_id":"abcdef0123456789abcdef9876543210","parent_id":"ababcdcdefefabde","id":"abcde56a89012345","transaction_id":"bed3456a89012345","name":"get/api/types","type":"request","start":0,"duration":13.9802981,"stacktrace":[],"context":null}
`),
[]byte(`{"trace_id":"abcdef0123456789abcdef9876543210","parent_id":"abcdef0123456789","id":"1234567890aaaade","transaction_id":"aff4567890aaaade","name":"SELECTFROMproduct_types","type":"db.postgresql.query","start":2.83092,"duration":3.781912,"stacktrace":[{"filename":"net.js","lineno":547},{"filename":"file2.js","lineno":12,"post_context":["ins.currentTransaction=prev","}"]},{"function":"onread","abs_path":"net.js","filename":"net.js","lineno":547,"library_frame":true,"vars":{"key":"value"},"module":"somemodule","colno":4,"context_line":"line3","pre_context":["vartrans=this.currentTransaction",""],"post_context":["ins.currentTransaction=prev","returnresult"]}],"context":{"db":{"instance":"customers","statement":"SELECT*FROMproduct_typesWHEREuser_id=?","type":"sql","user":"readonly_user"},"http":{"url":"http://localhost:8000","status_code":200,"method":"GET"}}}
`),
[]byte(`{"trace_id":"a7cdef0123456789abbbef9876543210","parent_id":"1bc77f0123456789","id":"123a767890aaaade","transaction_id":"aff1267890aa7ade","name":"SELECT FROM * ","type":"db.postgresql.query","start":52.83092,"duration":3.781212,"stacktrace":[],"context":{"db":{"instance":"oioisdjfoi sdfcustomers","statement":"SELECT FROM * HEREuser_id=?","type":"sql","user":"readonly useras"},"http":{"url":"http://localhost:8000","status_code":200,"method":"GET"},"tags":{"span_tag":"somsdfsfsadfsa fsdfdsafething","1san_tag":"somethingsadfsfwa mwosadfosdf","san_tag":"someth ingsfdsf","sp3a_tag":"somethin ga sdfsfsdfasdfsd","sp4n_tag":"somethasasdf sdfasdfing","sp5_tag":"somethaas2 dfsf121 f1sdfdsfs ding","spa6tag":"sometas dfdsfdsfsd fsdhing","spa7ag":"somethsds asdfdsfing","sp8g":"sodsdfsdfing"}}}
`),
[]byte(`{"trace_id":"abcdef0123456789abbbef9876543210","parent_id":"1bcdef0123456789","id":"123aa67890aaaade","transaction_id":"aff1267890aaaade","name":"SELECTFROMproduct_types","type":"db.postgresql.query","start":2.83092,"duration":3.781912,"stacktrace":[],"context":{"db":{"instance":"oioisdjfoisfojsdfsdfcustomers","statement":"SELECT*FROMproducsfsdfsaadfsfsfsfdsft_typesWHEREuser_id=?","type":"sql","user":"readonly_useras"},"http":{"url":"http://localhost:8000","status_code":200,"method":"GET"},"tags":{"span_tag":"somsdfsfsadfsafsdfdsafething","1span_tag":"somethingsadfsfwamwosadfosdf","s2pan_tag":"somethingsfdsf","sp3an_tag":"somethingasdfsfsdfasdfsd","spa4n_tag":"somethasasdfsdfasdfing","span5_tag":"somethaas2dfsf121f1sdfdsfsding","span_6tag":"sometasdfdsfdsfsdfsdhing","span_t7ag":"somethsdfasdfdsfing","spantag":"tag"}}}
`),
}


