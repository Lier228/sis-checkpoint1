# Student Transcript

A Go console application for SIS Checkpoint 1. It maintains a shared course catalog and student grades, then processes transcript tasks to calculate letter grades, credit totals, a weighted average score, and GPA on a 4.0 scale.

All processing is synchronous. Students, tasks, and results are stored in memory and are lost when the application exits.

## Requirements

- Go 1.27
- No external dependencies

## Run

Run these commands from the project directory containing `go.mod`:

```sh
go run ./cmd/sis
```

To run an example with two students:

```sh
go run ./cmd/sis --demo
```

The console menu is in Russian:

| Option | Action |
|---|---|
| 1 | Add a student |
| 2 | List students and their grades |
| 3 | Assign or correct a grade |
| 4 | Generate a transcript |
| 5 | Create a course in the shared catalog |
| 6 | List available courses |
| 0 | Exit |

Create courses once using option 5, providing a name and credit count. Add a student, then use option 3 to select the student and an existing course by its displayed ID and enter a score. Invalid student or course selections redisplay the corresponding list; enter 0 to cancel selection. Enter 999 to cancel score entry. Grading never creates courses or asks for a course name or credits. Selecting an already assigned course updates that student's score.

Use option 4 to select a student from the displayed list and calculate, store, and display the transcript immediately. Task creation and processing happen automatically; no task ID or confirmation is required.

## Input Rules

- Student and course names must be non-empty UTF-8 text without control characters.
- Scores must be integers from 0 to 100.
- Credits must be positive integers; their total must fit in an `int`.
- Course names must be unique in the catalog, ignoring case and surrounding whitespace.
- Grades can reference only existing students and catalog courses. Each student has at most one grade per course.
- A transcript task requires at least one graded course.

Invalid input, unknown IDs, and invalid task operations return errors.

## Grading and Calculations

| Score | Letter | Grade points |
|---|---|---|
| 95-100 | A | 4.00 |
| 90-94 | A- | 3.67 |
| 85-89 | B+ | 3.33 |
| 80-84 | B | 3.00 |
| 75-79 | B- | 2.67 |
| 70-74 | C+ | 2.33 |
| 65-69 | C | 2.00 |
| 60-64 | C- | 1.67 |
| 55-59 | D+ | 1.33 |
| 50-54 | D | 1.00 |
| 0-49 | F | 0.00 |

```text
GPA = sum(grade points * course credits) / total credits
Average score = sum(score * course credits) / total credits
Earned credits = sum(credits for courses with a score of at least 50)
```

Failed courses count toward total credits and the GPA denominator. Results are rounded to two decimal places only when displayed. This project uses a simplified grading scale without retakes, FX, or other special grades.

For example, scores of 95 in a 5-credit course and 85 in a 3-credit course produce a weighted average of 91.25 and a GPA of 3.75 when displayed.

## Task Lifecycle

In this project, a task is one transcript calculation. Each task has a unique ID, an input snapshot, a state, and a stored result. These are required by the [Checkpoint 1 specification](https://github.com/salikhP/GOLANG-FALL-2026-KBTU/blob/main/sis/SIS_CP1_SPECIFICATION.md).

```text
pending -> processing -> completed
                      -> failed
```

Only pending tasks may be processed. Successful processing stores the transcript; a calculator error stores the failure message. Reprocessing an active or finished task is rejected. Generating again creates a new task using current grades, preserving earlier results.

## Project Structure

```text
cmd/sis/
  main.go             Program entry point
  console.go          Menu, selection, input, and transcript output
  demo.go             Sample students and grades
  main_test.go        Console workflow tests
internal/transcript/
  model.go            Domain types, validation, and errors
  calculator.go       Grade conversion and transcript calculations
  calculator_test.go  Grading and calculation tests
  service.go          Student records, course catalog, and transcript generation
  service_test.go     Task operations, states, and data isolation tests
go.mod
README.md
```

The console handles input and output. `Service.GenerateTranscript` owns the complete generation operation, including task creation and processing. The console does not manage task states.

`Service` stores students, catalog courses, and tasks in separate maps. A `Course` contains its ID, name, and credits. A student's `Enrollment` contains only the course ID and that student's score. Task inputs resolve these references into independent snapshots. The service uses the `Processor` interface for calculation, implemented by `Calculator`. Returned slices are copied to prevent callers from modifying stored records.

## Verification

```sh
gofmt -l .
go build ./...
go vet ./...
go test ./...
```

`gofmt -l .` should produce no output. Tests cover task creation, invalid input, retrieval, missing IDs, processing, result correctness, task states, invalid operations, and multiple independent tasks. Additional tests check catalog duplicates, invalid course selection, score corrections, and independent grades for a shared course. Table-driven tests check grade boundaries and validation rules.
