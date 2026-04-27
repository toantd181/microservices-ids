FROM golang:1.25-alpine AS builder 
WORKDIR /build 

COPY go.mod go.sum ./

RUN go mod download

RUN go mod verify

COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o gateway \
    main.go 
    
FROM scratch AS runtime
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs
COPY --from=builder /build/gateway /gateway

USER 65532:65532
EXPOSE 8080
ENTRYPOINT [ "/gateway" ]