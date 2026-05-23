package datasource

var permissionIncludes = map[string][]string{
	"read":     {},
	"query":    {"read"},
	"download": {"query", "read"},
	"update":   {"download", "query", "read"},
	"delete":   {},
	"manage":   {"update", "download", "query", "read", "delete"},
}

var validPermissionTypes = map[string]bool{
	"read":     true,
	"query":    true,
	"download": true,
	"update":   true,
	"delete":   true,
	"manage":   true,
}

func IsValidPermissionType(permType string) bool {
	return validPermissionTypes[permType]
}

func GetIncludedPermissions(permType string) []string {
	included, ok := permissionIncludes[permType]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(included)+1)
	result = append(result, permType)
	result = append(result, included...)
	return result
}

func GetEffectivePermissions(permType string) []string {
	if !validPermissionTypes[permType] {
		return []string{permType}
	}
	var result []string
	for pt, includes := range permissionIncludes {
		if pt == permType {
			result = append(result, pt)
			continue
		}
		for _, inc := range includes {
			if inc == permType {
				result = append(result, pt)
				break
			}
		}
	}
	return result
}
