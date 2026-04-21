FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/wdw-fleet ./cmd/wdw-fleet

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/wdw-fleet /usr/local/bin/wdw-fleet
COPY migrations /migrations

EXPOSE 8080
ENTRYPOINT ["wdw-fleet"]
CMD ["serve"]
