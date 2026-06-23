FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod ./
COPY . .
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/digital-twin ./cmd/server

FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/digital-twin /app/digital-twin
COPY configs /app/configs
COPY web /app/web
RUN addgroup -S digitaltwin && adduser -S digitaltwin -G digitaltwin && mkdir -p /data/admin && chown -R digitaltwin:digitaltwin /data
USER digitaltwin

ENV DIGITAL_TWIN_CONFIG=/app/configs/app.yaml
ENV DIGITAL_TWIN_ADMIN_DATA=/data/admin
EXPOSE 8080

ENTRYPOINT ["/app/digital-twin"]
