// Copyright (c) 2020, Amazon.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package google

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAccessTokenFile(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()

	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		return path
	}

	t.Run("valid", func(t *testing.T) {
		path := write("ok.json", `{"access_token":"abc","expiry":"2026-06-02T13:00:00Z"}`)
		tok, err := loadAccessTokenFile(path, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok.AccessToken != "abc" {
			t.Errorf("AccessToken=%q, want abc", tok.AccessToken)
		}
		if !tok.Expiry.Equal(time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)) {
			t.Errorf("Expiry=%v", tok.Expiry)
		}
	})

	t.Run("missing_file", func(t *testing.T) {
		_, err := loadAccessTokenFile(filepath.Join(dir, "does-not-exist.json"), now)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("malformed_json", func(t *testing.T) {
		path := write("bad.json", `{not json`)
		if _, err := loadAccessTokenFile(path, now); err == nil {
			t.Fatal("expected error for malformed json")
		}
	})

	t.Run("empty_access_token", func(t *testing.T) {
		path := write("empty.json", `{"access_token":"","expiry":"2026-06-02T13:00:00Z"}`)
		if _, err := loadAccessTokenFile(path, now); err == nil {
			t.Fatal("expected error for empty access_token")
		}
	})

	t.Run("missing_expiry", func(t *testing.T) {
		path := write("noexp.json", `{"access_token":"abc"}`)
		if _, err := loadAccessTokenFile(path, now); err == nil {
			t.Fatal("expected error for missing expiry")
		}
	})

	t.Run("malformed_expiry", func(t *testing.T) {
		path := write("badexp.json", `{"access_token":"abc","expiry":"yesterday"}`)
		if _, err := loadAccessTokenFile(path, now); err == nil {
			t.Fatal("expected error for malformed expiry")
		}
	})

	t.Run("already_expired", func(t *testing.T) {
		path := write("expired.json", `{"access_token":"abc","expiry":"2026-06-02T11:00:00Z"}`)
		if _, err := loadAccessTokenFile(path, now); err == nil {
			t.Fatal("expected error for already-expired token")
		}
	})

	t.Run("expiry_equal_to_now", func(t *testing.T) {
		// not strictly in the future — should be rejected
		path := write("equal.json", `{"access_token":"abc","expiry":"2026-06-02T12:00:00Z"}`)
		if _, err := loadAccessTokenFile(path, now); err == nil {
			t.Fatal("expected error when expiry == now")
		}
	})
}

