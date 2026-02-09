# Deploying NOFX on Railway

This guide covers deploying NOFX to [Railway](https://railway.app) and **persisting your data** (user accounts, strategies, traders, etc.) across deploys.

---

## Local vs Railway: How They Differ

| Aspect | **Local** | **Railway** |
|--------|-----------|-------------|
| **Docker** | Optional. You can: (1) run `./nofx` directly (no Docker), or (2) use `./start.sh` → `docker compose` (Docker). | **Always Docker.** Railway builds and runs the image from `Dockerfile.railway`. |
| **Layout** | With `./start.sh`: **2 containers** (backend `nofx` + frontend `nofx-frontend`). With `./nofx`: **1 process**, no frontend container (you serve the built `web/` yourself or use a separate server). | **1 container (all‑in‑one).** `Dockerfile.railway` puts the Go binary + nginx + built frontend in one image. `railway/start.sh` runs nofx on 8081 and nginx on `PORT`. |
| **Port** | Backend: `API_SERVER_PORT` (default 8080) or `NOFX_BACKEND_PORT` in compose. Frontend: `NOFX_FRONTEND_PORT` (default 3000). | Railway sets **`PORT`** (e.g. 8080). Nginx listens on `PORT`; the Go app runs on **8081** via `API_SERVER_PORT=8081` in `railway/start.sh`. |
| **Database** | SQLite: `DB_PATH` (default `data/data.db`). With Docker Compose, `./data` is mounted at `/app/data` so `data/data.db` or `/app/data/data.db` persists on the host. | SQLite: `DB_PATH=/app/data/data.db` in the image. **Without a Volume, the filesystem is ephemeral** — data is lost on redeploy. With a Volume at `/app/data`, it persists. |
| **Env file** | `.env` from the project root (and `env_file` in docker-compose). `godotenv.Load()` in `main.go` also loads `.env` when running `./nofx`. | No `.env` in the image. Set variables in the **Railway service → Variables**. Railway injects them into the container at runtime. |
| **Encryption keys** | `.env`: `JWT_SECRET`, `DATA_ENCRYPTION_KEY`, `RSA_PRIVATE_KEY`. `./start.sh` can generate them if missing. | Same variable names. If **not set** in Railway, `railway/start.sh` **auto‑generates** them at container start. That means keys change on each new container unless you set them explicitly in Railway Variables. |
| **Persistence** | With Docker: `./data` → `/app/data` persists on the host. With `./nofx`: `data/data.db` in the project directory persists. | **By default: none.** Add a **Volume** at `/app/data` or use **PostgreSQL** so data survives redeploys. |

### Variables That Often Differ

| Variable | Local (typical) | Railway (typical) |
|----------|----------------|-------------------|
| `PORT` | Not used by the Go app. | **Set by Railway.** Used by nginx in `railway/start.sh` to listen. |
| `API_SERVER_PORT` | 8080 (or from `.env`). | **Set to 8081** in `railway/start.sh` (nginx proxies `/api/` to 8081). |
| `DB_PATH` | `data/data.db` (relative to workdir) or `/app/data/data.db` in Docker. | **`/app/data/data.db`** (set in `Dockerfile.railway`). Must be under `/app/data` if you use a Volume. |
| `DB_TYPE` | `sqlite` (default). | `sqlite` with a Volume, or `postgres` if you use Railway Postgres. |
| `JWT_SECRET`, `DATA_ENCRYPTION_KEY`, `RSA_PRIVATE_KEY` | In `.env`; often generated once by `./start.sh`. | **Set in Railway Variables** so they stay fixed across deploys. If unset, `railway/start.sh` generates new ones each start → sessions/decryption can break. |
| `TRANSPORT_ENCRYPTION` | Often `false` for local HTTP. | `false` or `true`; does **not** affect DB or “Candidate Coins (0)” (see main README). |
| `NOFX_BACKEND_PORT`, `NOFX_FRONTEND_PORT` | Used by **docker-compose** and `./start.sh` for host port mapping. | **Not used** on Railway; `PORT` is the single public port. |

### Is Railway Using Docker?

Yes. `railway.toml` points to `Dockerfile.railway`. Railway builds that image and runs it in a container. The main difference from “local Docker” is:

- **Local `docker-compose`**: two services (backend + frontend), `.env` and host volume `./data` for persistence.
- **Railway**: one image with backend + nginx + frontend, no `.env` (use Railway Variables), and **you must add a Volume or Postgres** for persistence.

---

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

## Troubleshooting: “Candidate Coins (0)” even with /app/data

A Volume at `/app/data` only **persists** the database. If the **strategy** stored in the DB has **no static coins** (or uses **ai500/oi_top**, which can fail on Railway), you will still see **0 coins** in the user prompt.

### 1. Set Coin Source to **Static** and add coins

1. Open **Strategy Studio**.
2. Select the **strategy** your trader uses.
3. Open the **Coin Source** (币种来源) section.
4. Set **Source Type** to **Static List**.
5. In **Custom Coins**, add at least **BTC** (e.g. type `BTC` or `BTCUSDT` and add). Optionally add ETH, SOL, DOGE.
6. Click **Save** (保存).

### 2. Make sure the trader uses that strategy

In **Config → Traders**, edit your trader and set **Strategy** to the same strategy you just saved. Save the trader.

### 3. If the strategy is **ai500** or **oi_top**

The **default** strategy uses **ai500**. That calls the NofxOS API; on Railway it can fail (no/ invalid NofxOS API key, or network). When it fails, candidate coins can be 0.

- **Preferred:** Switch the strategy to **Static** and add BTC (and others) as above; then Save.
- **Alternatively:** In the strategy’s **Indicators** section, set a valid **NofxOS API Key** and ensure Railway can reach the NofxOS API.

### 4. (Optional) Deploy a build that includes fallbacks

The image built from `Dockerfile.railway` uses the pre-built `ghcr.io/nofxaios/nofx/nofx-backend` binary, which may **not** include the “[BTC] when 0” fallbacks. To include them, build the backend from source:

1. In `railway.toml`, set:
   ```toml
   [build]
   dockerfilePath = "Dockerfile.railway.fromsource"
   ```
2. Redeploy. The build will be slower, but the running app will use the fallbacks (e.g. [BTC] when the strategy returns 0).

---

## Summary

| Goal                         | Approach                                        |
|-----------------------------|--------------------------------------------------|
| Keep SQLite and logs        | Add a **Volume**, mount path **`/app/data`**     |
| Use a managed DB            | Add **PostgreSQL**, set **`DB_TYPE=postgres`** and DB_* vars |
| Fix “0 coins” in prompt     | Strategy: **Static** + add **BTC** (and others) → **Save**; ensure the **trader** uses that strategy |

After that, your registration and other data will **no longer be deleted** on each deploy.
