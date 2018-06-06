package api

/*
Package `api` provides most of the read-eval-print-loop (repl) functionality that hey-apm needs to be executed in
interactive shell mode.

This includes a subpackage `io` with functions that do not necessarily are IO bound, but they are meant to be used
in a IO context, for example the `read` and `print` parts of a repl.

It does not expose an `eval` function as such, but most of the functions that hey-apm can perform. Some of them assume
that data has been parsed already, and all of them return at least some printable data (usually strings) describing
the outcome of the function. In this sense, there isn't a strong separation between read and eval, ie. there isn't an
intermediate data structure.
Some functions return 2 arguments, the second one representing a side effect to be performed.

Functions in this package do never modify state, instead this package exposes an `state` interface that must be
implemented and managed externally.
This package generally tries to make few assumptions. This is not always the case, eg. the `LoadTest` function requires
a running apm-server process and a healthy elastic search node.
*/
