package main

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

type profile struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Banner string `json:"banner"`
	Color  string `json:"color"`
	Color2 string `json:"color2"`
	Effect string `json:"effect"`
	Bio    string `json:"bio"`
}

const maxMedia = 8 << 20

var (
	hexColor  = regexp.MustCompile(`^(#[0-9a-f]{6})?$`)
	mediaID   = regexp.MustCompile(`^([A-Z2-7]{26})?$`)
	effects   = []string{"", "gradient", "glow", "rainbow"}
	imageMIME = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}
)

// validMedia aceita vazio ou um id gerado por uploadMedia, nunca uma URL.
func validMedia(id string) bool { return mediaID.MatchString(id) }

// listUsers devolve quem divide pelo menos um grupo com u.
func (s *server) listUsers(w http.ResponseWriter, r *http.Request, u user) {
	rows, err := s.db.Query(`
		SELECT id, name, avatar, banner, color, color2, effect, bio FROM users
		WHERE id IN (SELECT user_id FROM members WHERE server_id IN (SELECT server_id FROM members WHERE user_id = ?))
		   OR id = ?`, u.ID, u.ID)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	users := []profile{}
	for rows.Next() {
		var p profile
		if err := rows.Scan(&p.ID, &p.Name, &p.Avatar, &p.Banner, &p.Color, &p.Color2, &p.Effect, &p.Bio); err != nil {
			fail(w, err)
			return
		}
		users = append(users, p)
	}
	if err := rows.Err(); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, users)
}

func (s *server) updateProfile(w http.ResponseWriter, r *http.Request, u user) {
	var p profile
	if !decode(w, r, &p) {
		return
	}
	p.ID, p.Name = u.ID, u.Name
	p.Bio = strings.TrimSpace(p.Bio)
	p.Color, p.Color2 = strings.ToLower(p.Color), strings.ToLower(p.Color2)
	if !validMedia(p.Avatar) || !validMedia(p.Banner) || !hexColor.MatchString(p.Color) || !hexColor.MatchString(p.Color2) ||
		!slices.Contains(effects, p.Effect) || utf8.RuneCountInString(p.Bio) > 190 {
		http.Error(w, "perfil inválido", http.StatusBadRequest)
		return
	}
	_, err := s.db.Exec(`UPDATE users SET avatar = ?, banner = ?, color = ?, color2 = ?, effect = ?, bio = ? WHERE id = ?`,
		p.Avatar, p.Banner, p.Color, p.Color2, p.Effect, p.Bio, u.ID)
	if err != nil {
		fail(w, err)
		return
	}
	// ponytail: imagem antiga fica no banco; limpar órfãs se o volume crescer.
	s.hub.broadcast(map[string]any{"type": "profile", "user": p})
	writeJSON(w, p)
}

// uploadMedia guarda a imagem no próprio SQLite: para 10 pessoas não vale um storage à parte.
func (s *server) uploadMedia(w http.ResponseWriter, r *http.Request, _ user) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMedia))
	if err != nil {
		http.Error(w, "imagem maior que 8 MB", http.StatusRequestEntityTooLarge)
		return
	}
	// O tipo vem do conteúdo, não do header: assim não dá para subir HTML ou SVG.
	mime := http.DetectContentType(data)
	if !slices.Contains(imageMIME, mime) {
		http.Error(w, "só PNG, JPEG, GIF ou WebP", http.StatusBadRequest)
		return
	}
	id := rand.Text()
	if _, err := s.db.Exec(`INSERT INTO media (id, mime, data) VALUES (?, ?, ?)`, id, mime, data); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]string{"id": id})
}

// media é pública porque <img> não manda o token; o id aleatório faz o papel de senha.
func (s *server) media(w http.ResponseWriter, r *http.Request) {
	var mime string
	var data []byte
	err := s.db.QueryRow(`SELECT mime, data FROM media WHERE id = ?`, r.PathValue("id")).Scan(&mime, &data)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(data)
}
