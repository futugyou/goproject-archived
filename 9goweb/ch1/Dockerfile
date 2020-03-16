FROM golang:alpine AS development
ENV GO111MODULE=off
ENV GO15VENDOREXPERIMENT=1
COPY . $GOPATH/src/mypackage/myapp/
WORKDIR $GOPATH/src/mypackage/myapp/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go get -d -v
RUN go build -o /go/bin/helloapp
FROM scratch
COPY --from=development /go/bin/helloapp /go/bin/helloapp
EXPOSE 8080
ENTRYPOINT ["/go/bin/helloapp"]