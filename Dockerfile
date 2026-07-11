# syntax=docker/dockerfile:1

FROM node:22-alpine AS editor
WORKDIR /src/web/editor
COPY web/editor/package.json web/editor/package-lock.json ./
RUN npm ci
COPY web/editor/ ./
RUN npm run build

FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=editor /src/web/static/js/editor.js web/static/js/editor.js
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /waldi ./cmd/waldi

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /waldi /app/waldi
COPY web/templates web/templates
COPY web/static web/static
COPY migrations migrations
ENV WALDI_ADDR=:8080
EXPOSE 8080
CMD ["/app/waldi", "serve"]
