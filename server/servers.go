package main

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

type guild struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Owner int64  `json:"owner"`
}

// membership diz se u é dono do grupo; sql.ErrNoRows quando nem é membro.
func (s *server) membership(u user, serverID string) (owner bool, err error) {
	err = s.db.QueryRow(`
		SELECT g.owner_id IS ? FROM members m JOIN servers g ON g.id = m.server_id
		WHERE m.server_id = ? AND m.user_id = ?`, u.ID, serverID, u.ID).Scan(&owner)
	return owner, err
}

// channelAccess acha o canal só entre os grupos de que u participa.
func (s *server) channelAccess(u user, channelID string) (c channel, owner bool, err error) {
	err = s.db.QueryRow(`
		SELECT c.id, c.server_id, c.name, c.kind, g.owner_id IS ?
		FROM channels c
		JOIN servers g ON g.id = c.server_id
		JOIN members m ON m.server_id = c.server_id AND m.user_id = ?
		WHERE c.id = ?`, u.ID, u.ID, channelID).Scan(&c.ID, &c.Server, &c.Name, &c.Kind, &owner)
	return c, owner, err
}

// gate traduz o resultado de membership/channelAccess em resposta HTTP.
func gate(w http.ResponseWriter, err error, owner, needOwner bool) bool {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.Error(w, "não encontrado", http.StatusNotFound)
	case err != nil:
		fail(w, err)
	case needOwner && !owner:
		http.Error(w, "só o dono do grupo pode fazer isso", http.StatusForbidden)
	default:
		return true
	}
	return false
}

func (s *server) memberIDs(serverID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT user_id FROM members WHERE server_id = ?`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// notifyServer avisa os membros (e quem mais for passado) que o grupo mudou;
// o cliente recarrega o que precisa.
func (s *server) notifyServer(serverID int64, extra ...int64) {
	ids, err := s.memberIDs(serverID)
	if err != nil {
		log.Print(err)
		return
	}
	s.hub.sendTo(append(ids, extra...), map[string]any{"type": "server", "id": serverID})
}

func validName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	return name, n >= 1 && n <= 32
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func (s *server) listServers(w http.ResponseWriter, r *http.Request, u user) {
	rows, err := s.db.Query(`
		SELECT g.id, g.name, g.icon, coalesce(g.owner_id, 0)
		FROM servers g JOIN members m ON m.server_id = g.id
		WHERE m.user_id = ? ORDER BY g.id`, u.ID)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	guilds := []guild{}
	for rows.Next() {
		var g guild
		if err := rows.Scan(&g.ID, &g.Name, &g.Icon, &g.Owner); err != nil {
			fail(w, err)
			return
		}
		guilds = append(guilds, g)
	}
	if err := rows.Err(); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, guilds)
}

func (s *server) createServer(w http.ResponseWriter, r *http.Request, u user) {
	var in guild
	if !decode(w, r, &in) {
		return
	}
	g := guild{Owner: u.ID}
	var ok bool
	if g.Name, ok = validName(in.Name); !ok {
		http.Error(w, "nome do grupo precisa ter de 1 a 32 caracteres", http.StatusBadRequest)
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()
	err = tx.QueryRow(`INSERT INTO servers (name, owner_id) VALUES (?, ?) RETURNING id`, g.Name, u.ID).Scan(&g.ID)
	if err == nil {
		_, err = tx.Exec(`INSERT INTO members (server_id, user_id) VALUES (?, ?)`, g.ID, u.ID)
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO channels (server_id, name, kind) VALUES (?, 'geral', 'text'), (?, 'Sala', 'voice')`, g.ID, g.ID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, err)
		return
	}
	s.notifyServer(g.ID)
	writeJSON(w, g)
}

