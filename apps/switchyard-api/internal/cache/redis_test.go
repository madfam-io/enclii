package cache

import "testing"

func Test_parseRedisInfoInt(t *testing.T) {
	sampleInfo := `# Stats
total_connections_received:1234
total_commands_processed:56789
keyspace_hits:12345
keyspace_misses:678
rejected_connections:0
expired_keys:100
`

	tests := []struct {
		name string
		key  string
		want int64
	}{
		{"keyspace_hits", "keyspace_hits", 12345},
		{"keyspace_misses", "keyspace_misses", 678},
		{"total_commands_processed", "total_commands_processed", 56789},
		{"non_existent_key", "non_existent_key", 0},
		{"empty_key", "", 0},
		{"partial_match", "keyspace", 0}, // Should not match partial
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRedisInfoInt(sampleInfo, tt.key)
			if got != tt.want {
				t.Errorf("parseRedisInfoInt(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

func TestCacheMetrics(t *testing.T) {
	// Test that CacheMetrics struct is properly initialized
	metrics := &CacheMetrics{
		Hits:   100,
		Misses: 10,
		Errors: 5,
	}

	if metrics.Hits != 100 {
		t.Errorf("Expected Hits=100, got %d", metrics.Hits)
	}
	if metrics.Misses != 10 {
		t.Errorf("Expected Misses=10, got %d", metrics.Misses)
	}
	if metrics.Errors != 5 {
		t.Errorf("Expected Errors=5, got %d", metrics.Errors)
	}
}
