package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/sync/semaphore"
)

var configDir = getConfigDir()

var (
	fUpdate      = flag.Bool("update", false, "update tools instead of just installing them")
	fConfig      = flag.String("config", configDirFor("config"), "config file")
	fMods        = flag.String("mods", configDirFor("mods"), "module configuration directory")
	fVersions    = flag.String("versions", configDirFor("versions"), "versions output file")
	fCopyReplace = flag.Bool("copyreplace", true, "copy replacements from tool's go.mod")
	fWorkers     = flag.Int64("workers", int64(runtime.GOMAXPROCS(0))+1, "number of concurrent workers")
)

func getConfigDir() string {
	p, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return filepath.Join(p, "gotools")
}

func configDirFor(x string) string {
	return filepath.Join(configDir, x)
}

func main() {
	flag.Parse()

	f, err := os.Open(*fConfig)
	if err != nil {
		log.Fatal("error opening config", err)
	}
	defer f.Close()

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
		log.Fatal("config scanner error", err)
	}

	if curr != nil {
		tools = append(tools, curr)
	}

	sort.Slice(tools, func(i, j int) bool {
		return tools[i].name < tools[j].name
	})

	versions := make([]string, len(tools))

	sem := semaphore.NewWeighted(*fWorkers)

	for i := range tools {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			log.Println(err)
			return
		}

		go func(i int) {
			defer sem.Release(1)

			t := tools[i]

			if err := t.run(); err != nil {
				log.Println(err)
			}
			versions[i] = t.name + " " + t.finalVer
		}(i)
	}

	if err := sem.Acquire(context.Background(), *fWorkers); err != nil {
		log.Fatal(err)
	}

	vf, err := os.Create(*fVersions)
	if err != nil {
		log.Fatal("error creating version file", err)
	}
	defer vf.Close()

	for _, ver := range versions {
		if _, err := fmt.Fprintln(vf, ver); err != nil {
			log.Fatal("error writing to version file", err)
		}
	}
}

type tool struct {
	name   string
	verReq string
	wd     workingDir
	tags   string
	setup  []string

	finalVer string
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
		wd:     workingDir(filepath.Join(*fMods, mod)),
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

func (t *tool) run() error {
	if err := t.wd.mkdir(); err != nil {
		return err
	}

	hasGoMod := t.wd.contains("go.mod")

	var oldVer string
	if hasGoMod {
		var err error
		oldVer, err = t.version()
		if err != nil {
			return err
		}
	}

	if err := t.writeToolsGo(); err != nil {
		return err
	}

	if *fUpdate || !hasGoMod {
		if err := t.wd.rm("go.mod"); err != nil {
			return err
		}

		if err := t.wd.rm("go.sum"); err != nil {
			return err
		}

		if _, err := t.wd.run("go", "mod", "init", "tmpmod"); err != nil {
			return err
		}

		if _, err := t.wd.run("go", "get", "-d", t.name+"@"+t.verReq); err != nil {
			return err
		}

		if *fCopyReplace {
			toolMod, err := t.wd.run("go", "list", "-f", `{{.Module.GoMod}}`, t.name)
			if err != nil {
				return err
			}

			if toolMod != "" {
				data, err := os.ReadFile(toolMod)
				if err != nil {
					return err
				}

				mf, err := modfile.Parse(toolMod, data, nil)
				if err != nil {
					return err
				}

				for _, replace := range mf.Replace {
					if strings.Contains(replace.New.Path, "..") {
						continue
					}

					replacement := fmt.Sprintf("%s=%s@%s", replace.Old.Path, replace.New.Path, replace.New.Version)

					if _, err := t.wd.run("go", "mod", "edit", "-replace", replacement); err != nil {
						return err
					}
				}
			}
		}

		for _, cmdline := range t.setup {
			if _, err := t.wd.run("sh", "-c", cmdline); err != nil {
				return err
			}
		}

		if _, err := t.wd.run("go", "mod", "tidy"); err != nil {
			return err
		}
	}

	if err := t.install(); err != nil {
		return err
	}

	ver, err := t.version()
	if err != nil {
		return err
	}

	if oldVer == "" || oldVer == ver {
		fmt.Printf("%s %s\n", t.name, ver)
	} else {
		fmt.Printf("%s %s -> %s\n", t.name, oldVer, ver)
	}

	t.finalVer = ver
	return nil
}

func (t *tool) install() (err error) {
	if t.tags == "" {
		_, err = t.wd.run("go", "install", t.name)
	} else {
		_, err = t.wd.run("go", "install", "-tags", t.tags, t.name)
	}
	return err
}

func (t *tool) version() (string, error) {
	return t.wd.run("go", "list", "-f", "{{.Module.Version}}", t.name)
}

const toolsGo = `// +build tools

package tools

import _ "%s"
`

func (t *tool) writeToolsGo() error {
	if t.wd.contains("tools.go") {
		return nil
	}

	f, err := t.wd.create("tools.go")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, toolsGo, t.name)
	return err
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

type workingDir string

func (w workingDir) run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = string(w)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("%s\nSTDOUT\n%s\nSTDERR\n%s", cmd, stdout.String(), stderr.String())
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (w workingDir) contains(filename string) bool {
	filename = filepath.Join(string(w), filename)
	_, err := os.Stat(filename)
	return err == nil
}

func (w workingDir) mkdir() error {
	return os.MkdirAll(string(w), 0o700)
}

func (w workingDir) rm(filename string) error {
	filename = filepath.Join(string(w), filename)
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (w workingDir) create(filename string) (*os.File, error) {
	filename = filepath.Join(string(w), filename)
	return os.Create(filename)
}
