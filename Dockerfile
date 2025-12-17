FROM golang:1.26-rc-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker's build cache
# This layer is only rebuilt if the dependencies change
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application binary
# CGO_ENABLED=0 ensures a statically linked binary that doesn't need C libraries at runtime
RUN CGO_ENABLED=0 go build -o /go-app ./cmd/pulse

# --- Run Stage ---
# Start from scratch for the smallest possible final image
FROM scratch AS final

# Copy only the compiled binary from the builder stage
COPY --from=builder /go-app /go-app

# Expose the port your application listens on (optional, for documentation)
EXPOSE 8080

# Command to run the executable when the container starts
CMD ["/go-app"]
