[parallel]
dev: dev-go dev-wind

tidy:
  go mod tidy

dev-go: tidy
  go tool templ generate --watch --proxy="http://localhost:3000" --cmd="go run . --config-path=config.yaml"

dev-wind:
  tailwindcss -i ./input.css -o ./public/css/style.css --watch

wind:
  tailwindcss -i ./input.css -o ./public/css/style.css

run: tidy wind
	go tool templ generate && go run . --config-path=config.yaml

build: wind
	go tool templ generate && go build
