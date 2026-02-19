<#
.SYNOPSIS
    Validates LATEST.sql is in sync with migration files.
.DESCRIPTION
    PowerShell equivalent of validate-migrations.sh.
    Prevents deployment issues where new databases are missing tables/columns.
.EXAMPLE
    .\scripts\validate-migrations.ps1
#>

$ErrorActionPreference = "Stop"

$ScriptDir   = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
$LatestSql   = Join-Path $ProjectRoot "store\migration\sqlite\LATEST.sql"
$MigrationsDir = Join-Path $ProjectRoot "store\migration\sqlite\0.25"

Write-Host "Validating LATEST.sql against migrations..."
Write-Host ""

# Check files exist
if (-not (Test-Path $LatestSql)) {
    Write-Host "ERROR: LATEST.sql not found at $LatestSql" -ForegroundColor Red
    exit 1
}
if (-not (Test-Path $MigrationsDir)) {
    Write-Host "ERROR: Migrations directory not found at $MigrationsDir" -ForegroundColor Red
    exit 1
}

$MissingTables  = @()
$MissingColumns = @()

# Read LATEST.sql content (lowercase for case-insensitive matching)
$LatestContent = (Get-Content $LatestSql -Raw).ToLower()

# Temp-table patterns to ignore
$IgnorePatterns = @('_new$', '_old$', '_backup$', '_temp$', '_tmp$')

function Test-IgnoredTable($TableName) {
    foreach ($pat in $IgnorePatterns) {
        if ($TableName -match $pat) { return $true }
    }
    return $false
}

# Process each migration file
$migrations = Get-ChildItem -Path $MigrationsDir -Filter "*.sql" |
    Where-Object { $_.Name -match '^\d+__.+\.sql$' } |
    Sort-Object Name

foreach ($migration in $migrations) {
    $content = Get-Content $migration.FullName -Raw
    $lines   = $content -split "`n"

    foreach ($line in $lines) {
        # Check CREATE TABLE
        if ($line -match 'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z_]\w*)\s*\(') {
            $table = $Matches[1]
            $tableLower = $table.ToLower()

            if (Test-IgnoredTable $tableLower) { continue }

            # Skip if migration also drops this table (table recreation)
            if ($content -match "(?i)DROP TABLE.*$table") { continue }

            # Skip if migration renames to this table (temporary)
            if ($content -match "(?i)RENAME TO.*$table") { continue }

            # Check if table exists in LATEST.sql
            if ($LatestContent -notmatch "create table.*$tableLower") {
                $MissingTables += "$table (from $($migration.Name))"
            }
        }

        # Check ALTER TABLE ADD COLUMN
        if ($line -match 'ALTER\s+TABLE\s+([a-zA-Z_]\w*)\s+ADD\s+COLUMN\s+([a-zA-Z_]\w*)') {
            $table  = $Matches[1]
            $column = $Matches[2]
            $columnLower = $column.ToLower()

            if (Test-IgnoredTable $table.ToLower()) { continue }

            if ($LatestContent -notmatch $columnLower) {
                $MissingColumns += "$table.$column (from $($migration.Name))"
            }
        }
    }
}

# Report results
$Errors = 0

if ($MissingTables.Count -gt 0) {
    Write-Host "Missing tables in LATEST.sql:" -ForegroundColor Red
    foreach ($item in $MissingTables) { Write-Host "  - $item" }
    Write-Host ""
    $Errors = 1
}

if ($MissingColumns.Count -gt 0) {
    Write-Host "Missing columns in LATEST.sql:" -ForegroundColor Red
    foreach ($item in $MissingColumns) { Write-Host "  - $item" }
    Write-Host ""
    $Errors = 1
}

if ($Errors -eq 1) {
    Write-Host "Please update store\migration\sqlite\LATEST.sql to include the missing items." -ForegroundColor Yellow
    Write-Host "See docs\DOCS_DATABASE_MIGRATION.MD for details." -ForegroundColor Yellow
    exit 1
}

Write-Host "LATEST.sql is in sync with all migrations" -ForegroundColor Green
exit 0
