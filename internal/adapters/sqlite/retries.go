package sqlite

import (
	"context"
	"errors"
	"time"

	driversqlite "github.com/glebarez/go-sqlite"
	"github.com/jorgeluis594/happy-memory/internal/project"
	"gorm.io/gorm"
)

const (
	storeBusyCode = "STORE_BUSY"
	maxAttempts   = 3
)

var retryDelays = [...]time.Duration{50 * time.Millisecond, 100 * time.Millisecond}

func transaction(ctx context.Context, db *gorm.DB, operation func(*gorm.DB) error) error {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := db.WithContext(ctx).Transaction(operation)
		if !isBusy(err) {
			return err
		}
		if attempt == maxAttempts-1 {
			return project.NewError(storeBusyCode, errors.New("storage is busy"))
		}
		timer := time.NewTimer(retryDelays[attempt])
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func isBusy(err error) bool {
	var sqliteErr *driversqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	baseCode := sqliteErr.Code() & 0xff
	return baseCode == 5 || baseCode == 6
}
