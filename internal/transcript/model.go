package transcript

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrInvalidState = errors.New("invalid task state")
)

type Course struct {
	ID      uint64
	Name    string
	Credits int
}

// Enrollment references an existing catalog course; grades belong to students.
type Enrollment struct {
	CourseID uint64
	Score    int
}

type Student struct {
	ID          uint64
	Name        string
	Enrollments []Enrollment
}

type CourseScore struct {
	Course Course
	Score  int
}

// TranscriptInput resolves catalog references into an independent snapshot
type TranscriptInput struct {
	ID      uint64
	Name    string
	Courses []CourseScore
}

type Grade struct {
	Letter string
	Points float64
}

type TranscriptRow struct {
	CourseScore
	Grade Grade
}

type Transcript struct {
	StudentID     uint64
	StudentName   string
	Rows          []TranscriptRow
	TotalCredits  int
	EarnedCredits int
	AverageScore  float64
	GPA           float64
}

type State string

const (
	Pending    State = "pending"
	Processing State = "processing"
	Completed  State = "completed"
	Failed     State = "failed"
)

// Task keeps a snapshot of the student's input at creation time
type Task struct {
	ID      uint64
	Input   TranscriptInput
	State   State
	Result  *Transcript
	Failure string
}

func validateName(name string) error {
	if strings.TrimSpace(name) == "" || !utf8.ValidString(name) {
		return fmt.Errorf("%w: name must be non-empty UTF-8 text", ErrInvalidInput)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: name must not contain control characters", ErrInvalidInput)
		}
	}
	return nil
}

func validateCourse(course Course) error {
	if err := validateName(course.Name); err != nil {
		return fmt.Errorf("course name: %w", err)
	}
	if course.Credits <= 0 {
		return fmt.Errorf("%w: credits must be positive", ErrInvalidInput)
	}
	return nil
}

func validateScore(score int) error {
	if score < 0 || score > 100 {
		return fmt.Errorf("%w: score must be an integer from 0 to 100", ErrInvalidInput)
	}
	return nil
}

func validateCourses(courses []CourseScore) error {
	if len(courses) == 0 {
		return fmt.Errorf("%w: at least one course is required", ErrInvalidInput)
	}
	seen := make(map[uint64]bool, len(courses))
	totalCredits := 0
	maxInt := int(^uint(0) >> 1)
	for _, entry := range courses {
		course := entry.Course
		if err := validateCourse(course); err != nil {
			return err
		}
		if course.ID == 0 || seen[course.ID] {
			return fmt.Errorf("%w: course IDs must be non-zero and unique", ErrInvalidInput)
		}
		seen[course.ID] = true
		if course.Credits > maxInt-totalCredits {
			return fmt.Errorf("%w: credits must be positive and total credits must fit in int", ErrInvalidInput)
		}
		totalCredits += course.Credits
		if err := validateScore(entry.Score); err != nil {
			return err
		}
	}
	return nil
}
