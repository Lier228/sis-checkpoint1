package transcript

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func newTestService(t *testing.T, processor Processor) *Service {
	t.Helper()
	s, err := NewService(processor)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func addStudent(t *testing.T, s *Service, name string) Student {
	t.Helper()
	student, err := s.AddStudent(name)
	if err != nil {
		t.Fatal(err)
	}
	return student
}

func addCourse(t *testing.T, s *Service, name string, credits int) Course {
	t.Helper()
	course, err := s.AddCourse(name, credits)
	if err != nil {
		t.Fatal(err)
	}
	return course
}

func setScore(t *testing.T, s *Service, studentID, courseID uint64, score int) {
	t.Helper()
	if err := s.SetScore(studentID, courseID, score); err != nil {
		t.Fatal(err)
	}
}

func createTask(t *testing.T, s *Service, studentID uint64) Task {
	t.Helper()
	task, err := s.CreateTask(studentID)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestCatalogCreationAndValidation(t *testing.T) {
	s := newTestService(t, Calculator{})
	for _, tt := range []struct {
		name    string
		credits int
	}{
		{"", 3}, {" \t", 3}, {"\u2003", 3}, {"A\nB", 3}, {string([]byte{0xff}), 3}, {"OS", 0}, {"OS", -1},
	} {
		if _, err := s.AddCourse(tt.name, tt.credits); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("AddCourse(%q, %d): %v", tt.name, tt.credits, err)
		}
	}
	if len(s.ListCourses()) != 0 {
		t.Fatal("invalid courses were stored")
	}
	os := addCourse(t, s, " OS ", 5)
	mathCourse := addCourse(t, s, "Math", 3)
	if os.ID != 1 || os.Name != "OS" || mathCourse.ID != 2 {
		t.Fatalf("courses: %+v, %+v", os, mathCourse)
	}
	if _, err := s.AddCourse(" os ", 7); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("duplicate error: %v", err)
	}
	if got, err := s.GetCourse(os.ID); err != nil || got != os {
		t.Fatalf("GetCourse: %+v, %v", got, err)
	}
	if got := s.ListCourses(); !reflect.DeepEqual(got, []Course{os, mathCourse}) {
		t.Fatalf("catalog: %+v", got)
	}
	copyOfCatalog := s.ListCourses()
	copyOfCatalog[0].Name = "OF"
	got, err := s.GetCourse(os.ID)
	if err != nil || got.Name != "OS" {
		t.Fatalf("catalog leaked mutable data: %+v, %v", got, err)
	}
}

func TestInvalidStudentsAndEmptyTranscript(t *testing.T) {
	s := newTestService(t, Calculator{})
	for _, name := range []string{"", " \t", "\u2003", "A\nB", "\x1b[31m", string([]byte{0xff})} {
		if _, err := s.AddStudent(name); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("AddStudent(%q): %v", name, err)
		}
	}
	if len(s.ListStudents()) != 0 {
		t.Fatal("invalid students were stored")
	}
	student := addStudent(t, s, " Student ")
	if student.ID != 1 || student.Name != "Student" {
		t.Fatalf("student: %+v", student)
	}
	if _, err := s.CreateTask(student.ID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty task error: %v", err)
	}
	if len(s.tasks) != 0 {
		t.Fatal("invalid task was stored")
	}
}

