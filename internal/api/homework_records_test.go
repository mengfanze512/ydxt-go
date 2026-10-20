package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ydxt-go/internal/model"
)

func TestStudentHomeworkRecordsScopeAndFeedback(t *testing.T) {
	teacherTestRouter(t)
	mustCreate(t, model.DB, &model.CourseChapter{ID: 71, CourseID: 1, Title: "Breathing"})
	mustCreate(t, model.DB, &model.CourseLesson{ID: 72, ChapterID: 71, Title: "Long tone", Type: 1})
	mustCreate(t, model.DB, &model.CourseCohort{ID: 73, CourseID: 2, Name: "October class"})
	mustCreate(t, model.DB, &model.CourseLesson{ID: 74, CohortID: 73, Title: "Class practice", Type: 2})
	model.DB.Model(&model.LearningTask{}).Where("id = 1").Updates(map[string]any{"target_type": "lesson", "target_id": 72})
	model.DB.Model(&model.LearningTask{}).Where("id = 2").Updates(map[string]any{"target_type": "lesson", "target_id": 74})
	model.DB.Model(&model.HomeworkSubmission{}).Where("id = 3").Updates(map[string]any{
		"feedback": "Keep practicing", "is_passed": 1,
	})
	mustCreate(t, model.DB, &model.HomeworkSubmission{ID: 10, UserID: 5, TaskID: 1, Status: "graded", Feedback: "Private"})
	router := gin.New()
	router.GET("/records", func(c *gin.Context) {
		c.Set("userID", uint64(1))
		GetMyHomeworkSubmissions(c)
	})
	for _, test := range []struct {
		URL   string
		Total int
	}{
		{"/records", 3}, {"/records?task_id=1", 2}, {"/records?task_id=99", 0},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", test.URL, nil))
		var body struct {
			Data []struct {
				model.HomeworkSubmission
				TaskTitle    string `json:"task_title"`
				CourseTitle  string `json:"course_title"`
				TotalScore   int    `json:"total_score"`
				ChapterTitle string `json:"chapter_title"`
				LessonTitle  string `json:"lesson_title"`
				CohortName   string `json:"cohort_name"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || len(body.Data) != test.Total {
			t.Fatalf("records query failed: %s", response.Body.String())
		}
		for _, item := range body.Data {
			if item.TaskID == 1 && (item.ChapterTitle != "Breathing" || item.LessonTitle != "Long tone" || item.CohortName != "") {
				t.Fatalf("chapter/lesson ownership missing: %+v", item)
			}
			if item.TaskID == 2 && (item.CohortName != "October class" || item.LessonTitle != "Class practice") {
				t.Fatalf("cohort ownership missing: %+v", item)
			}
			if item.UserID != 1 || item.TaskTitle == "" || item.CourseTitle == "" {
				t.Fatalf("records lost identity scope or titles: %+v", item)
			}
			if item.ID == 3 && (item.Feedback != "Keep practicing" || item.Score != 60 || item.IsPassed != 1 || item.TotalScore != 80) {
				t.Fatalf("teacher review missing: %+v", item)
			}
		}
	}
}
