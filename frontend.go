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
			c.Type("html", "utf-8")
			index, err := distFS.Open("index.html")
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("index.html not found")
			}
			defer index.Close()
			stat, _ := index.Stat()
			content := make([]byte, stat.Size())
			if _, err := index.Read(content); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("failed to read index.html")
			}
			return c.Send(content)
		})
	}
}
