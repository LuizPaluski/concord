package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
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
	s := &server{db: db, hub: newHub(), lk: livekit{URL: "ws://lk", Key: "k", Secret: "s"}, files: t.TempDir()}
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

func TestAnexos(t *testing.T) {
	h := newHarness(t)
	invite, _ := h.s.bootstrapInvite()
	var ana, beto session
	h.call("POST", "/api/register", "", credentials{invite, "ana", "senha-forte"}, 200, &ana)
	var inv struct{ Code string }
	h.call("POST", "/api/servers/1/invites", ana.Token, nil, 200, &inv)
	h.call("POST", "/api/register", "", credentials{inv.Code, "beto", "senha-forte"}, 200, &beto)
	var chans []channel
	h.call("GET", "/api/servers/1/channels", ana.Token, nil, 200, &chans)
	msgs := "/api/channels/" + itoa(chans[0].ID) + "/messages"

	h.raw("POST", "/api/files?name=x.html", ana.Token, []byte("<html><script>alert(1)</script>"), 400)
	mp4 := append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), bytes.Repeat([]byte{7}, 2000)...)
	var a attachment
	json.Unmarshal(h.raw("POST", "/api/files?name=../../clipe.mp4", ana.Token, mp4, 200), &a)
	if a.Mime != "video/mp4" || a.Name != "clipe.mp4" || a.Size != int64(len(mp4)) {
		t.Fatalf("anexo errado: %+v", a)
	}

	h.call("POST", msgs, beto.Token, map[string]string{"file": a.ID}, 400, nil)
	h.call("POST", msgs, ana.Token, map[string]string{"file": a.ID}, 200, nil)
	var got []message
	h.call("GET", msgs, beto.Token, nil, 200, &got)
	if len(got) != 1 || got[0].File == nil || *got[0].File != a {
		t.Fatalf("mensagem sem o anexo: %+v", got)
	}

	req, _ := http.NewRequest("GET", h.ts.URL+"/files/"+a.ID, nil)
	req.Header.Set("Range", "bytes=0-9")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusPartialContent || res.Header.Get("Content-Type") != "video/mp4" {
		t.Fatalf("vídeo precisa aceitar Range: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
}

func TestTiroSoDoDono(t *testing.T) {
	h := newHarness(t)
	invite, _ := h.s.bootstrapInvite()
	var ana, beto session
	h.call("POST", "/api/register", "", credentials{invite, "ana", "senha-forte"}, 200, &ana)
	var inv struct{ Code string }
	h.call("POST", "/api/servers/1/invites", ana.Token, nil, 200, &inv)
	h.call("POST", "/api/register", "", credentials{inv.Code, "beto", "senha-forte"}, 200, &beto)
	var chans []channel
	h.call("GET", "/api/servers/1/channels", ana.Token, nil, 200, &chans)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dial := func(token string) *websocket.Conn {
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(h.ts.URL, "http")+"/ws?token="+token, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.CloseNow() })
		wsjson.Write(ctx, conn, map[string]any{"type": "voice", "channel": chans[1].ID})
		return conn
	}
	a, b := dial(ana.Token), dial(beto.Token)
	for inCall := 0; inCall < 2; {
		var ev struct{ Users []presence }
		if err := wsjson.Read(ctx, b, &ev); err != nil {
			t.Fatal(err)
		}
		inCall = 0
		for _, u := range ev.Users {
			if u.Voice == chans[1].ID {
				inCall++
			}
		}
	}

	// O tiro do beto é ignorado; o primeiro tiro que o beto recebe tem que ser o da ana.
	wsjson.Write(ctx, b, map[string]string{"type": "shot"})
	wsjson.Write(ctx, a, map[string]string{"type": "shot"})
	for {
		var ev map[string]any
		if err := wsjson.Read(ctx, b, &ev); err != nil {
			t.Fatal(err)
		}
		if ev["type"] == "shot" {
			if ev["from"] != "ana" {
				t.Fatalf("tiro de quem não é dono passou: %v", ev)
			}
			return
		}
	}
}

func TestSpotifyJunto(t *testing.T) {
	h := newHarness(t)
	invite, _ := h.s.bootstrapInvite()
	var ana, beto session
	h.call("POST", "/api/register", "", credentials{invite, "ana", "senha-forte"}, 200, &ana)
	var inv struct{ Code string }
	h.call("POST", "/api/servers/1/invites", ana.Token, nil, 200, &inv)
	h.call("POST", "/api/register", "", credentials{inv.Code, "beto", "senha-forte"}, 200, &beto)
	var chans []channel
	h.call("GET", "/api/servers/1/channels", ana.Token, nil, 200, &chans)
	sala := chans[1].ID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dial := func(token string) *websocket.Conn {
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(h.ts.URL, "http")+"/ws?token="+token, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.CloseNow() })
		return conn
	}
	// Lê até o próximo evento do Spotify, pulando presença.
	nextJam := func(conn *websocket.Conn) map[string]any {
		for {
			var ev map[string]any
			if err := wsjson.Read(ctx, conn, &ev); err != nil {
				t.Fatal(err)
			}
			if ev["type"] == "spotify" {
				j, _ := ev["jam"].(map[string]any)
				return j
			}
		}
	}

	a := dial(ana.Token)
	wsjson.Write(ctx, a, map[string]any{"type": "voice", "channel": sala})
	wsjson.Write(ctx, a, map[string]any{"type": "spotify", "state": map[string]any{"uri": "spotify:track:1"}})
	if j := nextJam(a); j["host"] != "ana" {
		t.Fatalf("anfitriã deveria receber o próprio jam: %v", j)
	}

	// Quem entra depois recebe a música que já está tocando.
	b := dial(beto.Token)
	wsjson.Write(ctx, b, map[string]any{"type": "voice", "channel": sala})
	if j := nextJam(b); j["host"] != "ana" || j["state"].(map[string]any)["uri"] != "spotify:track:1" {
		t.Fatalf("beto deveria pegar a música da ana: %v", j)
	}

	// beto não toma o lugar da ana; quando ela sai da call, o jam acaba.
	wsjson.Write(ctx, b, map[string]any{"type": "spotify", "state": map[string]any{"uri": "spotify:track:2"}})
	wsjson.Write(ctx, a, map[string]any{"type": "voice", "channel": 0})
	if j := nextJam(b); j != nil {
		t.Fatalf("jam deveria acabar quando a anfitriã sai: %v", j)
	}
}
