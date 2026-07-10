# Build stage - Frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ .
RUN npm run build

# Build stage - Backend
FROM golang:1.22-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
# Copy frontend dist to embed directory
COPY --from=frontend-builder /app/frontend/dist ./cmd/server/web/
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o /shelly ./cmd/server

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /shelly .
COPY backend/config.yaml .
RUN mkdir -p data/recordings
EXPOSE 8080
CMD ["./shelly"]
