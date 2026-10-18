package transcript

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Processor supplies transcript calculation to the service.
type Processor interface {
	Process(input TranscriptInput) (Transcript, error)
}

// Service owns the in-memory data. Use NewService and do not copy a Service
type Service struct {
	students      map[uint64]Student
	courses       map[uint64]Course
	tasks         map[uint64]Task
	nextStudentID uint64
	nextCourseID  uint64
	nextTaskID    uint64
	processor     Processor
}

func NewService(processor Processor) (*Service, error) {
	if processor == nil {
		return nil, fmt.Errorf("%w: processor is required", ErrInvalidInput)
	}
	return &Service{
		students: make(map[uint64]Student), courses: make(map[uint64]Course), tasks: make(map[uint64]Task),
		nextStudentID: 1, nextCourseID: 1, nextTaskID: 1, processor: processor,
	}, nil
}

func (s *Service) AddStudent(name string) (Student, error) {
	if err := validateName(name); err != nil {
		return Student{}, err
	}
	student := Student{ID: s.nextStudentID, Name: strings.TrimSpace(name)}
	s.students[student.ID] = student
	s.nextStudentID++
	return student, nil
}

func (s *Service) GetStudent(id uint64) (Student, error) {
	student, ok := s.students[id]
	if !ok {
		return Student{}, fmt.Errorf("%w: student %d", ErrNotFound, id)
	}
	return cloneStudent(student), nil
}

func (s *Service) ListStudents() []Student {
	students := make([]Student, 0, len(s.students))
	for _, student := range s.students {
		students = append(students, cloneStudent(student))
	}
	// Map iteration order is unspecified, IDs give a stable display order
	sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
	return students
}

// AddCourse is the only operation that creates a catalog entry
func (s *Service) AddCourse(name string, credits int) (Course, error) {
	course := Course{ID: s.nextCourseID, Name: strings.TrimSpace(name), Credits: credits}
	if err := validateCourse(course); err != nil {
		return Course{}, err
	}
	for _, existing := range s.courses {
		if strings.EqualFold(existing.Name, course.Name) {
			return Course{}, fmt.Errorf("%w: course %q already exists as #%d", ErrInvalidInput, course.Name, existing.ID)
		}
	}
	s.courses[course.ID] = course
	s.nextCourseID++
	return course, nil
}

func (s *Service) GetCourse(id uint64) (Course, error) {
	course, ok := s.courses[id]
	if !ok {
		return Course{}, fmt.Errorf("%w: course %d", ErrNotFound, id)
	}
	return course, nil
}

func (s *Service) ListCourses() []Course {
	courses := make([]Course, 0, len(s.courses))
	for _, course := range s.courses {
		courses = append(courses, course)
	}
	sort.Slice(courses, func(i, j int) bool { return courses[i].ID < courses[j].ID })
	return courses
}

// SetScore links a student to an existing course or corrects an existing score. It never creates a course or changes another student's grade
func (s *Service) SetScore(studentID, courseID uint64, score int) error {
	student, err := s.GetStudent(studentID)
	if err != nil {
		return err
	}
	course, err := s.GetCourse(courseID)
	if err != nil {
		return err
	}
	if err := validateScore(score); err != nil {
		return err
	}
	for i := range student.Enrollments {
		if student.Enrollments[i].CourseID == courseID {
			student.Enrollments[i].Score = score
			s.students[studentID] = student
			return nil
		}
	}
	totalCredits := course.Credits
	maxInt := int(^uint(0) >> 1)
	for _, enrollment := range student.Enrollments {
		existing, err := s.GetCourse(enrollment.CourseID)
		if err != nil {
			return err
		}
		if existing.Credits > maxInt-totalCredits {
			return fmt.Errorf("%w: total credits must fit in int", ErrInvalidInput)
		}
		totalCredits += existing.Credits
	}
	student.Enrollments = append(student.Enrollments, Enrollment{CourseID: courseID, Score: score})
	s.students[studentID] = student
	return nil
}

// GenerateTranscript is the complete application operation used by the console.
func (s *Service) GenerateTranscript(studentID uint64) (Transcript, error) {
	task, err := s.CreateTask(studentID)
	if err != nil {
		return Transcript{}, err
	}
	task, err = s.ProcessTask(task.ID)
	if err != nil {
		return Transcript{}, err
	}
	return *task.Result, nil
}

func (s *Service) CreateTask(studentID uint64) (Task, error) {
	student, err := s.GetStudent(studentID)
	if err != nil {
		return Task{}, err
	}
	input, err := s.inputFor(student)
	if err != nil {
		return Task{}, err
	}
	task := Task{ID: s.nextTaskID, Input: input, State: Pending}
	s.tasks[task.ID] = task
	s.nextTaskID++
	return cloneTask(task), nil
}

func (s *Service) GetTask(id uint64) (Task, error) {
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("%w: task %d", ErrNotFound, id)
	}
	return cloneTask(task), nil
}

func (s *Service) ProcessTask(id uint64) (Task, error) {
	task, err := s.GetTask(id)
	if err != nil {
		return Task{}, err
	}
	if task.State != Pending {
		return Task{}, fmt.Errorf("%w: task %d is %s; expected pending", ErrInvalidState, id, task.State)
	}
	task.State = Processing
	s.tasks[id] = task
	result, err := s.processor.Process(cloneInput(task.Input))
	if err != nil {
		task.State = Failed
		task.Failure = err.Error()
		s.tasks[id] = task
		return cloneTask(task), fmt.Errorf("process task %d: %w", id, err)
	}
	task.State = Completed
	task.Result = &result
	s.tasks[id] = cloneTask(task)
	return cloneTask(task), nil
}

func cloneStudent(student Student) Student {
	student.Enrollments = slices.Clone(student.Enrollments)
	return student
}

func (s *Service) inputFor(student Student) (TranscriptInput, error) {
	input := TranscriptInput{ID: student.ID, Name: student.Name,
		Courses: make([]CourseScore, 0, len(student.Enrollments))}
	for _, enrollment := range student.Enrollments {
		course, err := s.GetCourse(enrollment.CourseID)
		if err != nil {
			return TranscriptInput{}, err
		}
		input.Courses = append(input.Courses, CourseScore{Course: course, Score: enrollment.Score})
	}
	if err := validateCourses(input.Courses); err != nil {
		return TranscriptInput{}, err
	}
	return input, nil
}

func cloneInput(input TranscriptInput) TranscriptInput {
	input.Courses = slices.Clone(input.Courses)
	return input
}

func cloneTask(task Task) Task {
	task.Input = cloneInput(task.Input)
	if task.Result != nil {
		result := *task.Result
		result.Rows = slices.Clone(result.Rows)
		task.Result = &result
	}
	return task
}
