FROM golang:1.15.5 as build-env
RUN mkdir $GOPATH/src/go-dynamic-fetch
RUN mkdir $GOPATH/src/go-dynamic-fetch/cmd
RUN mkdir $GOPATH/src/go-dynamic-fetch/fetcher
WORKDIR $GOPATH/src/go-dynamic-fetch
COPY go.mod .
COPY go.sum .
COPY main.go .
COPY cmd/* ./cmd/
COPY fetcher/* ./fetcher/
RUN go mod download
RUN go install
RUN go install github.com/suntong/cascadia

FROM chromedp/headless-shell:latest
WORKDIR /
RUN mkdir go
COPY --from=build-env /go go
ENV PATH="/go/bin:$PATH"