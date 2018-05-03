package server

/*
Package `server` allows hey-apm to be executed in interactive shell mode. In interactive shell mode users connect
to a server listening on port 8234. The server then can read commands from the incoming connection, evaluate them,
and print (and save) the results.

Some commands are potentially long-running, and for user convenience they are executed in the background without
blocking the shell. To safeguard consistency only one of such commands can run at a time.
Commands sent before the current one completes are queued.
All commands running in the background are evaluated in the `client` package.
Server and client communicate trough a stateful connection that wraps the underlying tcp connection with some Golang
async primitives.

Most of the functionality that the user cares about is encapsulated in the `api` package. The evaluation functions are
responsible for calling the right `api` function.

For usage instructions see the readme file.

hey-apm doesn't try to protect itself from malicious users.
*/
