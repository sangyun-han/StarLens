package alert

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config is the effective alerting configuration: environment defaults with
// any UI-authored overrides applied on top.
type Config struct {
	Enabled       bool
	PollInterval  time.Duration
	Cooldown      time.Duration
	WebhookURL    string
	WebhookFormat string

	ErrorRowsRatio    float64
	ErrorRowsMinTotal int64
	MaxOffsetLag      int64
	MaxJournalLag     int64
}

// Override is the UI-authored partial configuration, persisted as JSON. Every
// field is a pointer: nil means "not overridden, fall back to the environment
// default". Durations are strings ("30s", "10m") so the file stays
// hand-editable.
type Override struct {
	Enabled           *bool    `json:"enabled,omitempty"`
	PollInterval      *string  `json:"pollInterval,omitempty"`
	Cooldown          *string  `json:"cooldown,omitempty"`
	WebhookURL        *string  `json:"webhookUrl,omitempty"`
	WebhookFormat     *string  `json:"webhookFormat,omitempty"`
	ErrorRowsRatio    *float64 `json:"errorRowsRatio,omitempty"`
	ErrorRowsMinTotal *int64   `json:"errorRowsMinTotal,omitempty"`
	MaxOffsetLag      *int64   `json:"maxOffsetLag,omitempty"`
	MaxJournalLag     *int64   `json:"maxJournalLag,omitempty"`
}

func (o Override) isEmpty() bool {
	return o.Enabled == nil && o.PollInterval == nil && o.Cooldown == nil &&
		o.WebhookURL == nil && o.WebhookFormat == nil && o.ErrorRowsRatio == nil &&
		o.ErrorRowsMinTotal == nil && o.MaxOffsetLag == nil && o.MaxJournalLag == nil
}

// minPollInterval keeps a UI typo from turning the evaluation loop into a
// busy-wait against the cluster.
const minPollInterval = 5 * time.Second

// ErrInvalidOverride wraps every validation failure so the API layer can map
// them to 400.
var ErrInvalidOverride = errors.New("invalid alert configuration")

// Validate checks an override without applying it.
func (o Override) Validate() error {
	if o.PollInterval != nil {
		d, err := time.ParseDuration(*o.PollInterval)
		if err != nil {
			return fmt.Errorf("%w: pollInterval %q is not a duration (want e.g. \"30s\")", ErrInvalidOverride, *o.PollInterval)
		}
		if d < minPollInterval {
			return fmt.Errorf("%w: pollInterval must be at least %s", ErrInvalidOverride, minPollInterval)
		}
	}
	if o.Cooldown != nil {
		d, err := time.ParseDuration(*o.Cooldown)
		if err != nil {
			return fmt.Errorf("%w: cooldown %q is not a duration (want e.g. \"10m\")", ErrInvalidOverride, *o.Cooldown)
		}
		if d < 0 {
			return fmt.Errorf("%w: cooldown must not be negative", ErrInvalidOverride)
		}
	}
	if o.WebhookURL != nil && *o.WebhookURL != "" {
		u, err := url.Parse(*o.WebhookURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: webhookUrl must be an http(s) URL", ErrInvalidOverride)
		}
	}
	if o.WebhookFormat != nil &&
		*o.WebhookFormat != WebhookFormatGeneric && *o.WebhookFormat != WebhookFormatSlack {
		return fmt.Errorf("%w: webhookFormat must be %q or %q",
			ErrInvalidOverride, WebhookFormatGeneric, WebhookFormatSlack)
	}
	if o.ErrorRowsRatio != nil && (*o.ErrorRowsRatio < 0 || *o.ErrorRowsRatio > 1) {
		return fmt.Errorf("%w: errorRowsRatio must be within [0, 1] (a fraction, not a percent)", ErrInvalidOverride)
	}
	if o.ErrorRowsMinTotal != nil && *o.ErrorRowsMinTotal < 0 {
		return fmt.Errorf("%w: errorRowsMinTotal must not be negative", ErrInvalidOverride)
	}
	if o.MaxOffsetLag != nil && *o.MaxOffsetLag < 0 {
		return fmt.Errorf("%w: maxOffsetLag must not be negative", ErrInvalidOverride)
	}
	if o.MaxJournalLag != nil && *o.MaxJournalLag < 0 {
		return fmt.Errorf("%w: maxJournalLag must not be negative", ErrInvalidOverride)
	}
	return nil
}

// Settings layers a persisted Override over environment defaults.
//
// Concurrency: Effective() is read on every poller tick and API call; Update()
// is rare. A plain RWMutex is plenty.
type Settings struct {
	mu       sync.RWMutex
	defaults Config
	override Override
	path     string
}

