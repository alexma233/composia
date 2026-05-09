package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appnotify "forgejo.alexma.top/alexma233/composia/internal/app/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
)

type recordingNotifier struct {
	events []appnotify.Event
}

func (notifier *recordingNotifier) Dispatch(event appnotify.Event) {
	notifier.events = append(notifier.events, event)
}

func TestAlertmanagerHandlerDispatchesAlertEvents(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	mux := http.NewServeMux()
	registerAlertmanagerHandler(mux, &config.ControllerNotificationsConfig{Alertmanager: &config.ControllerAlertmanagerNotificationConfig{ListenPath: "/hooks/alerts"}}, notifier)
	server := httptest.NewServer(mux)
	defer server.Close()
	payload := alertmanagerWebhookPayload{
		Receiver:    "ops",
		Status:      "firing",
		ExternalURL: "https://alertmanager.example.com",
		CommonLabels: map[string]string{
			"severity": "critical",
		},
		CommonAnnotations: map[string]string{
			"summary": "Service is down",
		},
		Alerts: []alertmanagerAlert{{
			Status: "firing",
			Labels: map[string]string{
				"alertname": "TargetDown",
				"instance":  "node-1:9090",
			},
			Annotations: map[string]string{
				"description": "The target is unreachable",
			},
			StartsAt:     "2026-05-09T12:00:00Z",
			GeneratorURL: "https://prom.example.com/graph",
			Fingerprint:  "fp-1",
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp, err := http.Post(server.URL+"/hooks/alerts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post alertmanager payload: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 accepted, got %d", resp.StatusCode)
	}
	if len(notifier.events) != 1 {
		t.Fatalf("expected one dispatched event, got %d", len(notifier.events))
	}
	event := notifier.events[0]
	if event.Type != corenotify.EventAlertmanagerAlert {
		t.Fatalf("expected alertmanager event type, got %s", event.Type)
	}
	if event.Alertmanager == nil || event.Alertmanager.AlertName != "TargetDown" {
		t.Fatalf("expected alert payload with alertname, got %+v", event.Alertmanager)
	}
	if event.Alertmanager.Severity != "critical" {
		t.Fatalf("expected merged common severity, got %+v", event.Alertmanager)
	}
	if event.Alertmanager.Description != "The target is unreachable" {
		t.Fatalf("expected description annotation, got %+v", event.Alertmanager)
	}
}

func TestAlertmanagerHandlerRejectsInvalidMethod(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	handler := &alertmanagerHandler{notifier: notifier}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}
}
