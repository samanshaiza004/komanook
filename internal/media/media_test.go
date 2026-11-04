package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"komanook/internal/database/store"
	"komanook/internal/testutil"
)

func pngImage(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.White)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestImageValidation(t *testing.T) {
	for _, bad := range [][]byte{[]byte("<svg><script>alert(1)</script></svg>"), []byte("GIF89a"), pngImage(t, 8193, 1)} {
		if _, err := Validate(bytes.NewReader(bad)); err == nil {
			t.Fatal("accepted unsupported/oversize image")
		}
	}
	if cfg, err := Validate(bytes.NewReader(pngImage(t, 20, 30))); err != nil || cfg.Width != 20 {
		t.Fatal(cfg, err)
	}
}

func TestTransactionalQueueAndNormalization(t *testing.T) {
	p := testutil.Database(t)
	ctx := context.Background()
	var userID int64
	if err := p.QueryRow(ctx, "INSERT INTO users(email,password_hash,role) VALUES('operator@example.test','unused','admin') RETURNING id").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	s, err := New(p, dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// Transaction failure leaves neither metadata, queue job nor quarantined file.
	if _, err = s.Accept(ctx, -1, bytes.NewReader(pngImage(t, 20, 20))); err == nil {
		t.Fatal("invalid owner accepted")
	}
	files, _ := os.ReadDir(filepath.Join(dir, "quarantine"))
	if len(files) != 0 {
		t.Fatal("failed transaction leaked source")
	}
	var n int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM river_job").Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	// Failure after metadata insertion must also roll the metadata and source back.
	_, err = p.Exec(ctx, `CREATE FUNCTION reject_test_job() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected queue failure'; END $$; CREATE TRIGGER reject_test_job BEFORE INSERT ON river_job FOR EACH ROW EXECUTE FUNCTION reject_test_job()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Accept(ctx, userID, bytes.NewReader(pngImage(t, 20, 20))); err == nil {
		t.Fatal("queue failure was ignored")
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM uploads").Scan(&n); err != nil || n != 0 {
		t.Fatal("metadata survived failed enqueue", n, err)
	}
	files, _ = os.ReadDir(filepath.Join(dir, "quarantine"))
	if len(files) != 0 {
		t.Fatal("failed enqueue leaked source")
	}
	if _, err = p.Exec(ctx, "DROP TRIGGER reject_test_job ON river_job; DROP FUNCTION reject_test_job()"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Accept(ctx, userID, bytes.NewReader(bytes.Repeat([]byte{'x'}, MaxBytes+1))); err != ErrImage {
		t.Fatal("byte limit", err)
	}
	// Work the durable queue, not a request goroutine or mocked worker.
	events, cancel := s.Jobs.Subscribe(river.EventKindJobCompleted)
	defer cancel()
	if err = s.Jobs.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Jobs.Stop(stop); err != nil {
			t.Error(err)
		}
	})
	id, err := s.Accept(ctx, userID, bytes.NewReader(pngImage(t, 2000, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-time.After(15 * time.Second):
		t.Fatal("background normalization timed out")
	}
	u, err := store.New(p).Upload(ctx, id)
	if err != nil || u.State != "ready" || u.Width != 1600 || u.Height != 800 {
		t.Fatalf("result=%+v err=%v", u, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "processed", u.Output))
	if err != nil {
		t.Fatal(err)
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" {
		t.Fatal(format, err)
	}
	if _, err = os.Stat(filepath.Join(dir, "quarantine", id)); !os.IsNotExist(err) {
		t.Fatal("original retained", err)
	}
	if err = s.Process(ctx, id); err != nil {
		t.Fatal("idempotent retry", err)
	}
	if err = s.Process(ctx, "../../etc/passwd"); err == nil {
		t.Fatal("path payload accepted")
	}
}
