FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup 

COPY go.mod go.sum ./
RUN go mod download 
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /watcher .


FROM scratch

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

WORKDIR /home/appuser

COPY --chown=appuser:appgroup --from=builder /watcher .
COPY --chown=appuser:appgroup config.yaml .
COPY --chown=appuser:appgroup known_hosts .ssh/known_hosts

USER appuser

ENTRYPOINT ["./watcher"]
