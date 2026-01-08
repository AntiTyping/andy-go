# Dockerfile for Go compiler development
# Build: docker build -t go-dev .
# Run:   docker run -it --rm -v $(pwd):/go-dev go-dev

FROM golang:1.24-bookworm

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    && rm -rf /var/lib/apt/lists/*

# Set up environment for building Go from source
ENV GOROOT_BOOTSTRAP=/usr/local/go
ENV PATH="/go-dev/bin:${PATH}"

# Working directory for Go source
WORKDIR /go-dev

# Default command: build and test
CMD ["bash", "-c", "./make.bash && echo 'Build successful!' && ./bin/go version"]
