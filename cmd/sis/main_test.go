package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"sis-checkpoint1/internal/transcript"
)

func TestDemo(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--demo"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Студент #1: Аружан", "Студент #2: Диас", "GPA: 3.75 / 4.00", "GPA: 1.88 / 4.00", "Зачтено: 5"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("demo missing %q:\n%s", want, out.String())
		}
	}
}

func TestInteractiveWorkflow(t *testing.T) {
	input := strings.Join([]string{
		"5", "Go", "5",
		"5", "Math", "3",
		"6",
		"1", "Аружан",
		"3", "1", "1", "101", "95",
		"3", "1", "2", "85",

		// GenerateTranscript
		"2", "4", "1",
		// Check 999
		"3", "1", "1", "999",
		// Check 95
		"4", "1",
		// Check reassigning the grade
		"3", "1", "1", "80",
		// Last transcript
		"4", "1",
		"0",
	}, "\n")

	var out bytes.Buffer
	if err := run(nil, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	output := out.String()

	for _, want := range []string{
		"Создана дисциплина #1: Go",
		"Создана дисциплина #2: Math",
		"Добавлен студент #1",
		"Повторите ввод",
		"дисциплин: 2",
		"Список студентов:\n#1 Аружан\nID студента (0 — отмена): ",
		"GPA: 3.75 / 4.00",
		"GPA: 3.12 / 4.00",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("workflow missing %q:\n%s", want, output)
		}
	}
	if strings.Count(output, "GPA:") != 3 {
		t.Errorf("expected 3 transcripts:\n%s", output)
	}

	// After 999 gpa shouldn't change
	if strings.Count(output, "GPA: 3.75 / 4.00") != 2 {
		t.Errorf("cancellation changed GPA:\n%s", output)
	}

	// Check GPA after reassigning grade
	if strings.Count(output, "GPA: 3.12 / 4.00") != 1 {
		t.Errorf("score update failed:\n%s", output)
	}

	// 999 wouldn't save
	if strings.Contains(output, "Оценка сохранена: Аружан — Go: 999") {
		t.Errorf("cancel value was saved as score:\n%s", output)
	}

	// three grades will be saved
	if strings.Count(output, "Оценка сохранена:") != 3 {
		t.Errorf("unexpected number of saved scores:\n%s", output)
	}

	for _, unwanted := range []string{
		"ID задачи:",
		"Обработать задачу",
		"Создать задачу",
		"invalid task state",
	} {
		if strings.Contains(output, unwanted) {
			t.Errorf("unexpected manual task step %q", unwanted)
		}
	}
}

func gradingConsole(t *testing.T, input string) (*console, *bytes.Buffer) {
	t.Helper()
	service, err := transcript.NewService(transcript.Calculator{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddStudent("Student"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddCourse("OS", 5); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	return &console{scanner: bufio.NewScanner(strings.NewReader(input)), out: out, service: service}, out
}

func TestWrongCourseSelectionRedisplaysCatalogWithoutCreatingCourses(t *testing.T) {
	c, out := gradingConsole(t, "1\nOF\n99\n-1\n18446744073709551616\n1\nabc\n101\n95\n")
	if err := c.setScore(); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "Доступные дисциплины:") != 5 || strings.Count(out.String(), "#1 OS (5 кредитов)") != 5 {
		t.Fatalf("catalog should be shown initially and after each invalid selection:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Выберите существующую дисциплину") {
		t.Fatalf("missing selection guidance:\n%s", out.String())
	}
	for _, forbidden := range []string{"Название дисциплины:", "Кредиты ("} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("grading should not request %q", forbidden)
		}
	}
	courses := c.service.ListCourses()
	student, err := c.service.GetStudent(1)
	if err != nil || len(courses) != 1 || courses[0].Name != "OS" || len(student.Enrollments) != 1 || student.Enrollments[0].CourseID != 1 || student.Enrollments[0].Score != 95 {
		t.Fatalf("wrong catalog or enrollment: %+v, %+v, %v", courses, student, err)
	}
}

func TestCancelCourseSelectionDoesNotChangeStudent(t *testing.T) {
	c, _ := gradingConsole(t, "1\nOF\n0\n")
	if err := c.setScore(); err != nil {
		t.Fatal(err)
	}
	student, err := c.service.GetStudent(1)
	if err != nil || len(student.Enrollments) != 0 || len(c.service.ListCourses()) != 1 {
		t.Fatalf("cancel changed data: %+v, %v", student, err)
	}
}

func TestEmptyCatalogGuidesUserToCreateCourse(t *testing.T) {
	var out bytes.Buffer
	if err := run(nil, strings.NewReader("1\nStudent\n3\n0\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Сначала создайте дисциплину в пункте 5") || strings.Contains(out.String(), "Номер дисциплины") || strings.Contains(out.String(), "Оценка (целое") {
		t.Fatalf("empty catalog should stop grading with guidance:\n%s", out.String())
	}
}

func TestDuplicateCourseIsRejectedInMenu(t *testing.T) {
	var out bytes.Buffer
	if err := run(nil, strings.NewReader("5\nOS\n5\n5\n os \n3\n6\n0\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "already exists") || strings.Contains(out.String(), "Создана дисциплина #2") || !strings.Contains(out.String(), "#1 OS (5 кредитов)") {
		t.Fatalf("duplicate handling:\n%s", out.String())
	}
}

func TestInteractiveInvalidIDsAndNumbers(t *testing.T) {
	var out bytes.Buffer
	if err := run(nil, strings.NewReader("1\nStudent\n4\nabc\n99\n0\n5\nOS\nabc\n0\n"), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Выберите существующего студента", "ожидалось целое число"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q:\n%s", want, out.String())
		}
	}
}

func TestEOFExitsCleanly(t *testing.T) {
	for _, input := range []string{"", "1\n", "3\n", "4\n", "5\n", "5\nOS\n"} {
		var out bytes.Buffer
		if err := run(nil, strings.NewReader(input), &out); err != nil {
			t.Errorf("input %q: %v", input, err)
		}
	}
	for _, input := range []string{"1\n", "1\n1\n", "1\nOF\n"} {
		c, _ := gradingConsole(t, input)
		if err := c.setScore(); !errors.Is(err, io.EOF) {
			t.Errorf("grading input %q: %v", input, err)
		}
	}
}

func TestGenerateWithoutGradesShowsError(t *testing.T) {
	var out bytes.Buffer
	if err := run(nil, strings.NewReader("1\nStudent\n4\n1\n0\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "invalid input") || strings.Contains(out.String(), "GPA:") {
		t.Fatalf("expected validation error:\n%s", out.String())
	}
}

func TestUnknownArgument(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--unknown"}, strings.NewReader(""), &out); err == nil {
		t.Fatal("expected usage error")
	}
}
