package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

var (
	gcpProjID       string
	gcpCredFilePath string
	concurrency     int
)

func init() {
	flag.StringVar(&gcpProjID, "project", "", "Google Project ID")
	flag.StringVar(&gcpCredFilePath, "credentials", "", "Path to credentials")
	flag.IntVar(&concurrency, "concurrency", 1000, "Max goroutines limit (default = 1000)")

	flag.Parse()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	g, ctx := errgroup.WithContext(ctx)

	// Wait for interruption
	g.Go(func() error {
		ic := make(chan os.Signal, 1)
		signal.Notify(ic, os.Interrupt, syscall.SIGTERM)

		select {
		case <-ic:
			cancel()
		case <-ctx.Done():
		}

		return ctx.Err()
	})

	// Export data to stdout
	g.Go(func() error {
		exp, err := newExporter(ctx, gcpProjID, gcpCredFilePath, concurrency)
		if err != nil {
			return err
		}

		data, err := exp.exportAll(ctx)
		if err != nil {
			return err
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		err = enc.Encode(data)
		if err != nil {
			return err
		}

		cancel()

		return nil
	})

	err := g.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("failed to export data from Firestore: %v", err)
	}
}
