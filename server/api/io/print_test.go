package io

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestReply(t *testing.T) {
	for _, test := range []struct {
		fn     func(*BufferWriter)
		output string
	}{
		{
			func(bw *BufferWriter) { Reply(bw, "") },
			"",
		},
		{
			func(bw *BufferWriter) { Reply(bw, "1") },
			"1",
		},

		{
			func(bw *BufferWriter) { Reply(bw, "1", "2", "3") },
			"1\n2\n3",
		},
		{
			func(bw *BufferWriter) { ReplyNL(bw, "1", "2", "3") },
			"1\n2\n3\n",
		},
		{
			func(bw *BufferWriter) { ReplyNL(bw, "") },
			"",
		},
		{
			func(bw *BufferWriter) { ReplyNL(bw, " ") },
			" \n",
		},
		{
			func(bw *BufferWriter) { ReplyEither(bw, errors.New("err")) },
			Red + "err",
		},
		{
			func(bw *BufferWriter) { ReplyEither(bw, errors.New("err"), "1") },
			Red + "err",
		},
		{
			func(bw *BufferWriter) { ReplyEither(bw, nil, "1") },
			"1",
		},
		{
			func(bw *BufferWriter) { ReplyEitherNL(bw, errors.New("err")) },
			Red + "err\n",
		},
		{
			func(bw *BufferWriter) { ReplyEitherNL(bw, nil, "1 2", "3") },
			"1 2\n3\n",
		},
		{
			func(bw *BufferWriter) { ReplyEitherNL(bw, nil) },
			"",
		},
		{
			func(bw *BufferWriter) { ReplyEither(bw, nil, "yellow green red") },
			Yellow + "yellow" + Grey + " " + Green + "green" + Grey + " " + Red + "red" + Grey,
		},
		{
			func(bw *BufferWriter) { Prompt(bw) },
			Cyan + ">>> ",
		},
	} {
		b := NewBufferWriter()
		test.fn(b)
		assert.Equal(t, test.output, b.String())
	}
}
