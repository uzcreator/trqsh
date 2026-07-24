package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthUser is the normalized profile returned by an OAuth provider.
type OAuthUser struct {
	Email     string
	Name      string
	AvatarURL string
	Provider  string
}

// OAuthProvider is a configured OAuth2 authorization-code provider.
type OAuthProvider struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserURL      string
	Scopes       []string
}

// GitHubProvider builds a GitHub OAuth provider from credentials.
func GitHubProvider(clientID, clientSecret, redirectURL string) OAuthProvider {
	return OAuthProvider{
		Name: "github", ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
		AuthURL:  "https://github.com/login/oauth/authorize",
		TokenURL: "https://github.com/login/oauth/access_token",
		UserURL:  "https://api.github.com/user",
		Scopes:   []string{"read:user", "user:email"},
	}
}

// GoogleProvider builds a Google OAuth provider from credentials.
func GoogleProvider(clientID, clientSecret, redirectURL string) OAuthProvider {
	return OAuthProvider{
		Name: "google", ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
		UserURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
		Scopes:   []string{"openid", "email", "profile"},
	}
}

// Configured reports whether the provider has credentials.
func (p OAuthProvider) Configured() bool {
	return p.ClientID != "" && p.ClientSecret != ""
}

// AuthCodeURL builds the authorization redirect URL for a state token.
func (p OAuthProvider) AuthCodeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(p.Scopes, " "))
	q.Set("state", state)
	return p.AuthURL + "?" + q.Encode()
}

// Exchange swaps an authorization code for the user's profile.
func (p OAuthProvider) Exchange(ctx context.Context, code string) (OAuthUser, error) {
	token, err := p.exchangeToken(ctx, code)
	if err != nil {
		return OAuthUser{}, err
	}
	return p.fetchUser(ctx, token)
}

func (p OAuthProvider) exchangeToken(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", p.RedirectURL)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth token exchange: status %d", resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("oauth token exchange: no access_token")
	}
	return tok.AccessToken, nil
}

func (p OAuthProvider) fetchUser(ctx context.Context, accessToken string) (OAuthUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserURL, nil)
	if err != nil {
		return OAuthUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient().Do(req)
	if err != nil {
		return OAuthUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return OAuthUser{}, fmt.Errorf("oauth userinfo: status %d", resp.StatusCode)
	}
	var raw struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Login   string `json:"login"`
		Avatar  string `json:"avatar_url"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return OAuthUser{}, err
	}
	name := raw.Name
	if name == "" {
		name = raw.Login
	}
	avatar := raw.Avatar
	if avatar == "" {
		avatar = raw.Picture
	}
	if raw.Email == "" && p.Name == "github" {
		// GitHub's /user omits private emails; the user:email scope lets us read
		// the account's primary verified address from /user/emails.
		raw.Email, err = p.fetchGitHubEmail(ctx, accessToken)
		if err != nil {
			return OAuthUser{}, err
		}
	}
	if raw.Email == "" {
		return OAuthUser{}, fmt.Errorf("oauth userinfo: no email (make it public or add email scope)")
	}
	return OAuthUser{Email: raw.Email, Name: name, AvatarURL: avatar, Provider: p.Name}, nil
}

// fetchGitHubEmail returns the account's primary verified email via the
// user:email scope, used when the public profile hides the address.
func (p OAuthProvider) fetchGitHubEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth userinfo: emails status %d", resp.StatusCode)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	var fallback string
	for _, e := range emails {
		if e.Verified && e.Primary {
			return e.Email, nil
		}
		if e.Verified && fallback == "" {
			fallback = e.Email
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("oauth userinfo: no verified email")
}

func httpClient() *http.Client { return &http.Client{Timeout: 15 * time.Second} }
