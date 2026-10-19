# Task Manager API

A simple REST API for managing tasks, built in Go using the standard library's `net/http` package and PostgreSQL for persistent data storage.

The API supports user creation and login as well as creating, listing, completing, and deleting tasks per user. 

## Requirements

- Go 1.26.3 or later
- PostgreSQL
- PostgreSQL driver: `github.com/jackc/pgx/v5`
- bcrypt: `golang.org/x/crypto`
- JWT library: `github.com/golang-jwt/jwt/v5`

## Database Setup

Create a PostgreSQL database for the application.

Then load the schema file into your database:

```powershell
psql -U <user> -h localhost -d <database> -f schema.sql
```

## Database Configuration

The application reads its PostgreSQL connection details from environment variables instead of storing them directly in the source code.

The following environment variables are required:

| Variable | Description |
|---|---|
| `SECRET_KEY` | The secret that signs login tokens. The server won't start without it; should be a long value |
| `DB_USER` | PostgreSQL username |
| `DB_PASSWORD` | PostgreSQL password |
| `DB_HOST` | PostgreSQL host |
| `DB_PORT` | PostgreSQL port |
| `DB_NAME` | PostgreSQL database name |

### Windows / PowerShell

Set the variables before starting the server:

```powershell
$env:SECRET_KEY="your_key"
$env:DB_USER="postgres"
$env:DB_PASSWORD="your_password"
$env:DB_HOST="localhost"
$env:DB_PORT="5432"
$env:DB_NAME="your_database_name"
```

Replace the example values with your own values.

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

The flow of interacting with the API is: register, log in, get a token, then send `Authorization: Bearer <token>` on every /tasks request. Tokens expire after 30 minutes.

### Authentication errors

Every `/tasks` endpoint requires the header `Authorization: Bearer <token>`. If the token is not accepted, the server responds with `401 Unauthorized`:

- **Missing or invalid token** — message: `Invalid or missing token`. This covers a missing header, a malformed or tampered token, and a token signed with the wrong key or algorithm.
- **Expired token** — message: `Token expired, please log in again`. Log in again to get a new token.
- **Malformed token payload** — message: `Invalid token claims` or `Invalid user ID claim`. The token verified, but its contents were not what the server expects.

### `POST /user`

Returns user ID and email.

**Request body:**

```json
{
  "email": "examplemail1@gmail.com", "password":"wwxxxyyyy"
}
```

**Response:** `201 Created`

**Example response:**

```json
{
  "id": 1, "email": "examplemail1@gmail.com"
}
```

**Errors:**

- `400 Bad Request` — invalid JSON, empty email, an email that isn't a plain address, an empty password, or a password outside 8-72 bytes
- `409 Conflict` — email already registered
- `500 Internal Server Error` — server errors


### `POST /login`

**Request body:**

```json
{
  "email": "examplemail1@gmail.com", "password":"wwxxxyyyy"
}
```

**Response:** `200 OK`

**Example response:**

```json
{
  "id": 1,
  "email": "examplemail1@gmail.com",
  "token": "some_token_string"
}
```

**Errors:**

- `400 Bad Request` — invalid JSON or empty fields
- `401 Unauthorized` — wrong password or unknown email, with the same message either way. This was done by design as it is important that no one, especially an unauthorized user, should be able to tell the difference for whether an email exists already or not.
- `500 Internal Server Error` — server errors

### `GET /tasks`

Returns all tasks belonging to an authorized user, sorted by ID. It returns [] if there are no tasks for the logged-in user. A user cannot see another user's task(s). It requires the header to have `Authorization: Bearer <token>`

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

**Errors:**