func TestSetScoreRequiresCatalogAndIsAtomic(t *testing.T) {
	s := newTestService(t, Calculator{})
	student := addStudent(t, s, "Student")
	course := addCourse(t, s, "OS", 5)
	for _, tt := range []struct {
		studentID, courseID uint64
		score               int
		want                error
	}{
		{student.ID, 99, 80, ErrNotFound}, {99, course.ID, 80, ErrNotFound},
		{student.ID, course.ID, -1, ErrInvalidInput}, {student.ID, course.ID, 101, ErrInvalidInput},
	} {
		if err := s.SetScore(tt.studentID, tt.courseID, tt.score); !errors.Is(err, tt.want) {
			t.Errorf("SetScore: %v", err)
		}
	}
	stored, err := s.GetStudent(student.ID)
	if err != nil || len(stored.Enrollments) != 0 || len(s.ListCourses()) != 1 {
		t.Fatalf("invalid operation changed data: %+v, %v", stored, err)
	}
	setScore(t, s, student.ID, course.ID, 80)
	setScore(t, s, student.ID, course.ID, 95)
	if err := s.SetScore(student.ID, course.ID, 101); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid correction: %v", err)
	}
	stored, err = s.GetStudent(student.ID)
	if err != nil || !reflect.DeepEqual(stored.Enrollments, []Enrollment{{course.ID, 95}}) {
		t.Fatalf("score update: %+v, %v", stored, err)
	}
	if got := s.ListCourses(); !reflect.DeepEqual(got, []Course{course}) {
		t.Fatalf("grading changed catalog: %+v", got)
	}
}

func TestMissingIDs(t *testing.T) {
	s := newTestService(t, Calculator{})
	for _, id := range []uint64{0, 1, 99} {
		if _, err := s.GetStudent(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetStudent(%d): %v", id, err)
		}
		if _, err := s.GetCourse(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetCourse(%d): %v", id, err)
		}
		if _, err := s.CreateTask(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("CreateTask(%d): %v", id, err)
		}
		if _, err := s.GetTask(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetTask(%d): %v", id, err)
		}
		if _, err := s.ProcessTask(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("ProcessTask(%d): %v", id, err)
		}
	}
}

func TestCreateRetrieveAndProcessTask(t *testing.T) {
	s := newTestService(t, Calculator{})
	student := addStudent(t, s, "Аружан")
	goCourse := addCourse(t, s, "Go", 5)
	mathCourse := addCourse(t, s, "Math", 3)
	setScore(t, s, student.ID, goCourse.ID, 95)
	setScore(t, s, student.ID, mathCourse.ID, 85)
	created := createTask(t, s, student.ID)
	if created.ID != 1 || created.State != Pending || created.Result != nil || created.Input.ID != student.ID || len(created.Input.Courses) != 2 {
		t.Fatalf("new task: %+v", created)
	}
	if got, err := s.GetTask(created.ID); err != nil || !reflect.DeepEqual(got, created) {
		t.Fatalf("GetTask: %+v, %v", got, err)
	}
	completed, err := s.ProcessTask(created.ID)
	if err != nil || completed.State != Completed || completed.Result == nil {
		t.Fatalf("ProcessTask: %+v, %v", completed, err)
	}
	if math.Abs(completed.Result.GPA-3.74875) > 1e-12 || completed.Result.TotalCredits != 8 || completed.Result.Rows[0].Course != goCourse {
		t.Fatalf("result: %+v", completed.Result)
	}
	if _, err := s.ProcessTask(created.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("repeat error: %v", err)
	}
	if got, err := s.GetTask(created.ID); err != nil || !reflect.DeepEqual(got, completed) {
		t.Fatalf("stored result: %+v, %v", got, err)
	}
}

