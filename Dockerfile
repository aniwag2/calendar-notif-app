# Theralert (Go rewrite) — multi-stage build.
# Stage 1 compiles Tailwind CSS and the Go binary; Stage 2 is a tiny runtime image.

FROM golang:1.25-bookworm AS build
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile Tailwind CSS with the standalone CLI (no Node runtime needed).
ARG TAILWIND_VERSION=v3.4.17
RUN curl -sSL -o /usr/local/bin/tailwindcss \
      https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-linux-x64 \
    && chmod +x /usr/local/bin/tailwindcss \
    && tailwindcss -c tailwind.theralert.config.js \
        -i web/static/css/input.css -o web/static/css/app.css --minify

# Build a static binary (templates + CSS + JS are embedded).
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /theralert ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /theralert /theralert
EXPOSE 3002
ENTRYPOINT ["/theralert"]
