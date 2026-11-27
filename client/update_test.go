package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.3.0", "0.2.0", true},
		{"0.10.0", "0.9.0", true},
		{"1.0.0", "0.99.99", true},
		{"0.2.1", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.1.9", "0.2.0", false},
		{"lixo", "0.2.0", false},
	}
	for _, c := range cases {
		if got := newer(c.latest, c.current); got != c.want {
			t.Errorf("newer(%q, %q) = %v, esperado %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestCheckUpdate(t *testing.T) {
	release := func(tag, asset, url string) string {
		return fmt.Sprintf(`{"tag_name":%q,"assets":[{"name":"outro.zip","browser_download_url":"x"},{"name":%q,"browser_download_url":%q}]}`, tag, asset, url)
	}
	official := downloadPrefix + "v9.0.0/Concord-Instalador.exe"
	cases := []struct {
		name, body string
		want       *Update
	}{
		{"versão nova", release("v9.0.0", "Concord-Instalador.exe", official), &Update{Version: "9.0.0", URL: official}},
		{"mesma versão", release("v"+version, "Concord-Instalador.exe", official), nil},
		{"sem instalador", release("v9.0.0", "outra-coisa.exe", official), nil},
		{"link fora do repositório", release("v9.0.0", "Concord-Instalador.exe", "https://exemplo.com/virus.exe"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, c.body) }))
			defer srv.Close()
			a := &App{ctx: context.Background(), releasesURL: srv.URL}
			got, err := a.CheckUpdate()
			if err != nil {
				t.Fatal(err)
			}
			if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
				t.Fatalf("CheckUpdate = %+v, esperado %+v", got, c.want)
			}
		})
	}
}

func TestCheckUpdateFalhaHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	a := &App{ctx: context.Background(), releasesURL: srv.URL}
	if _, err := a.CheckUpdate(); err == nil {
		t.Fatal("esperava erro com status 403")
	}
}

func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "MZ instalador") }))
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "setup.exe")
	if err := download(context.Background(), srv.URL, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "MZ instalador" {
		t.Fatalf("conteúdo baixado errado: %q", b)
	}
}

func TestVersionVemDoWailsJSON(t *testing.T) {
	if version == "" || !newer(version, "0.0.0") {
		t.Fatalf("versão embutida inválida: %q", version)
	}
}
