package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5/pgconn"
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

type User struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type CreateUserInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserLogin struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
}

type CreateLoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Token string `json:"token"`
}

var db *sql.DB

type contextKey string

const userIDKey contextKey = "userID"

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errorResponse := ErrorResponse{Error: message}
	json.NewEncoder(w).Encode(errorResponse)
}

func capitalizeTitle(title string) string {
	titleLower := strings.ToLower(title)
	titleToRunes := []rune(titleLower)
	firstChar := strings.ToUpper(string(titleToRunes[0]))
	capTitle := firstChar + string(titleToRunes[1:])
	return capTitle
}

func GetTasks(w http.ResponseWriter, r *http.Request) {
	taskList := []Task{}
	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	taskQuery := `
	SELECT id, title, done 
	FROM tasks 
	WHERE user_id = $1 
	ORDER BY id;
	`
	taskRows, err := db.Query(taskQuery, userID)
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
	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	err := json.NewDecoder(r.Body).Decode(&taskInput)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}
	if taskInput.Title == "" {
		writeError(w, http.StatusBadRequest, "Request cannot be empty")
		return
	}

	title := capitalizeTitle(taskInput.Title)

	insertValues := `
	INSERT INTO tasks (title, done, user_id) 
	VALUES ($1, $2, $3)
	RETURNING id;
	`
	err = db.QueryRow(insertValues, title, false, userID).Scan(&task.ID)
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
	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	idIntValue, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	updateValues := `
	UPDATE tasks
	SET done = true
	WHERE id = $1
	AND user_id = $2
	RETURNING id, title, done;
	`
	err = db.QueryRow(updateValues, idIntValue, userID).Scan(&task.ID, &task.Title, &task.Done)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	} else if err != nil {
		fmt.Println("QueryRow failed:", err)
		writeError(w, http.StatusInternalServerError, "Failed to update task")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func DeleteTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	idIntValue, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	deleteValues := `
	DELETE FROM tasks
	WHERE id = $1
	AND user_id = $2
	`
	deleteRow, err := db.Exec(deleteValues, idIntValue, userID)
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

func PostUser(w http.ResponseWriter, r *http.Request) {
	var user User
	var userInput CreateUserInput
	err := json.NewDecoder(r.Body).Decode(&userInput)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	if userInput.Email == "" {
		writeError(w, http.StatusBadRequest, "Email cannot be empty")
		return
	}

	userEmail, err := mail.ParseAddress(userInput.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Email must be a plain address, like name@example.com")
		return
	}

	if userInput.Email != userEmail.Address {
		writeError(w, http.StatusBadRequest, "Email must be a plain address, like name@example.com")
		return
	}

	if userInput.Password == "" {
		writeError(w, http.StatusBadRequest, "Password cannot be empty")
		return
	}

	const passwordMinLen = 8
	const passwordMaxLen = 72
	if len(userInput.Password) < passwordMinLen {
		writeError(w, http.StatusBadRequest, "Password must be at least 8 bytes long!")
		return
	} else if len(userInput.Password) > passwordMaxLen {
		writeError(w, http.StatusBadRequest, "Password must be at most 72 bytes!")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userInput.Password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("error generating password hash: ", err)
		writeError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	insertValues := `
	INSERT INTO users(email, password_hash)
	VALUES($1, $2)
	RETURNING id, email
	`
	err = db.QueryRow(insertValues, userInput.Email, hashedPassword).Scan(&user.ID, &user.Email)
	var dbErr *pgconn.PgError
	if errors.As(err, &dbErr) && dbErr.Code == "23505" {
		writeError(w, http.StatusConflict, "Email already registered")
		return
	} else if err != nil {
		fmt.Println("QueryRow/Scan failed:", err)
		writeError(w, http.StatusInternalServerError, "Failed to add user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func PostLogin(w http.ResponseWriter, r *http.Request) {
	var userLogin UserLogin
	var loginInput CreateLoginInput
	var loginResponse LoginResponse
	err := json.NewDecoder(r.Body).Decode(&loginInput)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	if loginInput.Email == "" {
		writeError(w, http.StatusBadRequest, "Email cannot be empty")
		return
	}

	if loginInput.Password == "" {
		writeError(w, http.StatusBadRequest, "Password cannot be empty")
		return
	}

	userRow := `
	SELECT id, email, password_hash 
	FROM users 
	WHERE email = $1
	`
	err = db.QueryRow(userRow, loginInput.Email).Scan(&userLogin.ID, &userLogin.Email, &userLogin.PasswordHash)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	} else if err != nil {
		fmt.Println("QueryRow err: ", err)
		writeError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(userLogin.PasswordHash), []byte(loginInput.Password))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	secretKey := os.Getenv("SECRET_KEY")
	expiryDate := jwt.NewNumericDate(time.Now().Add(30 * time.Minute))
	claims := jwt.MapClaims{"user_id": userLogin.ID, "exp": expiryDate}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secretKey))
	if err != nil {
		fmt.Println("error while creating token: ", err)
		writeError(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	loginResponse = LoginResponse{ID: userLogin.ID, Email: userLogin.Email, Token: token}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(loginResponse)
}

func RequireAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "Invalid or missing token")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
			return []byte(os.Getenv("SECRET_KEY")), nil
		},
			jwt.WithValidMethods([]string{"HS256"}))

		if errors.Is(err, jwt.ErrTokenExpired) {
			writeError(w, http.StatusUnauthorized, "Token expired, please log in again")
			return
		} else if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid or missing token")
			return
		}

		if !token.Valid {
			writeError(w, http.StatusUnauthorized, "Invalid or missing token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Invalid token claims")
			return
		}

		claimsValue, ok := claims["user_id"].(float64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Invalid user ID claim")
			return
		}

		claimsValueToInt := int(claimsValue)

		ctx := context.WithValue(r.Context(), userIDKey, claimsValueToInt)
		r = r.WithContext(ctx)
		handler(w, r)
	}
}

func main() {
	var err error
	urlEncUser := url.QueryEscape(os.Getenv("DB_USER"))
	urlEncPassword := url.QueryEscape(os.Getenv("DB_PASSWORD"))
	urlEncHost := url.QueryEscape(os.Getenv("DB_HOST"))
	urlPort := os.Getenv("DB_PORT")
	urlName := os.Getenv("DB_NAME")

	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		panic("SECRET_KEY environment variable is not set")
	}
	db, err = sql.Open("pgx", "postgres://"+urlEncUser+":"+urlEncPassword+"@"+urlEncHost+":"+urlPort+"/"+urlName)
	if err != nil {
		fmt.Println("Invalid database configuration")
		return
	}

	err = db.Ping()
	if err != nil {
		fmt.Println("Unable to connect to database:", err)
		return
	}

	http.HandleFunc("GET /tasks", RequireAuth(GetTasks))
	http.HandleFunc("POST /tasks", RequireAuth(PostTask))
	http.HandleFunc("PATCH /tasks/{id}", RequireAuth(PatchTask))
	http.HandleFunc("DELETE /tasks/{id}", RequireAuth(DeleteTask))
	http.HandleFunc("POST /user", PostUser)
	http.HandleFunc("POST /login", PostLogin)

	fmt.Println("server is running")
	http.ListenAndServe(":8080", nil)
}
