# Neon Deployment Instructions

Step-by-step guide to deploy bchat with Neon Postgres on Fly.io.

---

## Prerequisites

- Neon account (https://console.neon.tech)
- Fly.io account and CLI (`fly auth login`)
- bchat repo cloned locally

---

## Step 1: Create Neon Project

1. Go to https://console.neon.tech/app/projects
2. Click **New Project**
3. Settings:
   - Project name: `bchat-prod`
   - Region: `us-east-2` (closest to your Fly.io region)
   - Postgres version: `17` (latest)
4. Click **Create Project**
5. Copy the connection string from **Dashboard → Connect**

The connection string looks like:
```
postgresql://neondb_owner:xxxx@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require
```

---

## Step 2: Set Fly.io Secrets

```bash
cd /home/chaschel/Documents/go/bchat

# Database config
fly secrets set DB_DRIVER=postgres
fly secrets set DATABASE_URL="postgresql://neondb_owner:xxxx@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require"

# Verify secrets
fly secrets list
```

---

## Step 3: Update `.env` (Local Testing)

```bash
# Add to .env
DB_DRIVER=postgres
DATABASE_URL="postgresql://neondb_owner:xxxx@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require"
```

Test locally:
```bash
go run ./bin/memos --mode dev
```

---

## Step 4: Deploy

```bash
fly deploy
```

---

## Step 5: Verify

```bash
# Check logs
fly logs

# SSH into the machine
fly ssh console

# Check database connection
curl http://localhost:5230/api/v1/status
```

---

## Neon Console Monitoring

- **Dashboard**: https://console.neon.tech — monitor connections, compute time, storage
- **SQL Editor**: Run queries directly in the Neon console
- **Branching**: Create test branches for schema changes (optional)

---

## Troubleshooting

### Connection refused
- Check `DATABASE_URL` is set correctly in Fly secrets
- Verify Neon project is not suspended (free tier auto-suspends after inactivity)
- Check SSL: connection string must include `sslmode=require`

### Too many connections
- Neon free tier limit: ~20 concurrent connections
- Set `MaxOpenConns=5` in the Go driver (already in plan)
- Check for connection leaks (unclosed `*sql.DB`)

### Timeout on first request
- Neon free tier cold starts take ~2-5 seconds
- First request after inactivity will be slow
- Consider Neon Pro for always-on compute

### Migration errors
- Ensure baseline migration ran successfully
- Check `migration_history` table exists
- Verify all tables created: `psql $DATABASE_URL -c "\dt"`
