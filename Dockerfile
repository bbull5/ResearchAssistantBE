# Use the offical Golang image to create a build artifact
FROM golang:1.20 as builder

# Set the CWD inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app
RUN GOOS=linux GOARCH=amd64 go build -o main main.go

# Use the official AWS Lambda base image for Go
FROM Public.ecr.aws/lambda/go:latest

# Copy the build artifact
COPY --from=builder /app/main ${LAMBDA_TASK_ROOT}

# Command to run the Lambda function
CMD [ "main" ]
