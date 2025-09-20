# syntax=docker/dockerfile:1.4

FROM node:18-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json ./
RUN --mount=type=cache,target=/root/.npm npm install
COPY frontend .
RUN npm run build

FROM golang:1.21-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache build-base sqlite-dev
COPY backend ./backend
COPY --from=frontend-builder /app/frontend/dist ./frontend_dist
RUN set -eux; \
    rm -rf backend/frontend_dist; \
    mkdir -p backend/frontend_dist; \
    cp -r frontend_dist/. backend/frontend_dist/; \
    cd backend; \
    CGO_ENABLED=1 GOOS=linux go build -o kup-piksel .

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache sqlite-libs
COPY --from=backend-builder /app/backend/kup-piksel ./kup-piksel
COPY backend/config.example.json ./config.json
EXPOSE 3000
CMD ["./kup-piksel"]
