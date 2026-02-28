FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the sqs-consumer binary
RUN go build -o sqs-consumer ./cmd/sqs-consumer

FROM alpine:latest

# Install packages using apk with the --no-cache option
RUN apk --no-cache add curl ffmpeg imagemagick uuidgen

WORKDIR /app

# Copy the built binary from builder stage
COPY --from=builder /app/sqs-consumer .

# Copy workflow files
COPY ./examples /app/examples

# Set entrypoint
ENTRYPOINT ["./sqs-consumer"]

