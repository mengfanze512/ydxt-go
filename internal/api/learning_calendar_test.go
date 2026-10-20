package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

func TestLearningCalendarAndCheckin(t *testing.T) {
	db := deliveryTestDB(t)
	if err := db.AutoMigrate(&model.UserCheckin{}, &model.LearningTask{}); err != nil {
		t.Fatal(err)
	}
	now, _ := time.ParseInLocation("2006-01-02", learningToday(), learningTimezone)
	mustCreate(t, db, &model.Course{ID: 1, Title: "My course"})
	mustCreate(t, db, &model.Course{ID: 2, Title: "Other course"})
	mustCreate(t, db, &model.CourseCohort{ID: 10, CourseID: 1, Name: "My cohort", StartAt: now, EndAt: now.AddDate(0, 0, 1)})
	mustCreate(t, db, &model.CourseEnrollment{CourseID: 1, CohortID: 10, UserID: 1})
	mustCreate(t, db, &model.LearningTask{ID: 11, CourseID: 1, Title: "My homework", TaskType: "homework", Status: "published", DueAt: &now})
	mustCreate(t, db, &model.LearningTask{ID: 12, CourseID: 2, Title: "Private homework", TaskType: "homework", Status: "published", DueAt: &now})
	for _, day := range []int{-1, -2} {
		mustCreate(t, db, &model.UserCheckin{UserID: 1, CheckinDate: now.AddDate(0, 0, day).Format("2006-01-02")})
	}
	router := InitRouter()
	mustCreate(t, db, &model.Teacher{ID: 10, Status: 1})
	mustCreate(t, db, &model.Admin{ID: 5, RoleCode: "admin", Status: 1})
	for i := 0; i < 2; i++ {
		response := teacherTestRequest(t, router, 1, 0, "POST", "/api/v1/learning-calendar/checkin", nil)
		// The helper uses learner 5; use learner 1 below for the streak scenario.
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
	}
	local := gin.New()
	local.Use(func(c *gin.Context) { c.Set("userID", uint64(1)); c.Next() })
	local.POST("/checkin", DoCheckin)
	local.GET("/calendar", GetLearningCalendar)
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		local.ServeHTTP(rec, httptest.NewRequest("POST", "/checkin", nil))
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
	}
	var count int64
	db.Model(&model.UserCheckin{}).Where("user_id = ? AND checkin_date = ?", 1, learningToday()).Count(&count)
	if count != 1 {
		t.Fatal("duplicate daily checkin")
	}
	rec := httptest.NewRecorder()
	local.ServeHTTP(rec, httptest.NewRequest("GET", "/calendar?month="+now.Format("2006-01"), nil))
	var body struct {
		Data struct {
			Checked    bool                    `json:"checked_today"`
			Continuous int                     `json:"continuous_days"`
			Events     []learningCalendarEvent `json:"events"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || !body.Data.Checked || body.Data.Continuous != 3 || len(body.Data.Events) < 2 {
		t.Fatal(rec.Body.String())
	}
	for _, event := range body.Data.Events {
		if event.CourseID != 1 {
			t.Fatal("other learner event leaked")
		}
	}
	rec = httptest.NewRecorder()
	local.ServeHTTP(rec, httptest.NewRequest("GET", "/calendar?month=bad", nil))
	if rec.Code != 400 {
		t.Fatal("invalid month accepted")
	}
	for _, role := range []int8{2, 9} {
		teacherID := uint64(10)
		if role == 9 {
			teacherID = 0
		}
		for _, route := range []struct{ Method, URL string }{{"GET", "/api/v1/learning-calendar"}, {"POST", "/api/v1/learning-calendar/checkin"}} {
			response := teacherTestRequest(t, router, role, teacherID, route.Method, route.URL, nil)
			if response.Code != 403 {
				t.Fatalf("staff was not rejected by learner policy: %s", response.Body.String())
			}
		}
	}
}

func TestCheckinStreakAcrossMonth(t *testing.T) {
	dates := map[string]bool{"2026-03-01": true, "2026-02-28": true, "2026-02-27": true}
	if checkinStreak(dates, "2026-03-01") != 3 || checkinStreak(dates, "2026-03-02") != 3 || checkinStreak(dates, "2026-03-03") != 0 {
		t.Fatal("streak boundary is incorrect")
	}
}

func TestAdminCheckinListAndDelete(t *testing.T) {
	db := deliveryTestDB(t)
	if err := db.AutoMigrate(&model.UserCheckin{}, &model.LearningTask{}); err != nil {
		t.Fatal(err)
	}
	mustCreate(t, db, &model.Admin{ID: 5, RoleCode: "admin", Status: 1})
	mustCreate(t, db, &model.Teacher{ID: 10, Status: 1})
	db.Model(&model.User{}).Where("id = 1").Updates(map[string]any{"nickname": "Test learner", "phone": "13800000000"})
	mustCreate(t, db, &model.UserCheckin{ID: 101, UserID: 1, CheckinDate: learningToday()})
	mustCreate(t, db, &model.UserCheckin{ID: 102, UserID: 2, CheckinDate: "2026-01-01"})
	router := InitRouter()
	response := teacherTestRequest(t, router, 9, 0, "GET", "/api/v1/admin/checkins?keyword=13800000000&page_size=1&start_date="+learningToday()+"&end_date="+learningToday(), nil)
	var body struct {
		Data struct {
			Total int `json:"total"`
			Items []struct {
				ID    uint64 `json:"id"`
				Phone string `json:"phone"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || body.Data.Total != 1 || len(body.Data.Items) != 1 || body.Data.Items[0].ID != 101 || body.Data.Items[0].Phone != "13800000000" {
		t.Fatal(response.Body.String())
	}
	for _, role := range []int8{1, 2} {
		teacherID := uint64(0)
		if role == 2 {
			teacherID = 10
		}
		for _, route := range []struct{ Method, URL string }{{"GET", "/api/v1/admin/checkins"}, {"DELETE", "/api/v1/admin/checkins/101"}} {
			res := teacherTestRequest(t, router, role, teacherID, route.Method, route.URL, nil)
			if res.Code != 403 {
				t.Fatalf("non-admin checkin access: %s", res.Body.String())
			}
		}
	}
	response = teacherTestRequest(t, router, 9, 0, "GET", "/api/v1/admin/checkins?start_date=invalid", nil)
	if response.Code != 400 {
		t.Fatal("invalid date accepted")
	}
	response = teacherTestRequest(t, router, 9, 0, "DELETE", "/api/v1/admin/checkins/101", nil)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	response = teacherTestRequest(t, router, 9, 0, "DELETE", "/api/v1/admin/checkins/101", nil)
	if response.Code != 404 {
		t.Fatal("missing record deletion must fail")
	}
	var count int64
	db.Model(&model.UserCheckin{}).Where("id = 102").Count(&count)
	if count != 1 {
		t.Fatal("delete affected another record")
	}
	c, rec := deliveryTestContext(t, 1, "/calendar", nil)
	GetLearningCalendar(c)
	var calendar struct {
		Data struct {
			Checked bool `json:"checked_today"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &calendar); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || calendar.Data.Checked {
		t.Fatal("deleted checkin remained on learner calendar")
	}
}
