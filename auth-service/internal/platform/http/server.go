package http

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/madiyar/final-project/auth-service/internal/auth/usecase"
)

// Server exposes public HTTP routes (email verification links, password reset form).
type Server struct {
	httpServer *http.Server
}

// NewServer creates the public HTTP server for browser-based email actions.
func NewServer(addr string, authUC usecase.AuthUseCase) *Server {
	mux := http.NewServeMux()
	h := &actionHandler{uc: authUC}

	mux.HandleFunc("GET /verify-email", h.verifyEmail)
	mux.HandleFunc("GET /reset-password", h.resetPasswordForm)
	mux.HandleFunc("POST /reset-password", h.resetPasswordSubmit)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
		},
	}
}

// Start listens until shutdown or error.
func (s *Server) Start() error {
	if s == nil || s.httpServer == nil {
		return fmt.Errorf("http server is not initialized")
	}
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

type actionHandler struct {
	uc usecase.AuthUseCase
}

func (h *actionHandler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := h.uc.VerifyEmail(r.Context(), usecase.VerifyEmailInput{Token: token})
	if err != nil {
		renderPage(w, http.StatusBadRequest, "Verification failed", "The link is invalid or has expired.")
		return
	}
	renderPage(w, http.StatusOK, "Email verified",
		fmt.Sprintf("Thank you! Email %s has been verified. You can close this page and return to the app.", user.Email))
}

func (h *actionHandler) resetPasswordForm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		renderPage(w, http.StatusBadRequest, "Invalid link", "Missing reset token.")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = resetFormTemplate.Execute(w, map[string]string{
		"Token": template.HTMLEscapeString(token),
	})
}

func (h *actionHandler) resetPasswordSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPage(w, http.StatusBadRequest, "Error", "Invalid form data.")
		return
	}
	token := r.FormValue("token")
	password := r.FormValue("password")
	if err := h.uc.ResetPassword(r.Context(), usecase.ResetPasswordInput{
		Token:       token,
		NewPassword: password,
	}); err != nil {
		renderPage(w, http.StatusBadRequest, "Reset failed", "The link is invalid, expired, or the password is too weak.")
		return
	}
	renderPage(w, http.StatusOK, "Password updated", "Your password has been changed. You can log in with the new password.")
}

func renderPage(w http.ResponseWriter, status int, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTemplate.Execute(w, map[string]string{
		"Title": title,
		"Body":  body,
	})
}

var pageTemplate = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>{{.Title}}</title>
<style>body{font-family:Arial,sans-serif;max-width:520px;margin:40px auto;padding:0 16px;color:#222;}
h1{font-size:1.4rem;} .ok{color:#15803d;} .err{color:#b91c1c;}</style></head>
<body><h1>{{.Title}}</h1><p>{{.Body}}</p></body></html>`))

var resetFormTemplate = template.Must(template.New("reset").Parse(`<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>Reset password</title>
<style>body{font-family:Arial,sans-serif;max-width:420px;margin:40px auto;}
input{width:100%;padding:10px;margin:8px 0 16px;box-sizing:border-box;}
button{padding:10px 20px;background:#2563eb;color:#fff;border:0;border-radius:6px;cursor:pointer;}</style></head>
<body>
  <h1>Reset password</h1>
  <form method="POST" action="/reset-password">
    <input type="hidden" name="token" value="{{.Token}}">
    <label>New password (min 8 characters)</label>
    <input type="password" name="password" minlength="8" required>
    <button type="submit">Update password</button>
  </form>
</body></html>`))
