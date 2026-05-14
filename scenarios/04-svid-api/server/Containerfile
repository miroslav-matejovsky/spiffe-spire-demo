FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /svid-server ./scenarios/04-svid-api/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser
COPY --from=builder /svid-server /usr/local/bin/svid-server
USER 10001
ENTRYPOINT ["svid-server"]
