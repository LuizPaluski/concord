package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	dbPath := env("CONCORD_DB", "concord.db")
	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	files := filepath.Join(filepath.Dir(dbPath), "files")
	if err := os.MkdirAll(files, 0o755); err != nil {
		log.Fatal(err)
	}
	s := &server{
		db:    db,
		hub:   newHub(),
		files: files,
		lk:    livekit{URL: os.Getenv("LIVEKIT_URL"), Key: os.Getenv("LIVEKIT_KEY"), Secret: os.Getenv("LIVEKIT_SECRET")},
	}
	if s.lk.URL == "" || s.lk.Key == "" || s.lk.Secret == "" {
		log.Fatal("defina LIVEKIT_URL, LIVEKIT_KEY e LIVEKIT_SECRET")
	}
	code, err := s.bootstrapInvite()
	if err != nil {
		log.Fatal(err)
	}
	if code != "" {
		log.Printf("nenhum usuário cadastrado, convite inicial: %s", code)
	}
	addr := env("CONCORD_ADDR", ":8080")
	log.Printf("concord ouvindo em %s", addr)
	log.Fatal(http.ListenAndServe(addr, s.routes()))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
