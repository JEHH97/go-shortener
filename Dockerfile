FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/shortener ./cmd/shortener

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
WORKDIR /
COPY --from=build /out/shortener /shortener
LABEL org.opencontainers.image.source="https://github.com/JEHH97/go-shortener" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.title="Go Shortener" \
      org.opencontainers.image.description="A non-root URL shortener API with PostgreSQL, Redis, and aggregate click analytics."
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD ["/shortener", "healthcheck"]
ENTRYPOINT ["/shortener"]
