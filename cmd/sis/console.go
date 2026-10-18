package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"sis-checkpoint1/internal/transcript"
)

func run(args []string, input io.Reader, output io.Writer) error {
	service, err := transcript.NewService(transcript.Calculator{})
	if err != nil {
		return err
	}
	if len(args) == 1 && args[0] == "--demo" {
		return demo(service, output)
	}
	if len(args) != 0 {
		return fmt.Errorf("запуск: go run ./cmd/sis [--demo]")
	}
	ui := console{scanner: bufio.NewScanner(input), out: output, service: service}
	fmt.Fprintln(output, "Студенческие транскрипты — Checkpoint 1")
	fmt.Fprintln(output, "Данные хранятся в памяти и исчезнут после выхода.")
	for {
		fmt.Fprintln(output, "\n1. Добавить студента\n2. Список студентов\n3. Выставить или исправить оценку\n4. Сформировать транскрипт\n5. Создать дисциплину\n6. Список дисциплин\n0. Выход")
		choice, err := ui.read("Выберите действие: ")
		if errors.Is(err, io.EOF) || (err == nil && choice == "0") {
			return nil
		}
		if err != nil {
			return err
		}
		if err := ui.execute(choice); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			fmt.Fprintln(output, "Ошибка:", err)
		}
	}
}

type console struct {
	scanner *bufio.Scanner
	out     io.Writer
	service *transcript.Service
}

func (c *console) read(prompt string) (string, error) {
	fmt.Fprint(c.out, prompt)
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return strings.TrimSpace(c.scanner.Text()), nil
}

func (c *console) readInt(prompt string) (int, error) {
	value, err := c.read(prompt)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("ожидалось целое число: %q", value)
	}
	return n, nil
}

func (c *console) execute(choice string) error {
	switch choice {
	case "1":
		name, err := c.read("Имя студента: ")
		if err != nil {
			return err
		}
		student, err := c.service.AddStudent(name)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.out, "Добавлен студент #%d: %s\n", student.ID, student.Name)
	case "2":
		students := c.service.ListStudents()
		if len(students) == 0 {
			fmt.Fprintln(c.out, "Студентов пока нет.")
		}
		for _, student := range students {
			fmt.Fprintf(c.out, "#%d %s — дисциплин: %d\n", student.ID, student.Name, len(student.Enrollments))
			for _, enrollment := range student.Enrollments {
				course, err := c.service.GetCourse(enrollment.CourseID)
				if err != nil {
					return err
				}
				fmt.Fprintf(c.out, "  %s: кредиты=%d, оценка=%d\n", course.Name, course.Credits, enrollment.Score)
			}
		}
	case "3":
		return c.setScore()
	case "4":
		student, selected, err := c.selectStudent()
		if err != nil || !selected {
			return err
		}
		result, err := c.service.GenerateTranscript(student.ID)
		if err != nil {
			return err
		}
		return printTranscript(c.out, result)
	case "5":
		return c.addCourse()
	case "6":
		c.printCourses()
	default:
		return fmt.Errorf("выберите пункт от 0 до 6")
	}
	return nil
}

func (c *console) addCourse() error {
	name, err := c.read("Название дисциплины: ")
	if err != nil {
		return err
	}
	credits, err := c.readInt("Кредиты (целое положительное число): ")
	if err != nil {
		return err
	}
	course, err := c.service.AddCourse(name, credits)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Создана дисциплина #%d: %s (%d кредитов).\n", course.ID, course.Name, course.Credits)
	return nil
}

func (c *console) printCourses() {
	courses := c.service.ListCourses()
	if len(courses) == 0 {
		fmt.Fprintln(c.out, "Дисциплин пока нет. Сначала создайте дисциплину в пункте 5.")
		return
	}
	fmt.Fprintln(c.out, "Доступные дисциплины:")
	for _, course := range courses {
		fmt.Fprintf(c.out, "#%d %s (%d кредитов)\n", course.ID, course.Name, course.Credits)
	}
}

func (c *console) selectCourse() (transcript.Course, bool, error) {
	for {
		c.printCourses()
		value, err := c.read("Номер дисциплины из списка (0 — отмена): ")
		if err != nil {
			return transcript.Course{}, false, err
		}
		if value == "0" {
			return transcript.Course{}, false, nil
		}
		id, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			course, lookupErr := c.service.GetCourse(id)
			if lookupErr == nil {
				return course, true, nil
			}
		}
		fmt.Fprintln(c.out, "Выберите существующую дисциплину по номеру из списка.")
	}
}

func (c *console) selectStudent() (transcript.Student, bool, error) {
	students := c.service.ListStudents()
	if len(students) == 0 {
		fmt.Fprintln(c.out, "Студентов пока нет. Сначала добавьте студента в пункте 1.")
		return transcript.Student{}, false, nil
	}
	for {
		fmt.Fprintln(c.out, "Список студентов:")
		for _, student := range students {
			fmt.Fprintf(c.out, "#%d %s\n", student.ID, student.Name)
		}
		value, err := c.read("ID студента (0 — отмена): ")
		if err != nil {
			return transcript.Student{}, false, err
		}
		if value == "0" {
			return transcript.Student{}, false, nil
		}
		id, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			student, lookupErr := c.service.GetStudent(id)
			if lookupErr == nil {
				return student, true, nil
			}
		}
		fmt.Fprintln(c.out, "Выберите существующего студента по номеру из списка.")
	}
}

func (c *console) setScore() error {
	if len(c.service.ListCourses()) == 0 {
		c.printCourses()
		return nil
	}
	student, selected, err := c.selectStudent()
	if err != nil || !selected {
		return err
	}
	course, selected, err := c.selectCourse()
	if err != nil || !selected {
		return err
	}
	fmt.Fprintf(c.out, "%s — %s (%d кредитов)\n", student.Name, course.Name, course.Credits)
	for {
		score, err := c.readInt("Оценка (целое число 0–100) (999 — отмена): ")

		if err != nil {
			if errors.Is(err, io.EOF) || c.scanner.Err() != nil {
				return err
			}
			fmt.Fprintln(c.out, "Введите целое число.")
			continue
		}

		if score == 999 {
			return nil
		}

		if score < 0 || score > 100 {
			fmt.Fprintln(c.out, "Оценка должна быть от 0 до 100. Повторите ввод.")
			continue
		}

		if err := c.service.SetScore(student.ID, course.ID, score); err != nil {
			return err
		}

		fmt.Fprintf(c.out, "Оценка сохранена: %s — %s: %d.\n",
			student.Name, course.Name, score)

		return nil
	}
}

func printTranscript(out io.Writer, r transcript.Transcript) error {
	fmt.Fprintf(out, "Транскрипт\nСтудент #%d: %s\n", r.StudentID, r.StudentName)
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "Дисциплина\tКредиты\tБаллы\tБуква\tGPA-балл")
	for _, row := range r.Rows {
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%.2f\n", row.Course.Name, row.Course.Credits, row.Score, row.Grade.Letter, row.Grade.Points)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "Всего кредитов: %d | Зачтено: %d | Средний балл: %.2f | GPA: %.2f / 4.00\n",
		r.TotalCredits, r.EarnedCredits, r.AverageScore, r.GPA)
	return err
}