func TestSharedCourseIndependentScoresAndTaskSnapshots(t *testing.T) {
	s := newTestService(t, Calculator{})
	course := addCourse(t, s, "OS", 5)
	first := addStudent(t, s, "Same name")
	second := addStudent(t, s, "Same name")
	if first.ID == second.ID {
		t.Fatal("duplicate student IDs")
	}
	setScore(t, s, first.ID, course.ID, 95)
	setScore(t, s, second.ID, course.ID, 50)
	oldFirst := createTask(t, s, first.ID)
	other := createTask(t, s, second.ID)
	setScore(t, s, first.ID, course.ID, 80)
	newFirst := createTask(t, s, first.ID)
	if oldFirst.ID == other.ID || oldFirst.ID == newFirst.ID || other.ID == newFirst.ID {
		t.Fatal("duplicate task IDs")
	}
	for _, tt := range []struct {
		task  Task
		gpa   float64
		score int
	}{{other, 1, 50}, {oldFirst, 4, 95}, {newFirst, 3, 80}} {
		got, err := s.ProcessTask(tt.task.ID)
		if err != nil || got.Result == nil || got.Result.GPA != tt.gpa || got.Result.Rows[0].Score != tt.score || got.Result.Rows[0].Course != course {
			t.Fatalf("task %d: %+v, %v", tt.task.ID, got, err)
		}
	}
	setScore(t, s, first.ID, course.ID, 0)
	stored, err := s.GetTask(newFirst.ID)
	if err != nil || stored.Result.GPA != 3 {
		t.Fatalf("completed result changed: %+v, %v", stored, err)
	}
	if len(s.ListCourses()) != 1 {
		t.Fatal("student grading created extra courses")
	}
	students := s.ListStudents()
	if len(students) != 2 || students[0].ID != first.ID || students[1].Enrollments[0].Score != 50 {
		t.Fatalf("student list: %+v", students)
	}
}

type processorFunc func(TranscriptInput) (Transcript, error)

func (f processorFunc) Process(input TranscriptInput) (Transcript, error) { return f(input) }

func TestProcessingStateAndFailure(t *testing.T) {
	for _, shouldFail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[shouldFail], func(t *testing.T) {
			var s *Service
			var taskID uint64
			cause := errors.New("calculation failed")
			calls := 0
			s = newTestService(t, processorFunc(func(input TranscriptInput) (Transcript, error) {
				calls++
				active, err := s.GetTask(taskID)
				if err != nil || active.State != Processing || active.Result != nil {
					t.Fatalf("active: %+v, %v", active, err)
				}
				if _, err := s.ProcessTask(taskID); !errors.Is(err, ErrInvalidState) {
					t.Fatalf("reentrant error: %v", err)
				}
				if shouldFail {
					return Transcript{}, cause
				}
				return Calculator{}.Process(input)
			}))
			student := addStudent(t, s, "Student")
			course := addCourse(t, s, "OS", 5)
			setScore(t, s, student.ID, course.ID, 95)
			taskID = createTask(t, s, student.ID).ID
			got, err := s.ProcessTask(taskID)
			if calls != 1 {
				t.Fatalf("calls: %d", calls)
			}
			if shouldFail {
				if !errors.Is(err, cause) || got.State != Failed || got.Result != nil || got.Failure != cause.Error() {
					t.Fatalf("failure: %+v, %v", got, err)
				}
			} else if err != nil || got.State != Completed || got.Result == nil {
				t.Fatalf("success: %+v, %v", got, err)
			}
			stored, err := s.GetTask(taskID)
			if err != nil || !reflect.DeepEqual(stored, got) {
				t.Fatalf("stored: %+v, %v", stored, err)
			}
			if _, err := s.ProcessTask(taskID); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("repeat: %v", err)
			}
		})
	}
}

func TestCopiesProtectStoredData(t *testing.T) {
	var retained Transcript
	s := newTestService(t, processorFunc(func(input TranscriptInput) (Transcript, error) {
		result, err := (Calculator{}).Process(input)
		input.Courses[0].Score = 0
		retained = result
		return result, err
	}))
	student := addStudent(t, s, "Student")
	course := addCourse(t, s, "OS", 5)
	setScore(t, s, student.ID, course.ID, 95)
	studentCopy, err := s.GetStudent(student.ID)
	if err != nil {
		t.Fatal(err)
	}
	studentCopy.Enrollments[0].Score = 0
	s.ListStudents()[0].Enrollments[0].CourseID = 99
	task := createTask(t, s, student.ID)
	task.Input.Courses[0].Score = 0
	taskCopy, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	taskCopy.Input.Courses[0].Course.Name = "OF"
	completed, err := s.ProcessTask(task.ID)
	if err != nil || completed.Result == nil || completed.Result.GPA != 4 {
		t.Fatalf("result: %+v, %v", completed, err)
	}
	completed.Result.GPA = 0
	completed.Result.Rows[0].Score = 0
	retained.Rows[0].Grade.Letter = "F"
	stored, err := s.GetTask(task.ID)
	if err != nil || stored.Result.GPA != 4 || stored.Result.Rows[0].Score != 95 || stored.Result.Rows[0].Grade.Letter != "A" || stored.Input.Courses[0].Course.Name != "OS" {
		t.Fatalf("stored data mutated: %+v, %v", stored, err)
	}
	stored.Result.Rows[0].Course.Name = "OF"
	again, err := s.GetTask(task.ID)
	if err != nil || again.Result.Rows[0].Course.Name != "OS" {
		t.Fatalf("GetTask leaked a slice: %+v, %v", again, err)
	}
}

