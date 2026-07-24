package mail

import (
	"fmt"
	"html"
	"strings"
	"waldi/internal/i18n"
)

const emailStyles = `
body { margin:0; padding:0; background:#f5f0e8; color:#2a2a2a; font-family: Georgia, "Times New Roman", serif; }
.wrap { max-width: 520px; margin: 0 auto; padding: 40px 24px; }
.card { background:#fffdf8; border:1px solid #e8e0d4; padding: 32px 28px; }
.mark { color:#8a7f72; font-size: 24px; line-height: 1; }
h1 { font-size: 22px; font-weight: normal; margin: 16px 0 12px; }
p { font-size: 16px; line-height: 1.6; margin: 0 0 16px; }
.btn { display:inline-block; padding: 12px 20px; background:#2a2a2a; color:#fffdf8 !important; text-decoration:none; border-radius: 4px; }
.footer { margin-top: 24px; font-size: 13px; color:#8a7f72; }
`

const emailStylesRTL = `
body { font-family: "Vazirmatn", Tahoma, "Segoe UI", sans-serif; }
.wrap { text-align: right; }
`

func htmlEmail(lang, title, bodyHTML, footer string) string {
	dir := i18n.Dir(lang)
	styles := emailStyles
	if dir == "rtl" {
		styles += emailStylesRTL
	}
	return fmt.Sprintf(`<!doctype html><html lang="%s" dir="%s"><head><meta charset="utf-8"><style>%s</style></head><body><div class="wrap"><div class="card"><div class="mark">※</div><h1>%s</h1>%s<div class="footer">%s</div></div></div></body></html>`,
		html.EscapeString(lang), dir, styles, html.EscapeString(title), bodyHTML, footer)
}

// BrandName returns the From display name for the given locale (e.g. "Waldi"
// or "والدی"), shown in the recipient's inbox next to the From address.
func BrandName(lang string) string {
	return i18n.T(lang, "brand")
}

func buttonLink(href, label string) string {
	return fmt.Sprintf(`<p><a class="btn" href="%s">%s</a></p>`, html.EscapeString(href), html.EscapeString(label))
}

// VerificationEmail returns plain and HTML bodies for address verification.
func VerificationEmail(lang, verifyURL string) (subject, plain, htmlBody string) {
	switch lang {
	case "fa":
		subject = "تأیید ایمیل شما در والدی"
		plain = fmt.Sprintf("سلام.\n\nبرای شروع نوشتن در والدی، ایمیل خود را تأیید کنید:\n%s\n\nاگر این درخواست از طرف شما نبود، این پیام را نادیده بگیرید.\n\nوالدی ※", verifyURL)
		htmlBody = htmlEmail(lang, "تأیید ایمیل",
			fmt.Sprintf(`<p>سلام.</p><p>برای شروع نوشتن در والدی، ایمیل خود را تأیید کنید.</p>%s<p>اگر این درخواست از طرف شما نبود، این پیام را نادیده بگیرید.</p>`, buttonLink(verifyURL, "تأیید ایمیل")),
			"والدی ※")
	default:
		subject = "Confirm your Waldi email"
		plain = fmt.Sprintf("Hello.\n\nBefore you start writing on Waldi, please confirm your email:\n%s\n\nIf you didn't sign up, you can ignore this message.\n\nWaldi ※", verifyURL)
		htmlBody = htmlEmail(lang, "Confirm your email",
			fmt.Sprintf(`<p>Hello.</p><p>Before you start writing on Waldi, please confirm your email.</p>%s<p>If you didn't sign up, you can ignore this message.</p>`, buttonLink(verifyURL, "Confirm email")),
			"Waldi ※")
	}
	return subject, plain, htmlBody
}

