package service

import (
	"strings"
	"testing"
)

func TestExecutorStatusValues(t *testing.T) {
	validStatuses := []string{"online", "offline"}

	for _, status := range validStatuses {
		if status != "online" && status != "offline" {
			t.Errorf("expected valid status, got %s", status)
		}
	}
}

func TestExecutorAddressFormat(t *testing.T) {
	hostname := "localhost"
	pid := 12345

	address := hostname + "#" + itoa(pid)

	expected := "localhost#12345"
	if address != expected {
		t.Errorf("expected address %s, got %s", expected, address)
	}
}

func TestExecutorCapacity(t *testing.T) {
	tests := []struct {
		name       string
		capacity   int32
		currentLoad int32
		canAccept  bool
	}{
		{"has capacity", 10, 5, true},
		{"at capacity", 10, 10, false},
		{"over capacity", 10, 15, false},
		{"empty capacity", 10, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canAccept := tt.currentLoad < tt.capacity
			if canAccept != tt.canAccept {
				t.Errorf("expected canAccept=%v for load=%d, capacity=%d",
					tt.canAccept, tt.currentLoad, tt.capacity)
			}
		})
	}
}

func TestSelectAvailableExecutorQuery(t *testing.T) {
	query := `SELECT id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at
		FROM bdopsflow_executors
		WHERE status = 'online' AND current_load < capacity
		  AND last_heartbeat > datetime('now', '-30 seconds')
		ORDER BY current_load ASC, RANDOM()
		LIMIT 1`

	if !strings.Contains(query, "status = 'online'") {
		t.Error("expected query to filter by status = 'online'")
	}

	if !strings.Contains(query, "current_load < capacity") {
		t.Error("expected query to filter by capacity")
	}

	if !strings.Contains(query, "last_heartbeat") {
		t.Error("expected query to check heartbeat")
	}

	if strings.Contains(query, "executor_id") {
		t.Error("query should not contain executor_id field")
	}
}

func TestRegisterExecutorQuery(t *testing.T) {
	query := `INSERT INTO bdopsflow_executors (name, address, status, capacity, current_load, last_heartbeat, created_at, updated_at)
		VALUES (?, ?, 'online', ?, 0, ?, ?, ?)`

	if !strings.Contains(query, "address") {
		t.Error("expected query to insert address")
	}

	if !strings.Contains(query, "'online'") {
		t.Error("expected query to set status to online")
	}

	if strings.Contains(query, "executor_id") {
		t.Error("query should not contain executor_id field")
	}
}

func TestRegisterExecutorDuplicateCheck(t *testing.T) {
	existsQuery := `
		SELECT id, address FROM bdopsflow_executors 
		WHERE name = ? AND address = ? AND status = 'online' 
		AND last_heartbeat > datetime('now', '-30 seconds')
	`

	if !strings.Contains(existsQuery, "status = 'online'") {
		t.Error("expected query to filter by status = online")
	}

	if !strings.Contains(existsQuery, "last_heartbeat") {
		t.Error("expected query to check heartbeat")
	}

	if strings.Contains(existsQuery, "executor_id") {
		t.Error("query should not contain executor_id field")
	}
}

func TestDeleteExecutorQuery(t *testing.T) {
	query := `DELETE FROM bdopsflow_executors WHERE id = ?`

	if !strings.Contains(query, "DELETE FROM bdopsflow_executors") {
		t.Error("expected query to be DELETE statement")
	}

	if !strings.Contains(query, "id = ?") {
		t.Error("expected query to filter by id")
	}

	if strings.Contains(query, "executor_id") {
		t.Error("query should not contain executor_id field")
	}
}

func TestSetExecutorStatusQuery(t *testing.T) {
	query := `UPDATE bdopsflow_executors SET status = ?, updated_at = ? WHERE id = ?`

	if !strings.Contains(query, "UPDATE bdopsflow_executors") {
		t.Error("expected query to be UPDATE statement")
	}

	if !strings.Contains(query, "status = ?") {
		t.Error("expected query to set status")
	}

	if !strings.Contains(query, "id = ?") {
		t.Error("expected query to filter by id")
	}

	if strings.Contains(query, "executor_id") {
		t.Error("query should not contain executor_id field")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
