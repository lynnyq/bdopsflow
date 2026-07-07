package datasource

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// ==================== GrantPermission ====================

func TestDatasourceService_GrantPermission_RequiresRoleOrUser(t *testing.T) {
	svc := NewDatasourceService(&dsMockDB{}, nil, nil)

	perm := &model.DatasourcePermission{
		DatasourceID:   1,
		PermissionType: "query",
		// RoleID 和 UserID 都为 nil
	}

	err := svc.GrantPermission(context.Background(), perm)
	if err == nil {
		t.Fatal("expected error when neither role_id nor user_id is set")
	}
	if !strings.Contains(err.Error(), "role_id or user_id is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDatasourceService_GrantPermission_InvalidType(t *testing.T) {
	svc := NewDatasourceService(&dsMockDB{}, nil, nil)

	userID := int64(1)
	perm := &model.DatasourcePermission{
		DatasourceID:   1,
		UserID:         &userID,
		PermissionType: "invalid",
	}

	err := svc.GrantPermission(context.Background(), perm)
	if !errors.Is(err, ErrInvalidPermissionType) {
		t.Errorf("expected ErrInvalidPermissionType, got %v", err)
	}
}

func TestDatasourceService_GrantPermission_Success_UserID(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(10, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	userID := int64(5)
	grantedBy := int64(100)
	perm := &model.DatasourcePermission{
		DatasourceID:   1,
		UserID:         &userID,
		PermissionType: "query",
		GrantedBy:      &grantedBy,
	}

	err := svc.GrantPermission(context.Background(), perm)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if perm.ID != 10 {
		t.Errorf("expected ID=10, got %d", perm.ID)
	}
}

func TestDatasourceService_GrantPermission_Success_RoleID(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(20, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	roleID := int64(3)
	perm := &model.DatasourcePermission{
		DatasourceID:   1,
		RoleID:         &roleID,
		PermissionType: "read",
	}

	err := svc.GrantPermission(context.Background(), perm)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if perm.ID != 20 {
		t.Errorf("expected ID=20, got %d", perm.ID)
	}
}

func TestDatasourceService_GrantPermission_WithIncludedPerms(t *testing.T) {
	// "manage" 包含所有权限，所以会先删除低级权限
	db := &dsMockDB{
		writeResults: []rqlite.WriteResult{
			database.NewWriteResult(0, 0), // delete 旧权限
			database.NewWriteResult(30, 1), // insert 新权限
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	userID := int64(1)
	perm := &model.DatasourcePermission{
		DatasourceID:   1,
		UserID:         &userID,
		PermissionType: "manage",
	}

	err := svc.GrantPermission(context.Background(), perm)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if perm.ID != 30 {
		t.Errorf("expected ID=30, got %d", perm.ID)
	}
	// 应该有 2 次写入：删除 + 插入
	if len(db.writeStmts) != 2 {
		t.Errorf("expected 2 writes, got %d", len(db.writeStmts))
	}
}

func TestDatasourceService_GrantPermission_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("insert failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	userID := int64(1)
	perm := &model.DatasourcePermission{
		DatasourceID:   1,
		UserID:         &userID,
		PermissionType: "read",
	}

	err := svc.GrantPermission(context.Background(), perm)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to grant permission") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== UpdatePermission ====================

func TestDatasourceService_UpdatePermission_InvalidType(t *testing.T) {
	svc := NewDatasourceService(&dsMockDB{}, nil, nil)

	err := svc.UpdatePermission(context.Background(), 1, "invalid")
	if !errors.Is(err, ErrInvalidPermissionType) {
		t.Errorf("expected ErrInvalidPermissionType, got %v", err)
	}
}

func TestDatasourceService_UpdatePermission_NotFound(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{{int64(0)}}),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.UpdatePermission(context.Background(), 999, "query")
	if !errors.Is(err, ErrPermissionNotFound) {
		t.Errorf("expected ErrPermissionNotFound, got %v", err)
	}
}

func TestDatasourceService_UpdatePermission_Success(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{{int64(1)}}),
		writeResult: database.NewWriteResult(0, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.UpdatePermission(context.Background(), 1, "manage")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_UpdatePermission_QueryError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("query failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.UpdatePermission(context.Background(), 1, "query")
	if err == nil {
		t.Fatal("expected error on query failure")
	}
}

func TestDatasourceService_UpdatePermission_WriteError(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{{int64(1)}}),
		writeError:  errors.New("update failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.UpdatePermission(context.Background(), 1, "query")
	if err == nil {
		t.Fatal("expected error on write failure")
	}
}

// ==================== RevokePermission ====================

func TestDatasourceService_RevokePermission_Success(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(0, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.RevokePermission(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_RevokePermission_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("delete failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.RevokePermission(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to revoke permission") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== GetPermissions ====================

func TestDatasourceService_GetPermissions_Success(t *testing.T) {
	roleID := int64(2)
	userID := int64(5)
	grantedBy := int64(100)

	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{
			{int64(1), int64(10), roleID, nil, "query", grantedBy, "2025-01-01T00:00:00Z"},
			{int64(2), int64(10), nil, userID, "read", nil, "2025-01-02T00:00:00Z"},
		}),
	}
	svc := NewDatasourceService(db, nil, nil)

	perms, err := svc.GetPermissions(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(perms))
	}

	if perms[0].ID != 1 {
		t.Errorf("expected first perm ID=1, got %d", perms[0].ID)
	}
	if perms[0].PermissionType != "query" {
		t.Errorf("expected first perm type 'query', got %q", perms[0].PermissionType)
	}
	if perms[0].RoleID == nil || *perms[0].RoleID != 2 {
		t.Errorf("expected first perm RoleID=2, got %v", perms[0].RoleID)
	}
	if perms[0].UserID != nil {
		t.Errorf("expected first perm UserID nil, got %v", perms[0].UserID)
	}

	if perms[1].ID != 2 {
		t.Errorf("expected second perm ID=2, got %d", perms[1].ID)
	}
	if perms[1].UserID == nil || *perms[1].UserID != 5 {
		t.Errorf("expected second perm UserID=5, got %v", perms[1].UserID)
	}
	if perms[1].RoleID != nil {
		t.Errorf("expected second perm RoleID nil, got %v", perms[1].RoleID)
	}
}

func TestDatasourceService_GetPermissions_Empty(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{}),
	}
	svc := NewDatasourceService(db, nil, nil)

	perms, err := svc.GetPermissions(context.Background(), 999)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(perms) != 0 {
		t.Errorf("expected 0 permissions, got %d", len(perms))
	}
}

func TestDatasourceService_GetPermissions_QueryError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("query failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, err := svc.GetPermissions(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on query failure")
	}
	if !strings.Contains(err.Error(), "failed to get permissions") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== CheckPermission ====================

func TestDatasourceService_CheckPermission_UserPermFound(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{{int64(1)}}),
	}
	svc := NewDatasourceService(db, nil, nil)

	has, err := svc.CheckPermission(context.Background(), 1, 10, "query")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !has {
		t.Error("expected has=true")
	}
}

func TestDatasourceService_CheckPermission_RolePermFound(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}), // user perm not found
			database.NewQueryResultWithRows([][]interface{}{{int64(1)}}), // role perm found
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	has, err := svc.CheckPermission(context.Background(), 1, 10, "query")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !has {
		t.Error("expected has=true from role permission")
	}
}

func TestDatasourceService_CheckPermission_NotFound(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}), // user perm not found
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}), // role perm not found
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	has, err := svc.CheckPermission(context.Background(), 1, 10, "query")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if has {
		t.Error("expected has=false")
	}
}

func TestDatasourceService_CheckPermission_QueryError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("query failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	has, err := svc.CheckPermission(context.Background(), 1, 10, "query")
	if err == nil {
		t.Fatal("expected error on query failure")
	}
	if has {
		t.Error("expected has=false on error")
	}
}
