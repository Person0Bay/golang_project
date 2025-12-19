FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod из корня
COPY go.mod go.sum ./
RUN go mod download

# Копируем ВЕСЬ исходный код
COPY . .

# Билдим конкретный сервис
RUN CGO_ENABLED=0 go build -o /app/agg-svc ./agg-svc

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/agg-svc .

CMD ["./agg-svc"]