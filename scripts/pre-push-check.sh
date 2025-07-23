#!/bin/bash

# Pre-push validation script for sirseer-relay
# Run this before pushing to ensure code quality and prevent leaks

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Running pre-push validation checks...${NC}"
echo -e "${BLUE}========================================${NC}"

# Track if any checks fail
CHECKS_FAILED=0

# Function to run a check
run_check() {
    local name="$1"
    local command="$2"
    
    echo -e "\n${YELLOW}Running: ${name}${NC}"
    if eval "$command"; then
        echo -e "${GREEN}✓ ${name} passed${NC}"
    else
        echo -e "${RED}✗ ${name} failed${NC}"
        CHECKS_FAILED=1
    fi
}

# 1. Check for forbidden terms in all files
check_forbidden_terms() {
    local forbidden_terms=(
        "[Pp]hase [0-9]"
        "[Pp]hases"
        "PIVOT_PLAN"
        "CLAUDE\.md"
        "roadmap"
        "subtask"
        "task list"
        "internal planning"
        "development dialogue"
    )
    
    local found=0
    for term in "${forbidden_terms[@]}"; do
        # Exclude .git directory, binary files, and this script itself
        if git grep -l -E "$term" -- ':!.git' ':!*.png' ':!*.jpg' ':!*.pdf' ':!scripts/pre-push-check.sh' ':!.githooks/pre-commit' ':!.gitignore' > /dev/null 2>&1; then
            echo -e "${RED}Found forbidden term: $term${NC}"
            git grep -n -E "$term" -- ':!.git' ':!*.png' ':!*.jpg' ':!*.pdf' ':!scripts/pre-push-check.sh' ':!.githooks/pre-commit' ':!.gitignore' | head -5
            found=1
        fi
    done
    
    return $found
}

# 2. Check for debug statements
check_debug_statements() {
    local debug_patterns=(
        "console\.log"
        "fmt\.Printf"
        "println!"
        "debugger"
        "// DEBUG"
        "// TODO: remove"
    )
    
    local found=0
    for pattern in "${debug_patterns[@]}"; do
        if git grep -l -E "$pattern" -- '*.go' '*.js' '*.ts' '*.jsx' '*.tsx' ':!internal/*/doc.go' > /dev/null 2>&1; then
            echo -e "${RED}Found debug statement: $pattern${NC}"
            git grep -n -E "$pattern" -- '*.go' '*.js' '*.ts' '*.jsx' '*.tsx' ':!internal/*/doc.go' | head -5
            found=1
        fi
    done
    
    return $found
}

# 3. Run tests
run_tests() {
    if [ -f "Makefile" ] && grep -q "^test:" Makefile; then
        make test
    else
        echo -e "${YELLOW}No Makefile with test target found, skipping tests${NC}"
        return 0
    fi
}

# 4. Check commit messages
check_commit_messages() {
    # Get commits that will be pushed
    local remote_branch=$(git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || echo "origin/main")
    local commits=$(git log --oneline "$remote_branch"..HEAD)
    
    if [ -z "$commits" ]; then
        echo "No new commits to check"
        return 0
    fi
    
    local found=0
    while IFS= read -r commit; do
        if echo "$commit" | grep -E -i "(phase [0-9]|roadmap|CLAUDE|PIVOT|subtask|task list)" > /dev/null 2>&1; then
            echo -e "${RED}Problematic commit message: $commit${NC}"
            found=1
        fi
    done <<< "$commits"
    
    return $found
}

# 5. Check for uncommitted changes
check_uncommitted_changes() {
    if ! git diff --quiet || ! git diff --cached --quiet; then
        echo -e "${RED}You have uncommitted changes${NC}"
        git status --short
        return 1
    fi
    return 0
}

# 6. Verify .gitignore entries
check_gitignore() {
    local required_entries=(
        "CLAUDE.md"
        "PIVOT_PLAN.md"
        ".claude/"
    )
    
    local missing=0
    for entry in "${required_entries[@]}"; do
        if ! grep -q "^$entry" .gitignore 2>/dev/null; then
            echo -e "${RED}Missing .gitignore entry: $entry${NC}"
            missing=1
        fi
    done
    
    return $missing
}

# Run all checks
run_check "Forbidden terms check" "check_forbidden_terms"
run_check "Debug statements check" "check_debug_statements"
run_check "Uncommitted changes check" "check_uncommitted_changes"
run_check ".gitignore validation" "check_gitignore"
run_check "Commit message check" "check_commit_messages"
run_check "Test suite" "run_tests"

# Summary
echo -e "\n${BLUE}========================================${NC}"
if [ $CHECKS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All pre-push checks passed!${NC}"
    echo -e "${GREEN}Safe to push to remote repository.${NC}"
else
    echo -e "${RED}Pre-push validation failed!${NC}"
    echo -e "${RED}Please fix the issues above before pushing.${NC}"
    echo -e "\n${YELLOW}Remember: This is a PUBLIC repository visible to:${NC}"
    echo -e "${YELLOW}- Fortune 500 security teams${NC}"
    echo -e "${YELLOW}- Potential clients${NC}"
    echo -e "${YELLOW}- Competitors${NC}"
    echo -e "${YELLOW}- Open source community${NC}"
fi
echo -e "${BLUE}========================================${NC}"

exit $CHECKS_FAILED