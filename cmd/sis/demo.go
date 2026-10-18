package main

import (
	"fmt"
	"io"

	"sis-checkpoint1/internal/transcript"
)

func demo(service *transcript.Service, out io.Writer) error {
	goCourse, err := service.AddCourse("Golang", 5)
	if err != nil {
		return err
	}
	mathCourse, err := service.AddCourse("Математика", 3)
	if err != nil {
		return err
	}
	courses := []transcript.Course{goCourse, mathCourse}
	samples := []struct {
		name   string
		scores []int
	}{
		{"Аружан", []int{95, 85}},
		{"Диас", []int{80, 40}},
	}
	for _, sample := range samples {
		student, err := service.AddStudent(sample.name)
		if err != nil {
			return err
		}
		for i, course := range courses {
			if err := service.SetScore(student.ID, course.ID, sample.scores[i]); err != nil {
				return err
			}
		}
		result, err := service.GenerateTranscript(student.ID)
		if err != nil {
			return err
		}
		if err := printTranscript(out, result); err != nil {
			return err
		}
		fmt.Fprintln(out)
	}
	return nil
}
