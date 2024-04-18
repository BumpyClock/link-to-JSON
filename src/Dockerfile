# Use the official Golang image to create a build artifact.
# This image is based on Debian and includes the Go development environment.
FROM golang:1.22.2 as builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app as a static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o link2json .

# Use a Docker multi-stage build to minimize the size of the final image by using a minimal base image
FROM alpine:latest  

# Add ca-certificates in case you need HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage and .env file
COPY --from=builder /app/link2json .
COPY --from=builder /app/.env .

# Expose port 8080 to the outside world
EXPOSE 80

# Command to run the executable
CMD ["./link2json"]
