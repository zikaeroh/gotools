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

github.com/golangci/golangci-lint/cmd/golangci-lint

golang.org/x/tools/gopls@master
    run go get -d golang.org/x/tools@master
```

Will populate `$GOBIN` with all of the above listed tools, pinning them using modules stored in `~/.config/gotools/mods`.

Use `gotools -update` to delete all of the pinned versions and create them fresh.

By default, `gotools` will copy the replacements from the tool's `go.mod`. Pass `-copyreplace=false` to disable this behavior.

## Disclaimer

This is still a WIP. It's not the fastest when updating.
