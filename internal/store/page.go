package store

import "time"

// PageCursor requests items strictly older than (Before, LastID).
// LastID disambiguates rows that share the same sort timestamp.
type PageCursor struct {
	Before time.Time
	LastID int64
}

func (c PageCursor) Active() bool {
	return !c.Before.IsZero()
}

func cursorArgs(c PageCursor) (before *time.Time, lastID *int64) {
	if !c.Active() {
		return nil, nil
	}
	t := c.Before.UTC()
	before = &t
	if c.LastID > 0 {
		lastID = &c.LastID
	}
	return before, lastID
}
