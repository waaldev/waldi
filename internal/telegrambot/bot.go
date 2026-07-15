package telegrambot

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"waldi/internal/store"
)

const (
	usersListLimit  = 50
	postsListLimit  = 20
	inviteCodeBytes = 16

	// telegramMessageLimit stays under Telegram's hard 4096-char sendMessage
	// cap, which /users and /posts can otherwise exceed once the list grows.
	telegramMessageLimit = 4000
)

var reservedUsernames = map[string]bool{
	"admin": true, "api": true, "app": true, "blog": true, "cname": true,
	"cdn": true, "mail": true, "smtp": true, "imap": true, "pop": true,
	"ftp": true, "static": true, "media": true, "www": true,
}

type Config struct {
	Token      string
	AdminIDs   []int64
	BaseDomain string
	AppURL     string
	Store      *store.Store
	Logger     *slog.Logger
}

type Bot struct {
	client     *Client
	adminIDs   map[int64]bool
	baseDomain string
	appURL     string
	store      *store.Store
	logger     *slog.Logger
}

func New(cfg Config) (*Bot, error) {
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, errors.New("telegram bot token is required")
	}
	if len(cfg.AdminIDs) == 0 {
		return nil, errors.New("at least one telegram admin id is required")
	}
	if cfg.Store == nil {
		return nil, errors.New("store is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	baseDomain := strings.TrimPrefix(strings.TrimSpace(cfg.BaseDomain), ".")
	if baseDomain == "" {
		baseDomain = "waldi.blog"
	}

	appURL := strings.TrimRight(strings.TrimSpace(cfg.AppURL), "/")
	if appURL == "" {
		appURL = "https://" + baseDomain
	}

	admins := make(map[int64]bool, len(cfg.AdminIDs))
	for _, id := range cfg.AdminIDs {
		admins[id] = true
	}

	return &Bot{
		client:     NewClient(token),
		adminIDs:   admins,
		baseDomain: baseDomain,
		appURL:     appURL,
		store:      cfg.Store,
		logger:     logger,
	}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("telegram admin bot started")
	var offset int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := b.client.GetUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			b.logger.Error("telegram getUpdates", "err", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			if update.CallbackQuery != nil {
				b.handleCallback(ctx, update.CallbackQuery)
				continue
			}
			if update.Message != nil && update.Message.Text != "" {
				b.handleMessage(ctx, update.Message)
			}
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *Message) {
	if msg.From == nil || !b.adminIDs[msg.From.ID] {
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	parts := strings.Fields(text)
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/start", "/help":
		b.reply(ctx, msg.Chat.ID, helpText())
	case "/users":
		b.cmdUsers(ctx, msg.Chat.ID)
	case "/posts":
		b.cmdPosts(ctx, msg.Chat.ID)
	case "/pool":
		b.cmdPool(ctx, msg.Chat.ID)
	case "/verify":
		b.cmdVerify(ctx, msg.Chat.ID, args)
	case "/setusername":
		b.cmdSetUsername(ctx, msg.Chat.ID, args)
	case "/setemail":
		b.cmdSetEmail(ctx, msg.Chat.ID, args)
	case "/delete":
		b.cmdDelete(ctx, msg.Chat.ID, args)
	case "/invite":
		b.cmdInvite(ctx, msg.Chat.ID, strings.Join(args, " "))
	case "/wildcardfloor":
		b.cmdWildcardFloor(ctx, msg.Chat.ID, args)
	default:
		b.reply(ctx, msg.Chat.ID, "Unknown command. Send /help for available commands.")
	}
}

func (b *Bot) handleCallback(ctx context.Context, q *CallbackQuery) {
	if q.From == nil || !b.adminIDs[q.From.ID] {
		return
	}

	chatID := q.Message.Chat.ID
	data := strings.TrimSpace(q.Data)

	switch {
	case strings.HasPrefix(data, "pool_add:"):
		postID, err := strconv.ParseInt(strings.TrimPrefix(data, "pool_add:"), 10, 64)
		if err != nil || postID <= 0 {
			_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Invalid post id")
			return
		}
		if err := b.store.AddToWildcardPool(ctx, postID); err != nil {
			b.logger.Error("telegram wildcard pool add", "err", err, "post_id", postID)
			_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Failed")
			b.reply(ctx, chatID, "Could not add post to wildcard pool: "+err.Error())
			return
		}
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Added to wildcard pool")
		b.reply(ctx, chatID, fmt.Sprintf("Post %d added to the wildcard pool.", postID))

	case strings.HasPrefix(data, "pool_remove:"):
		postID, err := strconv.ParseInt(strings.TrimPrefix(data, "pool_remove:"), 10, 64)
		if err != nil || postID <= 0 {
			_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Invalid post id")
			return
		}
		if err := b.store.RemoveFromWildcardPool(ctx, postID); err != nil {
			b.logger.Error("telegram wildcard pool remove", "err", err, "post_id", postID)
			_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Failed")
			b.reply(ctx, chatID, "Could not remove post from wildcard pool: "+err.Error())
			return
		}
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Removed from wildcard pool")
		b.reply(ctx, chatID, fmt.Sprintf("Post %d removed from the wildcard pool.", postID))

	case strings.HasPrefix(data, "delete:"):
		ref := strings.TrimPrefix(data, "delete:")
		user, err := b.lookupUser(ctx, ref)
		if err != nil {
			_ = b.client.AnswerCallbackQuery(ctx, q.ID, "User not found")
			return
		}
		if err := b.store.DeleteUser(ctx, user.ID); err != nil {
			b.logger.Error("telegram delete user", "err", err, "user_id", user.ID)
			_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Failed")
			b.reply(ctx, chatID, "Could not delete user: "+err.Error())
			return
		}
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Deleted")
		b.reply(ctx, chatID, fmt.Sprintf("Deleted %s (%s).", user.Username, user.Email))
	}
}

func (b *Bot) cmdUsers(ctx context.Context, chatID int64) {
	users, err := b.store.ListUsersRecent(ctx, usersListLimit)
	if err != nil {
		b.logger.Error("telegram list users", "err", err)
		b.reply(ctx, chatID, "Failed to load users.")
		return
	}
	if len(users) == 0 {
		b.reply(ctx, chatID, "No users yet.")
		return
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Users (latest %d):", len(users)))
	for _, u := range users {
		verified := "unverified"
		if u.EmailVerified() {
			verified = "verified"
		}
		lines = append(lines, fmt.Sprintf(
			"• %s — %s (%s)\n  %s — joined %s",
			u.Username,
			u.Email,
			verified,
			b.blogURL(u),
			u.CreatedAt.Format("2006-01-02"),
		))
	}
	b.reply(ctx, chatID, strings.Join(lines, "\n"))
}

func (b *Bot) cmdPosts(ctx context.Context, chatID int64) {
	posts, err := b.store.RecentPublishedPosts(ctx, postsListLimit)
	if err != nil {
		b.logger.Error("telegram list posts", "err", err)
		b.reply(ctx, chatID, "Failed to load posts.")
		return
	}
	if len(posts) == 0 {
		b.reply(ctx, chatID, "No published posts yet.")
		return
	}

	var lines []string
	var buttons [][]InlineKeyboardButton
	lines = append(lines, fmt.Sprintf("Latest %d posts:", len(posts)))

	for _, p := range posts {
		url := b.postURL(p.Username, p.Slug)
		published := "—"
		if p.PublishedAt != nil {
			published = p.PublishedAt.Format("2006-01-02")
		}
		title := strings.TrimSpace(p.Title)
		if title == "" {
			title = "(untitled)"
		}
		lines = append(lines, fmt.Sprintf(
			"• #%d %s\n  %s by @%s (%s) — %s",
			p.ID,
			title,
			url,
			p.Username,
			p.BlogLang,
			published,
		))
		buttons = append(buttons, []InlineKeyboardButton{{
			Text:         fmt.Sprintf("Add to pool #%d", p.ID),
			CallbackData: fmt.Sprintf("pool_add:%d", p.ID),
		}})
	}

	markup := &InlineKeyboardMarkup{InlineKeyboard: buttons}
	if err := b.sendChunked(ctx, chatID, strings.Join(lines, "\n"), markup); err != nil {
		b.logger.Error("telegram send message", "err", err)
	}
}

func (b *Bot) cmdPool(ctx context.Context, chatID int64) {
	posts, err := b.store.ListWildcardPool(ctx, postsListLimit)
	if err != nil {
		b.logger.Error("telegram list wildcard pool", "err", err)
		b.reply(ctx, chatID, "Failed to load the wildcard pool.")
		return
	}
	if len(posts) == 0 {
		b.reply(ctx, chatID, "Wildcard pool is empty. Use /posts to add some.")
		return
	}

	var lines []string
	var buttons [][]InlineKeyboardButton
	lines = append(lines, fmt.Sprintf("Wildcard pool (%d):", len(posts)))

	for _, p := range posts {
		url := b.postURL(p.Username, p.Slug)
		title := strings.TrimSpace(p.Title)
		if title == "" {
			title = "(untitled)"
		}
		lines = append(lines, fmt.Sprintf(
			"• #%d %s\n  %s by @%s (%s)",
			p.ID,
			title,
			url,
			p.Username,
			p.BlogLang,
		))
		buttons = append(buttons, []InlineKeyboardButton{{
			Text:         fmt.Sprintf("Remove #%d", p.ID),
			CallbackData: fmt.Sprintf("pool_remove:%d", p.ID),
		}})
	}

	markup := &InlineKeyboardMarkup{InlineKeyboard: buttons}
	if err := b.sendChunked(ctx, chatID, strings.Join(lines, "\n"), markup); err != nil {
		b.logger.Error("telegram send message", "err", err)
	}
}

func (b *Bot) cmdVerify(ctx context.Context, chatID int64, args []string) {
	if len(args) != 1 {
		b.reply(ctx, chatID, "Usage: /verify email@example.com")
		return
	}
	email := strings.ToLower(strings.TrimSpace(args[0]))
	user, err := b.store.VerifyEmailByAddress(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		b.reply(ctx, chatID, "No user found with that email.")
		return
	}
	if err != nil {
		b.logger.Error("telegram verify", "err", err)
		b.reply(ctx, chatID, "Could not verify email.")
		return
	}
	b.reply(ctx, chatID, fmt.Sprintf("Verified %s (%s).", user.Email, user.Username))
}

func (b *Bot) cmdSetUsername(ctx context.Context, chatID int64, args []string) {
	if len(args) != 2 {
		b.reply(ctx, chatID, "Usage: /setusername USERNAME_OR_EMAIL new_username")
		return
	}
	user, err := b.lookupUser(ctx, args[0])
	if err != nil {
		b.reply(ctx, chatID, "User not found.")
		return
	}

	newUsername := strings.ToLower(strings.TrimSpace(args[1]))
	if !validUsername(newUsername) {
		b.reply(ctx, chatID, "Invalid username. Use 3–32 lowercase letters, numbers, or hyphens.")
		return
	}

	user, err = b.store.UpdateUsername(ctx, user.ID, newUsername)
	if errors.Is(err, store.ErrUsernameTaken) {
		b.reply(ctx, chatID, "That username is already taken.")
		return
	}
	if err != nil {
		b.logger.Error("telegram set username", "err", err)
		b.reply(ctx, chatID, "Could not update username: "+err.Error())
		return
	}
	b.reply(ctx, chatID, fmt.Sprintf("Username updated to %s (%s).", user.Username, user.Email))
}

func (b *Bot) cmdSetEmail(ctx context.Context, chatID int64, args []string) {
	if len(args) != 2 {
		b.reply(ctx, chatID, "Usage: /setemail USERNAME_OR_EMAIL new@example.com")
		return
	}
	user, err := b.lookupUser(ctx, args[0])
	if err != nil {
		b.reply(ctx, chatID, "User not found.")
		return
	}

	newEmail := strings.ToLower(strings.TrimSpace(args[1]))
	if newEmail == "" || !strings.Contains(newEmail, "@") {
		b.reply(ctx, chatID, "Invalid email address.")
		return
	}

	user, err = b.store.UpdateEmail(ctx, user.ID, newEmail)
	if errors.Is(err, store.ErrEmailTaken) {
		b.reply(ctx, chatID, "That email is already in use.")
		return
	}
	if err != nil {
		b.logger.Error("telegram set email", "err", err)
		b.reply(ctx, chatID, "Could not update email: "+err.Error())
		return
	}
	b.reply(ctx, chatID, fmt.Sprintf("Email updated to %s (%s).", user.Email, user.Username))
}

func (b *Bot) cmdDelete(ctx context.Context, chatID int64, args []string) {
	if len(args) != 1 {
		b.reply(ctx, chatID, "Usage: /delete USERNAME_OR_EMAIL")
		return
	}
	user, err := b.lookupUser(ctx, args[0])
	if err != nil {
		b.reply(ctx, chatID, "User not found.")
		return
	}

	ref := args[0]
	if strings.Contains(ref, "@") {
		ref = user.Email
	} else {
		ref = user.Username
	}

	text := fmt.Sprintf("Delete %s (%s)? This cannot be undone.", user.Username, user.Email)
	markup := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
		{Text: "Confirm delete", CallbackData: "delete:" + ref},
	}}}
	_ = b.client.SendMessage(ctx, chatID, text, markup)
}

