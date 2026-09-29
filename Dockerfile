FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /openstead-mcp ./cmd/openstead-mcp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /openstead-mcp /openstead-mcp
ENTRYPOINT ["/openstead-mcp"]
