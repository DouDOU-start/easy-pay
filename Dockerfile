FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/easypay ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /out/easypay /app/easypay
COPY configs /app/configs
EXPOSE 8080
ENTRYPOINT ["/app/easypay", "--config", "/app/configs/config.yaml"]
