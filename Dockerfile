FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/shortener ./cmd/shortener

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/shortener /shortener
EXPOSE 8080
ENTRYPOINT ["/shortener"]
