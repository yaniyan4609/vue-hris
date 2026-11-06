package httpapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"

	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

func mountPasswordReset(r chi.Router, service auth.Service, config Options) {
	r.With(checkOrigin(config)).Post("/password/forgot", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		if !readJSON(w, req, &body) {
			return
		}
		if len(body.Email) > 320 || strings.TrimSpace(body.Email) == "" {
			writeAPIError(w, req, 400, "VALIDATION_FAILED", "Email wajib diisi.")
			return
		}
		if !smtpConfigured() {
			writeAPIError(w, req, 503, "DELIVERY_UNAVAILABLE", "Pengiriman email reset belum dikonfigurasi.")
			return
		}
		err := service.IssuePasswordReset(req.Context(), body.Email, func(destination, token string) error {
			return sendResetMail(req.Context(), destination, token)
		})
		if err != nil {
			writeAPIError(w, req, 503, "DELIVERY_UNAVAILABLE", "Email reset belum dapat dikirim.")
			return
		}
		writeAPIData(w, 202, map[string]string{"message": "Jika akun tersedia, tautan reset dikirim ke email."}, nil)
	})
	r.With(checkOrigin(config)).Post("/password/reset", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Token       string `json:"token"`
			NewPassword string `json:"new_password"`
		}
		if !readJSON(w, req, &body) {
			return
		}
		err := service.ResetPassword(req.Context(), body.Token, body.NewPassword)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrPasswordPolicy):
				writeAPIError(w, req, 400, "VALIDATION_FAILED", "Kata sandi baru minimal 8 karakter.")
			case errors.Is(err, auth.ErrResetTokenInvalid):
				writeAPIError(w, req, 400, "RESET_TOKEN_INVALID", "Tautan reset tidak sah atau kedaluwarsa.")
			default:
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal mengubah kata sandi.")
			}
			return
		}
		clearSessionCookie(w, config.CookieSecure)
		w.WriteHeader(http.StatusNoContent)
	})
}

func smtpConfigured() bool {
	return os.Getenv("SMTP_HOST") != "" && os.Getenv("SMTP_PORT") != "" && os.Getenv("SMTP_FROM") != "" && os.Getenv("APP_ORIGIN") != ""
}

func sendResetMail(ctx context.Context, destination, token string) error {
	base := strings.TrimRight(os.Getenv("APP_ORIGIN"), "/")
	link := base + "/reset-password?token=" + url.QueryEscape(token)
	message := fmt.Sprintf("Gunakan tautan berikut untuk mengatur ulang kata sandi. Tautan berlaku 15 menit dan hanya dapat dipakai sekali.\r\n\r\n%s\r\n\r\nJika Anda tidak meminta reset, abaikan email ini.\r\n", link)
	return sendSMTPText(ctx, destination, "Reset kata sandi digaji.id", message)
}

func sendSetupMail(ctx context.Context, destination, token string) error {
	base := strings.TrimRight(os.Getenv("APP_ORIGIN"), "/")
	link := base + "/reset-password?token=" + url.QueryEscape(token)
	message := fmt.Sprintf("Akun digaji.id Anda sudah dibuat. Gunakan tautan berikut untuk menetapkan kata sandi pertama. Tautan berlaku 15 menit dan hanya dapat dipakai sekali.\r\n\r\n%s\r\n", link)
	return sendSMTPText(ctx, destination, "Aktifkan akun digaji.id", message)
}

func sendTenantInviteMail(ctx context.Context, destination, tenantName string) error {
	link := strings.TrimRight(os.Getenv("APP_ORIGIN"), "/")
	message := fmt.Sprintf("Akun Anda telah ditambahkan ke %s di digaji.id. Masuk melalui tautan berikut dan pilih perusahaan tersebut setelah login.\r\n\r\n%s\r\n", tenantName, link)
	return sendSMTPText(ctx, destination, "Undangan perusahaan digaji.id", message)
}

func sendSMTPText(ctx context.Context, destination, subject, body string) error {
	if strings.ContainsAny(subject, "\r\n") {
		return errors.New("invalid mail subject")
	}
	host, port := os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT")
	from, err := mail.ParseAddress(os.Getenv("SMTP_FROM"))
	if err != nil {
		return err
	}
	to, err := mail.ParseAddress(destination)
	if err != nil {
		return err
	}
	mode := os.Getenv("SMTP_TLS_MODE")
	if mode == "" {
		mode = "starttls"
	}
	if mode != "starttls" && mode != "implicit" {
		return errors.New("SMTP_TLS_MODE must be starttls or implicit")
	}
	address := net.JoinHostPort(host, port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	if mode == "implicit" {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if mode == "starttls" {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	username, password := os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD")
	if username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return err
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return err
	}
	if err := client.Rcpt(to.Address); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", from.Address, to.Address, subject, body)
	if _, err := writer.Write([]byte(message)); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}
