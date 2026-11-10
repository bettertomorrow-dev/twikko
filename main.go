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

	"multieditor/internal/schema"
	"multieditor/internal/server"
	"multieditor/internal/session"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout))
}

func run(ctx context.Context, args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("multieditor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("input", "", "input YAML file")
	output := flags.String("output", "", "output YAML file")
	port := flags.Int("port", 0, "localhost port")
	open := flags.Bool("open", false, "open the editor in a browser")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(stdout, "multieditor: error="+err.Error())
		return 1
	}
	if *input == "" || *output == "" || *port < 0 || *port > 65535 {
		fmt.Fprintln(stdout, "multieditor: error=--input and --output are required; --port must be valid")
		return 1
	}
	document, err := schema.Load(*input)
	if err != nil {
		fmt.Fprintln(stdout, "multieditor: error="+err.Error())
		return 1
	}
	absoluteOutput, err := filepath.Abs(*output)
	if err != nil {
		fmt.Fprintln(stdout, "multieditor: error=resolve output: "+err.Error())
		return 1
	}
	editor, err := server.New(session.New(document), absoluteOutput)
	if err != nil {
		fmt.Fprintln(stdout, "multieditor: error="+err.Error())
		return 1
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(stdout, "multieditor: error=listen: "+err.Error())
		return 1
	}
	httpServer := &http.Server{Handler: editor}
	editor.SetAllowedHost(listener.Addr().String())
	url := editor.URL(listener.Addr().String())
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()

	fmt.Fprintln(stdout, "multieditor: url="+url)
	if *open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(stdout, "multieditor: warning=open browser: "+err.Error())
		}
	}

	status := "cancelled"
	exitCode := 2
	select {
	case event := <-editor.Done():
		status = event.Status
		if event.Err != nil {
			fmt.Fprintln(stdout, "multieditor: error="+event.Err.Error())
			exitCode = 1
		} else if status == "saved" {
			exitCode = 0
		}
	case <-ctx.Done():
		if err := editor.Cancel(); err != nil {
			fmt.Fprintln(stdout, "multieditor: error="+err.Error())
			exitCode = 1
		}
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(stdout, "multieditor: error=serve: "+err.Error())
			exitCode = 1
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	fmt.Fprintf(stdout, "multieditor: status=%s output=%s\n", status, absoluteOutput)
	return exitCode
}

func openBrowser(url string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	return exec.Command(command, url).Start()
}
