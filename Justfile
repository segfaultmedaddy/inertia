export PARALLEL_CNT := `nproc --all`

setup:
    lefthook install -f

mod:
    go mod tidy
    go mod download

gen:
    go generate ./...

lint-fix:
  nix fmt
  typos -w
  golangci-lint run --fix ./...

test-all:
  go test -race -count=1 -parallel={{PARALLEL_CNT}} ./...
