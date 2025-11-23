FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./ 
RUN go mod download

COPY . .

# Build the application
# We disable CGO for a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o micam-go main.go

FROM alpine:latest

# Install ffmpeg
RUN apk add --no-cache ffmpeg

WORKDIR /app

COPY --from=builder /app/micam-go .

# Expose any necessary ports (though this app mainly connects outwards or pipes to local RTSP server)

CMD ["./micam-go"]
