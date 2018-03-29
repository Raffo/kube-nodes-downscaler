# builder image
FROM golang as builder

RUN go get -u github.com/golang/dep/cmd/dep
WORKDIR /go/src/github.com/Raffo/kube-nodes-downscaler
COPY . .
RUN dep ensure
RUN CGO_ENABLED=0 GOOS=linux go build . 

# final image
FROM alpine

COPY --from=builder /go/src/github.com/Raffo/kube-nodes-downscaler/kube-nodes-downscaler /bin/kube-nodes-downscaler

RUN apk add -U ca-certificates

ENTRYPOINT ["/bin/kube-nodes-downscaler"]