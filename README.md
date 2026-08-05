# happy-memory

CLI written in Go.

## Development

The project uses the Go version declared in `mise.toml`.

```bash
mise exec -- make check
```

Useful commands:

```bash
make fmt
make tidy
make build
```

Run tests with:

```bash
go test ./...
```

### Create a worktree

Create a branch from the current `HEAD`, add it as a sibling worktree, and move the
current terminal into it:

```bash
source scripts/create-worktree.sh
```

When prompted, enter the full branch name. For example, `feature/search` creates
the branch `feature/search` at `../happy-memory-feature-search`.

The script must be run with `source` so that its final `cd` changes the current
terminal's working directory. Running it as a separate process cannot change the
parent shell's directory.
