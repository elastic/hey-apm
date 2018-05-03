package client

/*
Package `client` implements the state interface required by the `api` package and the functions that modify the
state as a side effect. Such state describes an evaluation environment.

An evaluation environment models everything that affects the result of a command, other than the command itself.
That includes external dependencies like elasticsearch connections, apm server processes, and user specific data.

Each incoming connection will have its own evaluation environment.

`EvalAndUpdate` makes sure that the evaluation environment is always consistent, eg. a function which needs read access
to the directory of the apm-server will fail graciously if that directory doesn't exist or is not known.
Commands then depend on the result of previously executed commands.

The evaluation environment implemented in this package makes strong assumptions about the system it runs on,
namely r/w access to home directory, posix, git & go installed and globally available, and a docker daemon running.

`client.Connection` wraps a tcp connection and defines the client-server communication minutiae (client-server is an
implementation abstraction, all the code is executed in the server).
*/
