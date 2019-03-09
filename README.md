## OpenSlides Websocket Test

Go program, that tests parallel websocket connections to OpenSlides.

## Installation

You need `go` to install openslides-performance. If you have it, just call
```
go get github.com/openslides/openslides-performance/cmd/performance
```

## Run

After openslides-performance is installed, you can start the script
with the command `performance`

See `performance --help` see some configuration flags.

Example:

```
performance --admins 100 --users 100 --connect-test --keep-open-test
```

This will start the tests with 100 admin users and 100 non-admin users.
It will first connect all 200 clients and keep the connection open
afterwards

Some other values can be configured by changing the `config.go` file in
`pwk/oswstest/`. To change them you have to clone the repository, change
the files, compile and run the program with

```
go build ./cmd/performance && ./performance
```

## Tests

There are four tests:

### ConnectTest
`performance --connect-test`

Connects all clients. Measures the time until all clients are connected
and until they all got there first data.


### OneWriteTest
`performance --one-write-test`

Expects the first client to be an admin client and all clients
to be connected. Therefore the test requires, that the ConnectTest is run
before. This test sends one write request with the first client and
measures the time until all clients get the changed data.
oswstest.OneWriteTest,


### ManyWriteTest
`performance --many-write-test`

Expects at least one client to be an admin client and all clients
to be connected. Therefore the test requires, that the ConnectTest is run
before. This test sends one write request for each admin client and
measures the time until all write requests are send and until all data is
received.


### KeepOpenTest
`performance --keep-open-test`

Keeps the connections open. This is not usual for a testrun of this
program but can help to open a lot of connections with this tool to test
manuely how OpenSlides reacts with a lot of open connections.

### Default tests

If non test is selected, then ConnectTest, OneWriteTest and ManyWriteTest
are used.

## License

MIT
