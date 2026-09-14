The supplied **synthetic source fixture** shows this request path:

```mermaid
flowchart LR
    Browser -->|sends request| Gateway
    Gateway -->|validates, then calls| API
    API -->|calls| Worker
    Worker -->|writes| Storage
```

So the request reaches storage in this order: **Browser → Gateway → API → Worker → Storage**. The fixture does not establish any other links or whether the calls are synchronous or queued.
