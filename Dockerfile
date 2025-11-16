FROM golang:1.25-alpine AS build

RUN apk add --no-cache git build-base

WORKDIR /src
COPY . .
RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app ./cmd/server

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /app /app
COPY src/migrations /migrations
EXPOSE 8080
ENTRYPOINT ["/app"]
