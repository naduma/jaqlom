package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// Options holds CLI-derived startup settings.
type Options struct {
	Port       int
	ConfigPath string
	RootDir    string
	AssetsDir  string
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	opts, cfg, err := run(os.Args[1:], cwd, os.Stderr)
	if err != nil {
		os.Exit(1)
	}

	if err := serve(opts, cfg, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func serve(opts Options, cfg Config, stderr io.Writer) error {
	files, err := scanFiles(opts.RootDir, cfg)
	if err != nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "failed to scan files: %v\n", err)
		}
		return err
	}

	if opts.AssetsDir != "" {
		if _, err := os.Stat(opts.AssetsDir); err != nil {
			if stderr != nil {
				fmt.Fprintf(stderr, "assets directory not found: %s\n", opts.AssetsDir)
			}
			return err
		}
	} else if hasLocalAssetPaths(cfg) {
		if stderr != nil {
			fmt.Fprintf(stderr, "warning: config has local asset paths but -assets is not set\n")
		}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", opts.Port))
	if err != nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "failed to listen on port %d: %v\n", opts.Port, err)
		}
		return err
	}
	if stderr != nil {
		fmt.Fprintf(stderr, "listening on http://localhost:%d\n", ln.Addr().(*net.TCPAddr).Port)
	}
	return http.Serve(ln, newServer(opts.RootDir, files, cfg, opts.AssetsDir))
}

func run(args []string, cwd string, stderr io.Writer) (Options, Config, error) {
	opts, err := resolveOptions(args, cwd)
	if err != nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "failed to parse arguments: %v\n", err)
		}
		return Options{}, Config{}, err
	}

	cfg, err := loadConfig(opts.ConfigPath)
	if err != nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "failed to load config: %v\n", err)
		}
		return Options{}, Config{}, err
	}

	return opts, cfg, nil
}

func resolveOptions(args []string, cwd string) (Options, error) {
	fs := flag.NewFlagSet("jaqlom", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	port := fs.Int("port", 8080, "port to listen on")
	configPath := fs.String("config", "", "path to jaqlom.json")
	assetsDir := fs.String("assets", "", "path to local assets directory")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}

	rootDir := cwd
	if fs.NArg() > 0 {
		rootDir = fs.Arg(0)
		if !filepath.IsAbs(rootDir) {
			rootDir = filepath.Join(cwd, rootDir)
		}
	}

	rootDir = filepath.Clean(rootDir)
	resolvedConfigPath := *configPath
	if resolvedConfigPath == "" {
		resolvedConfigPath = filepath.Join(rootDir, "jaqlom.json")
	} else if !filepath.IsAbs(resolvedConfigPath) {
		resolvedConfigPath = filepath.Join(cwd, resolvedConfigPath)
	}
	resolvedConfigPath = filepath.Clean(resolvedConfigPath)

	resolvedAssetsDir := *assetsDir
	if resolvedAssetsDir != "" {
		if !filepath.IsAbs(resolvedAssetsDir) {
			resolvedAssetsDir = filepath.Join(cwd, resolvedAssetsDir)
		}
		resolvedAssetsDir = filepath.Clean(resolvedAssetsDir)
	}

	return Options{
		Port:       *port,
		ConfigPath: resolvedConfigPath,
		RootDir:    rootDir,
		AssetsDir:  resolvedAssetsDir,
	}, nil
}
