# Shared platform packages

Packages under `admin-backend/platform/` are the cross-backend contract layer used by **admin-backend**, **client-backend**, and integration tests.

| Package | Role |
|---------|------|
| `apiresponse` | Single JSON envelope (`success`, `message`, `error`, `data`) for handlers, middleware, and validation errors |
| `validate` | `BindAndValidate`, query/path helpers, Chapa tx_ref resolution, semantic checks |
| `logger` | Structured `slog` setup and request-scoped context |
| `env`, `cache`, `database`, `worker`, `logsafe` | Infrastructure helpers |

Client-backend imports these via the `github.com/bid-hub/admin-backend` module (`replace` in `go.mod`).

**Handler conventions**

- JSON responses: admin handlers use `adapters/handler/response` only (`Success`, `Error`, `Fail`). Client handlers use `handler.WriteSuccess`, `WriteError`, `WriteFail` (or the `Handler` receiver aliases).
- Bodies: `validate.BindAndValidate` for JSON POST/PUT.
- Query: `validate.BindPagination`, `BindLimit`, or `BindOffsetPagination` for lists; `validate.BindQuery` for richer filter DTOs.
- Path: `validate.ParamUUID` / `ParamString` (admin: `http.PathParamUUID` wrapper).
