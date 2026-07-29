package lifecycle

import (
	"context"
	"database/sql"
	"log"
	"time"

	"html-preview/internal/item"
)

const TrashRetention = 30 * 24 * time.Hour

type Service struct {
	DB      *sql.DB
	DataDir string
	items   *item.Service
}

func New(db *sql.DB, dataDir string) *Service {
	return &Service{
		DB:      db,
		DataDir: dataDir,
		items:   &item.Service{DB: db, DataDir: dataDir},
	}
}

func (s *Service) ExpireActive(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	result, err := s.DB.ExecContext(ctx, `
		UPDATE items
		SET status = 'trash', trashed_at = ?, updated_at = ?
		WHERE status = 'active'
		  AND expires_at IS NOT NULL
		  AND expires_at <= ?
	`, nowStr, nowStr, nowStr)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *Service) PurgeTrash(ctx context.Context) (int, error) {
	cutoff := time.Now().UTC().Add(-TrashRetention).Format(time.RFC3339)

	rows, err := s.DB.QueryContext(ctx, `
		SELECT id FROM items
		WHERE status = 'trash'
		  AND trashed_at IS NOT NULL
		  AND trashed_at <= ?
	`, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	purged := 0
	for _, id := range ids {
		if err := s.items.PermanentDelete(id); err != nil {
			log.Printf("lifecycle: purge item %s: %v", id, err)
			continue
		}
		purged++
	}
	return purged, nil
}

func (s *Service) RunOnce(ctx context.Context) {
	expired, err := s.ExpireActive(ctx)
	if err != nil {
		log.Printf("lifecycle: expire active: %v", err)
	} else if expired > 0 {
		log.Printf("lifecycle: moved %d expired item(s) to trash", expired)
	}

	purged, err := s.PurgeTrash(ctx)
	if err != nil {
		log.Printf("lifecycle: purge trash: %v", err)
	} else if purged > 0 {
		log.Printf("lifecycle: permanently deleted %d item(s) from trash", purged)
	}
}

func (s *Service) Run(ctx context.Context, interval time.Duration) {
	s.RunOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

func ParseInterval(raw string) time.Duration {
	if raw == "" {
		return time.Minute
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return time.Minute
	}
	return d
}
