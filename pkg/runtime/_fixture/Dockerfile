# Step 1: Use an official Go image as a build environment
FROM golang:1.23 AS builder

# Step 2: Set the Current Working Directory inside the container
WORKDIR /app

# Step 3: Copy go mod and sum files to the workspace
COPY go.mod go.sum ./

# Step 4: Copy the source code into the workspace
COPY . .

# Step 5: Build the Go app
RUN go build -o main .

# Step 6: Use a smaller base image for the final container
FROM alpine:3.18

# Step 7: Set the Current Working Directory inside the container
WORKDIR /app

# Step 8: Copy the binary from the builder stage
COPY --from=builder /app/main .

# Step 9: Expose the application on port 8080
EXPOSE 8080

# Step 10: Command to run the executable
CMD ["./main"]
