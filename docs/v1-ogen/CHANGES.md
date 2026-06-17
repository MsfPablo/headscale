# v1 API: behaviour changes (gRPC/grpc-gateway → ogen OpenAPI 3.0)

This file records every deliberate deviation of the new ogen-based v1 HTTP API
from the previous gRPC + grpc-gateway behaviour. Anything not listed here is
preserved: HTTP verbs and path templates, JSON property names (camelCase,
matching protojson), enum values (`SCREAMING_SNAKE`, e.g.
`REGISTER_METHOD_CLI`), RFC3339 timestamp strings, response envelopes
(`{"user": …}`, `{"nodes": […]}`, …), and the set of operations and their side
effects on the state layer.

Each entry: **what changed**, **why**, **client impact**.

## Wire format

### Integer IDs are JSON numbers, not strings

**What:** `id`, `nodeId`/`node_id`, `oldId`, `user`, and every other integer
identifier is now a JSON number (`"id": 42`). grpc-gateway emitted protojson's
int64-as-string form (`"id": "42"`).

**Why:** native integers are the idiomatic OpenAPI 3.0 / ogen representation,
type-safe end to end, and remove per-field string⇄int conversion. Headscale
identifiers are small and well within the JS safe-integer range.

**Client impact:** HTTP clients that read IDs as strings must read them as
numbers. `headscale … -o json/yaml` output changes accordingly (the CLI now
runs on the generated client).

### Error responses are RFC 7807 problem documents

**What:** errors are returned as `application/problem+json` with a body of
`{type?, title, status, detail, instance?}`. grpc-gateway returned an
`rpcStatus` envelope `{code, message, details[]}` as `application/json`, where
`code` was a gRPC status code and `details` was effectively always empty.

**Why:** RFC 7807 is the standard HTTP error shape; the gRPC `code`/`details`
fields were gRPC implementation leakage with no value over the HTTP status line.

**Client impact:** clients that parsed `{code, message}` must read
`{status, detail}` and the `application/problem+json` content type. The HTTP
status code itself is unchanged for equivalent conditions (e.g. unknown user →
404, invalid argument → 400, bad/My missing API key → 401).

## Behaviour

### Unknown resources return 404 consistently

**What:** operations that look up a resource by id return `404` when it does not
exist. Several gRPC handlers (e.g. `RenameUser`, `DeleteUser`) returned a plain
Go error, which grpc-gateway rendered as `500`; only a few (e.g. `GetNode`) used
an explicit not-found status.

**Why:** a missing resource is a client error, not a server error; 404 is the
correct, consistent status.

**Client impact:** clients that treated these as 500 should treat them as 404.

### Health on database failure

**What:** `GET /api/v1/health` returns `200 {"databaseConnectivity": true}` when
the database is reachable and `500` (problem document) when the ping fails. The
gRPC implementation returned the ping error, which grpc-gateway rendered as a
500; the `databaseConnectivity:false` body was never observable on failure.

**Why:** preserves the observable contract (200 healthy, 500 unhealthy) under
the new error shape.

**Client impact:** none beyond the problem-document error shape above.

## Delivery note (not a shipped behaviour change)

The grpc-gateway HTTP facade is replaced wholesale at `/api/v1` by the ogen
server in the foundation commit, rather than path-by-path. During development,
endpoints whose resource group has not yet been migrated return `501` over HTTP;
all are implemented before the branch is complete. The CLI is unaffected during
this window because it still uses the gRPC servers until its own migration. No
intermediate state is released.
