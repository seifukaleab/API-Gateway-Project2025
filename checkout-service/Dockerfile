# Use the official Golang image as a base image
FROM golang:1.24.2

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum to download dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code into the container
COPY . .

# Build the Go application
RUN go build -o main .

# Expose port 3005 to allow communication with the service
EXPOSE 3005

# Command to run the executable when the container starts
CMD ["./main"]
