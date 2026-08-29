package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"wisp/internal/api"
)

const schema = `
CREATE TABLE IF NOT EXISTS feeds (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	url            TEXT NOT NULL UNIQUE,
	title          TEXT NOT NULL,
	kind           TEXT NOT NULL,
	icon           BLOB,
	icon_type      TEXT NOT NULL DEFAULT '',
	title_override TEXT
);

CREATE TABLE IF NOT EXISTS items (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	feed_id         INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
	guid            TEXT NOT NULL,
	title           TEXT NOT NULL,
	link            TEXT NOT NULL,
	pub_date        TEXT NOT NULL DEFAULT '',
	audio_url       TEXT NOT NULL DEFAULT '',
	description     TEXT NOT NULL DEFAULT '',
	content_encoded TEXT NOT NULL DEFAULT '',
	download_filename TEXT NOT NULL DEFAULT '',
	transcript_url  TEXT NOT NULL DEFAULT '',
	transcript_type TEXT NOT NULL DEFAULT '',
	UNIQUE(feed_id, guid)
);
`

type SQLiteStore struct {
	db *sql.DB
}

// Open uses a single connection: SQLite serializes writers anyway, so this
// skips needing our own mutex.
func Open(path string) (*SQLiteStore, error) {
	dbConn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	dbConn.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := dbConn.Exec(pragma); err != nil {
			dbConn.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}

	if _, err := dbConn.Exec(schema); err != nil {
		dbConn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	// migrate old dbs; errors ignored (already-migrated or never-had-it, both fine)
	for _, stmt := range []string{
		"ALTER TABLE feeds ADD COLUMN icon BLOB",
		"ALTER TABLE feeds ADD COLUMN icon_type TEXT NOT NULL DEFAULT ''",
		"UPDATE feeds SET icon = favicon, icon_type = favicon_type WHERE icon IS NULL AND favicon IS NOT NULL",
		"ALTER TABLE feeds ADD COLUMN title_override TEXT",
		"ALTER TABLE items ADD COLUMN download_filename TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE items ADD COLUMN transcript_url TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE items ADD COLUMN transcript_type TEXT NOT NULL DEFAULT ''",
	} {
		_, _ = dbConn.Exec(stmt)
	}

	if err := migrateItemsGUID(dbConn); err != nil {
		dbConn.Close()
		return nil, fmt.Errorf("migrate items guid: %w", err)
	}

	return &SQLiteStore{db: dbConn}, nil
}

// migrateItemsGUID switches items' identity key from (feed_id, link) to
// (feed_id, guid): many feeds (e.g. Buzzsprout) omit <link> per item, which
// collapsed every one of their items into a single row under the old
// constraint. SQLite can't alter a table's UNIQUE constraint in place, so
// this recreates the table; existing rows backfill guid from their old
// link, which is safe — it was already unique under the constraint being
// replaced.
func migrateItemsGUID(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(items)")
	if err != nil {
		return err
	}
	hasGUID := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "guid" {
			hasGUID = true
		}
	}
	rows.Close()
	if hasGUID {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range []string{
		`ALTER TABLE items RENAME TO items_old`,
		`CREATE TABLE items (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			feed_id         INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
			guid            TEXT NOT NULL,
			title           TEXT NOT NULL,
			link            TEXT NOT NULL,
			pub_date        TEXT NOT NULL DEFAULT '',
			audio_url       TEXT NOT NULL DEFAULT '',
			description     TEXT NOT NULL DEFAULT '',
			content_encoded TEXT NOT NULL DEFAULT '',
			download_filename TEXT NOT NULL DEFAULT '',
			transcript_url  TEXT NOT NULL DEFAULT '',
			transcript_type TEXT NOT NULL DEFAULT '',
			UNIQUE(feed_id, guid)
		)`,
		`INSERT INTO items (id, feed_id, guid, title, link, pub_date, audio_url, description,
				content_encoded, download_filename, transcript_url, transcript_type)
			SELECT id, feed_id, link, title, link, pub_date, audio_url, description,
				content_encoded, download_filename, transcript_url, transcript_type
			FROM items_old`,
		`DROP TABLE items_old`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func feedKindToStr(k api.FeedKind) string {
	if k == api.FeedKindPodcast {
		return "podcast"
	}
	return "article"
}

func feedKindFromStr(s string) api.FeedKind {
	if s == "podcast" {
		return api.FeedKindPodcast
	}
	return api.FeedKindArticle
}

const feedColumns = "id, url, title, kind, icon, icon_type, title_override"

// title_override, when set, survives UpsertFeed's title overwrite on refresh.
func scanFeed(row interface{ Scan(...any) error }) (api.Feed, error) {
	var f api.Feed
	var kindStr string
	var titleOverride sql.NullString
	err := row.Scan(&f.ID, &f.URL, &f.Title, &kindStr, &f.Icon, &f.IconMime, &titleOverride)
	if err != nil {
		return api.Feed{}, err
	}
	f.Kind = feedKindFromStr(kindStr)
	if titleOverride.Valid && titleOverride.String != "" {
		f.Title = titleOverride.String
	}
	return f, nil
}

// UpsertFeed leaves an existing feed's icon untouched; use SetFeedIcon for that.
func (s *SQLiteStore) UpsertFeed(ctx context.Context, url, title string, kind api.FeedKind) (api.Feed, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO feeds (url, title, kind) VALUES (?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET title = excluded.title, kind = excluded.kind
		RETURNING `+feedColumns,
		url, title, feedKindToStr(kind))

	f, err := scanFeed(row)
	if err != nil {
		return api.Feed{}, fmt.Errorf("upsert feed: %w", err)
	}
	return f, nil
}

func (s *SQLiteStore) ListFeeds(ctx context.Context) ([]api.Feed, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+feedColumns+" FROM feeds ORDER BY title")
	if err != nil {
		return nil, fmt.Errorf("list feeds: %w", err)
	}
	defer rows.Close()

	var feeds []api.Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, fmt.Errorf("scan feed: %w", err)
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

func (s *SQLiteStore) GetFeed(ctx context.Context, feedID int64) (*api.Feed, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+feedColumns+" FROM feeds WHERE id = ?", feedID)
	f, err := scanFeed(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get feed %d: %w", feedID, err)
	}
	return &f, nil
}

func (s *SQLiteStore) SetFeedIcon(ctx context.Context, feedID int64, data []byte, mimeType string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE feeds SET icon = ?, icon_type = ? WHERE id = ?`,
		data, mimeType, feedID); err != nil {
		return fmt.Errorf("set icon for feed %d: %w", feedID, err)
	}
	return nil
}

// UpdateFeed sets feedID's title override (empty clears it) and URL.
func (s *SQLiteStore) UpdateFeed(ctx context.Context, feedID int64, title, url string) (api.Feed, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE feeds SET title_override = NULLIF(?, ''), url = ? WHERE id = ?
		RETURNING `+feedColumns,
		title, url, feedID)

	f, err := scanFeed(row)
	if err != nil {
		return api.Feed{}, fmt.Errorf("update feed %d: %w", feedID, err)
	}
	return f, nil
}

func (s *SQLiteStore) DeleteFeed(ctx context.Context, feedID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM feeds WHERE id = ?`, feedID); err != nil {
		return fmt.Errorf("delete feed %d: %w", feedID, err)
	}
	return nil
}

func (s *SQLiteStore) UpsertItems(ctx context.Context, feedID int64, items []api.Item) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("upsert items: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO items (feed_id, guid, title, link, pub_date, audio_url, description, content_encoded, transcript_url, transcript_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(feed_id, guid) DO UPDATE SET
			title = excluded.title,
			link = excluded.link,
			pub_date = excluded.pub_date,
			audio_url = excluded.audio_url,
			description = excluded.description,
			content_encoded = excluded.content_encoded,
			transcript_url = excluded.transcript_url,
			transcript_type = excluded.transcript_type`)
	if err != nil {
		return fmt.Errorf("upsert items: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		if _, err := stmt.ExecContext(ctx, feedID, item.GUID, item.Title, item.Link, item.PubDate,
			item.AudioURL, item.Description, item.ContentEncoded,
			item.TranscriptURL, item.TranscriptType); err != nil {
			return fmt.Errorf("upsert item %q: %w", item.GUID, err)
		}
	}

	return tx.Commit()
}

const itemColumns = "id, feed_id, guid, title, link, pub_date, audio_url, description, content_encoded, download_filename, transcript_url, transcript_type"

func scanItem(row interface{ Scan(...any) error }) (api.Item, error) {
	var it api.Item
	err := row.Scan(&it.ID, &it.FeedID, &it.GUID, &it.Title, &it.Link, &it.PubDate,
		&it.AudioURL, &it.Description, &it.ContentEncoded, &it.DownloadFilename,
		&it.TranscriptURL, &it.TranscriptType)
	return it, err
}

func (s *SQLiteStore) ListItems(ctx context.Context, feedID *int64) ([]api.Item, error) {
	query := "SELECT " + itemColumns + " FROM items"
	args := []any{}
	if feedID != nil {
		query += " WHERE feed_id = ?"
		args = append(args, *feedID)
	}
	query += " ORDER BY pub_date DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	var items []api.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetItem(ctx context.Context, itemID int64) (*api.Item, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM items WHERE id = ?", itemID)
	item, err := scanItem(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get item %d: %w", itemID, err)
	}
	return &item, nil
}

func (s *SQLiteStore) SetItemDownload(ctx context.Context, itemID int64, filename string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE items SET download_filename = ? WHERE id = ?`,
		filename, itemID); err != nil {
		return fmt.Errorf("set download for item %d: %w", itemID, err)
	}
	return nil
}
