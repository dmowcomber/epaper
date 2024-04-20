FROM golang:1.13.3-alpine3.10

WORKDIR /go/src/github.com/dmowcomber/epaper
COPY . /go/src/github.com/dmowcomber/epaper
RUN go install -mod=vendor ./examples/image

CMD "epaper"

