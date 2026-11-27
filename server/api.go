package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"golang.org/x/crypto/bcrypt"
)

type server struct {
	db  *sql.DB
	hub *hub
	lk  livekit
}

type user struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type channel struct {
	ID     int64  `json:"id"`
	Server int64  `json:"server"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
}

type message struct {
	ID        int64  `json:"id"`
	Channel   int64  `json:"channel"`
	Author    string `json:"author"`
	AuthorID  int64  `json:"authorId"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"createdAt"`
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", s.register)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("GET /api/users", s.auth(s.listUsers))
	mux.HandleFunc("POST /api/profile", s.auth(s.updateProfile))
	mux.HandleFunc("POST /api/media", s.auth(s.uploadMedia))
	mux.HandleFunc("GET /media/{id}", s.media)
	mux.HandleFunc("GET /api/servers", s.auth(s.listServers))
	mux.HandleFunc("POST /api/servers", s.auth(s.createServer))
	mux.HandleFunc("POST /api/servers/{id}", s.auth(s.updateServer))
	mux.HandleFunc("DELETE /api/servers/{id}", s.auth(s.deleteServer))
	mux.HandleFunc("POST /api/servers/{id}/leave", s.auth(s.leaveServer))
	mux.HandleFunc("GET /api/servers/{id}/members", s.auth(s.listMembers))
	mux.HandleFunc("GET /api/servers/{id}/channels", s.auth(s.serverChannels))
	mux.HandleFunc("POST /api/servers/{id}/channels", s.auth(s.createChannel))
	mux.HandleFunc("POST /api/servers/{id}/invites", s.auth(s.createInvite))
	mux.HandleFunc("POST /api/join", s.auth(s.joinServer))
	mux.HandleFunc("POST /api/channels/{id}", s.auth(s.renameChannel))
	mux.HandleFunc("DELETE /api/channels/{id}", s.auth(s.deleteChannel))
	mux.HandleFunc("GET /api/channels/{id}/messages", s.auth(s.listMessages))
	mux.HandleFunc("POST /api/channels/{id}/messages", s.auth(s.postMessage))
	mux.HandleFunc("GET /api/channels/{id}/voice", s.auth(s.voiceToken))
	// Rotas da 0.5.0, até todo mundo atualizar.
	mux.HandleFunc("GET /api/channels", s.auth(s.legacyChannels))
	mux.HandleFunc("POST /api/invites", s.auth(s.legacyInvite))
	mux.HandleFunc("GET /ws", s.ws)
	return cors(mux)
}

// O cliente desktop roda em outra origem (wails://). A autenticação é por
// bearer token, sem cookie, então liberar qualquer origem não abre CSRF.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) auth(next func(http.ResponseWriter, *http.Request, user)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := s.userByToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil {
			http.Error(w, "não autenticado", http.StatusUnauthorized)
			return
		}
		next(w, r, u)
	}
}

func (s *server) userByToken(token string) (user, error) {
	var u user
	err := s.db.QueryRow(`SELECT u.id, u.name FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token = ?`, token).
		Scan(&u.ID, &u.Name)
	return u, err
}

// bootstrapInvite garante um convite enquanto não houver ninguém cadastrado,
// senão o primeiro usuário não teria como entrar.
func (s *server) bootstrapInvite() (string, error) {
	var users int
	if err := s.db.QueryRow(`SELECT count(*) FROM users`).Scan(&users); err != nil || users > 0 {
		return "", err
	}
	var code string
	err := s.db.QueryRow(`SELECT code FROM invites WHERE used_by IS NULL LIMIT 1`).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		code = rand.Text()
		_, err = s.db.Exec(`INSERT INTO invites (code) VALUES (?)`, code)
	}
	return code, err
}

type credentials struct {
	Invite   string `json:"invite"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type session struct {
	Token string `json:"token"`
	User  user   `json:"user"`
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	c.Name = strings.TrimSpace(c.Name)
	if n := utf8.RuneCountInString(c.Name); n < 2 || n > 32 || len(c.Password) < 8 || len(c.Password) > 72 {
		http.Error(w, "nome precisa ter de 2 a 32 caracteres e senha de 8 a 72", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(w, err)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM users WHERE name = ?)`, c.Name).Scan(&exists); err != nil {
		fail(w, err)
		return
	}
	if exists {
		http.Error(w, "esse nome já está em uso", http.StatusConflict)
		return
	}
	u := user{Name: c.Name}
	if err := tx.QueryRow(`INSERT INTO users (name, pass) VALUES (?, ?) RETURNING id`, c.Name, hash).Scan(&u.ID); err != nil {
		fail(w, err)
		return
	}
	var serverID int64
	err = tx.QueryRow(`UPDATE invites SET used_by = ? WHERE code = ? AND used_by IS NULL RETURNING server_id`, u.ID, c.Invite).Scan(&serverID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "convite inválido ou já usado", http.StatusForbidden)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	// O primeiro cadastro num banco novo vira dono do grupo inicial.
	_, err = tx.Exec(`INSERT INTO members (server_id, user_id) VALUES (?, ?)`, serverID, u.ID)
	if err == nil {
		_, err = tx.Exec(`UPDATE servers SET owner_id = ? WHERE id = ? AND owner_id IS NULL`, u.ID, serverID)
	}
	if err != nil {
		fail(w, err)
		return
	}
	sess, err := newSession(tx, u)
	if err != nil {
		fail(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, err)
		return
	}
	s.notifyServer(serverID)
	writeJSON(w, sess)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	var u user
	var hash []byte
	err := s.db.QueryRow(`SELECT id, name, pass FROM users WHERE name = ?`, strings.TrimSpace(c.Name)).Scan(&u.ID, &u.Name, &hash)
	if err == nil {
		err = bcrypt.CompareHashAndPassword(hash, []byte(c.Password))
	}
	if err != nil {
		http.Error(w, "nome ou senha incorretos", http.StatusUnauthorized)
		return
	}
	sess, err := newSession(s.db, u)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, sess)
}

// q é o *sql.DB ou a *sql.Tx do cadastro.
func newSession(q interface {
	Exec(string, ...any) (sql.Result, error)
}, u user) (session, error) {
	sess := session{Token: rand.Text(), User: u}
	_, err := q.Exec(`INSERT INTO sessions (token, user_id) VALUES (?, ?)`, sess.Token, u.ID)
	return sess, err
}

func (s *server) legacyChannels(w http.ResponseWriter, r *http.Request, u user) {
	s.writeChannels(w, `SELECT c.id, c.server_id, c.name, c.kind FROM channels c JOIN members m ON m.server_id = c.server_id WHERE m.user_id = ? ORDER BY c.id`, u.ID)
}

func (s *server) legacyInvite(w http.ResponseWriter, r *http.Request, u user) {
	code := rand.Text()
	res, err := s.db.Exec(`INSERT INTO invites (code, server_id) SELECT ?, min(server_id) FROM members WHERE user_id = ? HAVING count(*) > 0`, code, u.ID)
	if err != nil {
		fail(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "você não está em nenhum grupo", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"code": code})
}

func (s *server) listMessages(w http.ResponseWriter, r *http.Request, u user) {
	c, owner, err := s.channelAccess(u, r.PathValue("id"))
	if !gate(w, err, owner, false) {
		return
	}
	before := int64(math.MaxInt64)
	if v := r.URL.Query().Get("before"); v != "" {
		if before, err = strconv.ParseInt(v, 10, 64); err != nil {
			http.Error(w, "before inválido", http.StatusBadRequest)
			return
		}
	}
	rows, err := s.db.Query(`
		SELECT m.id, m.channel_id, u.name, u.id, m.body, m.created_at
		FROM messages m JOIN users u ON u.id = m.user_id
		WHERE m.channel_id = ? AND m.id < ?
		ORDER BY m.id DESC LIMIT 50`, c.ID, before)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	msgs := []message{}
	for rows.Next() {
		var m message
		if err := rows.Scan(&m.ID, &m.Channel, &m.Author, &m.AuthorID, &m.Body, &m.CreatedAt); err != nil {
			fail(w, err)
			return
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		fail(w, err)
		return
	}
	slices.Reverse(msgs)
	writeJSON(w, msgs)
}

func (s *server) postMessage(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	m := message{Author: u.Name, AuthorID: u.ID, Body: strings.TrimSpace(in.Body)}
	if n := utf8.RuneCountInString(m.Body); n < 1 || n > 4000 {
		http.Error(w, "mensagem precisa ter de 1 a 4000 caracteres", http.StatusBadRequest)
		return
	}
	c, owner, err := s.channelAccess(u, r.PathValue("id"))
	if err == nil && c.Kind != "text" {
		err = sql.ErrNoRows
	}
	if !gate(w, err, owner, false) {
		return
	}
	m.Channel = c.ID
	err = s.db.QueryRow(`INSERT INTO messages (channel_id, user_id, body) VALUES (?, ?, ?) RETURNING id, created_at`, c.ID, u.ID, m.Body).
		Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		fail(w, err)
		return
	}
	ids, err := s.memberIDs(c.Server)
	if err != nil {
		fail(w, err)
		return
	}
	s.hub.sendTo(ids, map[string]any{"type": "message", "server": c.Server, "message": m})
	writeJSON(w, m)
}

func (s *server) voiceToken(w http.ResponseWriter, r *http.Request, u user) {
	c, owner, err := s.channelAccess(u, r.PathValue("id"))
	if err == nil && c.Kind != "voice" {
		err = sql.ErrNoRows
	}
	if !gate(w, err, owner, false) {
		return
	}
	writeJSON(w, map[string]string{
		"url":   s.lk.URL,
		"token": s.lk.token(u.Name, "channel-"+strconv.FormatInt(c.ID, 10)),
	})
}

func (s *server) ws(w http.ResponseWriter, r *http.Request) {
	// Navegador não manda header em WebSocket, então o token vem na query.
	u, err := s.userByToken(r.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, "não autenticado", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	c := &client{user: u, send: make(chan []byte, 64)}
	s.hub.add(c)
	defer s.hub.remove(c)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		defer conn.CloseNow()
		ping := time.NewTicker(30 * time.Second)
		defer ping.Stop()
		for {
			select {
			case b, ok := <-c.send:
				if !ok {
					return
				}
				wctx, done := context.WithTimeout(ctx, 10*time.Second)
				err := conn.Write(wctx, websocket.MessageText, b)
				done()
				if err != nil {
					return
				}
			case <-ping.C:
				if err := conn.Ping(ctx); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		var ev struct {
			Type string `json:"type"`
			voiceState
		}
		if err := wsjson.Read(ctx, conn, &ev); err != nil {
			return
		}
		if ev.Type == "voice" {
			s.hub.setVoice(c, ev.voiceState)
		}
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(v); err != nil {
		http.Error(w, "json inválido", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, err error) {
	log.Print(err)
	http.Error(w, "erro interno", http.StatusInternalServerError)
}
