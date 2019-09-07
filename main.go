package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const gobin = "github.com/myitcv/gobin"

var (
	binDir    = getBinDir()
	configDir = getConfigDir()
)

var (
	fUpdate   = flag.Bool("update", false, "update tools instead of just installing them")
	fConfig   = flag.String("config", configDirFor("config"), "config file")
	fMods     = flag.String("mods", configDirFor("mods"), "module configuration directory")
	fVersions = flag.String("versions", configDirFor("versions"), "versions output file")
)

func main() {
	flag.Parse()

	if *fUpdate {
		if err := os.RemoveAll(*fMods); err != nil {
			panic(err)
		}
		mkdir(*fMods)
	}

	f, err := os.Open(*fConfig)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	vf, err := os.Create(*fVersions)
	if err != nil {
		panic(err)
	}
	defer vf.Close()

	installGobin(vf)

	scanner := bufio.NewScanner(f)

	var curr *tool
	var tools []*tool

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" || line[0] == '#' {
			continue
		}

		if curr.parse(line) {
			if curr != nil {
				tools = append(tools, curr)
			}
			curr = newTool(line)
		}
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	if curr != nil {
		tools = append(tools, curr)
	}

	for _, t := range tools {
		t.run(vf)
	}
}

type tool struct {
	name  string
	mod   string
	tags  string
	setup []string
	goRun bool
}

func newTool(name string) *tool {
	mod := strings.ReplaceAll(name, "/", "_")
	return &tool{
		name: name,
		mod:  filepath.Join(*fMods, mod),
	}
}

func (t *tool) parse(line string) (next bool) {
	if t == nil {
		return true
	}

	cmd, args := splitSpace(line)
	switch cmd {
	case "tags":
		t.tags = args
	case "run":
		t.setup = append(t.setup, args)
	default:
		return true
	}
	return false
}

func (t *tool) run(vOut io.Writer) {
	mkdir(t.mod)
	if err := os.Chdir(t.mod); err != nil {
		panic(err)
	}

	if *fUpdate || notExists("go.mod") {
		if err := exec.Command("go", "mod", "init", "tmpmod").Run(); err != nil {
			panic(err)
		}

		for _, cmdline := range t.setup {
			run("sh", "-c", cmdline)
		}
	}

	t.install()
	ver := t.name + " " + t.version()
	fmt.Println(ver)
	fmt.Fprintln(vOut, ver)
}

func (t *tool) install() {
	binPath := t.runCmd("-p")
	run("cp", "-f", binPath, binDir+string(os.PathSeparator))
}

func (t *tool) version() string {
	_, ver := splitSpace(t.runCmd("-v"))
	return ver
}

func (t *tool) cmd(flags ...string) (name string, args []string) {
	name = "gobin"
	if t.goRun {
		name = "go"
		args = []string{"run", gobin}
	}

	args = append(args, "-m")

	if t.tags != "" {
		args = append(args, "-tags", t.tags)
	}

	args = append(args, flags...)
	args = append(args, t.name)

	return name, args
}

func (t *tool) runCmd(flags ...string) string {
	name, args := t.cmd(flags...)
	return run(name, args...)
}

func splitSpace(s string) (l string, r string) {
	parts := strings.SplitN(s, " ", 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], strings.TrimSpace(parts[1])
	}
}

func installGobin(vOut io.Writer) {
	_, err := exec.LookPath("gobin")

	t := newTool(gobin)
	t.goRun = err != nil
	t.run(vOut)
}

func mkdir(path string) {
	if err := os.MkdirAll(path, 0700); err != nil {
		panic(err)
	}
}

func run(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Println(cmd)
		fmt.Printf("STDOUT\n%s", stdout.String())
		fmt.Printf("STDERR\n%s", stderr.String())
		panic(err)
	}

	return strings.TrimSpace(stdout.String())
}

func notExists(file string) bool {
	_, err := os.Stat(file)
	return os.IsNotExist(err)
}

func getBinDir() string {
	v, ok := os.LookupEnv("GOBIN")
	if ok {
		return v
	}

	v, ok = os.LookupEnv("GOPATH")
	if ok {
		return filepath.Join(v, "bin")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return filepath.Join(home, "go", "bin")
}

func getConfigDir() string {
	p, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(p, "gotools")
}

func configDirFor(x string) string {
	return filepath.Join(configDir, x)
}
