# builder image
FROM golang as builder

RUN go get -u github.com/golang/dep/cmd/dep
WORKDIR /go/src/github.com/Raffo/kube-nodes-downscaler
COPY . .
RUN dep ensure
RUN make build.linux

# final image
FROM alpine

COPY --from=builder /go/src/github.com/Raffo/kube-nodes-downscaler/build/linux/kube-nodes-downscaler /bin/kube-nodes-downscaler

RUN apk add -U ca-certificates

ENTRYPOINT ["/bin/kube-nodes-downscaler"]
