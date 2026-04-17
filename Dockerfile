# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build
# Output lands in /app/webdist/dist/ (per vite.config.ts outDir)

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY webdist/webdist.go webdist/webdist.go
COPY webdist/dist/placeholder.txt webdist/dist/placeholder.txt
COPY --from=frontend /app/webdist/dist/ webdist/dist/
COPY data/enable.txt data/enable.txt
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Stage 3: Minimal runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /server /server
COPY --from=backend /app/data/enable.txt /data/enable.txt
ENTRYPOINT ["/server"]
CMD ["-addr", "0.0.0.0:8080", "-dict", "/data/enable.txt", "-dev-login=false"]
