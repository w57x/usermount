[parallel]
dev: dev-go dev-wind

dev-go:
  go tool templ generate --watch --proxy="http://localhost:3000" --cmd="go run ."

dev-wind:
  tailwindcss -i ./input.css -o ./public/css/style.css --watch

wind:
  tailwindcss -i ./input.css -o ./public/css/style.css

run: wind
	go tool templ generate && go run usermount.go

build: wind
	go tool templ generate && go build
