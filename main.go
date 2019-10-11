package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rogpeppe/go-internal/modfile"
)

var configDir = getConfigDir()

var (
	fUpdate      = flag.Bool("update", false, "update tools instead of just installing them")
	fConfig      = flag.String("config", configDirFor("config"), "config file")
	fMods        = flag.String("mods", configDirFor("mods"), "module configuration directory")
	fVersions    = flag.String("versions", configDirFor("versions"), "versions output file")
	fCopyReplace = flag.Bool("copyreplace", true, "copy replacements from tool's go.mod")
)

func main() {
	flag.Parse()

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
	name   string
	verReq string
	tmpMod string
	tags   string
	setup  []string
}

func newTool(name string) *tool {
	name, ver := splitFirstSep(name, "@")
	if ver == "" {
		ver = "upgrade"
	}

	mod := strings.ReplaceAll(name, "/", "_")
	return &tool{
		name:   name,
		verReq: ver,
		tmpMod: filepath.Join(*fMods, mod),
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
	mkdirCd(t.tmpMod)

	hasGoMod := exists("go.mod")

	var oldVer string
	if hasGoMod {
		oldVer = t.version()
	}

	t.writeToolsGo()

	if *fUpdate || !hasGoMod {
		rm("go.mod")
		rm("go.sum")
		run("go", "mod", "init", "tmpmod")
		run("go", "get", "-d", t.name+"@"+t.verReq)

		if *fCopyReplace {
			if toolMod := run("go", "list", "-f", `{{.Module.GoMod}}`, t.name); toolMod != "" {
				data, err := ioutil.ReadFile(toolMod)
				if err != nil {
					panic(err)
				}

				mf, err := modfile.Parse(toolMod, data, nil)
				if err != nil {
					panic(err)
				}

				for _, replace := range mf.Replace {
					if strings.Contains(replace.New.Path, "..") {
						continue
					}

					replacement := fmt.Sprintf("%s=%s@%s", replace.Old.Path, replace.New.Path, replace.New.Version)
					run("go", "mod", "edit", "-replace", replacement)
				}
			}
		}

		for _, cmdline := range t.setup {
			run("sh", "-c", cmdline)
		}

		run("go", "mod", "tidy")
	}

	t.install()

	ver := t.version()

	fmt.Fprintf(vOut, "%s %s\n", t.name, ver)

	if oldVer == "" || oldVer == ver {
		fmt.Printf("%s %s\n", t.name, ver)
	} else {
		fmt.Printf("%s %s -> %s\n", t.name, oldVer, ver)
	}
}

func (t *tool) install() {
	run("go", "install", "-tags", t.tags, t.name)
}

func (t *tool) version() string {
	return run("go", "list", "-f", "{{.Module.Version}}", t.name)
}

const toolsGo = `// +build tools

package tools

import _ "%s"
`

func (t *tool) writeToolsGo() {
	if exists("tools.go") {
		return
	}

	f, err := os.Create("tools.go")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, toolsGo, t.name); err != nil {
		panic(err)
	}
}

func splitSpace(s string) (l string, r string) {
	return splitFirstSep(s, " ")
}

func splitFirstSep(s string, sep string) (l string, r string) {
	parts := strings.SplitN(s, sep, 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], strings.TrimSpace(parts[1])
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

func mkdirCd(path string) {
	if err := os.MkdirAll(path, 0700); err != nil {
		panic(err)
	}
	if err := os.Chdir(path); err != nil {
		panic(err)
	}
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func rm(file string) {
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		panic(err)
	}
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
