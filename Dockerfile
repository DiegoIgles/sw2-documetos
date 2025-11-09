# syntax=docker/dockerfile:1.6
FROM golang:1.22-alpine AS build
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /bin/server ./main.go

FROM gcr.io/distroless/base-debian12
WORKDIR /app
ENV PORT=8081
COPY --from=build /bin/server /app/server
COPY openapi.json /app/openapi.json
EXPOSE 8081
USER 65532:65532
ENTRYPOINT ["/app/server"]
