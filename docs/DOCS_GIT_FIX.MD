The error is clear: **GitHub rejects files >100MB**. Your `lib/linux_amd64/liblancedb_go.a` (346.51 MB) exceeds the limit. You need **Git LFS** (Large File Storage).

## Fix: Use Git LFS

### Step 1: Install & Initialize Git LFS
```bash
# Install if you don't have it
sudo apt-get install git-lfs  # Ubuntu/Debian
# Or: brew install git-lfs   # macOS

# Initialize in your repository
git lfs install
```

### Step 2: Track the Large File
```bash
# Track this specific file
git lfs track "lib/linux_amd64/liblancedb_go.a"

# Or track all .a files if you have more:
git lfs track "*.a"

# Add the .gitattributes file that tracks LFS patterns
git add .gitattributes
```

### Step 3: Rewrite History (Critical!)
The large file is already committed, so you must **remove it from Git history** and re-add it with LFS:

```bash
# Remove the file from ALL previous commits (keeps file locally)
git filter-repo --path lib/linux_amd64/liblancedb_go.a --invert-paths

# If filter-repo isn't installed, use this instead:
git filter-branch --force --index-filter \
  'git rm --cached --ignore-unmatch lib/linux_amd64/liblancedb_go.a' \
  --prune-empty --tag-name-filter cat -- --all
```

### Step 4: Re-add & Re-commit
```bash
# Re-add the file (now tracked by LFS)
git add lib/linux_amd64/liblancedb_go.a

# Commit the changes
git commit -m "Add large file via Git LFS"

# Push again
git push origin master
```

---

## Alternative: Start Fresh (Simpler)

If you're okay with losing commit history (since it's a new repo):

```bash
# Rename old branch
git branch -m master old-master

# Create new clean branch
git checkout --orphan master

# Track large files with LFS first
git lfs track "lib/linux_amd64/liblancedb_go.a"
git add .gitattributes

# Add all other files (LFS-tracked files will be handled correctly)
git add .
git commit -m "Initial commit with LFS"

# Force push (since history is rewritten)
git push --force origin master
```

---

## Verify LFS is Working
```bash
# Check LFS-tracked files
git lfs ls-files

# Check file is tracked correctly
git lfs track
```

**Key point**: You **cannot** just install LFS and push again. You **must** rewrite history to remove the large file from previous commits or start with a fresh branch.