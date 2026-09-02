# Tavern Shelf Connector Protocol v1

Connector v1 is a local, user-paired HTTP API for character-card clients. Tavern Shelf Desktop listens on `http://127.0.0.1:8787` by default; the headless server exposes the same routes on its configured address.

## Pairing

The user generates a six-digit, five-minute pairing code in Shelf Tools. A client exchanges it once:

```http
POST /connector/v1/pair
Content-Type: application/json

{"code":"123456","clientName":"SillyTavern","clientVersion":"1.18.0"}
```

The response contains a random Bearer token. Only its SHA-256 hash is stored by Shelf. A successful new pairing revokes the previous token; five incorrect attempts invalidate the active code.

## API

- `GET /connector/v1/status` is unauthenticated and returns `protocol`, `version`, and `paired`.
- `GET /connector/v1/characters` returns Shelf character metadata.
- `GET /connector/v1/characters/{id}/source` streams the managed original source.
- `POST /connector/v1/imports` accepts a `multipart/form-data` field named `file`; only PNG and JSON character cards are accepted, up to 64 MiB.

All routes except status and pairing require `Authorization: Bearer <token>`.

Browser clients are accepted only from HTTP or HTTPS loopback origins. Connector responses use `Cache-Control: no-store`; clients should treat a `401` response as a revoked pairing. Imports return `{id, kind, name, duplicate}` and use Shelf's normal content-hash deduplication and safe staging process.

Shelf-only organization metadata such as favorites, private notes, and collection membership is deliberately excluded from Connector responses.
