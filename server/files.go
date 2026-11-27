package main

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// Anexo do chat. Fica em disco, não no SQLite, para o vídeo poder ser
// servido com Range (avançar e voltar sem baixar tudo).
type attachment struct {
	ID   string `json:"id"`
	Mime string `json:"mime"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

const maxFile = 100 << 20

var fileMIME = append([]string{"video/mp4", "video/webm"}, imageMIME...)

func (s *server) uploadFile(w http.ResponseWriter, r *http.Request, u user) {
	name := strings.TrimSpace(filepath.Base(r.URL.Query().Get("name")))
	if utf8.RuneCountInString(name) > 120 {
		name = string([]rune(name)[:120])
	}
	tmp, err := os.CreateTemp(s.files, "upload-*")
	if err != nil {
		fail(w, err)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	size, err := io.Copy(tmp, http.MaxBytesReader(w, r.Body, maxFile))
	if err != nil {
		http.Error(w, "arquivo maior que 100 MB", http.StatusRequestEntityTooLarge)
		return
	}
	head := make([]byte, 512)
	n, _ := tmp.ReadAt(head, 0)
	a := attachment{ID: rand.Text(), Mime: http.DetectContentType(head[:n]), Name: name, Size: size}
	if !slices.Contains(fileMIME, a.Mime) {
		http.Error(w, "só foto (PNG, JPEG, GIF, WebP) ou vídeo (MP4, WebM)", http.StatusBadRequest)
		return
	}
	if err := tmp.Close(); err != nil {
		fail(w, err)
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(s.files, a.ID)); err != nil {
		fail(w, err)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO files (id, mime, name, size, user_id) VALUES (?, ?, ?, ?, ?)`, a.ID, a.Mime, a.Name, a.Size, u.ID); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, a)
}

// file é pública pelo mesmo motivo de media: <img> e <video> não mandam token.
func (s *server) file(w http.ResponseWriter, r *http.Request) {
	var mime string
	id := r.PathValue("id")
	err := s.db.QueryRow(`SELECT mime FROM files WHERE id = ?`, id).Scan(&mime)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	f, err := os.Open(filepath.Join(s.files, id))
	if err != nil {
		fail(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, "", info.ModTime(), f)
}
