package store

import "context"

func syncDeclaredServicesForTests(ctx context.Context, db *DB, serviceNames ...string) error {
	declared := make(map[string][]string, len(serviceNames))
	for _, serviceName := range serviceNames {
		declared[serviceName] = []string{"main"}
	}
	return db.SyncDeclaredServices(ctx, declared)
}
