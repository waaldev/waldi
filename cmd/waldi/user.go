package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"waldi/internal/store"
)

func runUser(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: waldi user verify-email --email ADDRESS | verification-link --email ADDRESS")
	}
	switch args[0] {
	case "verify-email":
		return runUserVerifyEmail(args[1:])
	case "verification-link":
		return runUserVerificationLink(args[1:])
	default:
		return fmt.Errorf("unknown user command %q", args[0])
	}
}

func runUserVerifyEmail(args []string) error {
	cfg := loadConfig()
	email := ""

	fs := flag.NewFlagSet("user verify-email", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	fs.StringVar(&email, "email", "", "user email address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}
	if email == "" {
		return errors.New("--email is required")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	user, err := st.VerifyEmailByAddress(ctx, email)
	if err != nil {
		return err
	}
	fmt.Printf("verified %s (%s)\n", user.Email, user.Username)
	return nil
}

func runUserVerificationLink(args []string) error {
	cfg := loadConfig()
	email := ""

	fs := flag.NewFlagSet("user verification-link", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	fs.StringVar(&email, "email", "", "user email address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}
	if email == "" {
		return errors.New("--email is required")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	user, err := st.UserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user.EmailVerified() {
		return fmt.Errorf("%s is already verified", email)
	}
	if user.EmailVerifyToken == nil || strings.TrimSpace(*user.EmailVerifyToken) == "" {
		return fmt.Errorf("%s has no pending verification token", email)
	}

	baseURL := "https://" + strings.TrimPrefix(strings.TrimSpace(cfg.BaseDomain), ".")
	fmt.Printf("%s/verify-email?token=%s\n", baseURL, *user.EmailVerifyToken)
	return nil
}
