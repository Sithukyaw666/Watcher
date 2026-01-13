FROM golang:1.23.2-alpine AS builder

WORKDIR /app


COPY go.mod go.sum ./
RUN go mod download 
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /watcher .


FROM scratch


WORKDIR /home/appuser

COPY --from=builder /watcher .
COPY config.yaml .

ENTRYPOINT ["./watcher"]
