package store

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// realPostFilter excludes drafts, pages, and short/test posts from
// growth-metric denominators, matching the quality bar already used by the
// wildcard candidate query.
const realPostFilter = `
	p.status = 'published'
	and p.type = 'post'
	and p.word_count >= 50
	and lower(p.title) !~ '(^|[^a-z])test([^a-z]|$)'
	and lower(p.slug) !~ '(^|[^a-z])test([^a-z]|$)'
`

type WeeklyStats struct {
	PostsPublished          int
	PostsReaching100In24h   int
	PostsWithFollowOrLetter int
	PostsWithLetter         int
	PostsWithFollow         int
	AvgTimeToFirstFollower  *time.Duration

	CohortSize     int
	CohortRetained int

	ActiveReaders int
	RitualReaders int

	WildcardsShown       int
	WildcardsSkipped     int
	WildcardsCompletable int
	WildcardsCompleted   int

	WeeklyReaders int
	ActiveWriters int
}

// WeeklyStats aggregates growth metrics for the window [since, until), plus
// a writer-retention cohort whose first published post fell in
// [cohortStart, cohortEnd) — checked for activity again in [since, until).
func (s *Store) WeeklyStats(ctx context.Context, since, until, cohortStart, cohortEnd time.Time) (WeeklyStats, error) {
	var stats WeeklyStats

	if err := s.pool.QueryRow(ctx, `
		with weekly_posts as (
			select p.id, p.published_at
			from posts p
			where `+realPostFilter+`
			  and p.published_at >= $1 and p.published_at < $2
		),
		imp24 as (
			select wp.id, count(distinct i.reader_key) as impressions_24h
			from weekly_posts wp
			left join impressions i
				on i.post_id = wp.id and i.created_at < wp.published_at + interval '24 hours'
			group by wp.id
		),
		engagement as (
			select wp.id,
			       count(distinct f.follower_id) as follows,
			       count(distinct l.id) as letters,
			       min(f.created_at) as first_follow_at
			from weekly_posts wp
			left join follows f on f.source_post_id = wp.id
			left join letters l on l.post_id = wp.id
			group by wp.id
		)
		select
			count(*) as total,
			count(*) filter (where imp24.impressions_24h >= 100) as reaching_100,
			count(*) filter (where engagement.follows > 0 or engagement.letters > 0) as with_follow_or_letter,
			count(*) filter (where engagement.letters > 0) as with_letter,
			count(*) filter (where engagement.follows > 0) as with_follow,
			avg(extract(epoch from (engagement.first_follow_at - wp.published_at)))
				filter (where engagement.follows > 0) as avg_ttff_seconds
		from weekly_posts wp
		join imp24 on imp24.id = wp.id
		join engagement on engagement.id = wp.id
	`, since, until).Scan(
		&stats.PostsPublished,
		&stats.PostsReaching100In24h,
		&stats.PostsWithFollowOrLetter,
		&stats.PostsWithLetter,
		&stats.PostsWithFollow,
		scanNullableSeconds(&stats.AvgTimeToFirstFollower),
	); err != nil {
		return WeeklyStats{}, fmt.Errorf("loading weekly post stats: %w", err)
	}

	if err := s.pool.QueryRow(ctx, `
		with first_post as (
			select user_id, min(published_at) as first_pub
			from posts
			where status = 'published' and type = 'post'
			group by user_id
		),
		cohort as (
			select user_id from first_post where first_pub >= $1 and first_pub < $2
		),
		active as (
			select distinct user_id
			from posts
			where status = 'published' and type = 'post'
			  and published_at >= $3 and published_at < $4
		)
		select
			count(*) as cohort_size,
			count(*) filter (where c.user_id in (select user_id from active)) as retained
		from cohort c
	`, cohortStart, cohortEnd, since, until).Scan(&stats.CohortSize, &stats.CohortRetained); err != nil {
		return WeeklyStats{}, fmt.Errorf("loading writer retention: %w", err)
	}

	if err := s.pool.QueryRow(ctx, `
		with daily as (
			select reader_key, date(created_at) as day
			from impressions
			where created_at >= $1 and created_at < $2
			group by reader_key, date(created_at)
		),
		per_reader as (
			select reader_key, count(*) as days
			from daily
			group by reader_key
		)
		select count(*) as active, count(*) filter (where days >= 3) as ritual
		from per_reader
	`, since, until).Scan(&stats.ActiveReaders, &stats.RitualReaders); err != nil {
		return WeeklyStats{}, fmt.Errorf("loading reader ritual rate: %w", err)
	}

	if err := s.pool.QueryRow(ctx, `
		with weekly_wc as (
			select user_id, post_id, skipped
			from wildcards
			where date >= $1 and date < $2
		)
		select
			count(*) as shown,
			count(*) filter (where skipped) as skipped,
			count(*) filter (where not skipped) as completable,
			count(*) filter (
				where not skipped and exists (
					select 1 from impressions i
					join readings r on r.impression_id = i.id
					where i.post_id = weekly_wc.post_id
					  and i.reader_key = 'user:' || weekly_wc.user_id
					  and r.completed
				)
			) as completed
		from weekly_wc
	`, since, until).Scan(
		&stats.WildcardsShown,
		&stats.WildcardsSkipped,
		&stats.WildcardsCompletable,
		&stats.WildcardsCompleted,
	); err != nil {
		return WeeklyStats{}, fmt.Errorf("loading wildcard completion: %w", err)
	}

	if err := s.pool.QueryRow(ctx, `
		select
			(select count(distinct reader_key) from impressions where created_at >= $1 and created_at < $2),
			(select count(distinct p.user_id) from posts p where `+realPostFilter+` and p.published_at >= $1 and p.published_at < $2)
	`, since, until).Scan(&stats.WeeklyReaders, &stats.ActiveWriters); err != nil {
		return WeeklyStats{}, fmt.Errorf("loading weekly readers and active writers: %w", err)
	}

	return stats, nil
}

// scanNullableSeconds adapts a nullable float-seconds SQL column into a
// *time.Duration destination, leaving it nil when the column is null.
func scanNullableSeconds(dest **time.Duration) any {
	return &nullableSecondsScanner{dest: dest}
}

type nullableSecondsScanner struct {
	dest **time.Duration
}

func (n *nullableSecondsScanner) Scan(value any) error {
	if value == nil {
		*n.dest = nil
		return nil
	}
	var seconds float64
	switch v := value.(type) {
	case float64:
		seconds = v
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("parsing seconds column %q: %w", v, err)
		}
		seconds = parsed
	default:
		return fmt.Errorf("unexpected type %T for seconds column", value)
	}
	d := time.Duration(seconds * float64(time.Second))
	*n.dest = &d
	return nil
}
