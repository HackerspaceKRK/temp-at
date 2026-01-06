package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	oauth2Config *oauth2.Config
	oidcProvider *oidc.Provider
)

func initAuth() error {
	oidcConfig := ConfigInstance.Oidc
	if oidcConfig == nil {
		log.Printf("OIDC not configured, authorization is not available")
		return nil
	}

	ctx := context.Background()

	var err error
	oidcProvider, err = oidc.NewProvider(ctx, oidcConfig.IssuerURL)
	if err != nil {
		return fmt.Errorf("failed to initialize OIDC provider: %w", err)
	}

	scopes := []string{oidc.ScopeOpenID, "profile", "email"}
	if oidcConfig.ExtraScopes != nil {
		scopes = append(scopes, oidcConfig.ExtraScopes...)
	}

	oauth2Config = &oauth2.Config{
		ClientID:     oidcConfig.ClientID,
		ClientSecret: oidcConfig.ClientSecret,
		RedirectURL:  ConfigInstance.Web.PublicURL + "/api/v1/auth/callback",

		// Discovery returns the OAuth2 endpoints.
		Endpoint: oidcProvider.Endpoint(),

		// "openid" is a required scope for OpenID Connect flows.
		Scopes: scopes,
	}

	return nil
}

const (
	CookieName = "session_id"
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
	verifier := oidcProvider.Verifier(&oidc.Config{ClientID: ConfigInstance.Oidc.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to verify ID Token: " + err.Error())
	}

	// Get the claims
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Sub               string `json:"sub"`
		Sid               string `json:"sid"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse claims: " + err.Error())
	}

	db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to connect to database: " + err.Error())
	}

	session := SessionModel{
		ID:           GenerateUUIDv7(),
		Subject:      claims.Sub,
		IdPSessionID: claims.Sid,
		Username:     claims.PreferredUsername,
		AccessToken:  oauth2Token.AccessToken,
		RefreshToken: oauth2Token.RefreshToken,
		ExpiresAt:    oauth2Token.Expiry,
	}

	if err := db.Create(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create session: " + err.Error())
	}

	// Set the cookie
	c.Cookie(&fiber.Cookie{
		Name:     CookieName,
		Value:    session.ID,
		Expires:  time.Now().Add(30 * 24 * time.Hour), // Keep consistent with previous logic, or match token expiry?
		HTTPOnly: true,
		Secure:   false, // set to true if using HTTPS
		SameSite: "Lax",
	})

	return c.Redirect("/")
}

func handleLogout(c *fiber.Ctx) error {
	cookie := c.Cookies(CookieName)
	if cookie != "" {
		db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
		if err == nil {
			db.Delete(&SessionModel{}, "id = ?", cookie)
		}
	}

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

	db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Database error")
	}

	var session SessionModel
	if err := db.First(&session, "id = ?", cookie).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid session")
	}

	// Reconstruct the token
	token := &oauth2.Token{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		Expiry:       session.ExpiresAt,
		TokenType:    "Bearer",
	}

	ctx := context.Background()
	tokenSource := oauth2Config.TokenSource(ctx, token)

	// Get a fresh token (this will refresh if needed)
	newToken, err := tokenSource.Token()
	if err != nil {
		// If we can't refresh, the session is effectively invalid for OIDC purposes,
		// but maybe we still want to allow access if the session in DB is still valid?
		// The user request explicitly said "When calling handleMe call the OIDC userinfo endpoint".
		// If that fails, we probably should return error or at least fail the request.
		// However, let's just log it and maybe return 401 if we strictly need OIDC info.
		// Given the requirement, I'll error out.
		return c.Status(fiber.StatusUnauthorized).SendString("Failed to refresh token: " + err.Error())
	}

	// Update session if token changed
	if newToken.AccessToken != session.AccessToken {
		session.AccessToken = newToken.AccessToken
		session.RefreshToken = newToken.RefreshToken
		session.ExpiresAt = newToken.Expiry
		if err := db.Save(&session).Error; err != nil {
			log.Printf("Failed to update session with new token: %v", err)
		}
	}

	userInfo, err := oidcProvider.UserInfo(ctx, tokenSource)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user info: " + err.Error())
	}

	var claims map[string]interface{}
	if err := userInfo.Claims(&claims); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse user info claims: " + err.Error())
	}

	return c.JSON(claims)
}

func AuthMiddleware(c *fiber.Ctx) error {
	cookie := c.Cookies(CookieName)
	if cookie == "" {
		return c.Status(fiber.StatusUnauthorized).SendString("Not logged in")
	}

	db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Database error")
	}

	var session SessionModel
	if err := db.First(&session, "id = ?", cookie).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid session")
	}

	// Store user info in context for downstream handlers
	c.Locals("username", session.Username)

	return c.Next()
}

func handleBackchannelLogout(c *fiber.Ctx) error {
	logoutToken := c.FormValue("logout_token")
	if logoutToken == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing logout_token")
	}

	// Verify the logout token
	// It's a JWT. We need to verify signature and claims.
	// The key used to sign it is from the IdP.

	// We need a provider to get the verifier.
	// Assuming initAuth has run and configured things, but we might need to create a new provider instance if we want to be safe,
	// or re-use a global one if we had it. initAuth creates a local provider variable.
	// Let's create a new one.
	ctx := context.Background()

	verifier := oidcProvider.Verifier(&oidc.Config{ClientID: ConfigInstance.Oidc.ClientID})
	token, err := verifier.Verify(ctx, logoutToken)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid logout_token: " + err.Error())
	}

	var claims struct {
		Sid string `json:"sid"`
		Sub string `json:"sub"`
	}
	if err := token.Claims(&claims); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Failed to parse claims")
	}

	db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Database error")
	}

	if claims.Sid != "" {
		db.Delete(&SessionModel{}, "id_p_session_id = ?", claims.Sid)
	} else if claims.Sub != "" {
		// If sid is missing, logout all sessions for the user (sub)
		db.Delete(&SessionModel{}, "subject = ?", claims.Sub)
	}

	return c.SendStatus(fiber.StatusOK)
}
