package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

type harness struct {
	t  *testing.T
	s  *server
	ts *httptest.Server
}

func newHarness(t *testing.T) *harness {
	db, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &server{db: db, hub: newHub(), lk: livekit{URL: "ws://lk", Key: "k", Secret: "s"}}
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	return &harness{t, s, ts}
}

func (h *harness) raw(method, path, token string, body []byte, want int) []byte {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.ts.URL+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	var out bytes.Buffer
	out.ReadFrom(res.Body)
	if res.StatusCode != want {
		h.t.Fatalf("%s %s: status %d, esperado %d: %s", method, path, res.StatusCode, want, out.String())
	}
	return out.Bytes()
}

func (h *harness) call(method, path, token string, body any, want int, out any) {
	h.t.Helper()
	b, _ := json.Marshal(body)
	res := h.raw(method, path, token, b, want)
	if out != nil {
		json.Unmarshal(res, out)
	}
}

func TestGrupos(t *testing.T) {
	h := newHarness(t)
	invite, _ := h.s.bootstrapInvite()
	var ana, beto session
	h.call("POST", "/api/register", "", credentials{invite, "ana", "senha-forte"}, 200, &ana)

	var guilds []guild
	h.call("GET", "/api/servers", ana.Token, nil, 200, &guilds)
	if len(guilds) != 1 || guilds[0].Owner != ana.User.ID {
		t.Fatalf("primeiro cadastro deveria ser dono do grupo inicial: %+v", guilds)
	}
	concord := itoa(guilds[0].ID)

	var g guild
	h.call("POST", "/api/servers", ana.Token, guild{Name: "Jogos"}, 200, &g)
	jogos := itoa(g.ID)
	var inv struct{ Code string }
	h.call("POST", "/api/servers/"+concord+"/invites", ana.Token, nil, 200, &inv)
	h.call("POST", "/api/register", "", credentials{inv.Code, "beto", "senha-forte"}, 200, &beto)

	// beto está só no Concord: o grupo Jogos e seus canais não existem para ele.
	h.call("GET", "/api/servers", beto.Token, nil, 200, &guilds)
	if len(guilds) != 1 {
		t.Fatalf("beto deveria ver um grupo: %+v", guilds)
	}
	var chans []channel
	h.call("GET", "/api/servers/"+jogos+"/channels", ana.Token, nil, 200, &chans)
	geralJogos := itoa(chans[0].ID)
	h.call("GET", "/api/servers/"+jogos+"/channels", beto.Token, nil, 404, nil)
	h.call("GET", "/api/channels/"+geralJogos+"/messages", beto.Token, nil, 404, nil)
	h.call("POST", "/api/channels/"+geralJogos+"/messages", beto.Token, map[string]string{"body": "oi"}, 404, nil)
	h.call("GET", "/api/channels/"+itoa(chans[1].ID)+"/voice", beto.Token, nil, 404, nil)

	// Só o dono mexe nos canais.
	h.call("GET", "/api/servers/"+concord+"/channels", beto.Token, nil, 200, &chans)
	geral := itoa(chans[0].ID)
	h.call("POST", "/api/servers/"+concord+"/channels", beto.Token, channel{Name: "memes", Kind: "text"}, 403, nil)
	h.call("DELETE", "/api/channels/"+geral, beto.Token, nil, 403, nil)
	h.call("POST", "/api/channels/"+geral, beto.Token, channel{Name: "x"}, 403, nil)
	h.call("DELETE", "/api/servers/"+concord, beto.Token, nil, 403, nil)

	var memes channel
	h.call("POST", "/api/servers/"+concord+"/channels", ana.Token, channel{Name: "memes", Kind: "text"}, 200, &memes)
	h.call("POST", "/api/servers/"+jogos+"/channels", ana.Token, channel{Name: "memes", Kind: "text"}, 200, nil)
	h.call("POST", "/api/channels/"+itoa(memes.ID), ana.Token, channel{Name: "geral"}, 409, nil)
	h.call("POST", "/api/channels/"+itoa(memes.ID), ana.Token, channel{Name: "memes-2"}, 204, nil)
	h.call("POST", "/api/channels/"+itoa(memes.ID)+"/messages", beto.Token, map[string]string{"body": "kkk"}, 200, nil)
	h.call("DELETE", "/api/channels/"+itoa(memes.ID), ana.Token, nil, 204, nil)
	h.call("GET", "/api/channels/"+itoa(memes.ID)+"/messages", ana.Token, nil, 404, nil)

	// Convite de grupo para quem já tem conta; repetir não gasta outro convite.
	h.call("POST", "/api/servers/"+jogos+"/invites", ana.Token, nil, 200, &inv)
	h.call("POST", "/api/join", beto.Token, credentials{Invite: inv.Code}, 200, nil)
	h.call("POST", "/api/servers/"+jogos+"/invites", ana.Token, nil, 200, &inv)
	h.call("POST", "/api/join", beto.Token, credentials{Invite: inv.Code}, 409, nil)
	var members []int64
	h.call("GET", "/api/servers/"+jogos+"/members", beto.Token, nil, 200, &members)
	if len(members) != 2 {
		t.Fatalf("Jogos deveria ter 2 membros: %v", members)
	}
	h.call("POST", "/api/servers/"+jogos+"/leave", ana.Token, nil, 400, nil)
	h.call("POST", "/api/servers/"+jogos+"/leave", beto.Token, nil, 204, nil)
	h.call("GET", "/api/servers/"+jogos+"/members", beto.Token, nil, 404, nil)

	h.call("DELETE", "/api/servers/"+jogos, ana.Token, nil, 204, nil)
	h.call("GET", "/api/servers", ana.Token, nil, 200, &guilds)
	if len(guilds) != 1 {
		t.Fatalf("Jogos deveria ter sumido: %+v", guilds)
	}
}

