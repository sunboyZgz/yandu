package toolchain

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
)

func Command(args []string, defaultConfig, defaultRole string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("tools prepare | list | run <name>")
	}
	action := args[0]
	args = args[1:]
	name := ""
	if action == "run" {
		if len(args) == 0 {
			return fmt.Errorf("tools run requires a tool name")
		}
		name = args[0]
		args = args[1:]
	}
	f := flag.NewFlagSet("tools "+action, flag.ContinueOnError)
	f.SetOutput(errOut)
	config := f.String("config", defaultConfig, "Yandu tools.json path")
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	root := f.String("component-root", filepath.Dir(exe), "private bundle or source root")
	role := f.String("role", defaultRole, "local, cloud, readonly or build")
	noMedia := f.Bool("no-media", false, "skip optional media tools")
	jsonOutput := f.Bool("json", false, "JSON output")
	var overrides repeatedFlags
	f.Var(&overrides, "tool", "explicit name=absolute-path (repeatable)")
	if e = f.Parse(args); e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	switch action {
	case "prepare":
		if f.NArg() != 0 {
			return fmt.Errorf("unexpected tool arguments")
		}
		explicit := map[string]string{}
		for _, v := range overrides {
			n, p, ok := strings.Cut(v, "=")
			if !ok || p == "" {
				return fmt.Errorf("--tool expects name=path")
			}
			if _, ok := specifications()[n]; !ok {
				return fmt.Errorf("unknown tool %s", n)
			}
			explicit[n] = p
		}
		c, e := Prepare(ctx, Options{Role: *role, ConfigPath: *config, ComponentRoot: *root, NoMedia: *noMedia, Overrides: explicit})
		if e != nil {
			return e
		}
		return printConfig(c, *jsonOutput, out)
	case "list":
		c, e := Load(*config)
		if e != nil {
			return e
		}
		return printConfig(c, *jsonOutput, out)
	case "run":
		c, e := Load(*config)
		if e != nil {
			return e
		}
		path, e := Executable(ctx, c, name, *root)
		if e != nil {
			return e
		}
		command := exec.CommandContext(ctx, path, f.Args()...)
		command.Stdin = os.Stdin
		command.Stdout = out
		command.Stderr = errOut
		if e = command.Run(); e != nil && ctx.Err() == nil {
			return e
		}
		return nil
	default:
		return fmt.Errorf("unknown tools action %s", action)
	}
}

type repeatedFlags []string

func (r *repeatedFlags) String() string     { return strings.Join(*r, ",") }
func (r *repeatedFlags) Set(v string) error { *r = append(*r, v); return nil }
func printConfig(c Config, asJSON bool, out io.Writer) error {
	if asJSON {
		return json.NewEncoder(out).Encode(c)
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "工具\t版本\t来源\t绝对路径")
	for _, n := range Sorted(c) {
		t := c.Tools[n]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", n, t.Version, t.Source, t.Path)
	}
	return w.Flush()
}
