# wisp - Go + Wails v3. Run `just` to list recipes.
# Recipes shell out through `devenv shell` so they work without entering
# the dev environment first.

default:
    @just --list

gui: frontend
    devenv shell -- bash -c 'GDK_BACKEND=x11 go run ./cmd/gui'

cli:
    devenv shell -- go run ./cmd/cli

# No native window — plain HTTP server at localhost:34115, open in a browser.
gui-web: frontend
    devenv shell -- bash -c 'WAILS_SERVER_PORT=34115 go run -tags server ./cmd/gui'

# Native window, Vite HMR + auto Go rebuild.
dev:
    devenv up gui-dev

# http://localhost:34117 (not gui-web's 34115, so both can run at once); air rebuilds+restarts on Go changes, vite --watch rebuilds the frontend.
dev-web:
    devenv up gui-server-watch frontend-watch

# //go:embed reads dist; typecheck runs first since vite build alone doesn't.
frontend: typecheck
    devenv shell -- bash -c 'cd cmd/gui/frontend && npm install --silent && npm run build'

# Regenerate Wails JS bindings after changing a service's exported methods.
bindings:
    devenv shell -- bash -c 'cd cmd/gui && wails3 generate bindings -clean=true -time-type=Date'

typecheck:
    devenv shell -- bash -c 'cd cmd/gui/frontend && npm run typecheck'

build: frontend
    devenv shell -- bash -c 'mkdir -p bin && go build -o bin/wisp-cli ./cmd/cli && go build -o bin/wisp-gui ./cmd/gui'

test: frontend
    devenv shell -- go test ./...

# Excludes cmd/gui/build/{ios,android} — Wails' mobile cross-compile targets, not plain-buildable.
vet: frontend
    devenv shell -- go vet ./cmd/cli ./cmd/gui ./internal/...

fmt:
    devenv shell -- gofmt -l -w .

clean:
    rm -rf bin cmd/gui/frontend/dist
