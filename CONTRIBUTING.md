# Contributing

Thank you for helping improve Tavern Shelf. Keep changes focused on the local private-library product boundary described in `tavern-shelf-seed.md`, with user file safety taking priority over convenience.

Before opening a pull request:

```powershell
cd frontend
npm ci
npm run build
cd ..
gofmt -w ./cmd ./internal
go test ./...
go build ./...
git diff --check
```

Commit titles use Conventional Commits, for example `fix(import): preserve damaged source`. Never include real character cards, Library databases, pairing tokens, absolute user paths, or generated files outside the embedded web UI assets.

Bug reports should include the Tavern Shelf version, Windows version or headless platform, the operation that failed, and a redacted error message. Use synthetic cards when a reproduction file is necessary.
