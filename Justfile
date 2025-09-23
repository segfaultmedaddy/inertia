export PARALLEL_CNT := `nproc --all`

setup:
    lefthook install -f

mod:
    go mod tidy
    go mod download
    gomod2nix

lint-fix:
  nix fmt
  typos -w
  golangci-lint run --fix ./...

format-sql:
  npx prettier -w **/*.sql

test-all:
  go test -race -count=1 -parallel={{PARALLEL_CNT}} ./...
