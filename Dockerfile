FROM golang:1.25 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway    ./services/gateway/cmd && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/auth        ./services/auth/cmd && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/vitamins    ./services/vitamins/cmd && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/analytics   ./services/analytics/cmd && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/notifier    ./services/notifier/cmd

FROM alpine:3
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/ .
EXPOSE 8080
CMD ["./gateway"]