func (s *server) updateServer(w http.ResponseWriter, r *http.Request, u user) {
	owner, err := s.membership(u, r.PathValue("id"))
	if !gate(w, err, owner, true) {
		return
	}
	var in guild
	if !decode(w, r, &in) {
		return
	}
	name, ok := validName(in.Name)
	if !ok || !validMedia(in.Icon) {
		http.Error(w, "nome de 1 a 32 caracteres e ícone válido", http.StatusBadRequest)
		return
	}
	if _, err := s.db.Exec(`UPDATE servers SET name = ?, icon = ? WHERE id = ?`, name, in.Icon, pathID(r)); err != nil {
		fail(w, err)
		return
	}
	s.notifyServer(pathID(r))
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) deleteServer(w http.ResponseWriter, r *http.Request, u user) {
	owner, err := s.membership(u, r.PathValue("id"))
	if !gate(w, err, owner, true) {
		return
	}
	id := pathID(r)
	ids, err := s.memberIDs(id)
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
	for _, q := range []string{
		`DELETE FROM messages WHERE channel_id IN (SELECT id FROM channels WHERE server_id = ?)`,
		`DELETE FROM channels WHERE server_id = ?`,
		`DELETE FROM invites WHERE server_id = ?`,
		`DELETE FROM members WHERE server_id = ?`,
		`DELETE FROM servers WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			fail(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		fail(w, err)
		return
	}
	s.hub.sendTo(ids, map[string]any{"type": "server", "id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) leaveServer(w http.ResponseWriter, r *http.Request, u user) {
	owner, err := s.membership(u, r.PathValue("id"))
	if !gate(w, err, owner, false) {
		return
	}
	if owner {
		http.Error(w, "o dono não sai do grupo; dá para apagar o grupo", http.StatusBadRequest)
		return
	}
	if _, err := s.db.Exec(`DELETE FROM members WHERE server_id = ? AND user_id = ?`, pathID(r), u.ID); err != nil {
		fail(w, err)
		return
	}
	s.notifyServer(pathID(r), u.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listMembers(w http.ResponseWriter, r *http.Request, u user) {
	owner, err := s.membership(u, r.PathValue("id"))
	if !gate(w, err, owner, false) {
		return
	}
	ids, err := s.memberIDs(pathID(r))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, ids)
}

func (s *server) serverChannels(w http.ResponseWriter, r *http.Request, u user) {
	owner, err := s.membership(u, r.PathValue("id"))
	if !gate(w, err, owner, false) {
		return
	}
	s.writeChannels(w, `SELECT id, server_id, name, kind FROM channels WHERE server_id = ? ORDER BY id`, pathID(r))
}

func (s *server) writeChannels(w http.ResponseWriter, query string, args ...any) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	channels := []channel{}
	for rows.Next() {
		var c channel
		if err := rows.Scan(&c.ID, &c.Server, &c.Name, &c.Kind); err != nil {
			fail(w, err)
			return
		}
		channels = append(channels, c)
	}
	if err := rows.Err(); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, channels)
}

func (s *server) createChannel(w http.ResponseWriter, r *http.Request, u user) {
	owner, err := s.membership(u, r.PathValue("id"))
	if !gate(w, err, owner, true) {
		return
	}
	var c channel
	if !decode(w, r, &c) {
		return
	}
	var ok bool
	c.Name, ok = validName(c.Name)
	if !ok || (c.Kind != "text" && c.Kind != "voice") {
		http.Error(w, "nome de 1 a 32 caracteres e tipo text ou voice", http.StatusBadRequest)
		return
	}
	c.Server = pathID(r)
	err = s.db.QueryRow(`INSERT INTO channels (server_id, name, kind) VALUES (?, ?, ?) ON CONFLICT DO NOTHING RETURNING id`, c.Server, c.Name, c.Kind).Scan(&c.ID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "já existe um canal com esse nome", http.StatusConflict)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	s.notifyServer(c.Server)
	writeJSON(w, c)
}

func (s *server) renameChannel(w http.ResponseWriter, r *http.Request, u user) {
	c, owner, err := s.channelAccess(u, r.PathValue("id"))
	if !gate(w, err, owner, true) {
		return
	}
	var in channel
	if !decode(w, r, &in) {
		return
	}
	name, ok := validName(in.Name)
	if !ok {
		http.Error(w, "nome de 1 a 32 caracteres", http.StatusBadRequest)
		return
	}
	res, err := s.db.Exec(`UPDATE OR IGNORE channels SET name = ? WHERE id = ?`, name, c.ID)
	if err != nil {
		fail(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 && name != c.Name {
		http.Error(w, "já existe um canal com esse nome", http.StatusConflict)
		return
	}
	s.notifyServer(c.Server)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) deleteChannel(w http.ResponseWriter, r *http.Request, u user) {
	c, owner, err := s.channelAccess(u, r.PathValue("id"))
	if !gate(w, err, owner, true) {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec(`DELETE FROM messages WHERE channel_id = ?`, c.ID)
	if err == nil {
		_, err = tx.Exec(`DELETE FROM channels WHERE id = ?`, c.ID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, err)
		return
	}
	s.notifyServer(c.Server)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) createInvite(w http.ResponseWriter, r *http.Request, u user) {
	owner, err := s.membership(u, r.PathValue("id"))
	if !gate(w, err, owner, false) {
		return
	}
	code := rand.Text()
	if _, err := s.db.Exec(`INSERT INTO invites (code, server_id) VALUES (?, ?)`, code, pathID(r)); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]string{"code": code})
}

// joinServer usa um convite com uma conta que já existe.
func (s *server) joinServer(w http.ResponseWriter, r *http.Request, u user) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	code := strings.TrimSpace(in.Invite)
	tx, err := s.db.Begin()
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRow(`SELECT server_id FROM invites WHERE code = ? AND used_by IS NULL`, code).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "convite inválido ou já usado", http.StatusForbidden)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	res, err := tx.Exec(`INSERT OR IGNORE INTO members (server_id, user_id) VALUES (?, ?)`, id, u.ID)
	if err != nil {
		fail(w, err)
		return
	}
	// Quem já é membro não gasta o convite.
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "você já está nesse grupo", http.StatusConflict)
		return
	}
	if _, err := tx.Exec(`UPDATE invites SET used_by = ? WHERE code = ?`, u.ID, code); err != nil {
		fail(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, err)
		return
	}
	s.notifyServer(id)
	writeJSON(w, map[string]int64{"id": id})
}
