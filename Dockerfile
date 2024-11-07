# Start from the latest golang base image
FROM golang:1.22-alpine as builder

ARG version
ARG commit
ARG buildTime

# Create and change to the app directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code
COPY . .

# Build the Go app
RUN go build -ldflags="-X 'main.version=${version}' -X 'main.commit=${commit}' -X 'main.buildTime=${buildTime}'" -o signal

# Second stage: creating a small image for final output
FROM scratch

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/signal .

# Command to run the executable
CMD ["./signal"]