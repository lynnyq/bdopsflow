package datasource

import (
	"sort"
	"testing"
)

func TestIsValidPermissionType(t *testing.T) {
	tests := []struct {
		name     string
		permType string
		expected bool
	}{
		{"read is valid", "read", true},
		{"query is valid", "query", true},
		{"download is valid", "download", true},
		{"update is valid", "update", true},
		{"delete is valid", "delete", true},
		{"manage is valid", "manage", true},
		{"write is invalid", "write", false},
		{"admin is invalid", "admin", false},
		{"empty is invalid", "", false},
		{"READ is invalid", "READ", false},
		{"Query is invalid", "Query", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidPermissionType(tt.permType)
			if got != tt.expected {
				t.Errorf("IsValidPermissionType(%q) = %v, want %v", tt.permType, got, tt.expected)
			}
		})
	}
}

func TestGetIncludedPermissions(t *testing.T) {
	tests := []struct {
		name     string
		permType string
		expected []string
	}{
		{"read includes only read", "read", []string{"read"}},
		{"query includes read and query", "query", []string{"query", "read"}},
		{"download includes read, query, download", "download", []string{"download", "query", "read"}},
		{"update includes read, query, download, update", "update", []string{"update", "download", "query", "read"}},
		{"delete includes only delete", "delete", []string{"delete"}},
		{"manage includes all", "manage", []string{"manage", "update", "download", "query", "read", "delete"}},
		{"invalid returns nil", "write", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetIncludedPermissions(tt.permType)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("GetIncludedPermissions(%q) = %v, want nil", tt.permType, got)
				}
				return
			}
			sort.Strings(got)
			sort.Strings(tt.expected)
			if len(got) != len(tt.expected) {
				t.Errorf("GetIncludedPermissions(%q) = %v, want %v", tt.permType, got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("GetIncludedPermissions(%q) = %v, want %v", tt.permType, got, tt.expected)
					return
				}
			}
		})
	}
}

func TestGetEffectivePermissions(t *testing.T) {
	tests := []struct {
		name     string
		permType string
		expected []string
	}{
		{"read effective includes all that contain read", "read", []string{"read", "query", "download", "update", "manage"}},
		{"query effective includes query and above", "query", []string{"query", "download", "update", "manage"}},
		{"download effective includes download and above", "download", []string{"download", "update", "manage"}},
		{"update effective includes update and manage", "update", []string{"update", "manage"}},
		{"delete effective includes delete and manage", "delete", []string{"delete", "manage"}},
		{"manage effective includes manage only", "manage", []string{"manage"}},
		{"invalid returns itself", "write", []string{"write"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEffectivePermissions(tt.permType)
			sort.Strings(got)
			sort.Strings(tt.expected)
			if len(got) != len(tt.expected) {
				t.Errorf("GetEffectivePermissions(%q) = %v, want %v", tt.permType, got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("GetEffectivePermissions(%q) = %v, want %v", tt.permType, got, tt.expected)
					return
				}
			}
		})
	}
}

func TestGetEffectivePermissions_QueryIncludesDownload(t *testing.T) {
	effective := GetEffectivePermissions("query")
	found := false
	for _, p := range effective {
		if p == "download" {
			found = true
		}
	}
	if !found {
		t.Error("GetEffectivePermissions('query') should include download (download includes query)")
	}
}

func TestGetEffectivePermissions_DownloadIncludesUpdate(t *testing.T) {
	effective := GetEffectivePermissions("download")
	hasUpdate := false
	hasManage := false
	for _, p := range effective {
		if p == "update" {
			hasUpdate = true
		}
		if p == "manage" {
			hasManage = true
		}
	}
	if !hasUpdate {
		t.Error("GetEffectivePermissions('download') should include update")
	}
	if !hasManage {
		t.Error("GetEffectivePermissions('download') should include manage")
	}
}

func TestPermissionHierarchyConsistency(t *testing.T) {
	allTypes := []string{"read", "query", "download", "update", "delete", "manage"}

	for _, pt := range allTypes {
		included := GetIncludedPermissions(pt)
		effective := GetEffectivePermissions(pt)

		if !IsValidPermissionType(pt) {
			t.Errorf("permission %q should be valid", pt)
		}

		includedSet := make(map[string]bool)
		for _, p := range included {
			includedSet[p] = true
		}

		for _, p := range effective {
			if !IsValidPermissionType(p) {
				t.Errorf("permission %q: effective permission %q should be valid", pt, p)
			}
		}

		selfIncluded := false
		for _, p := range included {
			if p == pt {
				selfIncluded = true
			}
		}
		if !selfIncluded {
			t.Errorf("permission %q: should include itself in GetIncludedPermissions", pt)
		}

		selfEffective := false
		for _, p := range effective {
			if p == pt {
				selfEffective = true
			}
		}
		if !selfEffective {
			t.Errorf("permission %q: should include itself in GetEffectivePermissions", pt)
		}
	}
}

func TestPermissionHierarchy_Transitivity(t *testing.T) {
	if !containsPerm(GetIncludedPermissions("download"), "read") {
		t.Error("download should include read (transitive through query)")
	}
	if !containsPerm(GetIncludedPermissions("update"), "read") {
		t.Error("update should include read (transitive through download -> query)")
	}
	if !containsPerm(GetIncludedPermissions("manage"), "read") {
		t.Error("manage should include read")
	}
	if !containsPerm(GetIncludedPermissions("manage"), "delete") {
		t.Error("manage should include delete")
	}
	if containsPerm(GetIncludedPermissions("delete"), "update") {
		t.Error("delete should NOT include update")
	}
	if containsPerm(GetIncludedPermissions("delete"), "read") {
		t.Error("delete should NOT include read")
	}
}

func containsPerm(perms []string, target string) bool {
	for _, p := range perms {
		if p == target {
			return true
		}
	}
	return false
}

func containsPerms(perms []string, targets ...string) bool {
	set := make(map[string]bool)
	for _, p := range perms {
		set[p] = true
	}
	for _, t := range targets {
		if !set[t] {
			return false
		}
	}
	return true
}

func TestGetEffectivePermissions_ReadAccess(t *testing.T) {
	effective := GetEffectivePermissions("read")
	if !containsPerms(effective, "read", "query", "download", "update", "manage") {
		t.Errorf("GetEffectivePermissions('read') should include all main chain permissions, got %v", effective)
	}
	if containsPerm(effective, "delete") {
		t.Error("GetEffectivePermissions('read') should NOT include delete (delete doesn't include read)")
	}
}
