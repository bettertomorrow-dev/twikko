package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"twikko/internal/schema"
	"twikko/internal/server"
	"twikko/internal/session"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout))
}

func run(ctx context.Context, args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("twikko", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("input", "", "input YAML file")
	output := flags.String("output", "", "output YAML file")
	port := flags.Int("port", 0, "localhost port")
	showVersion := flags.Bool("version", false, "print version and exit")
	open := flags.Bool("open", false, "open the editor in a browser")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(stdout, "twikko: error="+err.Error())
		return 1
	}
	if *showVersion {
		fmt.Fprintln(stdout, "twikko "+version)
		return 0
	}
	if *input == "" || *output == "" || *port < 0 || *port > 65535 {
		fmt.Fprintln(stdout, "twikko: error=--input and --output are required; --port must be valid")
		return 1
	}
	document, err := schema.Load(*input)
	if err != nil {
		fmt.Fprintln(stdout, "twikko: error="+err.Error())
		return 1
	}
	absoluteOutput, err := filepath.Abs(*output)
	if err != nil {
		fmt.Fprintln(stdout, "twikko: error=resolve output: "+err.Error())
		return 1
	}
	editor, err := server.New(session.New(document), absoluteOutput)
	if err != nil {
		fmt.Fprintln(stdout, "twikko: error="+err.Error())
		return 1
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(stdout, "twikko: error=listen: "+err.Error())
		return 1
	}
	httpServer := &http.Server{Handler: editor}
	editor.SetAllowedHost(listener.Addr().String())
	url := editor.URL(listener.Addr().String())
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()

	fmt.Fprintln(stdout, "twikko: url="+url)
	if *open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(stdout, "twikko: warning=open browser: "+err.Error())
		}
	}

	var event server.Event
	select {
	case event = <-editor.Done():
	case <-ctx.Done():
		if err := editor.Cancel(); err != nil {
			event = server.Event{Status: "cancelled", Err: err}
		} else {
			event = <-editor.Done()
		}
	case err := <-serveErrors:
		event = server.Event{Status: "cancelled", Err: err}
	}
	status := event.Status
	exitCode := 2
	if event.Err != nil && !errors.Is(event.Err, http.ErrServerClosed) {
		fmt.Fprintln(stdout, "twikko: error="+event.Err.Error())
		exitCode = 1
	} else if status == "saved" {
		exitCode = 0
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	fmt.Fprintf(stdout, "twikko: status=%s output=%s\n", status, absoluteOutput)
	return exitCode
}

func openBrowser(url string) error {
	if runtime.GOOS == "windows" {
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	return exec.Command(command, url).Start()
}
