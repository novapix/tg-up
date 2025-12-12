go build -ldflags "-s -w \
-X tg-up/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
-X tg-up/version.CommitHash=$(git rev-parse --short HEAD)" \
-o tg-up main.go
