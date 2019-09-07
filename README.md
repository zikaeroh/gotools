# gotools

`gotools` is a way to manage `$GOBIN` using modules (with some help from [gobin](https://github.com/myitcv/gobin)).
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

# Unbreak until the maintainer fixes these replacements
github.com/golangci/golangci-lint/cmd/golangci-lint
    run go mod edit -replace github.com/go-critic/go-critic=github.com/go-critic/go-critic@v0.3.5-0.20190526074819-1df300866540
    run go mod edit -replace github.com/golangci/errcheck=github.com/golangci/errcheck@v0.0.0-20181223084120-ef45e06d44b6
    run go mod edit -replace github.com/golangci/go-tools=github.com/golangci/go-tools@v0.0.0-20190318060251-af6baa5dc196
    run go mod edit -replace github.com/golangci/gofmt=github.com/golangci/gofmt@v0.0.0-20181222123516-0b8337e80d98
    run go mod edit -replace github.com/golangci/gosec=github.com/golangci/gosec@v0.0.0-20190211064107-66fb7fc33547
    run go mod edit -replace github.com/golangci/ineffassign=github.com/golangci/ineffassign@v0.0.0-20190609212857-42439a7714cc
    run go mod edit -replace github.com/golangci/lint-1=github.com/golangci/lint-1@v0.0.0-20190420132249-ee948d087217
    run go mod edit -replace mvdan.cc/unparam=mvdan.cc/unparam@v0.0.0-20190209190245-fbb59629db34

# Use master of gopls
golang.org/x/tools/gopls
    run GOPROXY=direct go get golang.org/x/tools{,/gopls}@master
```

Will populate `$GOBIN` with all of the above listed tools, pinning them using modules stored in `~/.config/gotools/mods`.

Use `gotools -update` to delete all of the pinned versions and create them fresh.
