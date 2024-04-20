FROM golang:1.13.3-alpine3.10

WORKDIR /go/src/github.com/dmowcomber/go-epaper-demo
COPY . /go/src/github.com/dmowcomber/go-epaper-demo
RUN go install -mod=vendor ./examples/image

CMD "go-epaper-demo"

