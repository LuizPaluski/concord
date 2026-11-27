# Concord

Chat de texto, voz e compartilhamento de tela para um grupo pequeno de amigos.

- `server/`: API e WebSocket em Go, SQLite num arquivo só.
- `client/`: app desktop em Wails + Svelte.
- `deploy/`: Caddy (HTTPS), servidor Concord e LiveKit (voz e tela) via Docker Compose.

Entrar só com convite. Enquanto não houver usuário, o servidor imprime um convite inicial no log. Depois disso, qualquer pessoa logada gera convites pelo botão no canto inferior esquerdo.

## Rodando local

```sh
docker run --rm -p 7880:7880 -p 7881:7881 -p 7882:7882/udp \
  livekit/livekit-server:v1.13.11 --dev --bind 0.0.0.0 --node-ip 127.0.0.1

cd server
LIVEKIT_URL=ws://localhost:7880 LIVEKIT_KEY=devkey LIVEKIT_SECRET=secret go run .

# em outro terminal
cd client
wails dev
```

No app, o servidor é `http://localhost:8080`.

## Produção

Na VPS, com Docker instalado:

```sh
git clone <repo> concord && cd concord/deploy
cp .env.example .env   # preencha os domínios e gere o LIVEKIT_SECRET
docker compose up -d --build
docker compose logs concord   # pega o convite inicial
```

Os dois domínios precisam apontar para o IP da VPS. Sem domínio próprio, `chat.<ip>.sslip.io` e `lk.<ip>.sslip.io` funcionam sem configurar nada.

Portas a liberar no firewall: `80/tcp`, `443/tcp`, `7881/tcp` e `7882/udp`.

Backup é copiar o volume `concord` (um arquivo SQLite).

## Gerando o executável

```sh
cd client
wails build
```

Sai em `client/build/bin/concord.exe`. Para gerar instalador, `wails build -nsis` (precisa do NSIS instalado).
