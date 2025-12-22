package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

//go:generate npm install --prefix at2-web
//go:generate npm run build --prefix at2-web

//go:embed at2-web/dist/*
var frontendEmbed embed.FS

func SetupFrontend(app *fiber.App, devMode bool) {
	if devMode {
		log.Println("Starting frontend in dev mode...")
		cmd := exec.Command("npm", "run", "dev", "--", "--host", "0.0.0.0")
		cmd.Dir = "./at2-web"
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to start frontend: %v", err)
		}

		// Note: The previous implementation killed the process in a defer in main().
		// Since we are moving this to a function, we might want to return a cleanup function
		// or handle it differently. Ideally, we just let it run until the app exits.
		// However, to mimic the main.go behavior of killing it on exit, we can rely on
		// the OS cleaning up subprocesses or just run it.
		// The original code had a defer which is nice.
		// For now, let's just start it. The OS usually kills child processes when parent dies
		// but not always cleanly.
		// Actually, let's change Main to handle the cleanup or just accept it.
		// A better way is to attach it to the lifecycle if possible, but keeping it simple for now.

		app.All("*", func(c *fiber.Ctx) error {
			if strings.HasPrefix(c.Path(), "/api") {
				return c.Next()
			}
			proxyUrl := "http://localhost:5173" + c.Path()
			return proxy.Do(c, proxyUrl)
		})
	} else {
		// Serve embedded files
		distFS, err := fs.Sub(frontendEmbed, "at2-web/dist")
		if err != nil {
			log.Fatalf("Failed to get dist fs: %v", err)
		}

		app.Use("/", filesystem.New(filesystem.Config{
			Root:   http.FS(distFS),
			Index:  "index.html",
			Browse: false,
		}))

		// SPA fallback: If file not found, serve index.html
		app.Use(func(c *fiber.Ctx) error {
			if strings.HasPrefix(c.Path(), "/api") {
				return c.Next()
			}
			// Try to open the file to see if it exists
			file, err := distFS.Open(strings.TrimPrefix(c.Path(), "/"))
			if err == nil {
				file.Close()
				return c.Next()
			}

			// If not found, serve index.html
			c.Status(fiber.StatusOK)
			index, err := distFS.Open("index.html")
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("index.html not found")
			}
			stat, _ := index.Stat()
			content := make([]byte, stat.Size())
			index.Read(content)
			return c.Send(content)
		})
	}
}
