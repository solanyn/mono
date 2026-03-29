FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /lake-ingest ./cmd/ingest/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /lake-ingest /lake-ingest
EXPOSE 8081
ENTRYPOINT ["/lake-ingest"]
