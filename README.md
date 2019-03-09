## OpenSlides Websocket Test

Go program, that tests parallel websocket connections to OpenSlides.

## Installation

You need `go` to install oswstest. If you have it, just call
```
go get github.com/openslides/openslides-performance/cmd/performance
```

Afterwards, you can start the script with `performance`

See `performance --help` see some configuration flags.

Some other values can be configured by changing the `config.go` file in 
`cmd/performance/` and `pwk/oswstest/`. To change them you have to clone 
the repository, change the files, compile and run the program with

```
go build ./cmd/performance && ./performance
```

## License

MIT
