# Task Manager API

A simple REST API for managing tasks, built in Go using the standard library's `net/http` package and PostgreSQL for persistent data storage.

The API supports creating, listing, completing, and deleting tasks.

## Requirements

- Go 1.22 or later
- PostgreSQL
- PostgreSQL driver: `github.com/jackc/pgx/v5`

## Database Setup

Create a PostgreSQL database for the application.

Then create the `tasks` table:

```sql
CREATE TABLE tasks (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL
);
```

## Database Configuration

The application reads its PostgreSQL connection details from environment variables instead of storing them directly in the source code.

The following environment variables are required:

| Variable | Description |
|---|---|
| `DB_USER` | PostgreSQL username |
| `DB_PASSWORD` | PostgreSQL password |
| `DB_HOST` | PostgreSQL host |
| `DB_PORT` | PostgreSQL port |
| `DB_NAME` | PostgreSQL database name |

### Windows / PowerShell

Set the variables before starting the server:

```powershell
$env:DB_USER="postgres"
$env:DB_PASSWORD="your_password"
$env:DB_HOST="localhost"
$env:DB_PORT="5432"
$env:DB_NAME="task_manager"
```

Replace the example values with your own PostgreSQL configuration.

> **Important:** Do not commit your actual database password or other credentials to GitHub.

## Installing Dependencies

From the project directory:

```powershell
go mod tidy
```

This installs the dependencies declared by the project, including the PostgreSQL driver.

## Running the Server

Make sure PostgreSQL is running and the required environment variables have been configured.

Then run:

```powershell
go run main.go
```

The server starts on:

```text
http://localhost:8080
```

## API Endpoints

### `GET /tasks`

Returns all tasks, sorted by ID.

**Response:** `200 OK`

**Example response:**

```json
[
  {
    "id": 1,
    "title": "Update my gaming console",
    "done": false
  }
]
```

### `POST /tasks`

Creates a new task.

**Request body:**

```json
{
  "title": "update my gaming console"
}
```

**Response:** `201 Created`

**Example response:**

```json
{
  "id": 1,
  "title": "Update my gaming console",
  "done": false
}
```

**Errors:**

- `400 Bad Request` — invalid JSON in request body
- `400 Bad Request` — request cannot be empty
- `500 Internal Server Error` — failed to add task

The title is normalized so that the first character is uppercase and the remaining characters are lowercase.

### `PATCH /tasks/{id}`

Marks a task as complete.

No request body is required.

**Response:** `200 OK`

**Example response:**

```json
{
  "id": 1,
  "title": "Update my gaming console",
  "done": true
}
```

**Errors:**

- `400 Bad Request` — invalid task ID
- `404 Not Found` — task not found
- `500 Internal Server Error` — failed to update task

### `DELETE /tasks/{id}`

Deletes a task.

No request body is required.

**Response:** `204 No Content`

**Errors:**

- `400 Bad Request` — invalid task ID
- `500 Internal Server Error` — failed to delete task or something went wrong
- `404 Not Found` — task not found

### Error Response Format

All error responses use the following JSON structure:

```json
{
  "error": "description"
}
```

The exact error message depends on the error that occurred.

## Testing with curl (Windows / PowerShell)

PowerShell can interfere with inline JSON quoting when using `curl`, so requests with a body can be tested by writing the JSON to a file first.

### `POST /tasks`

Create a `body.json` file from PowerShell:

```powershell
[System.IO.File]::WriteAllText("body.json", '{"title":"update my gaming console"}')
```

Then send the request:

```powershell
curl.exe -X POST http://localhost:8080/tasks -H "Content-Type: application/json" -d "@body.json"
```

### `GET /tasks`

```powershell
curl.exe http://localhost:8080/tasks
```

### `PATCH /tasks/{id}`

```powershell
curl.exe -X PATCH http://localhost:8080/tasks/1
```

### `DELETE /tasks/{id}`

```powershell
curl.exe -X DELETE http://localhost:8080/tasks/1
```

## Storage

Task data is stored persistently in PostgreSQL.

Unlike the previous in-memory implementation, tasks are not lost when the server is restarted.

## Known Limitations

- No duplicate-title checking.
- No pagination on `GET /tasks`.