func (b *Bot) cmdInvite(ctx context.Context, chatID int64, note string) {
	code, err := newInviteCode()
	if err != nil {
		b.logger.Error("telegram invite code", "err", err)
		b.reply(ctx, chatID, "Could not generate invite code.")
		return
	}
	inv, err := b.store.CreateInvitation(ctx, code, strings.TrimSpace(note))
	if err != nil {
		b.logger.Error("telegram create invite", "err", err)
		b.reply(ctx, chatID, "Could not create invitation.")
		return
	}
	url := b.appURL + "/signup?invite=" + inv.Code
	b.reply(ctx, chatID, "Invitation created:\n"+url)
}

func (b *Bot) cmdWildcardFloor(ctx context.Context, chatID int64, args []string) {
	if len(args) == 0 {
		floor, err := b.store.WildcardImpressionFloor(ctx)
		if err != nil {
			b.logger.Error("telegram wildcard floor get", "err", err)
			b.reply(ctx, chatID, "Could not load wildcard impression floor.")
			return
		}
		b.reply(ctx, chatID, fmt.Sprintf("Wildcard impression floor: %d\n\nSend /wildcardfloor N to change it.", floor))
		return
	}
	if len(args) != 1 {
		b.reply(ctx, chatID, "Usage: /wildcardfloor [N]")
		return
	}

	floor, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || floor < 0 {
		b.reply(ctx, chatID, "Floor must be a non-negative integer.")
		return
	}
	if err := b.store.SetWildcardImpressionFloor(ctx, floor); err != nil {
		b.logger.Error("telegram wildcard floor set", "err", err)
		b.reply(ctx, chatID, "Could not update wildcard impression floor.")
		return
	}
	b.reply(ctx, chatID, fmt.Sprintf("Wildcard impression floor set to %d.", floor))
}

