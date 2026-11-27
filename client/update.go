package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	latestRelease  = "https://api.github.com/repos/LuizPaluski/concord/releases/latest"
	downloadPrefix = "https://github.com/LuizPaluski/concord/releases/download/"
	installerName  = "Concord-Instalador.exe"
)

//go:embed wails.json
var wailsJSON []byte

// A versão sai do wails.json, a mesma que vai para o instalador.
var version = func() string {
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsJSON, &cfg); err != nil {
		panic(err)
	}
	return cfg.Info.ProductVersion
}()

// Sem limite de tempo, um download travado deixaria o aviso preso em "Baixando…".
var client = &http.Client{Timeout: 2 * time.Minute}

type App struct {
	ctx         context.Context
	releasesURL string
}

type Update struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// CheckUpdate devolve a release mais nova do GitHub, ou nil se esta já é a última.
func (a *App) CheckUpdate() (*Update, error) {
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, a.releasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github respondeu %s", res.Status)
	}
	var rel struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return nil, err
	}
	latest := strings.TrimPrefix(rel.Tag, "v")
	if !newer(latest, version) {
		return nil, nil
	}
	for _, asset := range rel.Assets {
		if asset.Name == installerName && strings.HasPrefix(asset.URL, downloadPrefix) {
			return &Update{Version: latest, URL: asset.URL}, nil
		}
	}
	return nil, nil
}

// Install baixa o instalador, roda em modo silencioso e fecha o app. O instalador
// espera o .exe ser liberado e reabre o Concord quando termina.
func (a *App) Install(u Update) error {
	if !strings.HasPrefix(u.URL, downloadPrefix) {
		return errors.New("endereço de atualização fora do repositório do Concord")
	}
	dst := filepath.Join(os.TempDir(), "Concord-Instalador-"+u.Version+".exe")
	if err := download(a.ctx, u.URL, dst); err != nil {
		return err
	}
	if err := exec.Command(dst, "/S").Start(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

func download(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download respondeu %s", res.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// newer compara versões x.y.z; qualquer coisa fora desse formato nunca é mais nova.
func newer(latest, current string) bool {
	l, okL := parseVersion(latest)
	c, okC := parseVersion(current)
	if !okL || !okC {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}
