FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app ./services/vitamins/cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /app /app
EXPOSE 50052
CMD ["/app"]
