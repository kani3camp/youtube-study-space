# AGENTS.md

## Cursor Cloud specific instructions

### Overview

YouTube Study Space is a monorepo with 4 main services. See `CLAUDE.md` for full architecture and command reference.

### Services

| Service | Directory | Dev Command | Test Command | Lint Command |
|---|---|---|---|---|
| Go Backend | `system/` | `go run main.go` (needs GCP creds) | `go test -shuffle=on -v ./...` | `golangci-lint run --timeout=5m` |
| Frontend (Next.js) | `youtube-monitor/` | `pnpm dev` (port 3000) | `pnpm test` | `pnpm lint` |
| AWS CDK | `aws-cdk/` | N/A | `npm test` | N/A |
| Docs Site | `docs-site/` | `npm start` (port 3001) | N/A | `npm run lint` |

### Non-obvious caveats

- **Go version**: Requires Go 1.24+. The VM snapshot installs Go 1.24.2 at `/usr/local/go`. Ensure `PATH` includes `/usr/local/go/bin` and `$HOME/go/bin`.
- **Frontend `.env.local`**: The Next.js app will crash at startup without `NEXT_PUBLIC_DEBUG`, `NEXT_PUBLIC_CHANNEL_GL`, and `NEXT_PUBLIC_ROOM_CONFIG` env vars. For local dev without Firebase, create `youtube-monitor/.env.local` with:
  ```
  NEXT_PUBLIC_DEBUG=true
  NEXT_PUBLIC_CHANNEL_GL=false
  NEXT_PUBLIC_ROOM_CONFIG=DEV
  NEXT_PUBLIC_FIREBASE_PROJECT_ID=demo-project
  NEXT_PUBLIC_FIREBASE_API_KEY=demo-api-key
  ```
- **pnpm build scripts**: After `pnpm install`, a warning about ignored build scripts (`esbuild`, `@parcel/watcher`, etc.) is expected. The dev server and tests work without approving these builds.
- **Go backend** requires GCP credentials (`CREDENTIAL_FILE_LOCATION` env var pointing to a service account JSON) to run `main.go`. Unit tests run fine without credentials as they use mocks.
- **golangci-lint**: CI uses v1.64. Install with `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8`.
- **Frontend design resolution**: The study room UI is designed for 1920x1080. In a standard browser viewport, content appears on the right side with a "Loading..." spinner in the center (normal when no Firestore data is available).
