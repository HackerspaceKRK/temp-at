package main

import (
	"context"
	"encoding/json"
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
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "OIDC not configured"})
	}

	authCodeURL := oauth2Config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	return c.Redirect(authCodeURL, fiber.StatusFound)
}

func handleAuthCallback(c *fiber.Ctx) error {
	if oauth2Config == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "OIDC not configured"})
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing code in callback"})
	}

	ctx := context.Background()
	oauth2Token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to exchange token: " + err.Error()})
	}

	// Extract the ID Token from OAuth2 token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "No id_token field in oauth2 token."})
	}

	// Verify the ID Token signature and expiration.
	verifier := oidcProvider.Verifier(&oidc.Config{ClientID: ConfigInstance.Oidc.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to verify ID Token: " + err.Error()})
	}

	// Get the claims
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Sub               string `json:"sub"`
		Sid               string `json:"sid"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse claims: " + err.Error()})
	}

	// Fetch standard UserInfo claims to cache them
	userInfo, err := oidcProvider.UserInfo(ctx, oauth2Config.TokenSource(ctx, oauth2Token))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get user info: " + err.Error()})
	}
	var allClaims map[string]interface{}
	if err := userInfo.Claims(&allClaims); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse user info claims: " + err.Error()})
	}
	cachedClaimsJSON, _ := json.Marshal(allClaims)

	db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to connect to database: " + err.Error()})
	}

	session := SessionModel{
		ID:           GenerateUUIDv7(),
		Subject:      claims.Sub,
		IdPSessionID: claims.Sid,
		Username:     claims.PreferredUsername,
		AccessToken:  oauth2Token.AccessToken,
		RefreshToken: oauth2Token.RefreshToken,
		CachedClaims: string(cachedClaimsJSON),
		ExpiresAt:    oauth2Token.Expiry,
	}

	if err := db.Create(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create session: " + err.Error()})
	}

	// Set the cookie
	c.Cookie(&fiber.Cookie{
		Name:     CookieName,
		Value:    session.ID,
		Expires:  time.Now().Add(31 * 24 * time.Hour),
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
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not logged in"})
	}

	db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	var session SessionModel
	if err := db.First(&session, "id = ?", cookie).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid session"})
	}

	fast := c.Query("fast") == "true"
	if fast && session.CachedClaims != "" {
		var claims map[string]interface{}
		if err := json.Unmarshal([]byte(session.CachedClaims), &claims); err == nil {
			return c.JSON(extractUserInfo(claims))
		}
		// If unmarshal fails, we fall through to the slow path
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
		// Invalidate the session and return 401
		db.Delete(&SessionModel{}, "id = ?", cookie)
		c.ClearCookie(CookieName)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Failed to refresh token: " + err.Error()})
	}

	// Update session if token changed
	if newToken.AccessToken != session.AccessToken {
		session.AccessToken = newToken.AccessToken
		session.RefreshToken = newToken.RefreshToken
		session.ExpiresAt = time.Now().Add(31 * 24 * time.Hour)
		session.CachedClaims = "" // Invalidate cached claims as we will fetch new ones
		// We don't save yet, we save after fetching new claims
	}

	userInfo, err := oidcProvider.UserInfo(ctx, tokenSource)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get user info: " + err.Error()})
	}

	var claims map[string]interface{}
	if err := userInfo.Claims(&claims); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse user info claims: " + err.Error()})
	}

	// Update cached claims
	if claimsJSON, err := json.Marshal(claims); err == nil {
		session.CachedClaims = string(claimsJSON)
		if err := db.Save(&session).Error; err != nil {
			log.Printf("Failed to update session with new token: %v", err)
		}
	} else {
		// Just save the token update if we couldn't marshal claims for some reason
		if err := db.Save(&session).Error; err != nil {
			log.Printf("Failed to update session with new token: %v", err)
		}
	}

	// Extend the session cookie
	c.Cookie(&fiber.Cookie{
		Name:     CookieName,
		Value:    session.ID,
		Expires:  time.Now().Add(31 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   false, // set to true if using HTTPS
		SameSite: "Lax",
	})

	return c.JSON(extractUserInfo(claims))
}

func extractUserInfo(claims map[string]interface{}) fiber.Map {
	usernameClaim := ConfigInstance.Oidc.UsernameClaim
	if usernameClaim == "" {
		usernameClaim = "preferred_username"
	}

	username, _ := claims[usernameClaim].(string)

	var membershipExpirationDate interface{} = nil
	if ConfigInstance.Oidc.MembershipExpirationClaim != "" {
		if val, ok := claims[ConfigInstance.Oidc.MembershipExpirationClaim]; ok {
			membershipExpirationDate = val
		}
	}

	return fiber.Map{
		"username":                 username,
		"membershipExpirationDate": membershipExpirationDate,
	}
}

func AuthMiddleware(c *fiber.Ctx) error {
	cookie := c.Cookies(CookieName)
	if cookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not logged in"})
	}

	db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	var session SessionModel
	if err := db.First(&session, "id = ?", cookie).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid session"})
	}

	// Store user info in context for downstream handlers
	c.Locals("username", session.Username)

	return c.Next()
}

func handleBackchannelLogout(c *fiber.Ctx) error {
	logoutToken := c.FormValue("logout_token")
	if logoutToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing logout_token"})
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid logout_token: " + err.Error()})
	}

	var claims struct {
		Sid string `json:"sid"`
		Sub string `json:"sub"`
	}
	if err := token.Claims(&claims); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse claims"})
	}

	db, err := gorm.Open(sqlite.Open(ConfigInstance.Database.Path), &gorm.Config{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	if claims.Sid != "" {
		db.Delete(&SessionModel{}, "id_p_session_id = ?", claims.Sid)
	} else if claims.Sub != "" {
		// If sid is missing, logout all sessions for the user (sub)
		db.Delete(&SessionModel{}, "subject = ?", claims.Sub)
	}

	return c.SendStatus(fiber.StatusOK)
}
