package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"komanook/content"
)

type fixture struct {
	Slug, Title, Creator, Genre, Format, Synopsis, Cover, Rights, Chapter string
	Pages                                                                 []struct {
		Number, Width, Height int
		Image, Transcript     string
	}
}

func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	var fixtures []fixture
	if err := json.Unmarshal(content.Fixtures, &fixtures); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, s := range fixtures {
		var workID, seriesID, chapterID int64
		if err = tx.QueryRow(ctx, `INSERT INTO works(slug,title,creator,genre,cover) VALUES($1,$2,$3,$4,$5) ON CONFLICT(slug) DO UPDATE SET title=EXCLUDED.title,creator=EXCLUDED.creator,genre=EXCLUDED.genre,cover=EXCLUDED.cover RETURNING id`, s.Slug, s.Title, s.Creator, s.Genre, s.Cover).Scan(&workID); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `INSERT INTO series(work_id,slug,title,creator,genre,format,synopsis,cover,rights,published) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,true) ON CONFLICT(slug) DO UPDATE SET title=EXCLUDED.title,work_id=EXCLUDED.work_id RETURNING id`, workID, s.Slug, s.Title, s.Creator, s.Genre, s.Format, s.Synopsis, s.Cover, s.Rights).Scan(&seriesID); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `INSERT INTO chapters(series_id,number,title,published) VALUES($1,1,$2,true) ON CONFLICT(series_id,number) DO UPDATE SET title=EXCLUDED.title RETURNING id`, seriesID, s.Chapter).Scan(&chapterID); err != nil {
			return err
		}
		for _, p := range s.Pages {
			if _, err = tx.Exec(ctx, `INSERT INTO pages(chapter_id,number,image,width,height,transcript) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(chapter_id,number) DO UPDATE SET image=EXCLUDED.image,width=EXCLUDED.width,height=EXCLUDED.height,transcript=EXCLUDED.transcript`, chapterID, p.Number, p.Image, p.Width, p.Height, p.Transcript); err != nil {
				return fmt.Errorf("seed page: %w", err)
			}
		}
	}
	return tx.Commit(ctx)
}
