package main

import (
	"bytes"
	"context"
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

func TestFluxoCompleto(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &server{db: db, hub: newHub(), lk: livekit{URL: "ws://lk", Key: "k", Secret: "s"}}
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	call := func(method, path, token string, body any, want int) []byte {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, ts.URL+path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out bytes.Buffer
		out.ReadFrom(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s: status %d, esperado %d: %s", method, path, res.StatusCode, want, out.String())
		}
		return out.Bytes()
	}

	invite, err := s.bootstrapInvite()
	if err != nil || invite == "" {
		t.Fatalf("convite inicial: %q %v", invite, err)
	}

	var ana session
	json.Unmarshal(call("POST", "/api/register", "", credentials{invite, "ana", "senha-forte"}, 200), &ana)
	call("POST", "/api/register", "", credentials{invite, "beto", "senha-forte"}, 403)
	call("POST", "/api/register", "", credentials{"inventado", "beto", "senha-forte"}, 403)
	if code, _ := s.bootstrapInvite(); code != "" {
		t.Fatal("bootstrap não deveria gerar convite com usuário cadastrado")
	}

	var inv struct{ Code string }
	json.Unmarshal(call("POST", "/api/invites", ana.Token, nil, 200), &inv)
	call("POST", "/api/register", "", credentials{inv.Code, "ANA", "senha-forte"}, 409)
	call("POST", "/api/register", "", credentials{inv.Code, "beto", "senha-forte"}, 200)

	call("POST", "/api/login", "", credentials{Name: "ana", Password: "errada!!"}, 401)
	var login session
	json.Unmarshal(call("POST", "/api/login", "", credentials{Name: "ana", Password: "senha-forte"}, 200), &login)
	call("GET", "/api/channels", "token-falso", nil, 401)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws?token="+login.Token, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	var ev map[string]any
	if err := wsjson.Read(ctx, conn, &ev); err != nil || ev["type"] != "presence" {
		t.Fatalf("primeiro evento deveria ser presence: %v %v", ev, err)
	}

	var channels []channel
	json.Unmarshal(call("GET", "/api/channels", login.Token, nil, 200), &channels)
	text, voice := channels[0], channels[1]
	if text.Kind != "text" || voice.Kind != "voice" {
		t.Fatalf("canais padrão inesperados: %+v", channels)
	}

	call("POST", "/api/channels/999/messages", login.Token, map[string]string{"body": "oi"}, 404)
	call("POST", "/api/channels/"+itoa(voice.ID)+"/messages", login.Token, map[string]string{"body": "oi"}, 404)
	call("POST", "/api/channels/"+itoa(text.ID)+"/messages", login.Token, map[string]string{"body": "   "}, 400)
	call("POST", "/api/channels/"+itoa(text.ID)+"/messages", login.Token, map[string]string{"body": " oi, gente "}, 200)

	if err := wsjson.Read(ctx, conn, &ev); err != nil || ev["type"] != "message" {
		t.Fatalf("esperava evento message: %v %v", ev, err)
	}
	if m := ev["message"].(map[string]any); m["body"] != "oi, gente" || m["author"] != "ana" {
		t.Fatalf("mensagem errada: %v", m)
	}

	var msgs []message
	json.Unmarshal(call("GET", "/api/channels/"+itoa(text.ID)+"/messages", login.Token, nil, 200), &msgs)
	if len(msgs) != 1 || msgs[0].Body != "oi, gente" {
		t.Fatalf("histórico errado: %+v", msgs)
	}

	wsjson.Write(ctx, conn, map[string]any{"type": "voice", "channel": voice.ID})
	if err := wsjson.Read(ctx, conn, &ev); err != nil || ev["type"] != "presence" {
		t.Fatalf("esperava presence após entrar na voz: %v %v", ev, err)
	}
	if u := ev["users"].([]any)[0].(map[string]any); u["voice"] != float64(voice.ID) {
		t.Fatalf("presença de voz errada: %v", u)
	}

	call("GET", "/api/channels/"+itoa(text.ID)+"/voice", login.Token, nil, 404)
	var vt struct{ URL, Token string }
	json.Unmarshal(call("GET", "/api/channels/"+itoa(voice.ID)+"/voice", login.Token, nil, 200), &vt)
	if vt.URL != "ws://lk" || strings.Count(vt.Token, ".") != 2 {
		t.Fatalf("token de voz inválido: %+v", vt)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }
