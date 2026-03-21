FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o simple-wol .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /build/simple-wol /usr/local/bin/simple-wol
ENV DATA_DIR=/data
EXPOSE 8080
CMD ["simple-wol"]
