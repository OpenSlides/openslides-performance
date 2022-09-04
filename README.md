# OpenSlides performance

Tool to test the limits of OpenSlides.


## Install

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

## Docker

You can run the command with docker:

```
docker build . -t openslides-performance
docker run --network host openslides-performance
```
