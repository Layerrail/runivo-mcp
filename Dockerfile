FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /runivo-mcp ./cmd/runivo-mcp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /runivo-mcp /runivo-mcp
ENTRYPOINT ["/runivo-mcp"]