// DigestEmail returns subject/plain/HTML bodies for the daily activity
// digest, including an unsubscribe link/footer.
func DigestEmail(lang string, lines []string, unsubscribeURL string) (subject, plain, htmlBody string) {
	switch lang {
	case "fa":
		subject = "خلاصه روزانه والدی"
		unsubLine := fmt.Sprintf("برای لغو اشتراک این خلاصه روزانه:\n%s", unsubscribeURL)
		plain = fmt.Sprintf("%s\n\n%s\n\nوالدی ※", strings.Join(lines, "\n"), unsubLine)
		var itemsHTML strings.Builder
		for _, line := range lines {
			fmt.Fprintf(&itemsHTML, "<p>%s</p>", html.EscapeString(line))
		}
		footer := fmt.Sprintf(`والدی ※ &middot; <a href="%s">لغو اشتراک این خلاصه</a>`, html.EscapeString(unsubscribeURL))
		htmlBody = htmlEmail(lang, "خلاصه روزانه", itemsHTML.String(), footer)
	default:
		subject = "Your Waldi daily digest"
		unsubLine := fmt.Sprintf("Unsubscribe from this daily digest:\n%s", unsubscribeURL)
		plain = fmt.Sprintf("%s\n\n%s\n\nWaldi ※", strings.Join(lines, "\n"), unsubLine)
		var itemsHTML strings.Builder
		for _, line := range lines {
			fmt.Fprintf(&itemsHTML, "<p>%s</p>", html.EscapeString(line))
		}
		footer := fmt.Sprintf(`Waldi ※ &middot; <a href="%s">Unsubscribe from this digest</a>`, html.EscapeString(unsubscribeURL))
		htmlBody = htmlEmail(lang, "Your daily digest", itemsHTML.String(), footer)
	}
	return subject, plain, htmlBody
}

// DigestLine is one item in the reader digest: a sentence describing a
// post, with a link to it. URL is required - every reader digest line
// points somewhere, unlike the writer digest's own-post stat lines.
type DigestLine struct {
	Text string
	URL  string
}

// ReaderDigestEmail returns subject/plain/HTML bodies for the reader digest:
// new posts from followed writers plus the day's wildcard stranger, each
// linking to the post, librarian voice, sharing the writer digest's
// unsubscribe link/footer.
func ReaderDigestEmail(lang string, followeeLines []DigestLine, wildcardLine *DigestLine, unsubscribeURL string) (subject, plain, htmlBody string) {
	lines := make([]DigestLine, 0, len(followeeLines)+1)
	lines = append(lines, followeeLines...)
	if wildcardLine != nil {
		lines = append(lines, *wildcardLine)
	}

	plainLines := make([]string, 0, len(lines))
	var itemsHTML strings.Builder
	for _, line := range lines {
		plainLines = append(plainLines, line.Text+"\n"+line.URL)
		fmt.Fprintf(&itemsHTML, `<p><a href="%s">%s</a></p>`, html.EscapeString(line.URL), html.EscapeString(line.Text))
	}

	switch lang {
	case "fa":
		subject = "نامه صبحگاهی والدی"
		unsubLine := fmt.Sprintf("برای لغو اشتراک این نامه:\n%s", unsubscribeURL)
		plain = fmt.Sprintf("%s\n\n%s\n\nوالدی ※", strings.Join(plainLines, "\n\n"), unsubLine)
		footer := fmt.Sprintf(`والدی ※ &middot; <a href="%s">لغو اشتراک این نامه</a>`, html.EscapeString(unsubscribeURL))
		htmlBody = htmlEmail(lang, "نامه صبحگاهی", itemsHTML.String(), footer)
	default:
		subject = "Your Waldi morning letter"
		unsubLine := fmt.Sprintf("Unsubscribe from this letter:\n%s", unsubscribeURL)
		plain = fmt.Sprintf("%s\n\n%s\n\nWaldi ※", strings.Join(plainLines, "\n\n"), unsubLine)
		footer := fmt.Sprintf(`Waldi ※ &middot; <a href="%s">Unsubscribe from this letter</a>`, html.EscapeString(unsubscribeURL))
		htmlBody = htmlEmail(lang, "Your morning letter", itemsHTML.String(), footer)
	}
	return subject, plain, htmlBody
}

