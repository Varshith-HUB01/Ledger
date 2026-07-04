# Stage 1: Build the binary
FROM golang:alpine AS builder
WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build a statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Stage 2: Create the lean production image
FROM alpine:latest
WORKDIR /app

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["./main"]