func (b *Bot) lookupUser(ctx context.Context, ref string) (store.User, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if strings.Contains(ref, "@") {
		return b.store.UserByEmail(ctx, ref)
	}
	return b.store.UserByUsername(ctx, ref)
}

func (b *Bot) blogURL(u store.User) string {
	if domain, ok := u.ActiveCustomDomain(); ok {
		return "https://" + domain
	}
	return "https://" + u.Username + "." + b.baseDomain
}

func (b *Bot) postURL(username, slug string) string {
	return "https://" + username + "." + b.baseDomain + "/" + slug
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if err := b.sendChunked(ctx, chatID, text, nil); err != nil {
		b.logger.Error("telegram send message", "err", err)
	}
}

// Notify sends text to every configured admin, logging (but not returning) per-recipient errors.
func (b *Bot) Notify(ctx context.Context, text string) {
	for chatID := range b.adminIDs {
		if err := b.sendChunked(ctx, chatID, text, nil); err != nil {
			b.logger.Error("telegram notify", "err", err, "chat_id", chatID)
		}
	}
}

// sendChunked splits text into Telegram-sized pieces and sends them in
// order, attaching markup only to the final piece. Telegram's sendMessage
// rejects anything over 4096 UTF-8 characters outright, which /users and
// /posts can exceed as their lists grow; without chunking, that rejection
// was only logged server-side and the admin got no message at all.
func (b *Bot) sendChunked(ctx context.Context, chatID int64, text string, markup *InlineKeyboardMarkup) error {
	chunks := splitMessage(text, telegramMessageLimit)
	for i, chunk := range chunks {
		var m *InlineKeyboardMarkup
		if i == len(chunks)-1 {
			m = markup
		}
		if err := b.client.SendMessage(ctx, chatID, chunk, m); err != nil {
			return err
		}
	}
	return nil
}

