# Deploying NOFX on Railway

This guide covers deploying NOFX to [Railway](https://railway.app) and **persisting your data** (user accounts, strategies, traders, etc.) across deploys.

## Why does my data disappear on each deploy?

Railway runs your app in a **container**. Each **new deploy** creates a **new container** with a fresh filesystem. Anything written to the container’s local disk (e.g. `data/data.db`) is **lost** when that container is replaced.

To keep your data, you must store it in **persistent storage**. Railway offers two main options:

1. **Volumes** – persistent disk attached to your service (keeps SQLite `data.db` and logs).
2. **PostgreSQL** – managed database; set `DB_TYPE=postgres` and connection env vars.

---

## Option A: Railway Volume (recommended for SQLite)

A **Volume** is a persistent disk. When you mount it at `/app/data`, the database and logs written there **survive redeploys**.

### Steps

1. **Open your Railway project** → select your NOFX **service**.
2. **Add a Volume**
   - **Command Palette**: `⌘K` (Mac) or `Ctrl+K` (Windows) → “Add Volume”, or  
   - **Right‑click** on the project canvas → “Add Volume”.
3. **Attach the volume to your NOFX service** when prompted.
4. **Set the mount path**
   - In the volume or service settings, set **Mount Path** to:
     ```text
     /app/data
     ```
   - This must be exactly `/app/data` because:
     - `DB_PATH` is `/app/data/data.db` in the Railway image.
     - Logs go to `data/` (i.e. `/app/data/`) when the app runs with `WORKDIR /app`.

5. **Redeploy** (or let the next deploy run). From that point on, `data.db` and logs under `/app/data` will persist across deploys.

### Notes

- The **first** time you add the volume, the filesystem at `/app/data` will be empty. The app will create `data.db` on first run. **You will need to register again** (or restore a backup) if you had data in the previous, non‑persistent container.
- After the volume is attached, **new** registrations and data will persist across future deploys.
- Do **not** set `DB_PATH` to something outside `/app/data` if you want it on the volume. The default `DB_PATH=/app/data/data.db` is correct when the volume is at `/app/data`.

---

## Option B: PostgreSQL

If you prefer a managed database, use **PostgreSQL** and switch NOFX to it. Data is stored in Postgres instead of a local file, so it persists regardless of the container.

### Steps

1. **Add PostgreSQL** in Railway  
   - “New” → “Database” → “PostgreSQL”, or add the Postgres plugin to your project.

2. **Connect it to your NOFX service**  
   - In the Postgres service, use “Connect” / “Add to project” so your NOFX service can use it. Railway will set `DATABASE_URL` or `PGHOST`, `PGUSER`, etc.  
   - If you get `DATABASE_URL`, you can derive the individual vars from it, or set them explicitly.

3. **Set environment variables** on your **NOFX service**:

   | Variable       | Example / description                          |
   |----------------|-------------------------------------------------|
   | `DB_TYPE`      | `postgres`                                      |
   | `DB_HOST`      | from Railway (e.g. `containers-us-west-xxx.railway.app`) |
   | `DB_PORT`      | `5432` (or the port Railway shows)             |
   | `DB_USER`      | from Railway                                    |
   | `DB_PASSWORD`  | from Railway                                    |
   | `DB_NAME`      | `railway` or the DB name Railway creates       |
   | `DB_SSLMODE`   | `require` (Railway Postgres typically uses SSL) |

   Use the exact values from your Postgres service’s “Variables” or “Connect” tab.

4. **Remove or do not set** `DB_PATH` when using Postgres (it is only for SQLite).

5. **Deploy**. NOFX will create the tables in Postgres. Your users, strategies, and traders will persist across deploys.

---

## Summary

| Goal                         | Approach                                        |
|-----------------------------|--------------------------------------------------|
| Keep SQLite and logs        | Add a **Volume**, mount path **`/app/data`**     |
| Use a managed DB            | Add **PostgreSQL**, set **`DB_TYPE=postgres`** and DB_* vars |

After that, your registration and other data will **no longer be deleted** on each deploy.
