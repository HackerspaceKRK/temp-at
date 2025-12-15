package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
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

const (
	CookieName = "access_token"
)

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
	oauth2Token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to exchange token: " + err.Error())
	}

	// Extract the ID Token from OAuth2 token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("No id_token field in oauth2 token.")
	}

	// Verify the ID Token signature and expiration.
	provider, err := oidc.NewProvider(ctx, ConfigInstance.Oidc.IssuerURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get provider: " + err.Error())
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: ConfigInstance.Oidc.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to verify ID Token: " + err.Error())
	}

	// Get the claims
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse claims: " + err.Error())
	}

	// Generate a JWT for our session
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": claims.PreferredUsername,
		"exp":      time.Now().Add(30 * 24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(ConfigInstance.Web.JWTSecret))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to sign token: " + err.Error())
	}

	// Set the cookie
	c.Cookie(&fiber.Cookie{
		Name:     CookieName,
		Value:    tokenString,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   false, // set to true if using HTTPS
		SameSite: "Lax",
	})

	return c.Redirect("/")
}

func handleLogout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:    CookieName,
		Value:   "",
		Expires: time.Now().Add(-1 * time.Hour),
	})
	return c.SendStatus(fiber.StatusOK)
}

func handleMe(c *fiber.Ctx) error {
	cookie := c.Cookies(CookieName)
	if cookie == "" {
		return c.Status(fiber.StatusUnauthorized).SendString("Not logged in")
	}

	token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(ConfigInstance.Web.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Invalid claims")
	}

	return c.JSON(fiber.Map{
		"username": claims["username"],
	})
}
