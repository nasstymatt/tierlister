# tierlister

A web app for creating and managing tier lists, built with Go and SQLite.

## Build & Run

```bash
sqlc generate
go run .
```

Or build a binary:

```bash
go build -o tierlister .
./tierlister
```

Then open [http://localhost:8080](http://localhost:8080).
