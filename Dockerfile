FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o simple-wol .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /build/simple-wol /usr/local/bin/simple-wol
RUN mkdir -p /data && chown appuser:appgroup /data
ENV DATA_DIR=/data
EXPOSE 8080
USER appuser
CMD ["simple-wol"]
