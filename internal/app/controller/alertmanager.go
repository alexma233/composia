package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	appnotify "forgejo.alexma.top/alexma233/composia/internal/app/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
)

type notificationDispatcher interface {
	Dispatch(appnotify.Event)
}

type alertmanagerWebhookPayload struct {
	Receiver          string              `json:"receiver"`
	Status            string              `json:"status"`
	GroupLabels       map[string]string   `json:"groupLabels"`
	CommonLabels      map[string]string   `json:"commonLabels"`
	CommonAnnotations map[string]string   `json:"commonAnnotations"`
	ExternalURL       string              `json:"externalURL"`
	Alerts            []alertmanagerAlert `json:"alerts"`
}

type alertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

type alertmanagerHandler struct {
	notifier notificationDispatcher
}

func registerAlertmanagerHandler(mux *http.ServeMux, cfg *config.ControllerNotificationsConfig, notifier notificationDispatcher) {
	if mux == nil || cfg == nil || cfg.Alertmanager == nil || !cfg.Alertmanager.IsEnabled() {
		return
	}
	mux.Handle(cfg.Alertmanager.EffectiveListenPath(), &alertmanagerHandler{notifier: notifier})
}

func (handler *alertmanagerHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer func() { _ = req.Body.Close() }()
	var payload alertmanagerWebhookPayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "invalid alertmanager payload", http.StatusBadRequest)
		return
	}
	if len(payload.Alerts) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	for _, alert := range payload.Alerts {
		handler.dispatchAlert(payload, alert)
	}
	w.WriteHeader(http.StatusAccepted)
}

func (handler *alertmanagerHandler) dispatchAlert(payload alertmanagerWebhookPayload, alert alertmanagerAlert) {
	if handler == nil || handler.notifier == nil {
		return
	}
	labels := mergeStringMaps(payload.CommonLabels, alert.Labels)
	annotations := mergeStringMaps(payload.CommonAnnotations, alert.Annotations)
	startsAt := parseAlertmanagerTime(alert.StartsAt)
	endsAt := parseAlertmanagerTime(alert.EndsAt)
	handler.notifier.Dispatch(appnotify.Event{
		Type:       corenotify.EventAlertmanagerAlert,
		OccurredAt: derefTaskTime(startsAt),
		Alertmanager: &appnotify.AlertmanagerEvent{
			Receiver:          payload.Receiver,
			Status:            strings.TrimSpace(alert.Status),
			GroupStatus:       strings.TrimSpace(payload.Status),
			AlertName:         strings.TrimSpace(labels["alertname"]),
			Severity:          strings.TrimSpace(labels["severity"]),
			Instance:          strings.TrimSpace(labels["instance"]),
			Summary:           strings.TrimSpace(annotations["summary"]),
			Description:       strings.TrimSpace(annotations["description"]),
			ExternalURL:       strings.TrimSpace(payload.ExternalURL),
			GeneratorURL:      strings.TrimSpace(alert.GeneratorURL),
			Fingerprint:       strings.TrimSpace(alert.Fingerprint),
			StartsAt:          startsAt,
			EndsAt:            endsAt,
			Labels:            labels,
			Annotations:       annotations,
			GroupLabels:       cloneStringMap(payload.GroupLabels),
			CommonLabels:      cloneStringMap(payload.CommonLabels),
			CommonAnnotations: cloneStringMap(payload.CommonAnnotations),
		},
	})
}

func mergeStringMaps(left, right map[string]string) map[string]string {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	merged := cloneStringMap(left)
	if merged == nil {
		merged = make(map[string]string, len(right))
	}
	for key, value := range right {
		merged[key] = value
	}
	return merged
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func parseAlertmanagerTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}
