FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o guardit ./cmd/guardit

FROM alpine:3.19
WORKDIR /root/
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/guardit .
EXPOSE 8443
CMD ["./guardit"]