func TestCreditOverflowDoesNotAttachCourse(t *testing.T) {
	s := newTestService(t, Calculator{})
	student := addStudent(t, s, "Student")
	large := addCourse(t, s, "Large", int(^uint(0)>>1))
	extra := addCourse(t, s, "Extra", 1)
	setScore(t, s, student.ID, large.ID, 95)
	if err := s.SetScore(student.ID, extra.ID, 80); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("overflow: %v", err)
	}
	stored, err := s.GetStudent(student.ID)
	if err != nil || len(stored.Enrollments) != 1 {
		t.Fatalf("overflow changed records: %+v, %v", stored, err)
	}
}

func TestNilProcessor(t *testing.T) {
	if _, err := NewService(nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil processor: %v", err)
	}
}

func TestGenerateTranscriptStoresIndependentResults(t *testing.T) {
	s := newTestService(t, Calculator{})
	student := addStudent(t, s, "Student")
	course := addCourse(t, s, "OS", 5)
	if _, err := s.GenerateTranscript(student.ID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty transcript: %v", err)
	}
	if _, err := s.GenerateTranscript(99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing student: %v", err)
	}
	if len(s.tasks) != 0 {
		t.Fatal("invalid generation stored a task")
	}
	setScore(t, s, student.ID, course.ID, 95)
	first, err := s.GenerateTranscript(student.ID)
	if err != nil || first.GPA != 4 || len(first.Rows) != 1 {
		t.Fatalf("first transcript: %+v, %v", first, err)
	}
	first.Rows[0].Score = 0
	setScore(t, s, student.ID, course.ID, 80)
	second, err := s.GenerateTranscript(student.ID)
	if err != nil || second.GPA != 3 || second.Rows[0].Score != 80 {
		t.Fatalf("updated transcript: %+v, %v", second, err)
	}
	for _, tt := range []struct {
		id    uint64
		gpa   float64
		score int
	}{{1, 4, 95}, {2, 3, 80}} {
		task, err := s.GetTask(tt.id)
		if err != nil || task.State != Completed || task.Result == nil || task.Result.GPA != tt.gpa || task.Result.Rows[0].Score != tt.score {
			t.Fatalf("stored task %d: %+v, %v", tt.id, task, err)
		}
	}
}

func TestGenerateTranscriptReturnsProcessingError(t *testing.T) {
	cause := errors.New("calculation failed")
	s := newTestService(t, processorFunc(func(TranscriptInput) (Transcript, error) {
		return Transcript{}, cause
	}))
	student := addStudent(t, s, "Student")
	course := addCourse(t, s, "OS", 5)
	setScore(t, s, student.ID, course.ID, 95)
	if _, err := s.GenerateTranscript(student.ID); !errors.Is(err, cause) {
		t.Fatalf("generation error: %v", err)
	}
	task, err := s.GetTask(1)
	if err != nil || task.State != Failed || task.Result != nil || task.Failure != cause.Error() {
		t.Fatalf("failed generation: %+v, %v", task, err)
	}
}