- `401 Unauthorized` — see [Authentication errors](#authentication-errors)
- `500 Internal Server Error` — failed to retrieve tasks

### `POST /tasks`

Creates a new task for an authorized user. A user cannot create a task for another user. It requires the header to have `Authorization: Bearer <token>`

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

- `400 Bad Request` — invalid JSON or empty request
- `401 Unauthorized` — see [Authentication errors](#authentication-errors)
- `500 Internal Server Error` — failed to add task

The title is normalized so that the first character is uppercase and the remaining characters are lowercase.

### `PATCH /tasks/{id}`

Marks a task as complete for an authorized user. A user cannot modify a task that belongs to another user. It requires the header to have `Authorization: Bearer <token>`

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
- `401 Unauthorized` — see [Authentication errors](#authentication-errors)
- `404 Not Found` — task not found for both cases of where a task does not exist or belongs to another user. This is a deliberate decision in order to make it impossible for any user to tell that a task already exists for another user. So, they all get the generic "task not found"
- `500 Internal Server Error` — failed to update task

### `DELETE /tasks/{id}`

Deletes a task for an authorized user. A user cannot delete a task that belongs to another user.  It requires the header to have `Authorization: Bearer <token>`

No request body is required.

**Response:** `204 No Content`

**Errors:**

- `400 Bad Request` — invalid task ID
- `401 Unauthorized` — see [Authentication errors](#authentication-errors)
- `404 Not Found` — task not found for both cases of where a task does not exist or belongs to another user. This is a deliberate decision in order to make it impossible for any user to tell that a task already exists for another user. So, they all get the generic "task not found"
- `500 Internal Server Error` — failed to delete task or something went wrong

### Error Response Format

All error responses use the following JSON structure:

```json
{
  "error": "description"
}
```

The exact error message depends on the error that occurred.

## Testing with curl (Windows / PowerShell)

PowerShell can interfere with inline JSON quoting when using `curl`, so requests with a body can be tested by writing the JSON to a file first. Below is the flow for testing:

### `POST /user`

Create a `body.json` file from PowerShell:

```powershell
[System.IO.File]::WriteAllText("body.json", '{"email":"examplemail1@gmail.com", "password":"wwxxyyyy"}')
```

Then send the request:

```powershell
curl.exe -X POST http://localhost:8080/user -H "Content-Type: application/json" -d "@body.json"
```

### `POST /login`

Create a `body.json` file from PowerShell:

```powershell
[System.IO.File]::WriteAllText("body.json", '{"email":"examplemail1@gmail.com", "password":"wwxxyyyy"}')
```

Then send the request:

```powershell
curl.exe -X POST http://localhost:8080/login -H "Content-Type: application/json" -d "@body.json"
```

> The login response contains a `token`. In every `/tasks` request below, replace the whole placeholder `<the_token_you_got_when_you_logged_in>`, angle brackets included, with that token. Tokens expire after 30 minutes, so log in again if a request returns `401` with an expired-token message.

### `POST /tasks`

Create a `body.json` file from PowerShell:

```powershell
[System.IO.File]::WriteAllText("body.json", '{"title":"update my gaming console"}')
```

Then send the request:

```powershell
curl.exe -X POST http://localhost:8080/tasks -H "Authorization: Bearer <the_token_you_got_when_you_logged_in>" -H "Content-Type: application/json" -d "@body.json"
```

### `GET /tasks`

```powershell
curl.exe http://localhost:8080/tasks -H "Authorization: Bearer <the_token_you_got_when_you_logged_in>"
```

### `PATCH /tasks/{id}`

```powershell
curl.exe -X PATCH http://localhost:8080/tasks/1 -H "Authorization: Bearer <the_token_you_got_when_you_logged_in>" 
```

### `DELETE /tasks/{id}`

```powershell
curl.exe -X DELETE http://localhost:8080/tasks/1 -H "Authorization: Bearer <the_token_you_got_when_you_logged_in>" 
```

## Storage

Task data is stored persistently in PostgreSQL.

## Known Limitations

- No duplicate-title checking.
- No pagination on `GET /tasks`.
- There is no rate limiting on login.
- Tokens can’t be revoked before they expire.
- Password strength is checked only by length.
- There are no refresh tokens, so users log in again after 30 minutes.