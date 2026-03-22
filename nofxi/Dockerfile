# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /nofxi ./nofxi/cmd/nofxi/

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /nofxi .
COPY nofxi/config.example.yaml ./config.example.yaml

# Create data directory for SQLite
RUN mkdir -p /app/data

ENV TZ=Asia/Shanghai

EXPOSE 8900

ENTRYPOINT ["./nofxi"]
CMD ["--config", "/app/config.yaml"]
