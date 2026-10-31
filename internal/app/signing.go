package app

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"designhub/internal/auth"
)

// Electronic signatures are confirmed with the account password for local
// accounts and with a signing PIN for Google accounts, which have no
// DesignHub password.

const (
	signingPinMin = 6
	signingPinMax = 64
)

// checkSignature verifies the secret a user typed to sign. It returns nil
// when it matches, or an error for the user that ends with notDone (for
// example "The decision was not recorded."). Failures count toward the
// sign-in limit.
func (a *App) checkSignature(ctx context.Context, u *User, secret, notDone string) *Error {
	var pwHash, pinHash, method, name string
	if err := a.DB.QueryRow(ctx, `SELECT password_hash, signing_pin_hash, sign_in_method, lower(username) FROM users WHERE id = $1 AND active`, u.ID).
		Scan(&pwHash, &pinHash, &method, &name); err != nil {
		return &Error{Status: http.StatusUnauthorized, Code: "signature", Message: "Your account could not be checked. " + notDone}
	}
	hash, what := pwHash, "Password"
	if method == "google" {
		if pinHash == "" {
			return &Error{Status: http.StatusBadRequest, Code: "signing_pin_missing", Message: "Set a signing PIN in Settings, Security before signing. " + notDone}
		}
		hash, what = pinHash, "Signing PIN"
	}
	if a.Limiter.Blocked("user:" + name) {
		return &Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Too many failed attempts. Try again in 15 minutes. " + notDone}
	}
	if !auth.CheckPassword(hash, secret) {
		a.Limiter.Fail("user:" + name)
		return &Error{Status: http.StatusUnauthorized, Code: "signature", Message: what + " is incorrect. " + notDone}
	}
	return nil
}

func validSigningPin(pin, username string) *Error {
	n := utf8.RuneCountInString(pin)
	if n < signingPinMin || n > signingPinMax {
		return badRequest("The signing PIN must be %d to %d characters.", signingPinMin, signingPinMax)
	}
	if strings.Trim(pin, pin[:1]) == "" {
		return badRequest("The signing PIN cannot be one character repeated.")
	}
	if strings.Contains("01234567890", pin) || strings.Contains("98765432109", pin) {
		return badRequest("The signing PIN cannot be a run of digits.")
	}
	if strings.EqualFold(pin, username) {
		return badRequest("The signing PIN cannot be your username.")
	}
	return nil
}

// apiSetSigningPin sets or changes the signed-in Google user's signing PIN.
// Changing an existing PIN needs the current one.
func (a *App) apiSetSigningPin(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if u.SignIn != "google" {
		return badRequest("Your account signs with its password. A signing PIN is only used with Google sign-in.")
	}
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	var cur string
	if err := a.DB.QueryRow(ctx, `SELECT signing_pin_hash FROM users WHERE id = $1`, u.ID).Scan(&cur); err != nil {
		return err
	}
	if cur != "" {
		if e := a.checkSignature(ctx, u, in.Current, "The PIN was not changed."); e != nil {
			if e.Code == "signature" {
				e.Message = "Current signing PIN is incorrect. The PIN was not changed."
			}
			return e
		}
	}
	if e := validSigningPin(in.New, u.Username); e != nil {
		return e
	}
	hash, err := auth.HashPassword(in.New)
	if err != nil {
		return err
	}
	if _, err := a.DB.Exec(ctx, `UPDATE users SET signing_pin_hash = $2 WHERE id = $1`, u.ID, hash); err != nil {
		return err
	}
	action := "signing_pin.set"
	if cur != "" {
		action = "signing_pin.change"
	}
	a.Audit(ctx, u, clientIP(r), action, "", nil)
	return ok(w, nil)
}
