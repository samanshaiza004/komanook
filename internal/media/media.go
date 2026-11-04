package media

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	xdraw "golang.org/x/image/draw"
	"komanook/internal/database/store"
)

const MaxBytes = 10 << 20

var ErrImage = errors.New("use a JPEG or PNG under 10 MiB, 8192 pixels per side and 25 megapixels")

type Args struct {
	UploadID string `json:"upload_id"`
}

func (Args) Kind() string { return "normalize_image_v1" }

type Service struct {
	Pool *pgxpool.Pool
	Dir  string
	Jobs *river.Client[pgx.Tx]
}

func New(pool *pgxpool.Pool, dir string, logger *slog.Logger) (*Service, error) {
	for _, sub := range []string{"quarantine", "processed"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
			return nil, err
		}
	}
	s := &Service{Pool: pool, Dir: dir}
	w := river.NewWorkers()
	river.AddWorker(w, &Worker{Service: s})
	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Logger: logger, Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 2}}, Workers: w})
	s.Jobs = c
	return s, err
}

func Validate(r io.ReadSeeker) (image.Config, error) {
	c, format, err := image.DecodeConfig(r)
	if err != nil || (format != "png" && format != "jpeg") || c.Width < 1 || c.Height < 1 || c.Width > 8192 || c.Height > 8192 || int64(c.Width)*int64(c.Height) > 25_000_000 {
		return image.Config{}, ErrImage
	}
	if _, err = r.Seek(0, io.SeekStart); err != nil {
		return image.Config{}, err
	}
	return c, nil
}

// Quarantine and job metadata become visible together; files have only server-generated names.
func (s *Service) Accept(ctx context.Context, userID int64, r io.Reader) (string, error) {
	id := hex.EncodeToString(randomBytes())
	path := filepath.Join(s.Dir, "quarantine", id)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	keep := false
	defer func() {
		_ = f.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	n, err := io.Copy(f, io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return "", err
	}
	if n > MaxBytes {
		return "", ErrImage
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if _, err = Validate(f); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = store.New(tx).CreateUpload(ctx, store.CreateUploadParams{ID: id, UserID: userID}); err != nil {
		return "", err
	}
	if _, err = s.Jobs.InsertTx(ctx, tx, Args{UploadID: id}, &river.InsertOpts{MaxAttempts: 3}); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	keep = true
	return id, nil
}

func randomBytes() []byte {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

type Worker struct {
	river.WorkerDefaults[Args]
	Service *Service
}

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	return w.Service.Process(ctx, job.Args.UploadID)
}

func (s *Service) Process(ctx context.Context, id string) error {
	// Job IDs also cross a trust boundary: never use a payload as a filesystem path.
	if len(id) != 32 {
		return fmt.Errorf("invalid upload ID")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return err
	}
	q := store.New(s.Pool)
	u, err := q.Upload(ctx, id)
	if err != nil {
		return err
	}
	if u.State == "ready" {
		return nil
	}
	f, err := os.Open(filepath.Join(s.Dir, "quarantine", id))
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = Validate(f); err != nil {
		return river.JobCancel(err)
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return river.JobCancel(ErrImage)
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width > 1600 {
		height = height * 1600 / width
		width = 1600
		if height < 1 {
			height = 1
		}
	}
	// Flatten transparency to paper, discard original metadata and re-encode only known pixels.
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, b, draw.Over, nil)
	tmp, err := os.CreateTemp(filepath.Join(s.Dir, "processed"), ".normalize-")
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	if err = jpeg.Encode(tmp, out, &jpeg.Options{Quality: 85}); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if _, err = tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	if _, err = io.Copy(h, tmp); err != nil {
		return err
	}
	name := hex.EncodeToString(h.Sum(nil)) + ".jpg"
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), filepath.Join(s.Dir, "processed", name)); err != nil {
		return err
	}
	if err = q.FinishUpload(ctx, store.FinishUploadParams{ID: id, Output: name, Width: int32(width), Height: int32(height)}); err != nil {
		return err
	}
	// Original retained for retries until metadata is durable. Successful retries are idempotent.
	return os.Remove(filepath.Join(s.Dir, "quarantine", id))
}
