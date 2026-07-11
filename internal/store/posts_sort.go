package store

import "time"

// DashboardSortAt is the timestamp used to order posts on /write.
// Published posts sort by publish time; drafts sort by creation time so
// autosaves do not shuffle the list.
func (p Post) DashboardSortAt() time.Time {
	if p.Status == "published" && p.PublishedAt != nil {
		return *p.PublishedAt
	}
	return p.CreatedAt
}

const postDashboardSortSQL = `case when status = 'published' then published_at else created_at end`