// splitMessage breaks text into chunks no longer than limit, preferring to
// split on line boundaries so a single user/post entry isn't cut in half.
func splitMessage(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	var cur strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		if cur.Len() > 0 && cur.Len()+1+len(line) > limit {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)
		for cur.Len() > limit {
			s := cur.String()
			chunks = append(chunks, s[:limit])
			cur.Reset()
			cur.WriteString(s[limit:])
		}
	}
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	return chunks
}

// AdminNotifier pushes proactive messages to a fixed set of admin chat ids
// without the polling loop or Store dependency a full Bot requires — for
// one-shot CLI jobs (cron) that only need to send, not receive, messages.
type AdminNotifier struct {
	client   *Client
	adminIDs []int64
	logger   *slog.Logger
}

func NewAdminNotifier(token string, adminIDs []int64, logger *slog.Logger) *AdminNotifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &AdminNotifier{
		client:   NewClient(token),
		adminIDs: adminIDs,
		logger:   logger,
	}
}

func (n *AdminNotifier) Notify(ctx context.Context, text string) {
	for _, chatID := range n.adminIDs {
		if err := n.client.SendMessage(ctx, chatID, text, nil); err != nil {
			n.logger.Error("telegram notify", "err", err, "chat_id", chatID)
		}
	}
}

func validUsername(username string) bool {
	if len(username) < 3 || len(username) > 32 || reservedUsernames[username] {
		return false
	}
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func newInviteCode() (string, error) {
	buf := make([]byte, inviteCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func helpText() string {
	return strings.TrimSpace(`
Waldi admin bot

/users — list users (newest first) with blog URLs
/posts — latest posts with links; tap a button to add to the wildcard pool
/pool — show today's wildcard pool; tap a button to remove a post
/verify email@example.com — force-verify a user's email
/setusername USER EMAIL_OR_USERNAME new_username
/setemail USER EMAIL_OR_USERNAME new@example.com
/delete USERNAME_OR_EMAIL — delete a user (confirmation required)
/invite [note] — create a signup invitation link
/wildcardfloor [N] — show or set the wildcard impression floor (fallback when the pool has no match)
`)
}
