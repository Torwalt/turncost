default:
    @just --list

# Fast static checks: run after every edit.
verify:
    git diff --check
    test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
    go vet ./...
    go build ./...
    nixfmt --check flake.nix nix/*.nix
    python3 -m py_compile tools/commit_msg_check.py
    benchpress lint
    ploopy lint

# The test suite.
test:
    go test ./...

# Build the binary into bin/turncost.
build:
    go build -o bin/turncost ./cmd/turncost

# Format the repository.
fmt:
    gofmt -w .
    nixfmt flake.nix nix/*.nix

# Show the development tools and their versions.
doctor:
    git --version
    gh --version
    just --version
    tmux -V
    go version
    claude --version
    codex --version
    opencode --version
    ploopy --version
    benchpress --version
