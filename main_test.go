package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

func reqContext(req *http.Request, userID int) *http.Request {
	ctx := context.WithValue(req.Context(), userIDKey, userID)
	req = req.WithContext(ctx)
	return req
}

func TestCapitalizeTitle_All(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectPanic bool
	}{
		{name: "basic", input: "tester", expected: "Tester", expectPanic: false},
		{name: "empty string", input: "", expected: "", expectPanic: true},
		{name: "single string", input: "a", expected: "A", expectPanic: false},
		{name: "mixed strings", input: "dRAcuLA", expected: "Dracula", expectPanic: false},
		{name: "cap strings", input: "DONARUMMA", expected: "Donarumma", expectPanic: false},
		{name: "emojis", input: "🔥🔥", expected: "🔥🔥", expectPanic: false},
		{name: "accented char", input: "école", expected: "École", expectPanic: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.expectPanic == true {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("Expected PANIC but that didn't happen.")
					}
				}()
				capitalizeTitle(test.input)
			} else {
				result := capitalizeTitle(test.input)
				if result != test.expected {
					t.Errorf("capitalizeTitle(%v) = %v; expected: %v", test.input, result, test.expected)
				}
			}
		})
	}
}

func TestGetTasks_Success(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	rows := sqlmock.NewRows([]string{"id", "title", "done"}).
		AddRow(1, "Buy milk", false).
		AddRow(2, "Walk dog", true)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, done FROM tasks WHERE user_id = $1 ORDER BY id")).
		WithArgs(1).
		WillReturnRows(rows)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = reqContext(req, 1)

	GetTasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected: %v but got: %v instead", http.StatusOK, rec.Code)

	}

	if !strings.Contains(rec.Body.String(), "Buy milk") {
		t.Errorf("expected response body to contain %q, got: %s", "Buy milk", rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "Walk dog") {
		t.Errorf("expected response body to contain %q, got: %s", "Walk dog", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestGetTasks_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, done FROM tasks WHERE user_id = $1 ORDER BY id")).
		WithArgs(1).
		WillReturnError(errors.New("connection lost"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = reqContext(req, 1)

	GetTasks(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected %v but got: %v instead", http.StatusInternalServerError, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Failed to retrieve tasks") {
		t.Errorf("Expected response body to contain %v, got: %v instead", "Failed to retrieve tasks", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestGetTasks_Scan(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	rows := sqlmock.NewRows([]string{"id", "title", "done"}).
		AddRow("not-a-number", "Buy milk", false)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, done FROM tasks WHERE user_id = $1 ORDER BY id")).
		WithArgs(1).
		WillReturnRows(rows)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = reqContext(req, 1)

	GetTasks(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected %v but got: %v instead", http.StatusInternalServerError, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Failed to retrieve tasks") {
		t.Errorf("Expected response body to contain %v, got: %v instead", "Failed to retrieve tasks", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestGetTasks_RowsErr(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	rows := sqlmock.NewRows([]string{"id", "title", "done"}).
		AddRow(1, "Buy milk", false).
		AddRow(2, "Walk dog", true).
		RowError(1, errors.New("connection dropped"))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, done FROM tasks WHERE user_id = $1 ORDER BY id")).
		WithArgs(1).
		WillReturnRows(rows)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = reqContext(req, 1)

	GetTasks(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected %v but got: %v instead", http.StatusInternalServerError, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Failed to retrieve tasks") {
		t.Errorf("Expected response body to contain %v, got: %v instead", "Failed to retrieve tasks", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestGetTasks_NoTasks(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	rows := sqlmock.NewRows([]string{"id", "title", "done"})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, done FROM tasks WHERE user_id = $1 ORDER BY id")).
		WithArgs(1).
		WillReturnRows(rows)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = reqContext(req, 1)

	GetTasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected: %v but got: %v instead", http.StatusOK, rec.Code)

	}

	expected := "[]\n"
	if actual := rec.Body.String(); actual != expected {
		t.Errorf("expected body %v, got %v", expected, actual)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestPostTask_Success(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	row := sqlmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO tasks (title, done, user_id) VALUES ($1, $2, $3) RETURNING id")).
		WithArgs("Buy milk", false, 1).
		WillReturnRows(row)

	body := strings.NewReader(`{"title":"buy milk"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks", body)
	req = reqContext(req, 1)

	PostTask(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected %v, got: %v instead", http.StatusCreated, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Buy milk") {
		t.Errorf("expected body to contain %v, got: %v instead", "Buy milk", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestPostTask_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{"title":`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks", body)
	req = reqContext(req, 1)

	PostTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got: %v instead", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Invalid JSON in request body") {
		t.Errorf("expected %v, got: %v", "Invalid JSON in request body", rec.Body.String())
	}
}

func TestPostTask_EmptyInput(t *testing.T) {
	body := strings.NewReader(`{"title":""}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks", body)
	req = reqContext(req, 1)

	PostTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got: %v instead", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Request cannot be empty") {
		t.Errorf("expected %v, got: %v", "Request cannot be empty", rec.Body.String())
	}
}

func TestPatchTask_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/tasks/5", nil)
	req.SetPathValue("id", "5")

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	row := sqlmock.NewRows([]string{"id", "title", "done"}).
		AddRow(5, "Buy milk", true)

	mock.ExpectQuery(regexp.QuoteMeta("UPDATE tasks SET done = true WHERE id = $1 AND user_id = $2 RETURNING id, title, done")).
		WithArgs(5, 1).
		WillReturnRows(row)

	rec := httptest.NewRecorder()
	req = reqContext(req, 1)

	PatchTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %v, got: %v instead", http.StatusOK, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Buy milk") {
		t.Errorf("expected body to contain %v, got: %v instead", "Buy milk", rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "true") {
		t.Errorf("expected body to contain %v, got: %v instead", "true", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestPatchTask_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/tasks/abc", nil)
	req.SetPathValue("id", "abc")

	rec := httptest.NewRecorder()
	req = reqContext(req, 1)

	PatchTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got: %v instead", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Invalid task ID") {
		t.Errorf("expected body to contain %v, got: %v instead", "Invalid task ID", rec.Body.String())
	}
}

func TestPatchTask_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/tasks/999", nil)
	req.SetPathValue("id", "999")

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	mock.ExpectQuery(regexp.QuoteMeta("UPDATE tasks SET done = true WHERE id = $1 AND user_id = $2 RETURNING id, title, done")).
		WithArgs(999, 1).
		WillReturnError(sql.ErrNoRows)

	rec := httptest.NewRecorder()
	req = reqContext(req, 1)

	PatchTask(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %v, got: %v instead", http.StatusNotFound, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Task not found") {
		t.Errorf("expected body to contain %v, got: %v instead", "Task not found", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestDeleteTask_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/tasks/5", nil)
	req.SetPathValue("id", "5")

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM tasks WHERE id = $1 AND user_id = $2")).
		WithArgs(5, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	rec := httptest.NewRecorder()
	req = reqContext(req, 1)

	DeleteTask(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected %v, got: %v instead", http.StatusNoContent, rec.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestDeleteTask_InvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/tasks/abc", nil)
	req.SetPathValue("id", "abc")

	rec := httptest.NewRecorder()
	req = reqContext(req, 1)

	DeleteTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got: %v instead", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Invalid task ID") {
		t.Errorf("expected body to contain %v, got: %v instead", "Invalid task ID", rec.Body.String())
	}
}

func TestDeleteTask_RowsAffected(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/tasks/999", nil)
	req.SetPathValue("id", "999")

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM tasks WHERE id = $1 AND user_id = $2")).
		WithArgs(999, 1).
		WillReturnResult(sqlmock.NewResult(0, 0))

	rec := httptest.NewRecorder()
	req = reqContext(req, 1)

	DeleteTask(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %v, got: %v instead", http.StatusNotFound, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Task not found") {
		t.Errorf("expected body to contain %v, got: %v instead", "Task not found", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestPostUser_Success(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	row := sqlmock.NewRows([]string{"id", "email"}).
		AddRow(1, "testingemail@gmail.com")

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("testingemail@gmail.com", sqlmock.AnyArg()).
		WillReturnRows(row)

	body := strings.NewReader(`{"email":"testingemail@gmail.com", "password":"anYpASsworD2"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/user", body)

	PostUser(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected %v, got: %v instead", http.StatusCreated, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "testingemail@gmail.com") {
		t.Errorf("expected body to contain %v, got: %v instead", "testingemail@gmail.com", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestPostUser_DuplicateEmail(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	pgErr := &pgconn.PgError{Code: "23505"}
	mock.ExpectQuery("INSERT INTO users").
		WithArgs("testingemail@gmail.com", sqlmock.AnyArg()).
		WillReturnError(pgErr)

	body := strings.NewReader(`{"email":"testingemail@gmail.com", "password":"anYpASsworD2"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/user", body)

	PostUser(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected %v, got: %v instead", http.StatusConflict, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Email already registered") {
		t.Errorf("expected body to contain %v, got: %v instead", "Email already registered", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestPostUser_InvalidEmail(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		status  int
		message string
	}{
		{"not an email", `{"email":"not-an-email", "password":"123456"}`, http.StatusBadRequest, "Email must be a plain address, like name@example.com"},
		{"display name with brackets", `{"email":"tester <testing1@gmail.com>", "password":"123456"}`, http.StatusBadRequest, "Email must be a plain address, like name@example.com"},
		{"angle brackets without name", `{"email":"<testing2@gmail.com>", "password":"123456"}`, http.StatusBadRequest, "Email must be a plain address, like name@example.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := strings.NewReader(test.body)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/user", body)

			PostUser(rec, req)

			if rec.Code != test.status {
				t.Errorf("expected %v, got %v", test.status, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), test.message) {
				t.Errorf("expected body %v, got %v", test.message, rec.Body.String())
			}
		})
	}
}

func TestPostUser_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{"email":, "password":"anYpASsworD2"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/user", body)

	PostUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got: %v instead", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Invalid JSON in request body") {
		t.Errorf("expected body to contain %v, got: %v instead", "Invalid JSON in request body", rec.Body.String())
	}
}

func TestPostUser_EmptyEmailAndPassword(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantMsg    string
	}{
		{"empty email", `{"email":"", "password": "AnYpAsSWOrD22"}`, http.StatusBadRequest, "Email cannot be empty"},
		{"empty password", `{"email":"testingemail@gmail.com", "password":""}`, http.StatusBadRequest, "Password cannot be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := strings.NewReader(test.body)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/user", body)

			PostUser(rec, req)

			if rec.Code != test.wantStatus {
				t.Errorf("expected %v, got: %v instead", test.wantStatus, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), test.wantMsg) {
				t.Errorf("expected body to contain %v, got: %v instead", test.wantMsg, rec.Body.String())
			}
		})
	}
}

func TestPostUser_InvalidPassword(t *testing.T) {
	password1 := strings.Repeat("s", 73)
	password2 := strings.Repeat("qa", 300)
	password3 := strings.Repeat("🔥", 19)
	password4 := strings.Repeat("b", 264)

	makeBody := func(email, password string) string {
		body, err := json.Marshal(struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}{
			Email:    email,
			Password: password,
		})

		if err != nil {
			t.Fatal(err)
		}

		return string(body)
	}

	tests := []struct {
		name    string
		body    string
		status  int
		message string
	}{
		{"too short", makeBody("testing1@gmail.com", "123456"), http.StatusBadRequest, "Password must be at least 8 bytes long!"},
		{"too long", makeBody("testing1@gmail.com", password1), http.StatusBadRequest, "Password must be at most 72 bytes!"},
		{"way too long", makeBody("testing1@gmail.com", password2), http.StatusBadRequest, "Password must be at most 72 bytes!"},
		{"Unicode password exceeds byte limit", makeBody("testing1@gmail.com", password3), http.StatusBadRequest, "Password must be at most 72 bytes!"},
		{"264-byte password", makeBody("testing1@gmail.com", password4), http.StatusBadRequest, "Password must be at most 72 bytes!"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := strings.NewReader(test.body)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/user", body)

			PostUser(rec, req)

			if rec.Code != test.status {
				t.Errorf("expected %v, got %v", test.status, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), test.message) {
				t.Errorf("expected body %v, got %v", test.message, rec.Body.String())
			}
		})
	}
}

func TestPostUser_ValidPassword(t *testing.T) {
	password1 := strings.Repeat("a", 8)
	password2 := strings.Repeat("b", 72)

	makeBody := func(email, password string) string {
		body, err := json.Marshal(struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}{
			Email:    email,
			Password: password,
		})

		if err != nil {
			t.Fatal(err)
		}

		return string(body)
	}

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"8-bytes password", makeBody("testingmail@gmail.com", password1), http.StatusCreated},
		{"72-bytes password", makeBody("testingmail@gmail.com", password2), http.StatusCreated},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			mockDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create Mock DB %v", err)
			}
			defer mockDB.Close()

			db = mockDB

			row := sqlmock.NewRows([]string{"id", "email"}).
				AddRow(1, "testingmail@gmail.com")
			mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users(email, password_hash) VALUES($1, $2) RETURNING id, email")).
				WithArgs("testingmail@gmail.com", sqlmock.AnyArg()).
				WillReturnRows(row)

			body := strings.NewReader(test.body)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/user", body)

			PostUser(rec, req)

			if rec.Code != test.status {
				t.Errorf("expected %v, got: %v instead", test.status, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), "testingmail@gmail.com") {
				t.Errorf("expected body to have %v, got %v instead", "testingmail@gmail.com", rec.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet mock expectations: %v", err)
			}
		})
	}
}

func TestPostLogin_Success(t *testing.T) {
	t.Setenv("SECRET_KEY", "secretKey2")
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	passHash, err := bcrypt.GenerateFromPassword([]byte("AnYpAsSworD2"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate hash: %v", err)
	}
	row := sqlmock.NewRows([]string{"id", "email", "password_hash"}).
		AddRow(1, "testingmail@gmail.com", passHash)

	mock.ExpectQuery("SELECT id, email, password_hash").
		WithArgs("testingmail@gmail.com").
		WillReturnRows(row)

	body := strings.NewReader(`{"email":"testingmail@gmail.com", "password":"AnYpAsSworD2"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", body)

	PostLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %v, got: %v instead", http.StatusOK, rec.Code)
	}

	var loginResp LoginResponse
	err = json.NewDecoder(rec.Body).Decode(&loginResp)

	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if loginResp.Token == "" {
		t.Errorf("%v cannot be empty", loginResp.Token)
	}

	parts := strings.Split(loginResp.Token, ".")
	if len(parts) != 3 {
		t.Errorf("expected token to have 3 parts, got %v:", len(parts))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations %v:", err)
	}
}

func TestPostLogin_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{"email":, "password":'}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", body)

	PostLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got: %v instead", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Invalid JSON in request body") {
		t.Errorf("expected body to contain %v, got: %v instead", "Invalid JSON in request body", rec.Body.String())
	}
}

func TestPostLogin_EmptyEmailAndPassword(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		status  int
		message string
	}{
		{"empty email", `{"email":"", "password":"wHATEveR2"}`, http.StatusBadRequest, "Email cannot be empty"},
		{"empty password", `{"email":"testingmail@gmail.com", "password":""}`, http.StatusBadRequest, "Password cannot be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := strings.NewReader(test.body)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/login", body)

			PostLogin(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected %v, got: %v instead", http.StatusBadRequest, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), test.message) {
				t.Errorf("expected body to contain %v, got: %v instead", test.message, rec.Body.String())
			}
		})
	}
}

func TestPostLogin_InvalidCredentials(t *testing.T) {

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("hellOwOrLD21"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate hash: %v", err)
	}

	tests := []struct {
		name      string
		body      string
		status    int
		message   string
		setupMock func(mock sqlmock.Sqlmock)
	}{
		{
			name:    "not found",
			body:    `{"email":"testingmail@gmail.com", "password":"AnYThiNGGOES12"}`,
			status:  http.StatusUnauthorized,
			message: "Invalid email or password",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, email, password_hash").
					WithArgs("testingmail@gmail.com").
					WillReturnError(sql.ErrNoRows)
			},
		},

		{
			name:    "wrong password",
			body:    `{"email":"testingmail@gmail.com", "password":"AnYThiNGGOES12"}`,
			status:  http.StatusUnauthorized,
			message: "Invalid email or password",
			setupMock: func(mock sqlmock.Sqlmock) {
				row := sqlmock.NewRows([]string{"id", "email", "password_hash"}).
					AddRow(1, "testingmail@gmail.com", hashedPassword)
				mock.ExpectQuery("SELECT id, email, password_hash").
					WithArgs("testingmail@gmail.com").
					WillReturnRows(row)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock DB %v: ", err)
			}
			defer mockDB.Close()
			db = mockDB
			test.setupMock(mock)
			body := strings.NewReader(test.body)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/login", body)

			PostLogin(rec, req)

			if rec.Code != test.status {
				t.Errorf("expected %v, got: %v instead", http.StatusUnauthorized, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), test.message) {
				t.Errorf("expected body to contain %v, got: %v instead", test.message, rec.Body.String())
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet mock expectations: %v", err)
			}
		})
	}
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)

	called := false
	dummyHandler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	protectedHandler := RequireAuth(dummyHandler)

	protectedHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %v, got %v", http.StatusUnauthorized, rec.Code)
	}

	if called {
		t.Error("expected dummyHandler to not be called")
	}
}

func TestRequireAuth_InvalidTokenAndExpiredToken(t *testing.T) {
	t.Setenv("SECRET_KEY", "DireCTIONAl222")
	expiry := jwt.NewNumericDate(time.Now().Add(-35 * time.Minute))
	claim := jwt.MapClaims{"user_id": 1, "exp": expiry}
	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claim).SignedString([]byte("DireCTIONAl222"))
	if err != nil {
		t.Fatalf("couldn't create token: %v", err)
	}

	tests := []struct {
		name    string
		token   string
		status  int
		message string
	}{
		{
			name:    "expired token",
			token:   tokenStr,
			status:  http.StatusUnauthorized,
			message: "Token expired, please log in again",
		},
		{
			name:    "invalid token",
			token:   "this-is-not-a-valid-jwt",
			status:  http.StatusUnauthorized,
			message: "Invalid or missing token",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/tasks", nil)

			req.Header.Set("Authorization", "Bearer "+test.token)

			fakeHandler := func(w http.ResponseWriter, r *http.Request) {
				fmt.Println("this shouldn't print")
				w.WriteHeader(http.StatusOK)
			}

			protectedHandler := RequireAuth(fakeHandler)

			protectedHandler(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected %v, got %v instead", http.StatusUnauthorized, rec.Code)
			}

			if !strings.Contains(rec.Body.String(), test.message) {
				t.Errorf("expected message to be %v, got %v", test.message, rec.Body.String())
			}
		})
	}
}

func TestRequireAuth_Success(t *testing.T) {
	t.Setenv("SECRET_KEY", "DireCTIONAl222")
	expiry := jwt.NewNumericDate(time.Now().Add(10 * time.Minute))
	claim := jwt.MapClaims{"user_id": 1, "exp": expiry}
	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claim).SignedString([]byte("DireCTIONAl222"))
	if err != nil {
		t.Fatalf("couldn't create token: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)

	req.Header.Set("Authorization", "Bearer "+tokenStr)

	called := false
	fakeHandler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	protectedHandler := RequireAuth(fakeHandler)
	protectedHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rec.Code)
	}

	if !called {
		t.Errorf("expected handler to be called")
	}
}
