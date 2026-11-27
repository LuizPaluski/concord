package main

import (
	"log"
	"net/http"
	"path"
	"time"
)

const latestRelease = "https://github.com/LuizPaluski/concord/releases/latest"

// latestTag lê a tag da release mais nova pelo redirecionamento do GitHub,
// que não conta no limite de 60 chamadas por hora da API.
func latestTag(client *http.Client) (string, error) {
	res, err := client.Head(latestRelease)
	if err != nil {
		return "", err
	}
	res.Body.Close()
	return path.Base(res.Header.Get("Location")), nil
}

// watchReleases avisa todo app aberto quando sai versão nova, para o card de
// atualização aparecer sem ninguém precisar reabrir o Concord.
func (s *server) watchReleases(every time.Duration) {
	client := &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	known := ""
	for ; ; time.Sleep(every) {
		tag, err := latestTag(client)
		if err != nil || tag == "" || tag == "latest" {
			if err != nil {
				log.Print(err)
			}
			continue
		}
		if known != "" && tag != known {
			log.Printf("nova release %s, avisando os apps abertos", tag)
			s.hub.broadcast(map[string]any{"type": "release", "tag": tag})
		}
		known = tag
	}
}
