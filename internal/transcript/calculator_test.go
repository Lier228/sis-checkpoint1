package transcript

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

func TestGradeFor(t *testing.T) {
	tests := []struct {
		low, high int
		letter    string
		points    float64
	}{
		{95, 100, "A", 4}, {90, 94, "A-", 3.67}, {85, 89, "B+", 3.33},
		{80, 84, "B", 3}, {75, 79, "B-", 2.67}, {70, 74, "C+", 2.33},
		{65, 69, "C", 2}, {60, 64, "C-", 1.67}, {55, 59, "D+", 1.33},
		{50, 54, "D", 1}, {0, 49, "F", 0},
	}
	for _, tt := range tests {
		for _, score := range []int{tt.low, tt.high} {
			t.Run(fmt.Sprint(score), func(t *testing.T) {
				got, err := GradeFor(score)
				if err != nil || got != (Grade{tt.letter, tt.points}) {
					t.Fatalf("GradeFor(%d) = %+v, %v", score, got, err)
				}
			})
		}
	}
	for _, score := range []int{-1, 101} {
		if _, err := GradeFor(score); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("GradeFor(%d) error = %v", score, err)
		}
	}
}

func TestCalculatorWeightedResult(t *testing.T) {
	student := TranscriptInput{ID: 7, Name: "Аружан", Courses: []CourseScore{
		{Course: Course{ID: 1, Name: "Go", Credits: 5}, Score: 95},
		{Course: Course{ID: 2, Name: "Math", Credits: 3}, Score: 85},
		{Course: Course{ID: 3, Name: "History", Credits: 2}, Score: 40},
	}}
	got, err := (Calculator{}).Process(student)
	if err != nil {
		t.Fatal(err)
	}
	if got.StudentID != 7 || got.StudentName != "Аружан" || len(got.Rows) != 3 || got.TotalCredits != 10 || got.EarnedCredits != 8 {
		t.Fatalf("unexpected transcript: %+v", got)
	}
	// (4*5 + 3.33*3 + 0*2) / 10 = 2.999, not an unweighted average.
	if math.Abs(got.GPA-2.999) > 1e-12 || math.Abs(got.AverageScore-81) > 1e-12 {
		t.Fatalf("GPA = %v, average = %v", got.GPA, got.AverageScore)
	}
	wantGrades := []Grade{{"A", 4}, {"B+", 3.33}, {"F", 0}}
	for i, row := range got.Rows {
		if row.CourseScore != student.Courses[i] || row.Grade != wantGrades[i] {
			t.Errorf("row %d = %+v", i, row)
		}
	}
}

func TestCalculatorAllFailedAndMinimumPass(t *testing.T) {
	for _, tt := range []struct {
		score, earned int
		gpa           float64
	}{{0, 0, 0}, {49, 0, 0}, {50, 3, 1}, {100, 3, 4}} {
		t.Run(fmt.Sprint(tt.score), func(t *testing.T) {
			got, err := (Calculator{}).Process(TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{1, "Go", 3}, tt.score}}})
			if err != nil || got.GPA != tt.gpa || got.EarnedCredits != tt.earned || got.AverageScore != float64(tt.score) {
				t.Fatalf("result = %+v, %v", got, err)
			}
		})
	}
}

func TestCalculatorRejectsInvalidInput(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	tests := []struct {
		name    string
		student TranscriptInput
	}{
		{"empty name", TranscriptInput{Courses: []CourseScore{{Course{1, "Go", 3}, 80}}}},
		{"empty courses", TranscriptInput{Name: "Student"}},
		{"empty course name", TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{1, " ", 3}, 80}}}},
		{"zero credits", TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{1, "Go", 0}, 80}}}},
		{"negative credits", TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{1, "Go", -1}, 80}}}},
		{"negative score", TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{1, "Go", 3}, -1}}}},
		{"score above 100", TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{1, "Go", 3}, 101}}}},
		{"missing course ID", TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{0, "Go", 3}, 80}}}},
		{"duplicate course", TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{1, "Go", 3}, 80}, {Course{1, "Go", 3}, 90}}}},
		{"credit overflow", TranscriptInput{Name: "Student", Courses: []CourseScore{{Course{1, "Go", maxInt}, 80}, {Course{2, "Math", 1}, 90}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := (Calculator{}).Process(tt.student); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
