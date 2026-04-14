package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"album-store/internal/db"
	"album-store/internal/models"
	"album-store/internal/queue"
	"album-store/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	DB       *db.DB
	Redis    *redis.Client
	S3       *storage.S3Store
	SQS      *queue.SQSQueue
	S3Bucket string
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, models.HealthResponse{Status: "ok"})
}

func (h *Handler) PutAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	var body models.Album
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, models.ErrorResponse{Error: "invalid body"})
		return
	}
	_, _, _, _, err := h.DB.GetAlbum(albumID)
	isNew := err != nil
	if err := h.DB.UpsertAlbum(albumID, body.Title, body.Description, body.Owner); err != nil {
		log.Printf("upsert album: %v", err)
		writeJSON(w, 500, models.ErrorResponse{Error: "internal error"})
		return
	}
	status := 200
	if isNew {
		status = 201
	}
	writeJSON(w, status, models.Album{
		AlbumID:     albumID,
		Title:       body.Title,
		Description: body.Description,
		Owner:       body.Owner,
	})
}

func (h *Handler) GetAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	id, title, desc, owner, err := h.DB.GetAlbum(albumID)
	if err != nil {
		writeJSON(w, 404, models.ErrorResponse{Error: "not found"})
		return
	}
	writeJSON(w, 200, models.Album{AlbumID: id, Title: title, Description: desc, Owner: owner})
}

func (h *Handler) ListAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := h.DB.ListAlbums()
	if err != nil {
		writeJSON(w, 500, models.ErrorResponse{Error: "internal error"})
		return
	}
	writeJSON(w, 200, albums)
}

func (h *Handler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")

	if err := r.ParseMultipartForm(210 << 20); err != nil {
		writeJSON(w, 400, models.ErrorResponse{Error: "invalid multipart"})
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		writeJSON(w, 400, models.ErrorResponse{Error: "missing photo field"})
		return
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, file); err != nil {
		writeJSON(w, 500, models.ErrorResponse{Error: "read error"})
		return
	}

	photoID := uuid.New().String()
	s3Key := fmt.Sprintf("photos/%s/%s", albumID, photoID)

	seqKey := fmt.Sprintf("seq:%s", albumID)
	seq, err := h.Redis.Incr(context.Background(), seqKey).Result()
	if err != nil {
		log.Printf("redis incr: %v", err)
		writeJSON(w, 500, models.ErrorResponse{Error: "internal error"})
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.DB.InsertPhoto(photoID, albumID, seq, s3Key); err != nil {
		log.Printf("insert photo: %v", err)
		writeJSON(w, 500, models.ErrorResponse{Error: "db error"})
		return
	}

	// Return 202 immediately
	writeJSON(w, 202, models.PhotoAccepted{
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
	})

	// Upload and mark completed in background
	// Use copies of all data — request context is gone after 202
	data := make([]byte, buf.Len())
	copy(data, buf.Bytes())
	ct := contentType

	go func() {
		ctx := context.Background()

		uploadURL, err := h.S3.Upload(ctx, s3Key, bytes.NewReader(data), ct)
		if err != nil {
			log.Printf("s3 upload photo=%s: %v", photoID, err)
			h.DB.MarkPhotoFailed(photoID)
			return
		}

		// Mark completed directly — don't wait for worker
		if err := h.DB.MarkPhotoCompleted(photoID, uploadURL); err != nil {
			log.Printf("mark completed photo=%s: %v", photoID, err)
			return
		}

		// Also publish to SQS for durability (worker is a no-op if already completed)
		h.SQS.Publish(ctx, struct {
			PhotoID string `json:"photo_id"`
			AlbumID string `json:"album_id"`
			S3Key   string `json:"s3_key"`
			URL     string `json:"url"`
		}{PhotoID: photoID, AlbumID: albumID, S3Key: s3Key, URL: uploadURL})
	}()
}

func (h *Handler) GetPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	photoID := chi.URLParam(r, "photo_id")
	pid, aid, seq, status, url, err := h.DB.GetPhoto(albumID, photoID)
	if err != nil {
		writeJSON(w, 404, models.ErrorResponse{Error: "not found"})
		return
	}
	writeJSON(w, 200, models.Photo{
		PhotoID: pid,
		AlbumID: aid,
		Seq:     seq,
		Status:  status,
		URL:     url,
	})
}

func (h *Handler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	photoID := chi.URLParam(r, "photo_id")

	s3Key, err := h.DB.GetPhotoS3Key(photoID)
	if err != nil {
		w.WriteHeader(204)
		return
	}
	if err := h.S3.Delete(r.Context(), s3Key); err != nil {
		log.Printf("s3 delete: %v", err)
	}
	if err := h.DB.DeletePhoto(albumID, photoID); err != nil {
		log.Printf("db delete: %v", err)
		writeJSON(w, 500, models.ErrorResponse{Error: "db error"})
		return
	}
	w.WriteHeader(204)
}
