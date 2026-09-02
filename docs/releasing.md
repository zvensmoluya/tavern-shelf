# Releasing Tavern Shelf

The root `VERSION` file is the single release version. The Windows installer, binaries, archive names, and runtime `-version` output are derived from it.

To prepare a release:

1. Update `VERSION` and user-facing documentation.
2. Run `npm run build` in `frontend`, then `go test ./...` and `go build ./...`.
3. Commit the release preparation.
4. Create and push an annotated tag matching `v` plus `VERSION`, for example `v0.1.0`.

The `Release` GitHub Actions workflow builds on `windows-latest` and publishes:

- the per-user Inno Setup installer;
- a portable Windows desktop executable;
- a zipped Windows headless server;
- `SHA256SUMS.txt`.

The workflow can also be started manually to obtain artifacts without publishing a GitHub Release. Releases are currently unsigned; do not describe them as signed until a real code-signing step and protected signing secret are configured.
