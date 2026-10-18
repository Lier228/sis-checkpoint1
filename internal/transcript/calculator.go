package transcript

type Calculator struct{}

// GradeFor implements the project's documented numeric-only grading scale
func GradeFor(score int) (Grade, error) {
	if err := validateScore(score); err != nil {
		return Grade{}, err
	}
	switch {
	case score >= 95:
		return Grade{"A", 4.00}, nil
	case score >= 90:
		return Grade{"A-", 3.67}, nil
	case score >= 85:
		return Grade{"B+", 3.33}, nil
	case score >= 80:
		return Grade{"B", 3.00}, nil
	case score >= 75:
		return Grade{"B-", 2.67}, nil
	case score >= 70:
		return Grade{"C+", 2.33}, nil
	case score >= 65:
		return Grade{"C", 2.00}, nil
	case score >= 60:
		return Grade{"C-", 1.67}, nil
	case score >= 55:
		return Grade{"D+", 1.33}, nil
	case score >= 50:
		return Grade{"D", 1.00}, nil
	default:
		return Grade{"F", 0.00}, nil
	}
}

func (Calculator) Process(student TranscriptInput) (Transcript, error) {
	if err := validateName(student.Name); err != nil {
		return Transcript{}, err
	}
	if err := validateCourses(student.Courses); err != nil {
		return Transcript{}, err
	}
	result := Transcript{StudentID: student.ID, StudentName: student.Name,
		Rows: make([]TranscriptRow, 0, len(student.Courses))}
	var weightedPoints, weightedScores float64
	for _, entry := range student.Courses {
		course := entry.Course
		grade, err := GradeFor(entry.Score)
		if err != nil {
			return Transcript{}, err
		}
		result.Rows = append(result.Rows, TranscriptRow{CourseScore: entry, Grade: grade})
		result.TotalCredits += course.Credits
		if entry.Score >= 50 {
			result.EarnedCredits += course.Credits
		}
		weightedPoints += grade.Points * float64(course.Credits)
		weightedScores += float64(entry.Score) * float64(course.Credits)
	}
	// round only when displaying the transcript
	result.GPA = weightedPoints / float64(result.TotalCredits)
	result.AverageScore = weightedScores / float64(result.TotalCredits)
	return result, nil
}
