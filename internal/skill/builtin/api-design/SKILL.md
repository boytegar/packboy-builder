---
name: api-design
description: RESTful and gRPC API design best practices: endpoint design, status codes, versioning, pagination, error handling, and authentication. Use when designing or reviewing APIs.
origin: builtin
---

# API Design

## Purpose
Design and review APIs that are consistent, predictable, and developer-friendly.

## When to use
- Designing a new REST or gRPC API
- Reviewing an existing API for consistency
- Adding endpoints to an existing API
- Designing API versioning or pagination strategy

## REST principles
- Resource-oriented URLs: `/users/{id}`, `/orders/{id}/items`
- HTTP methods map to actions: GET (read), POST (create), PUT/PATCH (update), DELETE (remove)
- Stateless server; client sends all context per request
- Standard status codes:
  - 2xx: success (200 OK, 201 Created, 204 No Content)
  - 4xx: client error (400 Bad Request, 401 Unauthorized, 403 Forbidden, 404 Not Found, 409 Conflict, 422 Unprocessable Entity)
  - 5xx: server error (500 Internal, 502 Bad Gateway, 503 Unavailable)
- Pagination: cursor-based preferred over offset for large datasets
- Consistent error format: `{"error": {"code": "...", "message": "...", "details": {...}}}`

## Versioning
- URL versioning: `/v1/users` — simplest, most cacheable
- Header versioning: `Accept: application/vnd.api+json;version=1`
- Never make breaking changes without a new version

## Authentication
- Bearer tokens for APIs: `Authorization: Bearer <token>`
- API keys for service-to-service
- OAuth2 for user-delegated access
- Never pass secrets in URL parameters (they appear in logs)

## Error handling
- Use appropriate HTTP status codes
- Provide actionable error messages
- Include a request ID for tracing
- Don't expose internal details in error messages

## Idempotency
- GET, PUT, DELETE should be idempotent
- POST is not idempotent; provide idempotency keys for payment-like operations

## Naming
- Plural nouns for collections: `/users`, not `/user`
- Lowercase, hyphen-separated: `/order-items`, not `/orderItems`
- Consistent: don't mix `/users/{id}/orders` and `/orders/user/{id}`

## gRPC specifics
- Define services in .proto files
- Use streaming for large or real-time data
- Binary protobuf for wire efficiency; JSON for human-readable

## Documentation
- OpenAPI/Swagger for REST APIs
- Generate documentation from code annotations
- Include request/response examples
- Document authentication requirements
