FROM golang:1.19.0-alpine as builder
WORKDIR /root/

RUN apk add git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build


# Productive build
FROM scratch

LABEL org.opencontainers.image.title="OpenSlides Performance Service"
LABEL org.opencontainers.image.description="Tool to test the performance of OpenSlides."
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/OpenSlides/openslides-performance"

COPY --from=builder /root/openslides-performance .

ENTRYPOINT ["/openslides-performance"]
