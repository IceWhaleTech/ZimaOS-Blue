package task

import (
	"database/sql"
	"time"
)

// TimeToSQL formats a time.Time as RFC3339 for SQLite TEXT columns.
func TimeToSQL(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// TimeFromSQL parses an RFC3339 string from SQLite back to time.Time.
// Returns zero time on parse failure.
func TimeFromSQL(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NullTimeToSQL converts a *time.Time to sql.NullString (RFC3339).
// Returns an invalid NullString if t is nil.
func NullTimeToSQL(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}

// NullTimeFromSQL converts a sql.NullString (RFC3339) back to *time.Time.
// Returns nil if the NullString is not valid.
func NullTimeFromSQL(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	t, _ := time.Parse(time.RFC3339, ns.String)
	return &t
}
