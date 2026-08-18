package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type CreateTaskInput struct {
	Title string `json:"title"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

var db *sql.DB

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errorResponse := ErrorResponse{Error: message}
	json.NewEncoder(w).Encode(errorResponse)
}

func GetTasks(w http.ResponseWriter, r *http.Request) {
	var taskList []Task
	taskRows, err := db.Query("SELECT id, title, done FROM tasks ORDER BY id")
	if err != nil {
		fmt.Println("Query failed:", err)
		writeError(w, http.StatusInternalServerError, "Failed to retrieve tasks")
		return
	}
	defer taskRows.Close()
	for taskRows.Next() {
		var task Task
		err := taskRows.Scan(&task.ID, &task.Title, &task.Done)
		if err != nil {
			fmt.Println("Scan failed:", err)
			writeError(w, http.StatusInternalServerError, "Failed to retrieve tasks")
			return
		}
		taskList = append(taskList, task)
	}

	err = taskRows.Err()
	if err != nil {
		fmt.Println("An error occured:", err)
		writeError(w, http.StatusInternalServerError, "Failed to retrieve tasks")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(taskList)
}

func PostTask(w http.ResponseWriter, r *http.Request) {
	var taskInput CreateTaskInput
	var task Task
	err := json.NewDecoder(r.Body).Decode(&taskInput)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}
	if taskInput.Title == "" {
		writeError(w, http.StatusBadRequest, "Request cannot be empty")
		return
	}

	titleLower := strings.ToLower(taskInput.Title)
	firstChar := strings.ToUpper(string(titleLower[0]))
	title := firstChar + titleLower[1:]

	insertValues := `
	INSERT INTO tasks (title, done) 
	VALUES ($1, $2)
	RETURNING id
	`
	err = db.QueryRow(insertValues, title, false).Scan(&task.ID)
	if err != nil {
		fmt.Println("QueryRow/Scan failed:", err)
		writeError(w, http.StatusInternalServerError, "Failed to add task")
		return
	}
	
	task = Task{ID: task.ID, Title: title, Done: false}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func PatchTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	idIntValue, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	updateValues := `
	UPDATE tasks
	SET done = true
	WHERE id = $1
	RETURNING id, title, done
	`
	err = db.QueryRow(updateValues, idIntValue).Scan(&task.ID, &task.Title, &task.Done)
	if err == sql.ErrNoRows {
		fmt.Println(err)
		writeError(w, http.StatusNotFound, "Task not found")
		return
	} else if err != nil {
		fmt.Println("QueryRow failed", err)
		writeError(w, http.StatusInternalServerError, "Failed to update task")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func DeleteTask(w http.ResponseWriter, r *http.Request) {
	idIntValue, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	deleteValues := `
	DELETE FROM tasks
	WHERE id = $1
	`
	deleteRow, err := db.Exec(deleteValues, idIntValue)
	if err != nil {
		fmt.Println("Exec failed:", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete task")
		return
	}

	rowsAffected, err := deleteRow.RowsAffected()
	if err != nil {
		fmt.Println("RowsAffected failed:", err)
		writeError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func main() {
	var err error
	urlEncUser := url.QueryEscape(os.Getenv("DB_USER"))
	urlEncPassword := url.QueryEscape(os.Getenv("DB_PASSWORD"))
	urlEncHost := url.QueryEscape(os.Getenv("DB_HOST"))
	urlPort := os.Getenv("DB_PORT")
	urlName := os.Getenv("DB_NAME")
	db, err = sql.Open("pgx", "postgres://" + urlEncUser + ":" + urlEncPassword + "@" + urlEncHost + ":" + urlPort + "/" + urlName)
	if err != nil {
		fmt.Println("Invalid database configuration")
		return
	}

	err = db.Ping()
	if err != nil {
		fmt.Println("Unable to connect to database:", err)
		return
	}

	http.HandleFunc("GET /tasks", GetTasks)
	http.HandleFunc("POST /tasks", PostTask)
	http.HandleFunc("PATCH /tasks/{id}", PatchTask)
	http.HandleFunc("DELETE /tasks/{id}", DeleteTask)

	fmt.Println("server is running")
	http.ListenAndServe(":8080", nil)
}
