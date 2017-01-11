package goblob

import (
	"errors"
	"fmt"

	"github.com/cheggaaa/pb"
	"golang.org/x/sync/errgroup"
)

// Migrator moves blobs from one Store to another
type Migrator interface {
	Migrate(dst Store, src Store) error
}

type migrator struct {
	concurrentMigrators int
	blobMigrator        BlobMigrator
}

type StatusBar interface {
	Increment() int
	FinishPrint(str string)
}

func New(concurrent int, blobMigrator BlobMigrator) Migrator {
	return &migrator{
		concurrentMigrators: concurrent,
		blobMigrator:        blobMigrator,
	}
}

func (m *migrator) Migrate(dst Store, src Store) error {
	if src == nil {
		return errors.New("src is an empty store")
	}

	if dst == nil {
		return errors.New("dst is an empty store")
	}

	blobs, err := src.List()
	if err != nil {
		return err
	}

	if len(blobs) == 0 {
		return errors.New("the source store has no files")
	}

	bar := pb.StartNew(len(blobs))
	bar.Format("<.- >")

	fmt.Printf("Migrating blobs from %s to %s\n", src.Name(), dst.Name())

	blobsToMigrate := make(chan *Blob)
	go func() {
		for _, blob := range blobs {
			if !dst.Exists(blob) {
				blobsToMigrate <- blob
			}
		}

		close(blobsToMigrate)
	}()

	var g errgroup.Group
	for i := 0; i < m.concurrentMigrators; i++ {
		g.Go(func() error {
			for blob := range blobsToMigrate {
				err := m.blobMigrator.Migrate(blob)
				if err != nil {
					return err
				}
				bar.Increment()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		bar.FinishPrint(fmt.Sprintf("Error migrating blobs from %s to %s\n", src.Name(), dst.Name()))
		return err
	}

	bar.FinishPrint(fmt.Sprintf("Done migrating blobs from %s to %s\n", src.Name(), dst.Name()))
	return nil
}