// LoadSettings reads the override file if it exists. A missing file is the
// normal fresh state. A corrupt or invalid file is reported through the error
// while still returning usable Settings with no overrides — a broken override
// must not keep the server from booting.
func LoadSettings(path string, defaults Config) (*Settings, error) {
	s := &Settings{defaults: defaults, path: path}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("alert: read override file %s: %w", path, err)
	}

	var override Override
	if err := json.Unmarshal(raw, &override); err != nil {
		return s, fmt.Errorf("alert: parse override file %s: %w", path, err)
	}
	if err := override.Validate(); err != nil {
		return s, fmt.Errorf("alert: override file %s: %w", path, err)
	}

	s.override = override
	return s, nil
}

// Effective returns defaults with the current override applied.
func (s *Settings) Effective() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := s.defaults
	o := s.override
	if o.Enabled != nil {
		c.Enabled = *o.Enabled
	}
	if o.PollInterval != nil {
		c.PollInterval = mustDuration(*o.PollInterval, c.PollInterval)
	}
	if o.Cooldown != nil {
		c.Cooldown = mustDuration(*o.Cooldown, c.Cooldown)
	}
	if o.WebhookURL != nil {
		c.WebhookURL = *o.WebhookURL
	}
	if o.WebhookFormat != nil {
		c.WebhookFormat = *o.WebhookFormat
	}
	if o.ErrorRowsRatio != nil {
		c.ErrorRowsRatio = *o.ErrorRowsRatio
	}
	if o.ErrorRowsMinTotal != nil {
		c.ErrorRowsMinTotal = *o.ErrorRowsMinTotal
	}
	if o.MaxOffsetLag != nil {
		c.MaxOffsetLag = *o.MaxOffsetLag
	}
	if o.MaxJournalLag != nil {
		c.MaxJournalLag = *o.MaxJournalLag
	}
	return c
}

// Overridden lists the JSON field names currently overridden via the UI, so
// the frontend can show which values no longer track the environment.
func (s *Settings) Overridden() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fields := make([]string, 0, 8)
	o := s.override
	appendIf := func(set bool, name string) {
		if set {
			fields = append(fields, name)
		}
	}
	appendIf(o.Enabled != nil, "enabled")
	appendIf(o.PollInterval != nil, "pollInterval")
	appendIf(o.Cooldown != nil, "cooldown")
	appendIf(o.WebhookURL != nil, "webhookUrl")
	appendIf(o.WebhookFormat != nil, "webhookFormat")
	appendIf(o.ErrorRowsRatio != nil, "errorRowsRatio")
	appendIf(o.ErrorRowsMinTotal != nil, "errorRowsMinTotal")
	appendIf(o.MaxOffsetLag != nil, "maxOffsetLag")
	appendIf(o.MaxJournalLag != nil, "maxJournalLag")
	return fields
}

// Update validates the patch, merges it into the stored override (or replaces
// everything when reset is true) and persists the result atomically. Non-nil
// patch fields become overrides; nil fields are left as they were.
func (s *Settings) Update(patch Override, reset bool) error {
	if err := patch.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	merged := s.override
	if reset {
		merged = Override{}
	}
	if patch.Enabled != nil {
		merged.Enabled = patch.Enabled
	}
	if patch.PollInterval != nil {
		merged.PollInterval = patch.PollInterval
	}
	if patch.Cooldown != nil {
		merged.Cooldown = patch.Cooldown
	}
	if patch.WebhookURL != nil {
		merged.WebhookURL = patch.WebhookURL
	}
	if patch.WebhookFormat != nil {
		merged.WebhookFormat = patch.WebhookFormat
	}
	if patch.ErrorRowsRatio != nil {
		merged.ErrorRowsRatio = patch.ErrorRowsRatio
	}
	if patch.ErrorRowsMinTotal != nil {
		merged.ErrorRowsMinTotal = patch.ErrorRowsMinTotal
	}
	if patch.MaxOffsetLag != nil {
		merged.MaxOffsetLag = patch.MaxOffsetLag
	}
	if patch.MaxJournalLag != nil {
		merged.MaxJournalLag = patch.MaxJournalLag
	}

	if err := s.persist(merged); err != nil {
		return err
	}
	s.override = merged
	return nil
}

// persist writes atomically (tmp + rename); an empty override removes the file
// so "reset to environment" leaves no residue behind.
func (s *Settings) persist(o Override) error {
	if o.isEmpty() {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("alert: remove override file: %w", err)
		}
		return nil
	}

	raw, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("alert: encode override: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("alert: create override directory: %w", err)
	}
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("alert: write override file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("alert: replace override file: %w", err)
	}
	return nil
}

// MaskWebhookURL hides the secret path of a webhook while keeping enough to
// recognize the destination ("https://hooks.slack.com/•••").
func MaskWebhookURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "•••"
	}
	return u.Scheme + "://" + u.Host + "/•••"
}

// mustDuration parses a validated duration string; the fallback only guards
// against a file edited by hand after validation.
func mustDuration(raw string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}
