package compose

var SingleSpan = []byte(`{"trace_id": "abcdef0123456789abcdef9876543210", "parent_id": "abcdef0123456789", "id": "1234567890aaaade", "transaction_id": "aff4567890aaaade", "name": "SELECT FROM product_types", "type": "db.postgresql.query", "start": 2.83092, "duration": 3.781912, "stacktrace": [], "context": { "db": { "instance": "customers", "statement": "SELECT * FROM product_types WHERE user_id=?", "type": "sql", "user": "readonly_user" }, "http": { "url": "http://localhost:8000" } } }`)
