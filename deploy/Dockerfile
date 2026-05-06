FROM golang:1.22-alpine AS builder
RUN apk add --no-cache nodejs npm
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN cd web && npm install && npm run build
RUN CGO_ENABLED=0 go build -o /out/easypay ./backend/cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /out/easypay /app/easypay
COPY backend/config.yaml /app/config.yaml
EXPOSE 8080
ENTRYPOINT ["/app/easypay", "--config", "/app/config.yaml"]
