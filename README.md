## OpenSlides Websocket Test

Go program, that tests parallel websocket connections to OpenSlides.

## Installation

You need ```go``` to install oswstest. If you have it, just call
```
go get github.com/openslides/openslides-performance/cmd/performance
```

Afterwards, you can start the script with ```oswstest```

Currently, the only way to configure oswstest is by changing the
constants and variables in the file ```config.go```. Therefore
you should clone this repository, change the file, compile and
run oswstest with

```
go build ./cmd/performance && ./performance
```

## License

MIT
