# Database migrations

This project uses a **single squashed initial migration** (`00001_initial_schema.sql`).

## Fresh database

Migrations run automatically on API startup (`database.Migrate`) and in CI via `go run ./cmd/migrate`.

## Reset after a migration squash

If `goose_db_version` references old versions (2, 3, …), reset the database:

```bash
# Docker Compose (example   adjust service/volume names to your setup)
docker compose down -v
docker compose up -d postgres
# then restart admin-backend so Goose applies 00001
```

Or on an existing Postgres instance:

```sql
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO public;
```

Then restart the admin API so `00001_initial_schema.sql` runs from scratch.
