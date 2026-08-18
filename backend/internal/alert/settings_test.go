package alert

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testDefaults() Config {
	return Config{
		Enabled:           true,
		PollInterval:      30 * time.Second,
		Cooldown:          10 * time.Minute,
		WebhookURL:        "",
		WebhookFormat:     WebhookFormatGeneric,
		ErrorRowsRatio:    0.01,
		ErrorRowsMinTotal: 10_000,
		MaxOffsetLag:      0,
	}
}

func settingsPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "alerts.json")
}

func ptr[T any](v T) *T { return &v }

func TestLoadSettingsMissingFileIsFresh(t *testing.T) {
	s, err := LoadSettings(settingsPath(t), testDefaults())
	if err != nil {
		t.Fatalf("LoadSettings() error = %v, want nil for a missing file", err)
	}
	if got := s.Effective(); got != testDefaults() {
		t.Errorf("Effective() = %+v, want pure defaults", got)
	}
	if len(s.Overridden()) != 0 {
		t.Errorf("Overridden() = %v, want empty", s.Overridden())
	}
}

func TestUpdateOverlaysAndPersists(t *testing.T) {
	path := settingsPath(t)
	s, _ := LoadSettings(path, testDefaults())

	patch := Override{
		Cooldown:       ptr("5m"),
		WebhookURL:     ptr("https://hooks.slack.com/services/T0/B0/xyz"),
		WebhookFormat:  ptr(WebhookFormatSlack),
		ErrorRowsRatio: ptr(0.05),
	}
	if err := s.Update(patch, false); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	effective := s.Effective()
	if effective.Cooldown != 5*time.Minute || effective.WebhookFormat != WebhookFormatSlack {
		t.Errorf("Effective() = %+v", effective)
	}
	if effective.ErrorRowsRatio != 0.05 {
		t.Errorf("ErrorRowsRatio = %v, want 0.05", effective.ErrorRowsRatio)
	}
	// Fields the patch did not touch keep their environment defaults.
	if effective.PollInterval != 30*time.Second || effective.ErrorRowsMinTotal != 10_000 {
		t.Errorf("untouched fields drifted: %+v", effective)
	}

	// A restart must see the same effective configuration.
	reloaded, err := LoadSettings(path, testDefaults())
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	if reloaded.Effective() != effective {
		t.Errorf("reloaded = %+v, want %+v", reloaded.Effective(), effective)
	}
	if got := len(reloaded.Overridden()); got != 4 {
		t.Errorf("Overridden() length = %d, want 4 (%v)", got, reloaded.Overridden())
	}
}

func TestUpdateMergesSuccessivePatches(t *testing.T) {
	s, _ := LoadSettings(settingsPath(t), testDefaults())

	_ = s.Update(Override{Cooldown: ptr("5m")}, false)
	_ = s.Update(Override{ErrorRowsRatio: ptr(0.2)}, false)

	effective := s.Effective()
	if effective.Cooldown != 5*time.Minute || effective.ErrorRowsRatio != 0.2 {
		t.Errorf("second patch must not erase the first: %+v", effective)
	}
}

func TestResetClearsOverridesAndRemovesFile(t *testing.T) {
	path := settingsPath(t)
	s, _ := LoadSettings(path, testDefaults())

	_ = s.Update(Override{Cooldown: ptr("5m"), WebhookURL: ptr("https://x.example/hook")}, false)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("override file should exist after update: %v", err)
	}

	if err := s.Update(Override{}, true); err != nil {
		t.Fatalf("reset error = %v", err)
	}
	if got := s.Effective(); got != testDefaults() {
		t.Errorf("Effective() after reset = %+v, want defaults", got)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("override file should be removed on reset, stat err = %v", err)
	}
}

func TestUpdateExplicitEmptyWebhookOverridesEnv(t *testing.T) {
	defaults := testDefaults()
	defaults.WebhookURL = "https://hooks.slack.com/from-env"
	s, _ := LoadSettings(settingsPath(t), defaults)

	// "" is an explicit override: disable the webhook even though env has one.
	if err := s.Update(Override{WebhookURL: ptr("")}, false); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got := s.Effective().WebhookURL; got != "" {
		t.Errorf("WebhookURL = %q, want empty override to win over env", got)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	cases := map[string]Override{
		"bad duration":       {Cooldown: ptr("banana")},
		"interval too small": {PollInterval: ptr("100ms")},
		"negative cooldown":  {Cooldown: ptr("-1m")},
		"bad url":            {WebhookURL: ptr("not a url")},
		"ftp url":            {WebhookURL: ptr("ftp://example.com/x")},
		"bad format":         {WebhookFormat: ptr("teams")},
		"ratio over 1":       {ErrorRowsRatio: ptr(1.5)},
		"negative min total": {ErrorRowsMinTotal: ptr(int64(-1))},
		"negative lag":       {MaxOffsetLag: ptr(int64(-5))},
	}

	s, _ := LoadSettings(settingsPath(t), testDefaults())
	for name, patch := range cases {
		t.Run(name, func(t *testing.T) {
			err := s.Update(patch, false)
			if !errors.Is(err, ErrInvalidOverride) {
				t.Errorf("Update(%+v) = %v, want ErrInvalidOverride", patch, err)
			}
		})
	}
	// None of the rejected patches may have leaked into the effective config.
	if got := s.Effective(); got != testDefaults() {
		t.Errorf("rejected patches mutated config: %+v", got)
	}
}

func TestLoadSettingsCorruptFileKeepsBooting(t *testing.T) {
	path := settingsPath(t)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSettings(path, testDefaults())
	if err == nil {
		t.Error("corrupt file should be reported")
	}
	if s == nil || s.Effective() != testDefaults() {
		t.Error("corrupt file must still yield usable defaults-only settings")
	}
}

func TestMaskWebhookURL(t *testing.T) {
	cases := map[string]string{
		"": "",
		"https://hooks.slack.com/services/T0/B0/secret": "https://hooks.slack.com/•••",
		"http://internal:9000/hook":                     "http://internal:9000/•••",
		"::bad::":                                       "•••",
	}
	for in, want := range cases {
		if got := MaskWebhookURL(in); got != want {
			t.Errorf("MaskWebhookURL(%q) = %q, want %q", in, got, want)
		}
	}
}
