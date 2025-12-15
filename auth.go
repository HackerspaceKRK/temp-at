package main

import (
	"context"
	"fmt"
	"log"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
)

var oauth2Config *oauth2.Config

func initAuth() error {
	oidcConfig := ConfigInstance.Oidc
	if oidcConfig == nil {
		log.Printf("OIDC not configured, authorization is not available")
		return nil
	}

	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, oidcConfig.IssuerURL)
	if err != nil {
		return fmt.Errorf("failed to initialize OIDC provider: %w", err)
	}

	oauth2Config = &oauth2.Config{
		ClientID:     oidcConfig.ClientID,
		ClientSecret: oidcConfig.ClientSecret,
		RedirectURL:  ConfigInstance.Web.PublicURL + "/api/v1/auth/callback",

		// Discovery returns the OAuth2 endpoints.
		Endpoint: provider.Endpoint(),

		// "openid" is a required scope for OpenID Connect flows.
		Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return nil
}

func handleLoginRequest(c *fiber.Ctx) error {
	if oauth2Config == nil {
		return c.Status(fiber.StatusUnauthorized).SendString("OIDC not configured")
	}

	authCodeURL := oauth2Config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	return c.Redirect(authCodeURL, fiber.StatusFound)
}

func handleAuthCallback(c *fiber.Ctx) error {
	if oauth2Config == nil {
		return c.Status(fiber.StatusUnauthorized).SendString("OIDC not configured")
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing code in callback")
	}

	ctx := context.Background()
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to exchange token: " + err.Error())
	}

	// Here you can extract user info from the token if needed.
	// For simplicity, we just return the access token.
	return c.JSON(fiber.Map{
		"access_token":  token.AccessToken,
		"token_type":    token.TokenType,
		"refresh_token": token.RefreshToken,
		"expiry":        token.Expiry,
	})
}