// PasswordResetEmail returns plain and HTML bodies for password reset.
func PasswordResetEmail(lang, resetURL string) (subject, plain, htmlBody string) {
	switch lang {
	case "fa":
		subject = "بازیابی رمز عبور والدی"
		plain = fmt.Sprintf("سلام.\n\nبرای انتخاب رمز عبور جدید، این پیوند را باز کنید:\n%s\n\nاین پیوند یک ساعت معتبر است.\n\nوالدی ※", resetURL)
		htmlBody = htmlEmail(lang, "بازیابی رمز عبور",
			fmt.Sprintf(`<p>سلام.</p><p>برای انتخاب رمز عبور جدید، این دکمه را بزنید.</p>%s<p>این پیوند یک ساعت معتبر است.</p>`, buttonLink(resetURL, "انتخاب رمز جدید")),
			"والدی ※")
	default:
		subject = "Reset your Waldi password"
		plain = fmt.Sprintf("Hello.\n\nUse this link to choose a new password:\n%s\n\nThis link expires in one hour.\n\nWaldi ※", resetURL)
		htmlBody = htmlEmail(lang, "Reset your password",
			fmt.Sprintf(`<p>Hello.</p><p>Use the button below to choose a new password.</p>%s<p>This link expires in one hour.</p>`, buttonLink(resetURL, "Choose new password")),
			"Waldi ※")
	}
	return subject, plain, htmlBody
}

// ReactivationEmail returns plain and HTML bodies for the re-permission
// email sent once to accounts inactive ~30 days, asking whether they still
// want digest email. Digests stay paused unless resumeURL is clicked -
// silence is the safe default, not an assumption of continued interest.
func ReactivationEmail(lang, resumeURL string) (subject, plain, htmlBody string) {
	switch lang {
	case "fa":
		subject = "هنوز خلاصه\u200cهای والدی را می\u200cخواهید؟"
		plain = fmt.Sprintf("سلام.\n\nمدتی است سری به والدی نزده\u200cاید، برای همین فرستادن خلاصه\u200cهای روزانه را موقتاً متوقف کردیم.\n\nاگر هنوز دلتان می\u200cخواهد این نامه\u200cها را دریافت کنید، این پیوند را باز کنید:\n%s\n\nاگر کاری نکنید، دیگر برایتان ایمیلی نمی\u200cفرستیم.\n\nوالدی ※", resumeURL)
		htmlBody = htmlEmail(lang, "هنوز می\u200cخواهید؟",
			fmt.Sprintf("<p>سلام.</p><p>مدتی است سری به والدی نزده\u200cاید، برای همین فرستادن خلاصه\u200cهای روزانه را موقتاً متوقف کردیم.</p><p>اگر هنوز دلتان می\u200cخواهد این نامه\u200cها را دریافت کنید:</p>%s<p>اگر کاری نکنید، دیگر برایتان ایمیلی نمی\u200cفرستیم.</p>", buttonLink(resumeURL, "بله، ادامه بده")),
			"والدی ※")
	default:
		subject = "Still want Waldi's digests?"
		plain = fmt.Sprintf("Hello.\n\nYou haven't been by Waldi in a while, so we've paused your daily digests.\n\nIf you'd still like to receive them, open this link:\n%s\n\nIf you don't, we won't email you again.\n\nWaldi ※", resumeURL)
		htmlBody = htmlEmail(lang, "Still want these?",
			fmt.Sprintf(`<p>Hello.</p><p>You haven't been by Waldi in a while, so we've paused your daily digests.</p><p>If you'd still like to receive them:</p>%s<p>If you don't, we won't email you again.</p>`, buttonLink(resumeURL, "Yes, keep sending")),
			"Waldi ※")
	}
	return subject, plain, htmlBody
}
