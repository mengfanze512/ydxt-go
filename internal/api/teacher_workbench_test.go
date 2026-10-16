package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
)

func teacherTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db := deliveryTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.LearningTask{}, &model.HomeworkSubmission{}); err != nil {
		t.Fatal(err)
	}
	for _, course := range []model.Course{
		{ID: 1, TeacherID: 10, Title: "First", Level: "system", Status: 1},
		{ID: 2, TeacherID: 20, Title: "Other", Level: "class", Status: 1},
	} {
		mustCreate(t, db, &course)
	}
	db.Model(&model.User{}).Where("id = 1").Update("nickname", "Student")
	mustCreate(t, db, &model.Teacher{ID: 10, Status: 1})
	mustCreate(t, db, &model.Teacher{ID: 20, Status: 1})
	mustCreate(t, db, &model.LearningTask{ID: 1, CourseID: 1, Title: "First task", TotalScore: 80, PassScore: 50})
	mustCreate(t, db, &model.LearningTask{ID: 2, CourseID: 2, Title: "Other task", TotalScore: 100})
	for _, item := range []model.HomeworkSubmission{
		{ID: 1, UserID: 1, TaskID: 1, Status: "submitted", MediaType: "audio"},
		{ID: 2, UserID: 1, TaskID: 2, Status: "submitted"},
		{ID: 3, UserID: 1, TaskID: 1, Status: "graded", Score: 60},
	} {
		mustCreate(t, db, &item)
	}
	return InitRouter()
}

func teacherTestRequest(t *testing.T, router *gin.Engine, role int8, teacherID uint64, method, url string, body any) *httptest.ResponseRecorder {
	t.Helper()
	account := "teacher"
	if role == 9 {
		account = "admin"
	}
	if role == 1 {
		account = "user"
	}
	userID, roleCode := uint64(5), account
	if role == 2 && teacherID > 0 {
		userID = teacherID
	}
	if role == 1 {
		roleCode = "student"
	}
	token, err := utils.GenerateToken(userID, role, account, roleCode, teacherID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(body)
	request := httptest.NewRequest(method, url, bytes.NewReader(raw))
	request.Header.Set("X-Token", token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestTeacherWorkbenchScopeAndPagination(t *testing.T) {
	router := teacherTestRouter(t)
	response := teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/teacher/workbench", nil)
	var overview struct {
		Data struct {
			CourseCount    int                   `json:"course_count"`
			PendingCount   int                   `json:"pending_count"`
			GradedCount    int                   `json:"graded_count"`
			RecentHomework []teacherHomeworkItem `json:"recent_homework"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || overview.Data.CourseCount != 1 || overview.Data.PendingCount != 1 || overview.Data.GradedCount != 1 || len(overview.Data.RecentHomework) != 1 || overview.Data.RecentHomework[0].CourseID != 1 {
		t.Fatalf("teacher overview leaked or miscounted: %s", response.Body.String())
	}
	response = teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/teacher/homework/submissions?page_size=1&status=submitted", nil)
	var list struct {
		Data struct {
			Items        []teacherHomeworkItem `json:"items"`
			Total        int                   `json:"total"`
			PendingCount int                   `json:"pending_count"`
			GradedCount  int                   `json:"graded_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || list.Data.Total != 1 || list.Data.PendingCount != 1 || list.Data.GradedCount != 1 || len(list.Data.Items) != 1 || list.Data.Items[0].StudentName != "Student" {
		t.Fatalf("teacher homework scope/count failed: %s", response.Body.String())
	}
	for _, url := range []string{
		"/api/v1/teacher/courses?page_size=0",
		"/api/v1/teacher/courses?level=invalid",
		"/api/v1/teacher/homework/submissions?status=invalid",
	} {
		if response := teacherTestRequest(t, router, 2, 10, "GET", url, nil); response.Code != 400 {
			t.Fatalf("invalid parameters accepted: %s", response.Body.String())
		}
	}
	response = teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/teacher/courses?level=class", nil)
	var courseList struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &courseList)
	if response.Code != 200 || courseList.Data.Total != 0 {
		t.Fatal("course filter leaked other teacher's course")
	}
}

func TestTeacherGradeAndMediaPermissions(t *testing.T) {
	router := teacherTestRouter(t)
	for _, action := range []struct{ Method, URL string }{
		{"POST", "/api/v1/teacher/homework/submissions/2/grade"},
		{"GET", "/api/v1/teacher/homework/submissions/2/play-url"},
	} {
		response := teacherTestRequest(t, router, 2, 10, action.Method, action.URL, gin.H{"score": 70, "is_passed": 1, "feedback": "Good"})
		if response.Code != http.StatusNotFound {
			t.Fatalf("other teacher's submission accessible: %s", response.Body.String())
		}
	}
	for _, role := range []int8{1, 2} {
		if response := teacherTestRequest(t, router, role, 0, "GET", "/api/v1/teacher/workbench", nil); response.Code != 403 && response.Code != 401 {
			t.Fatalf("invalid teacher identity granted: %s", response.Body.String())
		}
	}
	for _, body := range []gin.H{
		{"score": 81, "is_passed": 1, "feedback": "Good"},
		{"score": 70, "is_passed": 1, "feedback": " "},
		{"is_passed": 1, "feedback": "Good"},
		{"score": 70, "is_passed": 2, "feedback": "Good"},
	} {
		if response := teacherTestRequest(t, router, 2, 10, "POST", "/api/v1/teacher/homework/submissions/1/grade", body); response.Code != 400 {
			t.Fatalf("invalid grade accepted: %s", response.Body.String())
		}
	}
	response := teacherTestRequest(t, router, 2, 10, "POST", "/api/v1/teacher/homework/submissions/1/grade", gin.H{"score": 70, "is_passed": 1, "feedback": " Keep practicing "})
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var saved model.HomeworkSubmission
	model.DB.First(&saved, 1)
	if saved.Status != "graded" || saved.Score != 70 || saved.Feedback != "Keep practicing" || saved.GraderID == nil || *saved.GraderID != 10 {
		t.Fatalf("grade not persisted: %+v", saved)
	}
}

func TestLegacyTeacherAdminEndpointsStayScoped(t *testing.T) {
	router := teacherTestRouter(t)
	for _, url := range []string{"/api/v1/admin/courses/2/delivery-settings", "/api/v1/admin/learning/tasks/2/submissions"} {
		response := teacherTestRequest(t, router, 2, 10, "GET", url, nil)
		if response.Code != 403 {
			t.Fatalf("unowned teaching resource accessible: %s", response.Body.String())
		}
	}
	response := teacherTestRequest(t, router, 2, 10, "POST", "/api/v1/admin/homework/submissions/2/grade", gin.H{"score": 70, "is_passed": 1, "feedback": "Good"})
	if response.Code != 404 {
		t.Fatal("legacy grading bypassed teacher scope")
	}
	response = teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/admin/learning/tasks", nil)
	var tasks struct {
		Data []model.LearningTask `json:"data"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &tasks)
	if response.Code != 200 || len(tasks.Data) != 1 || tasks.Data[0].CourseID != 1 {
		t.Fatal("legacy task list leaked teacher data")
	}
}
