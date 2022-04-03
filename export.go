package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"cloud.google.com/go/firestore"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type exporter struct {
	cli               *firestore.Client
	goroutinesLimitCh chan struct{}
}

func newExporter(ctx context.Context, projectID, pathToCreds string, maxGoroutinesLimit int) (*exporter, error) {
	cli, err := firestore.NewClient(ctx, projectID, option.WithCredentialsFile(pathToCreds))
	if err != nil {
		return nil, fmt.Errorf("[exporter] failed to create firestore client: %w", err)
	}

	return &exporter{
		cli:               cli,
		goroutinesLimitCh: make(chan struct{}, maxGoroutinesLimit),
	}, nil
}

func (e *exporter) exportAll(ctx context.Context) (map[string]any, error) {
	return e.exportCollections(ctx, e.cli.Collections(ctx))
}

func (e *exporter) exportCollections(ctx context.Context, collectionsIter *firestore.CollectionIterator) (map[string]any, error) {
	res := make(map[string]any, 0)
	resMu := &sync.Mutex{}

	eg, ctx := errgroup.WithContext(ctx)

loop:
	for {
		collRef, err := collectionsIter.Next()

		switch {
		case errors.Is(err, iterator.Done):
			break loop
		case err != nil:
			return nil, fmt.Errorf("[exporter] failed to iterate over all collections: %w", err)
		}

		e.goroutinesLimitCh <- struct{}{}
		eg.Go(func() error {
			defer func() {
				<-e.goroutinesLimitCh
			}()

			resMu.Lock()
			res[collRef.ID], err = e.exportCollection(ctx, collRef)
			resMu.Unlock()

			if err != nil {
				return fmt.Errorf("[exporter] failed to export collection %s: %w", collRef.Path, err)
			}

			return nil
		})
	}

	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (e *exporter) exportCollection(ctx context.Context, collRef *firestore.CollectionRef) (map[string]any, error) {
	iter := collRef.DocumentRefs(ctx)

	res := make(map[string]any, 0)
	resMu := &sync.Mutex{}

	eg, ctx := errgroup.WithContext(ctx)
loop:
	for {
		docRef, err := iter.Next()

		switch {
		case errors.Is(err, iterator.Done):
			break loop
		case err != nil:
			return nil, fmt.Errorf("[exporter] failed to iterate over documents in collection %s: %w", collRef.Path, err)
		}

		e.goroutinesLimitCh <- struct{}{}

		eg.Go(func() error {
			defer func() {
				<-e.goroutinesLimitCh
			}()

			subColls, err := e.exportCollections(ctx, docRef.Collections(ctx))
			if err != nil {
				return fmt.Errorf("[exporter] failed to export sub-collections for document %s: %w", docRef.Path, err)
			}

			docData := make(map[string]any, 0)

			for key, val := range subColls {
				docData[key] = val
			}

			doc, err := docRef.Get(ctx)
			if err != nil && status.Code(err) != codes.NotFound {
				return fmt.Errorf("[exporter] failed to get document data %s: %w", docRef.Path, err)
			}

			for key, val := range doc.Data() {
				docData[key] = val
			}

			resMu.Lock()
			res[docRef.ID] = docData
			resMu.Unlock()

			return nil
		})
	}

	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	return res, nil
}
