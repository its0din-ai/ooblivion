# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ooblivion ./cmd/ooblivion

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 oob
WORKDIR /app
COPY --from=build /out/ooblivion /app/ooblivion
RUN mkdir -p /app/data && chown -R oob:oob /app
USER oob
EXPOSE 8080
ENTRYPOINT ["/app/ooblivion"]
