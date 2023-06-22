# OpenSlides performance

Tool to test the limits of OpenSlides.


## Install

### From github

Binaries of the tool get be fetched from the last release:

https://github.com/OpenSlides/openslides-performance/releases


### With Docker

The tool can also be run with docker:

docker run --network=host ghcr.io/openslides/openslides-performance:latest

The argument `--network=host` is needed for the most commands to test a local
instance. For remote instances, it is not needed.


### With installed go

```
go install github.com/OpenSlides/openslides-performance@latest
```


### With checked out repo

```
go build
```


## Run

You can see the usage of the command by calling it.

```
openslides-performance
```


## Configuration

The tool is configurated with commandline arguments. See 

```
openslides-performance --help
```

for all commands and there arguments.

It is possible to set defaults via file. The file has to be called `config.json`
and has to be in pwd. The content can be any commandline argument.

For example:

```
{
    "domain": "example.com",
    "username": "admin",
    "password": 12345
}
```
