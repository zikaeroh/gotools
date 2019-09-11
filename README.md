# gotools

`gotools` is a way to manage `$GOBIN` using modules.
It's able to handle more complicated cases, including forcing certain versions, build tags, or running arbitrary
commands in the tool's module file.

A config file like `~/.config/gotools/config`:

```
# Benchmarking utils
golang.org/x/tools/cmd/benchcmp
golang.org/x/perf/cmd/benchstat
github.com/cespare/prettybench

# Database
github.com/golang-migrate/migrate/v4/cmd/migrate
    tags postgres

golang.org/x/tools/cmd/godoc
golang.org/x/lint/golint
golang.org/x/tools/cmd/goimports
github.com/go-delve/delve/cmd/dlv
mvdan.cc/gofumpt
mvdan.cc/gofumpt/gofumports

github.com/golangci/golangci-lint/cmd/golangci-lint@v1.18.0
    run go mod edit -replace github.com/ultraware/funlen=github.com/golangci/funlen@v0.0.0-20190909161642-5e59b9546114
    run go mod edit -replace golang.org/x/tools=github.com/golangci/tools@3540c026601b
    run go mod download

golang.org/x/tools/gopls@master
    run go get -d golang.org/x/tools@master
```

Will populate `$GOBIN` with all of the above listed tools, pinning them using modules stored in `~/.config/gotools/mods`.

Use `gotools -update` to delete all of the pinned versions and create them fresh.


## Disclaimer

This is still a WIP. It's not the fastest when updating.
