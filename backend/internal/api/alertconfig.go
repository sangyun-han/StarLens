package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/internal/alert"
)

// alertSettings is the slice of alert.Settings this controller needs.
type alertSettings interface {
	Effective() alert.Config
	Overridden() []string
	Update(patch alert.Override, reset bool) error
}

// ApplyFunc pushes an effective configuration into the running alerting
// components (manager cooldown, webhook notifier, rule thresholds).
type ApplyFunc func(alert.Config) error

// AlertConfigHandler exposes runtime alert configuration.
//
// The UI never sees the full webhook URL: it is a secret, so reads return a
// masked hint and the frontend treats the field as write-only.
type AlertConfigHandler struct {
	settings alertSettings
	apply    ApplyFunc
	// editable mirrors ALERT_CONFIG_UI. When false the config stays readable
	// but every write is refused — recommended until StarLens has
	// authentication, since anyone who can reach the dashboard could otherwise
	// redirect alerts.
	editable bool
}

// NewAlertConfigHandler wires the controller to the settings store.
func NewAlertConfigHandler(settings alertSettings, apply ApplyFunc, editable bool) *AlertConfigHandler {
	return &AlertConfigHandler{settings: settings, apply: apply, editable: editable}
}

// register mounts alert configuration routes on the /api/v1 group.
func (h *AlertConfigHandler) register(v1 *gin.RouterGroup) {
	v1.GET("/alerts/config", h.Get)
	v1.PUT("/alerts/config", h.Put)
}

// alertConfigView is the read shape shared by GET and a successful PUT.
type alertConfigView struct {
	Editable   bool     `json:"editable"`
	Overridden []string `json:"overridden"`
	Config     struct {
		Enabled      bool   `json:"enabled"`
		PollInterval string `json:"pollInterval"`
		Cooldown     string `json:"cooldown"`
		// WebhookConfigured plus a masked hint replace the secret URL.
		WebhookConfigured bool    `json:"webhookConfigured"`
		WebhookHint       string  `json:"webhookHint,omitempty"`
		WebhookFormat     string  `json:"webhookFormat"`
		ErrorRowsRatio    float64 `json:"errorRowsRatio"`
		ErrorRowsMinTotal int64   `json:"errorRowsMinTotal"`
		MaxOffsetLag      int64   `json:"maxOffsetLag"`
		MaxJournalLag     int64   `json:"maxJournalLag"`
	} `json:"config"`
}

func (h *AlertConfigHandler) view() alertConfigView {
	effective := h.settings.Effective()

	view := alertConfigView{Editable: h.editable, Overridden: h.settings.Overridden()}
	if view.Overridden == nil {
		view.Overridden = []string{}
	}
	view.Config.Enabled = effective.Enabled
	view.Config.PollInterval = effective.PollInterval.String()
	view.Config.Cooldown = effective.Cooldown.String()
	view.Config.WebhookConfigured = effective.WebhookURL != ""
	view.Config.WebhookHint = alert.MaskWebhookURL(effective.WebhookURL)
	view.Config.WebhookFormat = effective.WebhookFormat
	view.Config.ErrorRowsRatio = effective.ErrorRowsRatio
	view.Config.ErrorRowsMinTotal = effective.ErrorRowsMinTotal
	view.Config.MaxOffsetLag = effective.MaxOffsetLag
	view.Config.MaxJournalLag = effective.MaxJournalLag
	return view
}

// Get handles GET /api/v1/alerts/config.
func (h *AlertConfigHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, h.view())
}

// alertConfigPatch is the write shape: every field optional, nil = unchanged.
// reset=true first clears every UI override (reverting to the environment)
// before the provided fields are applied.
type alertConfigPatch struct {
	Reset bool `json:"reset"`
	alert.Override
}

// Put handles PUT /api/v1/alerts/config: validate, persist, apply — the new
// configuration is live on the next evaluation tick, no restart involved.
func (h *AlertConfigHandler) Put(c *gin.Context) {
	if !h.editable {
		respondError(c, http.StatusForbidden, "config_ui_disabled",
			"Alert configuration via the API is disabled (ALERT_CONFIG_UI=false); edit the environment instead.", nil)
		return
	}

	var patch alertConfigPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request",
			"Request body must be a JSON alert configuration patch.", err)
		return
	}

	if err := h.settings.Update(patch.Override, patch.Reset); err != nil {
		if errors.Is(err, alert.ErrInvalidOverride) {
			respondError(c, http.StatusBadRequest, "invalid_config", "The configuration is invalid.", err)
			return
		}
		respondError(c, http.StatusInternalServerError, "config_persist_failed",
			"Could not persist the configuration.", err)
		return
	}

	if err := h.apply(h.settings.Effective()); err != nil {
		respondError(c, http.StatusInternalServerError, "config_apply_failed",
			"The configuration was saved but could not be applied.", err)
		return
	}

	c.JSON(http.StatusOK, h.view())
}
