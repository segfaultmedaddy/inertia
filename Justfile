export CPU_COUNT := `nproc --all`

setup:
    lefthook install -f

mod:
    go mod tidy
    go mod download

gen:
    go generate ./...

lint:
    modernize ./...
    golangci-lint run ./...

lint-fix:
    modernize --fix ./...
    golangci-lint run --fix ./...
    nix fmt

test-all:
    go test -race -count=1 -parallel={{CPU_COUNT}} ./...
