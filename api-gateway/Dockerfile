# Use the official Golang image as a base image
FROM golang:1.24.2

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum to download dependencies first
# This leverages Docker's layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code into the container
COPY . .

# Build the Go application
# -o main specifies the output file name
RUN go build -o main .

# Expose port 8080 to allow communication with the service
EXPOSE 8080

# Command to run the executable when the container starts
CMD ["./main"]