package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"time"
)

type DB struct {
	*sql.DB
}

func New(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(100)
	conn.SetMaxIdleConns(50)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{conn}, nil
}

// ── Albums ──────────────────────────────────────────────────────────────────

func (d *DB) UpsertAlbum(albumID, title, description, owner string) error {
	_, err := d.Exec(`
		INSERT INTO albums (album_id, title, description, owner)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (album_id) DO UPDATE
		  SET title=$2, description=$3, owner=$4
	`, albumID, title, description, owner)
	return err
}

func (d *DB) GetAlbum(albumID string) (string, string, string, string, error) {
	var id, title, desc, owner string
	err := d.QueryRow(`
		SELECT album_id, title, description, owner FROM albums WHERE album_id=$1
	`, albumID).Scan(&id, &title, &desc, &owner)
	return id, title, desc, owner, err
}

func (d *DB) ListAlbums() ([]map[string]string, error) {
	rows, err := d.Query(`SELECT album_id, title, description, owner FROM albums`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var albums []map[string]string
	for rows.Next() {
		var id, title, desc, owner string
		if err := rows.Scan(&id, &title, &desc, &owner); err != nil {
			return nil, err
		}
		albums = append(albums, map[string]string{
			"album_id":    id,
			"title":       title,
			"description": desc,
			"owner":       owner,
		})
	}
	if albums == nil {
		albums = []map[string]string{}
	}
	return albums, nil
}

// ── Photos ──────────────────────────────────────────────────────────────────

func (d *DB) InsertPhoto(photoID, albumID string, seq int64, s3Key string) error {
	_, err := d.Exec(`
		INSERT INTO photos (photo_id, album_id, seq, status, s3_key)
		VALUES ($1, $2, $3, 'processing', $4)
	`, photoID, albumID, seq, s3Key)
	return err
}

func (d *DB) GetPhoto(albumID, photoID string) (string, string, int64, string, *string, error) {
	var pid, aid, status string
	var seq int64
	var url sql.NullString
	err := d.QueryRow(`
		SELECT photo_id, album_id, seq, status, url
		FROM photos
		WHERE album_id=$1 AND photo_id=$2
	`, albumID, photoID).Scan(&pid, &aid, &seq, &status, &url)
	var urlPtr *string
	if url.Valid {
		urlPtr = &url.String
	}
	return pid, aid, seq, status, urlPtr, err
}

func (d *DB) MarkPhotoCompleted(photoID, url string) error {
	_, err := d.Exec(`
		UPDATE photos SET status='completed', url=$2 WHERE photo_id=$1
	`, photoID, url)
	return err
}

func (d *DB) MarkPhotoFailed(photoID string) error {
	_, err := d.Exec(`UPDATE photos SET status='failed' WHERE photo_id=$1`, photoID)
	return err
}

func (d *DB) GetPhotoS3Key(photoID string) (string, error) {
	var key string
	err := d.QueryRow(`SELECT s3_key FROM photos WHERE photo_id=$1`, photoID).Scan(&key)
	return key, err
}

func (d *DB) DeletePhoto(albumID, photoID string) error {
	_, err := d.Exec(`DELETE FROM photos WHERE album_id=$1 AND photo_id=$2`, albumID, photoID)
	return err
}
