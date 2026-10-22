package api

import (
	"strings"
	"testing"
	"ydxt-go/internal/model"
)

func TestMyProfileUpdatesIdentityScope(t *testing.T) {
	router := teacherTestRouter(t)
	mustCreate(t, model.DB, &model.Admin{ID: 5, RoleCode: "admin", Status: 1})
	for _, test := range []struct {
		role      int8
		teacherID uint64
		table     string
		column    string
		id        uint64
	}{
		{1, 0, "users", "nickname", 5},
		{2, 10, "teachers", "name", 10},
		{9, 0, "admins", "name", 5},
	} {
		res := teacherTestRequest(t, router, test.role, test.teacherID, "PATCH", "/api/v1/auth/profile", map[string]any{"nickname": "New name", "role": 9, "phone": "changed", "id": 1})
		if res.Code != 200 || !strings.Contains(res.Body.String(), `"nickname":"New name"`) {
			t.Fatal(res.Body.String())
		}
		var name string
		model.DB.Table(test.table).Where("id = ?", test.id).Select(test.column).Scan(&name)
		if name != "New name" {
			t.Fatal("profile not persisted")
		}
	}
	var other model.User
	model.DB.First(&other, 1)
	if other.Nickname != "Student" {
		t.Fatal("another learner changed")
	}
	for _, payload := range []map[string]any{
		{"nickname": " "}, {"nickname": strings.Repeat("a", 33)}, {"avatar": "http://example.com/a.png"}, {"role": 9},
	} {
		res := teacherTestRequest(t, router, 1, 0, "PATCH", "/api/v1/auth/profile", payload)
		if res.Code != 400 {
			t.Fatal(res.Body.String())
		}
	}
	res := teacherTestRequest(t, router, 1, 0, "PATCH", "/api/v1/auth/profile", map[string]any{"avatar": "cloud://test/avatars/test.png"})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var learner model.User
	model.DB.First(&learner, 5)
	if learner.Avatar != "cloud://test/avatars/test.png" || learner.Role != 1 || learner.Phone == "changed" {
		t.Fatal("avatar update changed protected identity fields")
	}
	res = teacherTestRequest(t, router, 9, 0, "PATCH", "/api/v1/auth/profile", map[string]any{"avatar": "cloud://test/avatars/test.png"})
	if res.Code != 400 {
		t.Fatal("admin avatar requires schema support")
	}
}
