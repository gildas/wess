FROM golang:1.20 as builder

# Copy the source code
WORKDIR /app
COPY . .

# Cache the dependencies
RUN go mod download

# Build the binary
RUN GIT_TERMINAL_PROMPT=1 \
    CGO_ENABLED=0 \
	GOOS=linux \
	go build -o main .

# ---
FROM alpine:3.18 as system

# Create a harmless user
RUN adduser -D -g '' docker

# ---
FROM scratch

LABEL org.opencontainers.image.title="Sample Web Server with WESS"
LABEL org.opencontainers.image.description="Demo of WEb Simple Server"
LABEL org.opencontainers.image.vendor="Gildas Cherruel"
LABEL org.opencontainers.image.authors="Gildas Cherruel <gildas@breizh.org>"

# set our environment variables
ARG PROBE_PORT=
ENV PROBE_PORT ${PROBE_PORT}
ARG TRACE_PROBE=
ENV TRACE_PROBE ${TRACE_PROBE}
ARG PORT=3000
ENV PORT ${PORT}

# Expose web server port
EXPOSE ${PORT}

# Add useful files from the system
COPY --from=system /etc/passwd /etc/passwd

# Copy the binary
WORKDIR /
COPY --from=builder /app/main .

# Run as a non-root user
USER docker

# Run the web server
ENTRYPOINT ["/main"]