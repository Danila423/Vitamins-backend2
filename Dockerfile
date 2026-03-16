FROM golang:1.23 AS build

WORKDIR /app

# 1. Кладём только модули
COPY go.mod go.sum ./
RUN go mod download

# 2. Кладём весь код
COPY . .

# 3. Собираем
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api

FROM alpine:3
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /app/api .
EXPOSE 8080
CMD ["./api"]
