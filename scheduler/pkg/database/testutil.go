package database

import (
	"reflect"
	"unsafe"

	rqlite "github.com/rqlite/gorqlite"
)

// NewQueryResultWithRows 构造一个包含指定行数据的 QueryResult，用于单元测试。
// rows 是按行组织的值切片，每个 []interface{} 代表一行。
// 调用 Next() 后可通过 Slice() 获取行数据。
//
// 示例：
//
//	qr := database.NewQueryResultWithRows([][]interface{}{
//	    {int64(1), "alice", true},
//	    {int64(2), "bob", false},
//	})
func NewQueryResultWithRows(rows [][]interface{}) rqlite.QueryResult {
	qr := rqlite.QueryResult{}
	qr.Err = nil
	// Slice()/Map()/Next() 等方法内部会访问 conn.ID 用于 trace 日志，
	// 即使 trace 关闭也会先求值参数 qr.conn.ID，因此必须设置非 nil 的 conn。
	setUnexportedField(&qr, "conn", &rqlite.Connection{ID: "test"})
	setUnexportedField(&qr, "values", rows)
	setUnexportedField(&qr, "rowNumber", int64(-1))
	return qr
}

// NewQueryResultWithErr 构造一个带错误的 QueryResult，用于单元测试。
func NewQueryResultWithErr(err error) rqlite.QueryResult {
	qr := rqlite.QueryResult{}
	qr.Err = err
	return qr
}

// NewWriteResult 构造一个 WriteResult，用于单元测试。
func NewWriteResult(lastInsertID, rowsAffected int64) rqlite.WriteResult {
	return rqlite.WriteResult{
		LastInsertID: lastInsertID,
		RowsAffected: rowsAffected,
	}
}

// setUnexportedField 通过反射设置结构体的私有字段，仅供测试使用。
func setUnexportedField(qr *rqlite.QueryResult, field string, value interface{}) {
	v := reflect.ValueOf(qr).Elem()
	f := v.FieldByName(field)
	if !f.IsValid() {
		return
	}
	// 使用 unsafe 绕过 unexported 限制（同一包外访问私有字段）
	ptr := unsafe.Pointer(f.UnsafeAddr())
	reflect.NewAt(f.Type(), ptr).Elem().Set(reflect.ValueOf(value))
}
