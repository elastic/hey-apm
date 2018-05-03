package io

import (
	"io"
	"math/rand"
	"strings"

	"github.com/elastic/hey-apm/server/strcoll"
)

// ANSI color codes
const (
	Red     = "\x1b[31m"
	Green   = "\x1b[32m"
	Yellow  = "\x1b[33m"
	Magenta = "\x1b[35m"
	Cyan    = "\x1b[36m"
	Grey    = "\x1b[37m"
)

func Reply(w io.Writer, msg ...string) bool {
	if strcoll.Nth(0, msg) != "" {
		w.Write([]byte(strings.Join(msg, "\n")))
		return true
	}
	return false
}

func ReplyNL(w io.Writer, msg ...string) bool {
	if Reply(w, msg...) {
		return Reply(w, "\n")
	}
	return false
}

func ReplyEither(w io.Writer, err error, msg ...string) bool {
	if err != nil {
		return Reply(w, Red+strings.TrimSpace(err.Error()))
	} else {
		for i, line := range msg {
			words := strings.Split(line, " ")
			for j, word := range words {
				// ad-hoc for elastic search health status
				var color string
				switch word {
				case "yellow":
					color = Yellow
				case "green":
					color = Green
				case "red":
					color = Red
				}
				if color != "" {
					words[j] = color + word + Grey
					msg[i] = strings.Join(words, " ")
				}
			}
		}
		return Reply(w, msg...)
	}
}

func ReplyEitherNL(w io.Writer, err error, msg ...string) {
	if ReplyEither(w, err, msg...) {
		Reply(w, "\n")
	}
}

func Prompt(w io.Writer) {
	Reply(w, Cyan+">>> ")
}

// provides a visual cue to commands executed in the system. also feels so 80's
func ReplyWithDots(w io.Writer, args ...string) {
	dots := make([]byte, rand.Intn(32)+4)
	for i := range dots {
		dots[i] = '.'
	}
	ReplyNL(w, Magenta+string(dots)+strings.Join(args, " ")+Grey)
}
