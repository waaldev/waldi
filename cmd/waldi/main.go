package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"waldi/internal/cdn"
	"waldi/internal/jobs"
	"waldi/internal/mail"
	"waldi/internal/migrate"
	"waldi/internal/storage"
	"waldi/internal/store"
	"waldi/internal/telegrambot"
	"waldi/internal/web"

	"golang.org/x/crypto/acme/autocert"
)

type Config struct {
	Addr             string
	Environment      string
	BaseDomain       string
	TelegramBotToken string
	TelegramAdminIDs []int64
	AppURL           string
	DatabaseURL      string
	SMTPHost         string
	SMTPPort         string
	SMTPUsername     string
	SMTPPassword     string
	SMTPFrom         string
	S3Endpoint       string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3UseSSL         bool
	S3PublicURL      string
	TLSEnable        bool
	TLSAddr          string
	AutocertEmail    string
	AutocertCacheDir string
	CFZoneID         string
	CFAPIToken       string
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "waldi: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "serve":
		return serve(os.Args[2:])
	case "migrate":
		return runMigrations(os.Args[2:])
	case "digest":
		return runDigest(os.Args[2:])
	case "reader-digest":
		return runReaderDigest(os.Args[2:])
	case "weekly-stats":
		return runWeeklyStats(os.Args[2:])
	case "wildcard":
		return runWildcard(os.Args[2:])
	case "invite":
		return runInvite(os.Args[2:])
	case "user":
		return runUser(os.Args[2:])
	case "import":
		return runImport(os.Args[2:])
	case "posts":
		return runPosts(os.Args[2:])
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func runWildcard(args []string) error {
	if len(args) > 0 && args[0] == "set" {
		return runWildcardSet(args[1:])
	}

	cfg := loadConfig()
	limit := 1000

	fs := flag.NewFlagSet("wildcard", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	fs.IntVar(&limit, "limit", limit, "maximum users to assign")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	floor, err := st.WildcardImpressionFloor(ctx)
	if err != nil {
		return fmt.Errorf("loading wildcard impression floor: %w", err)
	}

	return jobs.WildcardJob{
		Store:           st,
		Logger:          newLogger(cfg.Environment),
		Limit:           limit,
		ImpressionFloor: floor,
	}.Run(ctx)
}

func runWildcardSet(args []string) error {
	cfg := loadConfig()
	var (
		postID int64
		author string
		slug   string
		user   string
		date   string
		limit  = 1000
	)

	fs := flag.NewFlagSet("wildcard set", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	fs.Int64Var(&postID, "post-id", 0, "published post id to feature")
	fs.StringVar(&author, "author", "", "post author username (with --slug)")
	fs.StringVar(&slug, "slug", "", "post slug (with --author)")
	fs.StringVar(&user, "user", "", "single reader username (default: all users)")
	fs.StringVar(&date, "date", "", "assignment date YYYY-MM-DD (default: today)")
	fs.IntVar(&limit, "limit", limit, "maximum users when setting for all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}

	var day time.Time
	if date != "" {
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			return fmt.Errorf("parsing --date: %w", err)
		}
		day = jobs.BeginningOfDay(parsed)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	return jobs.WildcardSetJob{
		Store:  st,
		Logger: newLogger(cfg.Environment),
		PostID: postID,
		Author: author,
		Slug:   slug,
		User:   user,
		Limit:  limit,
		Day:    day,
	}.Run(ctx)
}

func runDigest(args []string) error {
	cfg := loadConfig()

	fs := flag.NewFlagSet("digest", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	logger := newLogger(cfg.Environment)
	return jobs.DigestJob{
		Store:   st,
		Logger:  logger,
		Mailer:  newMailer(cfg, logger),
		BaseURL: appURL(cfg),
	}.Run(ctx)
}

func runReaderDigest(args []string) error {
	cfg := loadConfig()

	fs := flag.NewFlagSet("reader-digest", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	logger := newLogger(cfg.Environment)
	return jobs.ReaderDigestJob{
		Store:      st,
		Logger:     logger,
		Mailer:     newMailer(cfg, logger),
		BaseURL:    appURL(cfg),
		BaseDomain: cfg.BaseDomain,
	}.Run(ctx)
}

func runWeeklyStats(args []string) error {
	cfg := loadConfig()

	fs := flag.NewFlagSet("weekly-stats", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}
	if cfg.TelegramBotToken == "" || len(cfg.TelegramAdminIDs) == 0 {
		return errors.New("WALDI_TELEGRAM_BOT_TOKEN and WALDI_TELEGRAM_ADMIN_IDS are required")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	logger := newLogger(cfg.Environment)
	return jobs.WeeklyStatsJob{
		Store:    st,
		Logger:   logger,
		Notifier: telegrambot.NewAdminNotifier(cfg.TelegramBotToken, cfg.TelegramAdminIDs, logger),
	}.Run(ctx)
}

func appURL(cfg Config) string {
	if cfg.AppURL != "" {
		return cfg.AppURL
	}
	return "https://" + cfg.BaseDomain
}

func newMailer(cfg Config, logger *slog.Logger) mail.Mailer {
	if cfg.SMTPHost == "" {
		return mail.NoopMailer{Logger: logger}
	}
	return mail.NewSMTPMailer(mail.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})
}

func runMigrations(args []string) error {
	cfg := loadConfig()

	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	if err := migrate.Up(ctx, st.Pool(), "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

func serve(args []string) error {
	cfg := loadConfig()

	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := newLogger(cfg.Environment)
	var st *store.Store
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		st, err = store.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("opening store: %w", err)
		}
		defer st.Close()
	}

	images, s3media, err := newImageStore(cfg)
	if err != nil {
		return fmt.Errorf("creating image store: %w", err)
	}

	var bot *telegrambot.Bot
	if cfg.TelegramBotToken != "" {
		bot, err = telegrambot.New(telegrambot.Config{
			Token:      cfg.TelegramBotToken,
			AdminIDs:   cfg.TelegramAdminIDs,
			BaseDomain: cfg.BaseDomain,
			AppURL:     cfg.AppURL,
			Store:      st,
			Logger:     logger,
		})
		if err != nil {
			return fmt.Errorf("creating telegram bot: %w", err)
		}
	}

	webCfg := web.Config{
		BaseDomain: cfg.BaseDomain,
		DevMode:    cfg.Environment == "dev",
		Logger:     logger,
		Store:      st,
		Mailer:     newMailer(cfg, logger),
		Images:     images,
		S3Media:    s3media,
		CDNPurger:  newCDNPurger(cfg),
	}
	if bot != nil {
		webCfg.Notifier = bot
	}

	srv, err := web.NewServer(webCfg)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}

	errc := make(chan error, 2)

	if bot != nil {
		go func() {
			if err := bot.Run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
				errc <- fmt.Errorf("telegram bot: %w", err)
			}
		}()
	}

	if cfg.TLSEnable {
		if st == nil {
			return errors.New("WALDI_TLS_ENABLE requires WALDI_DATABASE_URL")
		}

		manager := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache(cfg.AutocertCacheDir),
			Email:      cfg.AutocertEmail,
			HostPolicy: customDomainHostPolicy(st, cfg.BaseDomain),
		}

		httpServer := &http.Server{
			Addr:              ":80",
			Handler:           manager.HTTPHandler(nil),
			ReadHeaderTimeout: 5 * time.Second,
		}
		tlsServer := &http.Server{
			Addr:              cfg.TLSAddr,
			Handler:           srv,
			TLSConfig:         manager.TLSConfig(),
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			logger.Info("serving acme http-01 challenges", "addr", ":80")
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()
		go func() {
			logger.Info("serving tls", "addr", cfg.TLSAddr, "base_domain", cfg.BaseDomain)
			if err := tlsServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

		select {
		case err := <-errc:
			return fmt.Errorf("serving http: %w", err)
		case <-stop:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(ctx)
			return tlsServer.Shutdown(ctx)
		}
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("serving", "addr", cfg.Addr, "base_domain", cfg.BaseDomain)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errc:
		return fmt.Errorf("serving http: %w", err)
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(ctx)
	}
}

// customDomainHostPolicy only allows autocert to issue certificates for
// verified custom domains; the base domain and its writer subdomains are
// expected to already be covered by separately managed wildcard certs.
func customDomainHostPolicy(st *store.Store, baseDomain string) autocert.HostPolicy {
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	return func(ctx context.Context, host string) error {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" || host == baseDomain || strings.HasSuffix(host, "."+baseDomain) {
			return fmt.Errorf("host %q is not a custom domain", host)
		}
		if _, err := st.UserByCustomDomain(ctx, host); err != nil {
			return fmt.Errorf("host %q is not a verified custom domain: %w", host, err)
		}
		return nil
	}
}

func newImageStore(cfg Config) (storage.ImageStore, *storage.S3Store, error) {
	if cfg.S3Endpoint != "" {
		s3, err := storage.NewS3Store(storage.S3Config{
			Endpoint:  cfg.S3Endpoint,
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
			Bucket:    cfg.S3Bucket,
			UseSSL:    cfg.S3UseSSL,
			PublicURL: cfg.S3PublicURL,
		})
		if err != nil {
			return nil, nil, err
		}
		return s3, s3, nil
	}
	return storage.LocalStore{RootDir: "web/static/uploads"}, nil, nil
}

func loadConfig() Config {
	environment := env("WALDI_ENV", "dev")
	return Config{
		Addr:             env("WALDI_ADDR", ":8080"),
		Environment:      environment,
		BaseDomain:       env("WALDI_BASE_DOMAIN", "waldi.blog"),
		TelegramBotToken: env("WALDI_TELEGRAM_BOT_TOKEN", ""),
		TelegramAdminIDs: parseInt64List(env("WALDI_TELEGRAM_ADMIN_IDS", "")),
		AppURL:           env("WALDI_APP_URL", ""),
		DatabaseURL:      env("WALDI_DATABASE_URL", ""),
		SMTPHost:         env("WALDI_SMTP_HOST", ""),
		SMTPPort:         env("WALDI_SMTP_PORT", "587"),
		SMTPUsername:     env("WALDI_SMTP_USERNAME", ""),
		SMTPPassword:     env("WALDI_SMTP_PASSWORD", ""),
		SMTPFrom:         env("WALDI_SMTP_FROM", "no-reply@waldi.blog"),
		S3Endpoint:       env("WALDI_S3_ENDPOINT", ""),
		S3AccessKey:      env("WALDI_S3_ACCESS_KEY", ""),
		S3SecretKey:      env("WALDI_S3_SECRET_KEY", ""),
		S3Bucket:         env("WALDI_S3_BUCKET", "waldi"),
		S3UseSSL:         envBool("WALDI_S3_USE_SSL", true),
		S3PublicURL:      env("WALDI_S3_PUBLIC_URL", ""),

		TLSEnable:        envBool("WALDI_TLS_ENABLE", false),
		TLSAddr:          env("WALDI_TLS_ADDR", ":443"),
		AutocertEmail:    env("WALDI_AUTOCERT_EMAIL", ""),
		AutocertCacheDir: env("WALDI_AUTOCERT_CACHE_DIR", "/var/lib/waldi/autocert"),

		CFZoneID:   env("WALDI_CF_ZONE_ID", ""),
		CFAPIToken: firstNonEmpty(env("WALDI_CF_API_TOKEN", ""), env("CLOUDFLARE_API_TOKEN", "")),
	}
}

func newCDNPurger(cfg Config) cdn.Purger {
	if cfg.CFZoneID == "" || cfg.CFAPIToken == "" {
		return nil
	}
	return cdn.NewCloudflarePurger(cfg.CFZoneID, cfg.CFAPIToken)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseInt64List(value string) []int64 {
	var out []int64
	for part := range strings.SplitSeq(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func newLogger(environment string) *slog.Logger {
	if environment == "prod" {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
