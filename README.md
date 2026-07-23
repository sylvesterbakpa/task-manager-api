# Task Manager API

A simple REST API for managing tasks, built in Go using the standard library's `net/http` package. It supports creating, listing, completing, and deleting tasks, with data stored in memory.

## Requirements 

- Requires Go 1.22 or later.

## Running the server 

`go run main.go`

Server starts on `http://localhost:8080`.

## Endpoints

**`GET /tasks`** — returns all tasks, sorted by ID.
→ `200 OK`, e.g. `[{"id":1,"title":"update my gaming console","done":false}]`

**`POST /tasks`** — creates a new task.
Request body: `{"title": "update my gaming console"}`
→ `201 Created`, returns the created task.
Errors: `400` malformed JSON, `400` missing/empty title.

**`PATCH /tasks/{id}`** — marks a task complete. No request body.
→ `200 OK`, returns the updated task.
Errors: `400` invalid ID format, `404` task not found.

**`DELETE /tasks/{id}`** — deletes a task. No request body.
→ `204 No Content`, empty body.
Errors: `400` invalid ID format, `404` task not found.

All errors return: `{"error": "description"}`

## Testing with curl (Windows/PowerShell)

PowerShell mangles inline JSON quoting when passed straight to curl, so requests with a body are tested by writing the JSON to a file first, either by manually creating a .json file, or creating one from the terminal: 

```powershell
[System.IO.File]::WriteAllText("body.json", '{"title":"update my gaming console"}')
curl.exe -X POST http://localhost:8080/tasks -H "Content-Type: application/json" -d "@body.json"
```

`GET`, `PATCH`, and `DELETE` don't need a body, so they can be run directly:

```powershell
curl.exe http://localhost:8080/tasks
curl.exe -X PATCH http://localhost:8080/tasks/1
curl.exe -X DELETE http://localhost:8080/tasks/1
```

## Known limitations

- In-memory only — all tasks are lost on server restart.
- No duplicate-title checking.
- No pagination on `GET /tasks`.