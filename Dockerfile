FROM alpine:3.19

RUN apk --no-cache add ca-certificates

# BINARY_NAME will be passed as a build argument by GoReleaser
ARG BINARY_NAME

# Copy the binary from the build context
COPY ${BINARY_NAME} /usr/local/bin/app

# Default application port (Dewy will forward host port to this container port)
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/app"]
