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
		"/api/v1/teacher/homework/submissions?course_id=undefined",
		"/api/v1/teacher/homework/submissions?course_id=0",
		"/api/v1/teacher/homework/submissions?course_id=-1",
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

func TestTeacherHomeworkAllCoursesAndSelectedCourse(t *testing.T) {
	router := teacherTestRouter(t)
	for _, filter := range []string{"", "&course_id=1", "&course_id="} {
		response := teacherTestRequest(t, router, 2, 10, "GET",
			"/api/v1/teacher/homework/submissions?status=submitted"+filter, nil)
		var body struct {
			Data struct {
				Items []teacherHomeworkItem `json:"items"`
				Total int                   `json:"total"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || body.Data.Total != 1 || len(body.Data.Items) != 1 || body.Data.Items[0].CourseID != 1 {
			t.Fatalf("homework filter rejected or leaked scope: %s", response.Body.String())
		}
	}
	response := teacherTestRequest(t, router, 2, 10, "GET",
		"/api/v1/teacher/homework/submissions?status=submitted&course_id=2", nil)
	var body struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != 200 || body.Data.Total != 0 {
		t.Fatal("selected course bypassed teacher scope")
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

func TestTeacherCourseWorkbenchGroupsAndCounts(t *testing.T) {
	router := teacherTestRouter(t)
	db := model.DB
	mustCreate(t, db, &model.Course{ID: 3, TeacherID: 10, Title: "My class", Level: "class", Status: 1})
	mustCreate(t, db, &model.Course{ID: 4, TeacherID: 10, Title: "No homework yet", Level: "single", Status: 1})
	mustCreate(t, db, &model.CourseChapter{ID: 5, CourseID: 1, Title: "My chapter"})
	mustCreate(t, db, &model.CourseChapter{ID: 6, CourseID: 2, Title: "Other chapter"})
	mustCreate(t, db, &model.CourseCohort{ID: 31, CourseID: 3, Name: "Class cohort"})
	mustCreate(t, db, &model.CourseLesson{ID: 61, ChapterID: 5, Type: 1, Title: "My video"})
	mustCreate(t, db, &model.CourseLesson{ID: 62, ChapterID: 5, Type: 3, Title: "Disabled lesson"})
	mustCreate(t, db, &model.CourseLesson{ID: 63, CohortID: 31, Type: 2, Title: "My cohort content"})
	mustCreate(t, db, &model.CourseLesson{ID: 64, ChapterID: 6, Type: 1, Title: "Other video"})
	mustCreate(t, db, &model.LearningTask{ID: 3, CourseID: 3, Title: "Class task", TotalScore: 100})
	mustCreate(t, db, &model.HomeworkSubmission{ID: 4, TaskID: 3, UserID: 1, Status: "submitted"})
	for _, test := range []struct {
		ID     string
		Kind   string
		Lesson uint64
	}{{"1", "chapter", 61}, {"3", "cohort", 63}} {
		response := teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/teacher/courses/"+test.ID+"/workbench", nil)
		var body struct {
			Data struct {
				Course      teacherCourseItem     `json:"course"`
				Catalog     []teacherCatalogGroup `json:"catalog"`
				LessonCount int                   `json:"lesson_count"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || body.Data.Course.PendingCount != 1 || body.Data.LessonCount != 1 ||
			len(body.Data.Catalog) != 1 || body.Data.Catalog[0].Kind != test.Kind ||
			len(body.Data.Catalog[0].Lessons) != 1 || body.Data.Catalog[0].Lessons[0].ID != test.Lesson {
			t.Fatalf("course workbench lost scope or grouped incorrectly: %s", response.Body.String())
		}
	}
	response := teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/teacher/courses/2/workbench", nil)
	if response.Code != 404 {
		t.Fatal("teacher opened another teacher's workbench")
	}
        response = teacherTestRequest(t, router, 1, 0, "GET", "/api/v1/teacher/courses/1/workbench", nil)
	if response.Code != 403 {
		t.Fatal("student opened teacher workbench")
	}
	for _, test := range []struct {
		Query string
		Total int
	}{{"", 3}, {"?homework_status=submitted&page_size=1", 2}, {"?homework_status=graded", 1}} {
		response = teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/teacher/courses"+test.Query, nil)
		var body struct {
			Data struct {
				Items []teacherCourseItem `json:"items"`
				Total int                 `json:"total"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != 200 || body.Data.Total != test.Total {
			t.Fatalf("course-level homework filtering failed: %s", response.Body.String())
		}
		for _, item := range body.Data.Items {
			if item.ID == 2 || (item.ID == 4 && (item.PendingCount != 0 || item.GradedCount != 0)) {
				t.Fatal("course counts leaked or empty course counts were incorrect")
			}
		}
	}
	response = teacherTestRequest(t, router, 2, 10, "POST", "/api/v1/teacher/homework/submissions/1/grade",
		gin.H{"score": 75, "is_passed": 1, "feedback": "Good"})
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	response = teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/teacher/courses/1/workbench", nil)
	var body struct {
		Data struct {
			Course teacherCourseItem `json:"course"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Data.Course.PendingCount != 0 || body.Data.Course.GradedCount != 2 {
		t.Fatal("workbench counts did not refresh after grading")
	}
	response = teacherTestRequest(t, router, 2, 10, "GET", "/api/v1/teacher/courses?homework_status=invalid", nil)
	if response.Code != 400 {
		t.Fatal("invalid homework status accepted")
	}
}
