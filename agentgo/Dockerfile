# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -ldflags="-s -w" -o /server ./cmd/server/

# Stage 2: Runtime (golang:1.25-alpine - provides shell + wget for healthcheck)
FROM golang:1.25-alpine

COPY --from=builder /server /server

# Copy prompt templates (required at runtime)
COPY --from=builder /app/prompts ./prompts/

# Copy skills so the agent can find them at ./skills/ and ./project-skills/
COPY --from=builder /app/skills ./skills/
COPY --from=builder /app/project-skills ./project-skills/

EXPOSE 8080

ENV PORT=8080

CMD ["/server"]
