#!/bin/bash
# Git Binary Cleanup Script
# Removes large binary files from Git history to reduce repository size
# WARNING: This rewrites Git history - coordinate with team before running

set -e

echo "🧹 Git Binary Cleanup Script"
echo "=============================="
echo ""

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Error: Not in a git repository"
    exit 1
fi

# Backup current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "📍 Current branch: $CURRENT_BRANCH"
echo ""

# Warn user
echo "⚠️  WARNING: This will rewrite Git history!"
echo "⚠️  This should only be run on branches that haven't been shared widely."
echo "⚠️  After running, you'll need to force push: git push --force-with-lease"
echo ""
read -p "Continue? (yes/no): " -r
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "❌ Aborted"
    exit 1
fi

echo ""
echo "🔍 Finding large files in Git history..."
echo ""

# List files larger than 50MB in Git history
git rev-list --objects --all |
  git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' |
  awk '$1 == "blob" && $3 > 52428800 {print $3/1048576 " MB", $4}' |
  sort -rn |
  tee /tmp/large-git-files.txt

LARGE_FILE_COUNT=$(wc -l < /tmp/large-git-files.txt | tr -d ' ')

if [ "$LARGE_FILE_COUNT" -eq 0 ]; then
    echo ""
    echo "✅ No large files found (>50MB)"
    exit 0
fi

echo ""
echo "📊 Found $LARGE_FILE_COUNT large file(s)"
echo ""

# Check if git-filter-repo is installed
if ! command -v git-filter-repo &> /dev/null; then
    echo "📦 git-filter-repo not found. Installing..."
    
    if command -v brew &> /dev/null; then
        brew install git-filter-repo
    elif command -v pip3 &> /dev/null; then
        pip3 install git-filter-repo
    else
        echo "❌ Error: Please install git-filter-repo manually:"
        echo "   brew install git-filter-repo"
        echo "   OR"
        echo "   pip3 install git-filter-repo"
        exit 1
    fi
fi

echo ""
echo "🔧 Creating list of binary patterns to remove..."

# Create patterns file for git-filter-repo
cat > /tmp/git-cleanup-patterns.txt <<'EOF'
# Binary executables
bin/mirador-core
bin/server
bin/mirador-test
cmd/server/server

# macOS binaries
*.dylib
*.so

# Large test fixtures (if any)
# Add specific patterns as needed
EOF

echo ""
echo "📝 Patterns to remove:"
cat /tmp/git-cleanup-patterns.txt
echo ""

read -p "Proceed with cleanup? (yes/no): " -r
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "❌ Aborted"
    exit 1
fi

echo ""
echo "🧹 Running git-filter-repo to remove binaries..."
echo ""

# Run git-filter-repo with path-based filtering
git filter-repo --invert-paths --paths-from-file /tmp/git-cleanup-patterns.txt --force

echo ""
echo "✅ Git history cleaned!"
echo ""
echo "📊 Repository size comparison:"
du -sh .git
echo ""

echo "📋 Next steps:"
echo ""
echo "1. Verify the cleanup worked:"
echo "   git log --all --oneline | head -20"
echo ""
echo "2. Force push to remote (⚠️  COORDINATE WITH TEAM FIRST):"
echo "   git push --force-with-lease origin $CURRENT_BRANCH"
echo ""
echo "3. Team members need to re-clone or reset:"
echo "   git fetch origin"
echo "   git reset --hard origin/$CURRENT_BRANCH"
echo ""
echo "4. Clean up reflog and garbage collect locally:"
echo "   git reflog expire --expire=now --all"
echo "   git gc --prune=now --aggressive"
echo ""

# Cleanup temp files
rm -f /tmp/large-git-files.txt /tmp/git-cleanup-patterns.txt

echo "✅ Cleanup script complete!"