func TestPerfil(t *testing.T) {
	h := newHarness(t)
	invite, _ := h.s.bootstrapInvite()
	var ana session
	h.call("POST", "/api/register", "", credentials{invite, "ana", "senha-forte"}, 200, &ana)

	h.raw("POST", "/api/media", ana.Token, []byte("<html><script>alert(1)</script>"), 400)
	gif := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")
	var up struct{ ID string }
	json.Unmarshal(h.raw("POST", "/api/media", ana.Token, gif, 200), &up)
	if got := h.raw("GET", "/media/"+up.ID, "", nil, 200); !bytes.Equal(got, gif) {
		t.Fatal("imagem voltou diferente")
	}

	h.call("POST", "/api/profile", ana.Token, profile{Avatar: "https://evil.example/x.png"}, 400, nil)
	h.call("POST", "/api/profile", ana.Token, profile{Effect: "explodir"}, 400, nil)
	h.call("POST", "/api/profile", ana.Token, profile{Color: "red"}, 400, nil)
	want := profile{Avatar: up.ID, Color: "#ff00aa", Color2: "#00ffcc", Effect: "gradient", Bio: "oi"}
	h.call("POST", "/api/profile", ana.Token, want, 200, nil)
	var users []profile
	h.call("GET", "/api/users", ana.Token, nil, 200, &users)
	want.ID, want.Name = ana.User.ID, "ana"
	if len(users) != 1 || users[0] != want {
		t.Fatalf("perfil salvo errado: %+v", users)
	}
}

// O banco de produção nasceu na versão 0; a migração precisa levar tudo para o grupo 1.
func TestMigraBancoAntigo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.Exec(migrations[0] + `
		INSERT INTO users (name, pass) VALUES ('luiz', x''), ('kramer', x'');
		INSERT INTO messages (channel_id, user_id, body) VALUES (1, 2, 'salve');
		INSERT INTO invites (code) VALUES ('ABERTO');`)
	old.Close()
	if err != nil {
		t.Fatal(err)
	}

	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var owner, members, inServer1, invite1 int
	var body string
	db.QueryRow(`SELECT owner_id FROM servers WHERE id = 1`).Scan(&owner)
	db.QueryRow(`SELECT count(*) FROM members WHERE server_id = 1`).Scan(&members)
	db.QueryRow(`SELECT count(*) FROM channels WHERE server_id = 1`).Scan(&inServer1)
	db.QueryRow(`SELECT server_id FROM invites WHERE code = 'ABERTO'`).Scan(&invite1)
	db.QueryRow(`SELECT body FROM messages WHERE channel_id = 1`).Scan(&body)
	if owner != 1 || members != 2 || inServer1 != 2 || invite1 != 1 || body != "salve" {
		t.Fatalf("migração errada: dono %d, membros %d, canais %d, convite %d, msg %q", owner, members, inServer1, invite1, body)
	}
	var fk int
	db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	if fk != 1 {
		t.Fatal("chaves estrangeiras ficaram desligadas depois da migração")
	}
}
