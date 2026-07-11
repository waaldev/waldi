package importblogir

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"
)

// Export mirrors the blog.ir "posts archive" XML export format.
type Export struct {
	XMLName xml.Name `xml:"BLOG"`
	Info    BlogInfo `xml:"BLOG_INFO"`
	Posts   []Post   `xml:"POSTS>POST"`
}

type BlogInfo struct {
	Domain string `xml:"DOMAIN"`
	Title  string `xml:"TITLE"`
}

type Post struct {
	Title            string `xml:"TITLE"`
	Number           int64  `xml:"NUMBER"`
	URL              string `xml:"URL"`
	Link             string `xml:"LINK"`
	Content          string `xml:"CONTENT"`
	LastModifiedDate string `xml:"LAST_MODIFIED_DATE"`
	CreatedDate      string `xml:"CREATED_DATE"`
}

func LoadExport(path string) (Export, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Export{}, fmt.Errorf("reading export: %w", err)
	}
	return ParseExport(raw)
}

// ParseExport parses an in-memory blog.ir posts archive XML export, e.g. one
// received as an uploaded file rather than read from disk.
func ParseExport(raw []byte) (Export, error) {
	var exp Export
	if err := xml.Unmarshal(raw, &exp); err != nil {
		return Export{}, fmt.Errorf("parsing export: %w", err)
	}
	return exp, nil
}

// ParseTime parses blog.ir's "2006-01-02 15:04:05.999999" timestamps.
func ParseTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time %q", raw)
}
