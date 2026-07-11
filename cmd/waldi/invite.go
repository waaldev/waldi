package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"strings"
	"waldi/internal/store"
)

func runInvite(args []string) error {
	if len(args) > 0 && args[0] == "create" {
		return runInviteCreate(args[1:])
	}
	return errors.New("usage: waldi invite create [--count N] [--note TEXT]")
}

func runInviteCreate(args []string) error {
	cfg := loadConfig()
	count := 1
	note := ""

	fs := flag.NewFlagSet("invite create", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	fs.IntVar(&count, "count", count, "number of invitations to create")
	fs.StringVar(&note, "note", note, "optional note for your records")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}
	if count < 1 {
		return errors.New("--count must be at least 1")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	baseURL := "https://" + strings.TrimPrefix(strings.TrimSpace(cfg.BaseDomain), ".")
	for i := 0; i < count; i++ {
		code, err := newInviteCode()
		if err != nil {
			return fmt.Errorf("generating invite code: %w", err)
		}
		inv, err := st.CreateInvitation(ctx, code, note)
		if err != nil {
			return fmt.Errorf("creating invitation: %w", err)
		}
		fmt.Printf("%s/signup?invite=%s\n", baseURL, inv.Code)
	}
	return nil
}

func newInviteCode() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}
