package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsPath = "/metrics"

type controllerMetricsCollector struct {
	db        *store.DB
	startedAt time.Time

	controllerInfoDesc       *prometheus.Desc
	controllerUptimeDesc     *prometheus.Desc
	configuredNodesDesc      *prometheus.Desc
	nodeOnlineDesc           *prometheus.Desc
	serviceStatusDesc        *prometheus.Desc
	taskStatusDesc           *prometheus.Desc
	backupStatusDesc         *prometheus.Desc
	imageUpdateAvailableDesc *prometheus.Desc
}

func newControllerMetricsCollector(db *store.DB, startedAt time.Time) *controllerMetricsCollector {
	return &controllerMetricsCollector{
		db:                       db,
		startedAt:                startedAt,
		controllerInfoDesc:       prometheus.NewDesc("composia_controller_info", "Static composia controller build information.", []string{"version"}, nil),
		controllerUptimeDesc:     prometheus.NewDesc("composia_controller_uptime_seconds", "How long the current controller process has been running.", nil, nil),
		configuredNodesDesc:      prometheus.NewDesc("composia_nodes", "Number of configured nodes.", nil, nil),
		nodeOnlineDesc:           prometheus.NewDesc("composia_node_online", "Whether a configured node is online.", []string{"node_id"}, nil),
		serviceStatusDesc:        prometheus.NewDesc("composia_services", "Number of declared services by runtime status.", []string{"status"}, nil),
		taskStatusDesc:           prometheus.NewDesc("composia_tasks", "Number of tasks by type and status.", []string{"type", "status"}, nil),
		backupStatusDesc:         prometheus.NewDesc("composia_backups", "Number of backups by status.", []string{"status"}, nil),
		imageUpdateAvailableDesc: prometheus.NewDesc("composia_image_update_available", "Number of detected image updates by service and node.", []string{"service_name", "node_id"}, nil),
	}
}

func (collector *controllerMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.controllerInfoDesc
	ch <- collector.controllerUptimeDesc
	ch <- collector.configuredNodesDesc
	ch <- collector.nodeOnlineDesc
	ch <- collector.serviceStatusDesc
	ch <- collector.taskStatusDesc
	ch <- collector.backupStatusDesc
	ch <- collector.imageUpdateAvailableDesc
}

func (collector *controllerMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(collector.controllerInfoDesc, prometheus.GaugeValue, 1, version.Value)
	ch <- prometheus.MustNewConstMetric(collector.controllerUptimeDesc, prometheus.GaugeValue, time.Since(collector.startedAt).Seconds())

	if collector.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	collector.collectNodes(ctx, ch)
	collector.collectServices(ctx, ch)
	collector.collectTasks(ctx, ch)
	collector.collectBackups(ctx, ch)
	collector.collectImageUpdates(ctx, ch)
}

func (collector *controllerMetricsCollector) collectNodes(ctx context.Context, ch chan<- prometheus.Metric) {
	snapshots, err := collector.db.ListNodeSnapshots(ctx)
	if err != nil {
		log.Printf("metrics: list node snapshots failed: %v", err)
		return
	}
	configured := 0
	for _, snapshot := range snapshots {
		if !snapshot.IsConfigured {
			continue
		}
		configured++
		value := 0.0
		if snapshot.IsOnline {
			value = 1
		}
		ch <- prometheus.MustNewConstMetric(collector.nodeOnlineDesc, prometheus.GaugeValue, value, snapshot.NodeID)
	}
	ch <- prometheus.MustNewConstMetric(collector.configuredNodesDesc, prometheus.GaugeValue, float64(configured))
}

func (collector *controllerMetricsCollector) collectServices(ctx context.Context, ch chan<- prometheus.Metric) {
	rows, err := collector.db.SQL().QueryContext(ctx, `
		SELECT runtime_status, COUNT(*)
		FROM services
		WHERE is_declared = 1
		GROUP BY runtime_status
	`)
	if err != nil {
		log.Printf("metrics: query service counts failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			log.Printf("metrics: scan service count failed: %v", err)
			return
		}
		ch <- prometheus.MustNewConstMetric(collector.serviceStatusDesc, prometheus.GaugeValue, float64(count), status)
	}
	if err := rows.Err(); err != nil {
		log.Printf("metrics: iterate service counts failed: %v", err)
	}
}

func (collector *controllerMetricsCollector) collectTasks(ctx context.Context, ch chan<- prometheus.Metric) {
	rows, err := collector.db.SQL().QueryContext(ctx, `
		SELECT type, status, COUNT(*)
		FROM tasks
		GROUP BY type, status
	`)
	if err != nil {
		log.Printf("metrics: query task counts failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var taskType, status string
		var count int64
		if err := rows.Scan(&taskType, &status, &count); err != nil {
			log.Printf("metrics: scan task count failed: %v", err)
			return
		}
		ch <- prometheus.MustNewConstMetric(collector.taskStatusDesc, prometheus.GaugeValue, float64(count), taskType, status)
	}
	if err := rows.Err(); err != nil {
		log.Printf("metrics: iterate task counts failed: %v", err)
	}
}

func (collector *controllerMetricsCollector) collectBackups(ctx context.Context, ch chan<- prometheus.Metric) {
	rows, err := collector.db.SQL().QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM backups
		GROUP BY status
	`)
	if err != nil {
		log.Printf("metrics: query backup counts failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			log.Printf("metrics: scan backup count failed: %v", err)
			return
		}
		ch <- prometheus.MustNewConstMetric(collector.backupStatusDesc, prometheus.GaugeValue, float64(count), status)
	}
	if err := rows.Err(); err != nil {
		log.Printf("metrics: iterate backup counts failed: %v", err)
	}
}

func (collector *controllerMetricsCollector) collectImageUpdates(ctx context.Context, ch chan<- prometheus.Metric) {
	rows, err := collector.db.SQL().QueryContext(ctx, `
		SELECT service_name, node_id, COUNT(*)
		FROM service_image_update_checks
		WHERE update_available = 1 AND check_status != ?
		GROUP BY service_name, node_id
	`, store.ImageCheckStatusError)
	if err != nil {
		log.Printf("metrics: query image update counts failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var serviceName, nodeID string
		var count int64
		if err := rows.Scan(&serviceName, &nodeID, &count); err != nil {
			log.Printf("metrics: scan image update count failed: %v", err)
			return
		}
		ch <- prometheus.MustNewConstMetric(collector.imageUpdateAvailableDesc, prometheus.GaugeValue, float64(count), serviceName, nodeID)
	}
	if err := rows.Err(); err != nil {
		log.Printf("metrics: iterate image update counts failed: %v", err)
	}
}

func registerMetricsHandler(mux *http.ServeMux, db *store.DB, accessTokens map[string]string, startedAt time.Time) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(newControllerMetricsCollector(db, startedAt))
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	mux.Handle(metricsPath, requireAccessTokenAuth(accessTokens, handler))
}

func requireAccessTokenAuth(tokens map[string]string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token, err := bearerToken(req.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, ok := tokens[token]; !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func bearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", errors.New("missing authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("invalid authorization header")
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	return token, nil
}
