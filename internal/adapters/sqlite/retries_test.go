package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/project"
	"gorm.io/gorm"
)

func TestTransactionRetriesTemporaryContentionAndReportsPersistentBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), databaseFilename)
	locker, err := OpenOrCreate(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = locker.Close() })
	writer, err := OpenOrCreate(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	if _, err = locker.SQL().Exec(`CREATE TABLE retry_fixture (id INTEGER PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	connection, err := locker.SQL().Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if _, err = connection.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	if _, err = connection.ExecContext(context.Background(), `INSERT INTO retry_fixture VALUES (1,'locked')`); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		time.Sleep(125 * time.Millisecond)
		_, _ = connection.ExecContext(context.Background(), `COMMIT`)
		close(done)
	}()
	err = transaction(context.Background(), writer.GORM(), func(tx *gorm.DB) error { return tx.Exec(`INSERT INTO retry_fixture VALUES (2,'retried')`).Error })
	<-done
	if err != nil {
		t.Fatalf("temporary contention: %v", err)
	}
	if _, err = connection.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	err = transaction(context.Background(), writer.GORM(), func(tx *gorm.DB) error { return tx.Exec(`INSERT INTO retry_fixture VALUES (3,'busy')`).Error })
	if _, rollbackErr := connection.ExecContext(context.Background(), `ROLLBACK`); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if project.Code(err) != storeBusyCode {
		t.Fatalf("persistent contention error=%v code=%s", err, project.Code(err))
	}
	var count int
	if err = writer.SQL().QueryRow(`SELECT count(*) FROM retry_fixture WHERE id=3`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial write count=%d err=%v", count, err)
	}
}
