package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
	"ydxt-go/internal/config"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func deliveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldConfig := model.DB, config.GlobalConfig
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	// Test-only schema, never an application startup migration.
	if err := db.AutoMigrate(&model.Course{}, &model.CourseChapter{}, &model.CourseLesson{},
		&model.Order{}, &model.OrderItem{}, &model.CourseCohort{}, &model.CourseEnrollment{},
		&model.CourseDeliverySetting{}, &model.CourseAttendance{}, &model.UserLessonProgress{}); err != nil {
		t.Fatal(err)
	}
	model.DB = db
	config.GlobalConfig = &config.Config{JWT: config.JWTConfig{Secret: "test-only", ExpireHours: 1}}
	t.Cleanup(func() { sqlDB, _ := db.DB(); _ = sqlDB.Close(); model.DB, config.GlobalConfig = oldDB, oldConfig })
	return db
}

func deliveryTestContext(t *testing.T, user uint64, path string, payload any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	body, _ := json.Marshal(payload)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if user != 0 {
		token, err := utils.GenerateToken(user, 1, "user", "student", 0)
		if err != nil {
			t.Fatal(err)
		}
		c.Request.Header.Set("X-Token", token)
		c.Set("userID", user)
	}
	return c, rec
}

func mustCreate(t *testing.T, db *gorm.DB, row any) {
	t.Helper()
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryLegacyAccessAndRenewal(t *testing.T) {
	db := deliveryTestDB(t)
	mustCreate(t, db, &model.Course{ID: 1, Level: "system", Status: 1})
	mustCreate(t, db, &model.CourseChapter{ID: 1, CourseID: 1})
	lesson := model.CourseLesson{ID: 1, ChapterID: 1, Type: 1}
	mustCreate(t, db, &lesson)
	if err := db.Exec("CREATE TABLE user_courses (user_id INTEGER, course_id INTEGER, expired_at DATETIME)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO user_courses (user_id, course_id) VALUES (1,1)").Error; err != nil {
		t.Fatal(err)
	}
	c, _ := deliveryTestContext(t, 1, "/content", nil)
	if err := courseAccess(c, lesson); err != nil {
		t.Fatalf("legacy authorization: %v", err)
	}
	expired := time.Now().Add(-time.Hour)
	db.Table("user_courses").Where("user_id = 1").Update("expired_at", expired)
	if err := courseAccess(c, lesson); err == nil {
		t.Fatal("expired legacy authorization granted")
	}

	mustCreate(t, db, &model.Order{ID: 1, OrderNo: "refunded", UserID: 1, Status: "refunded"})
	mustCreate(t, db, &model.CourseEnrollment{CourseID: 1, UserID: 1, OrderID: 1})
	mustCreate(t, db, &model.Order{ID: 2, OrderNo: "renew", UserID: 1, Status: "unpaid"})
	mustCreate(t, db, &model.OrderItem{OrderID: 2, GoodsID: 1, ItemType: "course", Quantity: 1})
	c, rec := deliveryTestContext(t, 1, "/pay", nil)
	c.Params = gin.Params{{Key: "id", Value: "2"}}
	PayOrder(c)
	if rec.Code != 200 {
		t.Fatalf("renewal: %s", rec.Body.String())
	}
	if err := courseAccess(c, lesson); err != nil {
		t.Fatalf("renewed authorization: %v", err)
	}
	removed := false
	err := deleteDeliveryContent(1, func(tx *gorm.DB) error { removed = true; return nil })
	if err == nil || removed {
		t.Fatal("sold course deletion was not blocked")
	}
	c, rec = deliveryTestContext(t, 1, "/settings", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	GetDeliverySettings(c)
	var response struct {
		Data struct {
			HasAccess bool `json:"has_access"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Data.HasAccess {
		t.Fatal("settings omitted effective entitlement")
	}
}

func TestDeliveryProgressIsScopedToCohort(t *testing.T) {
	db := deliveryTestDB(t)
	mustCreate(t, db, &model.Course{ID: 1, Level: "class", Status: 1})
	mustCreate(t, db, &model.CourseCohort{ID: 1, CourseID: 1, Name: "First"})
	mustCreate(t, db, &model.CourseChapter{ID: 1, CourseID: 1})
	mustCreate(t, db, &model.CourseLesson{ID: 1, ChapterID: 1, CohortID: 1})
	mustCreate(t, db, &model.CourseLesson{ID: 2, ChapterID: 1, CohortID: 2})
	mustCreate(t, db, &model.UserLessonProgress{UserID: 1, CourseID: 1, LessonID: 1, IsCompleted: 1})
	c, rec := deliveryTestContext(t, 1, "/progress?cohort_id=1", nil)
	c.Params = gin.Params{{Key: "course_id", Value: "1"}}
	GetMyCourseProgress(c)
	var response struct {
		Data model.UserCourseProgress `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || response.Data.TotalLessons != 1 || response.Data.ProgressPercent != 100 {
		t.Fatalf("cohort progress included other lessons: %s", rec.Body.String())
	}
}

func TestDeliveryPaymentCapacityAndIdempotency(t *testing.T) {
	db := deliveryTestDB(t)
	mustCreate(t, db, &model.Course{ID: 1, Title: "Class", Level: "class", Status: 1})
	mustCreate(t, db, &model.CourseCohort{ID: 1, CourseID: 1, Name: "First", Capacity: 1, Price: 100, Status: "published", EnrollmentDeadline: time.Now().Add(time.Hour)})
	for id := uint64(1); id <= 2; id++ {
		mustCreate(t, db, &model.Order{ID: id, OrderNo: fmt.Sprint("test", id), UserID: id, Status: "unpaid", PayAmount: 100})
		mustCreate(t, db, &model.OrderItem{OrderID: id, GoodsID: 1, ItemType: "course", CohortID: 1, Quantity: 1})
	}
	pay := func(user uint64) *httptest.ResponseRecorder {
		c, rec := deliveryTestContext(t, user, "/pay", nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(user)}}
		PayOrder(c)
		return rec
	}
	if rec := pay(1); rec.Code != 200 {
		t.Fatalf("first payment: %s", rec.Body.String())
	}
	if rec := pay(1); rec.Code != 200 {
		t.Fatalf("repeat payment: %s", rec.Body.String())
	}
	if rec := pay(2); rec.Code != 400 {
		t.Fatalf("capacity bypassed: %s", rec.Body.String())
	}
	var count int64
	db.Model(&model.CourseEnrollment{}).Count(&count)
	if count != 1 {
		t.Fatalf("enrollment count = %d", count)
	}
	var order model.Order
	db.First(&order, 2)
	if order.Status != "unpaid" {
		t.Fatal("failed payment was not rolled back")
	}
}

func TestDeliveryPaidAccessExpiryAndSequence(t *testing.T) {
	db := deliveryTestDB(t)
	mustCreate(t, db, &model.Course{ID: 1, Level: "system", Status: 1})
	mustCreate(t, db, &model.CourseChapter{ID: 1, CourseID: 1, SortOrder: 1})
	lessons := []model.CourseLesson{{ID: 1, ChapterID: 1, Type: 1, SortOrder: 1}, {ID: 2, ChapterID: 1, Type: 1, SortOrder: 2}}
	for i := range lessons {
		mustCreate(t, db, &lessons[i])
	}
	c, _ := deliveryTestContext(t, 1, "/content", nil)
	if err := courseAccess(c, lessons[0]); err == nil {
		t.Fatal("unpaid access granted")
	}
	mustCreate(t, db, &model.Order{ID: 1, OrderNo: "paid", UserID: 1, Status: "paid"})
	mustCreate(t, db, &model.CourseEnrollment{CourseID: 1, UserID: 1, OrderID: 1})
	mustCreate(t, db, &model.CourseDeliverySetting{CourseID: 1, Sequential: true})
	if err := courseAccess(c, lessons[0]); err != nil {
		t.Fatal(err)
	}
	if err := courseAccess(c, lessons[1]); err == nil {
		t.Fatal("sequence bypassed")
	}
	mustCreate(t, db, &model.UserLessonProgress{UserID: 1, CourseID: 1, LessonID: 1, IsCompleted: 1})
	if err := courseAccess(c, lessons[1]); err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-time.Hour)
	db.Model(&model.CourseEnrollment{}).Where("user_id = 1").Update("expires_at", expired)
	if err := courseAccess(c, lessons[1]); err == nil {
		t.Fatal("expired access granted")
	}
}

func TestDeliveryCohortIsolationReplayAndAttendance(t *testing.T) {
	db := deliveryTestDB(t)
	mustCreate(t, db, &model.Course{ID: 1, Level: "class", Status: 1})
	mustCreate(t, db, &model.CourseChapter{ID: 1, CourseID: 1})
	mustCreate(t, db, &model.Order{ID: 1, OrderNo: "class", UserID: 1, Status: "paid"})
	mustCreate(t, db, &model.CourseEnrollment{CourseID: 1, CohortID: 1, UserID: 1, OrderID: 1})
	lesson := model.CourseLesson{ID: 1, ChapterID: 1, CohortID: 2, Type: 3, LiveStatus: 1}
	mustCreate(t, db, &lesson)
	c, _ := deliveryTestContext(t, 1, "/content", nil)
	if err := courseAccess(c, lesson); err == nil {
		t.Fatal("different cohort access granted")
	}
	db.Model(&lesson).Update("cohort_id", 1)
	get := func() *httptest.ResponseRecorder {
		c, rec := deliveryTestContext(t, 1, "/content", nil)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		GetDeliveryLesson(c)
		return rec
	}
	if rec := get(); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	get()
	var count int64
	db.Model(&model.CourseAttendance{}).Count(&count)
	if count != 1 {
		t.Fatalf("attendance duplicated: %d", count)
	}
	db.Model(&lesson).Updates(map[string]any{"live_status": 2, "replay_enabled": true, "replay_status": "processing"})
	if rec := get(); rec.Code != 400 {
		t.Fatal("processing replay accessible")
	}
	db.Model(&lesson).Updates(map[string]any{"replay_status": "ready", "replay_file_id": "cloud://test/replay.mp4"})
	if rec := get(); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
}

func TestDeliverySingleLessonAndRedaction(t *testing.T) {
	db := deliveryTestDB(t)
	mustCreate(t, db, &model.Course{ID: 1, Level: "single", Status: 1})
	mustCreate(t, db, &model.CourseChapter{ID: 1, CourseID: 1})
	create := func() *httptest.ResponseRecorder {
		c, rec := deliveryTestContext(t, 0, "/lesson", gin.H{"title": "Video", "type": 1, "media_url": "https://example.com/video.mp4"})
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		CreateLesson(c)
		return rec
	}
	if rec := create(); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := create(); rec.Code != 400 {
		t.Fatal("single course accepted two lessons")
	}
	var lessons []model.CourseLesson
	db.Find(&lessons)
	c, _ := deliveryTestContext(t, 0, "/catalog", nil)
	redactDeliveryLessons(c, 1, lessons)
	if lessons[0].MediaURL != "" || lessons[0].MediaFileID != "" {
		t.Fatal("catalog exposed media")
	}
}

func TestDeliveryValidationAndForgedProgress(t *testing.T) {
	db := deliveryTestDB(t)
	mustCreate(t, db, &model.Course{ID: 1, Level: "system", Status: 1})
	mustCreate(t, db, &model.CourseChapter{ID: 1, CourseID: 1})
	mustCreate(t, db, &model.CourseLesson{ID: 1, ChapterID: 1, IsFree: 1})
	c, rec := deliveryTestContext(t, 1, "/progress", gin.H{"course_id": 2, "lesson_id": 1, "is_completed": 1})
	UpsertLessonProgress(c)
	if rec.Code != 400 {
		t.Fatal("forged course progress accepted")
	}
	if parseTime("2026-10-14 10:00:00").Format("-07:00") != "+08:00" {
		t.Fatal("schedule not parsed in Beijing time")
	}
	req := lessonRequest{Type: 3, LiveStart: "2026-10-14 10:00:00", Duration: 3600, MediaURL: "https://example.com/live.m3u8", ReplayEnabled: true, ReplayStatus: "ready"}
	if validateDeliveryLesson(1, req) == nil {
		t.Fatal("ready replay accepted without file ID")
	}
}
