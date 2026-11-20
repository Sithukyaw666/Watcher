# API Reference

Watcher exposes a REST and SSE API on `http://localhost:8080` (default). All responses are JSON unless noted otherwise.

---

## REST Endpoints

### `GET /api/history`

Returns the list of all past deployments.

**Response**

```json
[
  {
    "id": "abc123",
    "commit": "a1b2c3d",
    "message": "chore: bump nginx to 1.25",
    "author": "Jane Doe",
    "status": "success",
    "timestamp": "2024-11-01T12:00:00Z"
  }
]
```

---

### `GET /api/current_deployment`

Returns the last successful (stable) deployment snapshot.

**Response**

```json
{
  "commit": "a1b2c3d",
  "message": "chore: bump nginx to 1.25",
  "composeSnapshot": "..."
}
```

---

### `GET /api/history/view`

Returns the exact Docker Compose YAML that was deployed for a specific commit.

**Query Parameters**

| Parameter | Description |
|---|---|
| `hash` | Full or short commit SHA |

**Example**

```
GET /api/history/view?hash=a1b2c3d
```

**Response**: `text/plain` — the raw YAML content.

---

### `GET /api/graph`

Returns the current service dependency graph and live status of each service.

**Response**

```json
{
  "services": [
    {
      "name": "api",
      "status": "running",
      "dependsOn": ["db", "cache"]
    }
  ]
}
```

---

## SSE Streams

Server-Sent Events (SSE) streams deliver real-time data. Connect with any SSE-capable client or `EventSource` in the browser.

### `GET /api/stream/metrics`

Live CPU and memory metrics for a container.

**Query Parameters**

| Parameter | Description |
|---|---|
| `service` | Service name as defined in the Compose file |

**Example**

```
GET /api/stream/metrics?service=api
```

**Event payload**

```json
{
  "cpu_percent": 12.4,
  "memory_usage_mb": 128.0,
  "memory_limit_mb": 512.0
}
```

---

### `GET /api/stream/logs`

Live tail of `stdout` and `stderr` for a container.

**Query Parameters**

| Parameter | Description |
|---|---|
| `service` | Service name as defined in the Compose file |

**Example**

```
GET /api/stream/logs?service=api
```

Each SSE event contains one log line as plain text.

---

### `GET /api/system/events`

Real-time pulse of the GitOps reconciliation engine. Use this to build status integrations or monitor the sync loop externally.

**Event types**

| Event | Description |
|---|---|
| `syncing` | Watcher is polling the Git remote |
| `reconciling` | A new commit was detected and deployment is in progress |
| `idle` | No changes detected; loop is at rest |
| `rollback` | A failed deployment triggered an automatic rollback |

**Example (JavaScript)**

```js
const events = new EventSource('/api/system/events');
events.onmessage = (e) => {
  const { state } = JSON.parse(e.data);
  console.log('Engine state:', state);
};
```
