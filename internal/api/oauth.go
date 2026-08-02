package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jdholdren/seymour/internal/seymour"
)

const (
	oauthStateCookie    = "oauth_state"
	sessionCookie       = "session"
	oauthStateCookieTTL = 10 * time.Minute
	sessionCookieTTL    = 30 * 24 * time.Hour
)

// oauthState is what's stashed in the oauth_state cookie for the length of the
// round trip to the IDP and back, so the callback can verify the request and
// know where to send the browser once login is done.
type oauthState struct {
	State        string `json:"state"`
	RedirectPath string `json:"redirect_path"`
}

// session is what's stored, signed and encrypted, in the session cookie.
type session struct {
	UserID string `json:"user_id"`
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// isSafeRedirectPath guards against open redirects: it must be a path on the
// frontend's own origin, not an absolute or protocol-relative URL to somewhere else.
func isSafeRedirectPath(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//")
}

func (s Server) startGithubLogin(w http.ResponseWriter, r *http.Request) error {
	state, err := randomState()
	if err != nil {
		return seymour.E(err, http.StatusInternalServerError)
	}

	redirectPath := r.URL.Query().Get("s")
	if redirectPath == "" || !isSafeRedirectPath(redirectPath) {
		redirectPath = "/"
	}

	encoded, err := s.secureCookie.Encode(oauthStateCookie, oauthState{
		State:        state,
		RedirectPath: redirectPath,
	})
	if err != nil {
		return seymour.E(err, http.StatusInternalServerError)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    encoded,
		Path:     "/",
		Expires:  time.Now().Add(oauthStateCookieTTL),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, s.oauthCfg.AuthCodeURL(state), http.StatusFound)
	return nil
}

func (s Server) githubOAuthCallback(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil {
		return seymour.E("missing oauth state", http.StatusBadRequest)
	}

	var state oauthState
	if err := s.secureCookie.Decode(oauthStateCookie, stateCookie.Value, &state); err != nil {
		return seymour.E("invalid oauth state", http.StatusBadRequest)
	}

	if q := r.URL.Query().Get("state"); q == "" || q != state.State {
		return seymour.E("oauth state mismatch", http.StatusBadRequest)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	token, err := s.oauthCfg.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		return seymour.E(err, http.StatusBadGateway)
	}

	client := s.oauthCfg.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return seymour.E(err, http.StatusBadGateway)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return seymour.E("error fetching github user", http.StatusBadGateway)
	}

	var ghUser struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return seymour.E(err, http.StatusBadGateway)
	}

	user, _, err := s.users.EnsureUser(ctx, seymour.Idp("github"), strconv.FormatInt(ghUser.ID, 10))
	if err != nil {
		return err
	}

	encoded, err := s.secureCookie.Encode(sessionCookie, session{UserID: user.ID})
	if err != nil {
		return seymour.E(err, http.StatusInternalServerError)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    encoded,
		Path:     "/",
		Expires:  time.Now().Add(sessionCookieTTL),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, s.frontendURL.ResolveReference(&url.URL{Path: state.RedirectPath}).String(), http.StatusFound)
	return nil
}

func (s Server) logout(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusNoContent)
	return nil
}
