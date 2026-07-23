package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"sync"
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

var tasks = make(map[int]Task)
var counter int
var mu sync.Mutex

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errorResponse := ErrorResponse{Error: message}
	json.NewEncoder(w).Encode(errorResponse)
}

func GetTasks(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	taskList := []Task{}
	for _, task := range tasks {
		taskList = append(taskList, task)
	}
	slices.SortFunc(taskList, func(a, b Task) int  {
		return cmp.Compare(a.ID, b.ID)
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(taskList)
}

func PostTask(w http.ResponseWriter, r *http.Request) {
	var taskInput CreateTaskInput
	err := json.NewDecoder(r.Body).Decode(&taskInput)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}
	if taskInput.Title == "" {
		writeError(w, http.StatusBadRequest, "Request cannot be empty")
		return
	}
	mu.Lock()
	defer mu.Unlock()
	counter++
	task := Task{ID: counter, Title: taskInput.Title, Done: false}
	tasks[counter] = task
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func PatchTask(w http.ResponseWriter, r *http.Request) {
	idIntValue, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Enter a valid task ID")
		return
	}
	mu.Lock()
	defer mu.Unlock()
	task, ok := tasks[idIntValue]
	if ok {
		task.Done = true
		tasks[idIntValue] = task
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(task)

	} else {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
}

func DeleteTask(w http.ResponseWriter, r *http.Request) {
	idIntValue, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Enter a valid task ID")
		return
	}
	mu.Lock()
	defer mu.Unlock()
	_, ok := tasks[idIntValue]
	if ok {
		delete(tasks, idIntValue)
		w.WriteHeader(http.StatusNoContent)
	} else {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
}

func main() {
	http.HandleFunc("GET /tasks", GetTasks)
	http.HandleFunc("POST /tasks", PostTask)
	http.HandleFunc("PATCH /tasks/{id}", PatchTask)
	http.HandleFunc("DELETE /tasks/{id}", DeleteTask)

	fmt.Println("server is running")
	http.ListenAndServe(":8080", nil)
}
