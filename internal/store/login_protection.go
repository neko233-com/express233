package store

import (
	"database/sql"
	"time"
)

// LoginIPPolicy controls persistent IP-based credential-stuffing protection.
// Values are supplied by the API layer so deployments can tune them safely.
type LoginIPPolicy struct {
	FailureLimit int
	Window       time.Duration
	BaseBan      time.Duration
	MaxBan       time.Duration
}

type LoginIPBan struct {
	IP            string `json:"ip"`
	Failures      int    `json:"failures"`
	BanCount      int    `json:"ban_count"`
	WindowStarted string `json:"window_started"`
	BannedUntil   string `json:"banned_until"`
	LastFailure   string `json:"last_failure"`
}

func (s *Store) migrateLoginProtection() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS login_ip_bans (
  ip TEXT PRIMARY KEY,
  failures INTEGER NOT NULL DEFAULT 0,
  ban_count INTEGER NOT NULL DEFAULT 0,
  window_started TEXT NOT NULL,
  banned_until TEXT NOT NULL DEFAULT '',
  last_failure TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_login_ip_bans_until ON login_ip_bans(banned_until);
`)
	return err
}

func (s *Store) LoginIPBlocked(ip string, now time.Time) (time.Duration, bool, error) {
	var until string
	err := s.db.QueryRow(`SELECT banned_until FROM login_ip_bans WHERE ip=?`, ip).Scan(&until)
	if err == sql.ErrNoRows || until == "" {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	t, err := time.Parse(time.RFC3339Nano, until)
	if err != nil || !t.After(now) {
		return 0, false, nil
	}
	return time.Until(t).Round(time.Second), true, nil
}

// RecordLoginIPFailure advances the counter atomically and returns the current
// ban. A repeat offender receives an exponential ban capped by MaxBan.
func (s *Store) RecordLoginIPFailure(ip string, now time.Time, policy LoginIPPolicy) (LoginIPBan, error) {
	out := LoginIPBan{IP: ip}
	tx, err := s.db.Begin()
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback() }()
	var windowStarted, bannedUntil string
	err = tx.QueryRow(`SELECT failures,ban_count,window_started,banned_until,last_failure FROM login_ip_bans WHERE ip=?`, ip).Scan(&out.Failures, &out.BanCount, &windowStarted, &bannedUntil, &out.LastFailure)
	if err == sql.ErrNoRows {
		out.Failures, out.BanCount = 0, 0
		windowStarted = now.UTC().Format(time.RFC3339Nano)
	} else if err != nil {
		return out, err
	}
	started, parseErr := time.Parse(time.RFC3339Nano, windowStarted)
	if parseErr != nil || now.Sub(started) >= policy.Window {
		out.Failures = 0
		windowStarted = now.UTC().Format(time.RFC3339Nano)
	}
	out.Failures++
	if out.Failures >= policy.FailureLimit {
		out.BanCount++
		ban := policy.BaseBan
		for i := 1; i < out.BanCount && ban < policy.MaxBan; i++ {
			ban *= 2
		}
		if ban > policy.MaxBan {
			ban = policy.MaxBan
		}
		bannedUntil = now.Add(ban).UTC().Format(time.RFC3339Nano)
		out.Failures = 0
		windowStarted = now.UTC().Format(time.RFC3339Nano)
	}
	out.WindowStarted, out.BannedUntil, out.LastFailure = windowStarted, bannedUntil, now.UTC().Format(time.RFC3339Nano)
	_, err = tx.Exec(`INSERT INTO login_ip_bans(ip,failures,ban_count,window_started,banned_until,last_failure) VALUES(?,?,?,?,?,?) ON CONFLICT(ip) DO UPDATE SET failures=excluded.failures,ban_count=excluded.ban_count,window_started=excluded.window_started,banned_until=excluded.banned_until,last_failure=excluded.last_failure`, out.IP, out.Failures, out.BanCount, out.WindowStarted, out.BannedUntil, out.LastFailure)
	if err != nil {
		return LoginIPBan{}, err
	}
	return out, tx.Commit()
}

func (s *Store) ClearLoginIPFailures(ip string) error {
	_, err := s.db.Exec(`DELETE FROM login_ip_bans WHERE ip=?`, ip)
	return err
}

func (s *Store) ListLoginIPBans(now time.Time, limit int) ([]LoginIPBan, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT ip,failures,ban_count,window_started,banned_until,last_failure FROM login_ip_bans WHERE banned_until > ? ORDER BY banned_until DESC LIMIT ?`, now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LoginIPBan
	for rows.Next() {
		var b LoginIPBan
		if err := rows.Scan(&b.IP, &b.Failures, &b.BanCount, &b.WindowStarted, &b.BannedUntil, &b.LastFailure); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
